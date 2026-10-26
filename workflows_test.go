package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The CI and release workflows cannot be run without a repository, so these
// tests check what can be checked from here: that they are valid, that the
// supply chain is pinned, that permissions are minimal, and that the versions
// they name agree with the rest of the project.

type workflowFile struct {
	Name        string            `yaml:"name"`
	On          map[string]any    `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Env         map[string]string `yaml:"env"`
	Jobs        map[string]job    `yaml:"jobs"`
}

type job struct {
	RunsOn      any               `yaml:"runs-on"`
	Uses        string            `yaml:"uses"`
	Needs       any               `yaml:"needs"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []step            `yaml:"steps"`
	Strategy    struct {
		FailFast *bool `yaml:"fail-fast"`
		Matrix   struct {
			Include []map[string]string `yaml:"include"`
		} `yaml:"matrix"`
	} `yaml:"strategy"`
}

type step struct {
	ID   string            `yaml:"id"`
	If   string            `yaml:"if"`
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	Env  map[string]string `yaml:"env"`
	With map[string]any    `yaml:"with"`
}

func loadWorkflows(t *testing.T) map[string]workflowFile {
	t.Helper()
	paths, err := filepath.Glob(".github/workflows/*.yml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no workflows found: %v", err)
	}
	out := map[string]workflowFile{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var w workflowFile
		if err := yaml.Unmarshal(data, &w); err != nil {
			t.Fatalf("%s is not valid YAML: %v", p, err)
		}
		out[filepath.Base(p)] = w
	}
	return out
}

func TestWorkflowsAreValidAndHaveJobs(t *testing.T) {
	for name, w := range loadWorkflows(t) {
		if w.Name == "" || len(w.Jobs) == 0 || len(w.On) == 0 {
			t.Errorf("%s: needs a name, triggers and jobs (got %q, %d triggers, %d jobs)", name, w.Name, len(w.On), len(w.Jobs))
		}
		for id, j := range w.Jobs {
			if j.Uses == "" && (len(j.Steps) == 0 || j.RunsOn == nil) {
				t.Errorf("%s job %s: needs runs-on and steps, or uses", name, id)
			}
		}
	}
}

// A version tag can be moved to different code after the fact; a commit SHA
// cannot. The trailing comment says which release the SHA is, for humans.
func TestEveryActionIsPinnedToACommit(t *testing.T) {
	// owner/repo, optionally followed by a subpath (actions/cache/restore).
	sha := regexp.MustCompile(`^[\w.-]+/[\w.-]+(/[\w./-]+)?@[0-9a-f]{40}$`)
	for name := range loadWorkflows(t) {
		data, _ := os.ReadFile(filepath.Join(".github/workflows", name))
		for i, line := range strings.Split(string(data), "\n") {
			trim := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
			if !strings.HasPrefix(trim, "uses:") {
				continue
			}
			ref := strings.TrimSpace(strings.TrimPrefix(trim, "uses:"))
			if strings.HasPrefix(ref, "./") {
				continue // a workflow in this repository
			}
			action, comment, _ := strings.Cut(ref, "#")
			if !sha.MatchString(strings.TrimSpace(action)) {
				t.Errorf("%s:%d pins %q to something other than a full commit SHA", name, i+1, strings.TrimSpace(action))
			}
			if !regexp.MustCompile(`^\s*v\d`).MatchString(comment) {
				t.Errorf("%s:%d should say which release the SHA is (a '# vX.Y.Z' comment)", name, i+1)
			}
		}
	}
}

func TestPermissionsAreMinimal(t *testing.T) {
	for name, w := range loadWorkflows(t) {
		if len(w.Permissions) == 0 {
			t.Errorf("%s: set top-level permissions explicitly instead of inheriting the default", name)
		}
		for scope, level := range w.Permissions {
			if level != "read" {
				t.Errorf("%s: top-level permission %s: %s should be read-only", name, scope, level)
			}
		}
		for id, j := range w.Jobs {
			for scope, level := range j.Permissions {
				if level == "write" && !(name == "release.yml" && id == "release" && scope == "contents") {
					t.Errorf("%s job %s grants %s: write; only the release job may write contents", name, id, scope)
				}
			}
		}
	}
}

func TestNoRiskyPatterns(t *testing.T) {
	pipeToShell := regexp.MustCompile(`(curl|wget)[^\n|]*\|\s*(sudo\s+)?(ba|z)?sh\b`)
	for name := range loadWorkflows(t) {
		data, _ := os.ReadFile(filepath.Join(".github/workflows", name))
		text := string(data)
		for what, bad := range map[string]bool{
			"pull_request_target (runs untrusted code with secrets)": strings.Contains(text, "pull_request_target"),
			"an unpinned @latest":                           regexp.MustCompile(`@(latest|main|master)\b`).MatchString(text),
			"a download piped into a shell":                 pipeToShell.MatchString(text),
			"ubuntu-latest (the GTK 4 packages need 24.04)": strings.Contains(text, "ubuntu-latest"),
		} {
			if bad {
				t.Errorf("%s contains %s", name, what)
			}
		}
	}
}

func goModField(t *testing.T, pattern string) string {
	t.Helper()
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(pattern).FindSubmatch(data)
	if m == nil {
		t.Fatalf("go.mod has no match for %s", pattern)
	}
	return string(m[1])
}

// The CLI that generates bindings and packages must be the library version
// the app is built with, or the generated code and the runtime disagree.
func TestCIUsesTheSameWailsVersionAsGoMod(t *testing.T) {
	want := goModField(t, `github\.com/wailsapp/wails/v3 (v[^\s]+)`)
	got := loadWorkflows(t)["ci.yml"].Env["WAILS_VERSION"]
	if got != want {
		t.Fatalf("ci.yml installs Wails %q but go.mod uses %q; update both together", got, want)
	}
}

func TestCIUsesAGoVersionNewEnoughForTheModule(t *testing.T) {
	required := goModField(t, `(?m)^go (\d+\.\d+)`)
	got := loadWorkflows(t)["ci.yml"].Env["GO_VERSION"]
	if !strings.HasPrefix(got, required) {
		t.Fatalf("ci.yml uses Go %q but go.mod needs %s (D2 v0.9.0 requires Go 1.27)", got, required)
	}
}

func TestBuildCoversEveryPlatformAndTheReleaseNeedsThemAll(t *testing.T) {
	ws := loadWorkflows(t)
	var platforms []string
	for _, inc := range ws["ci.yml"].Jobs["build"].Strategy.Matrix.Include {
		platforms = append(platforms, inc["platform"])
	}
	if strings.Join(platforms, ",") != "linux,windows,macos" {
		t.Fatalf("build matrix = %v", platforms)
	}
	release, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"macos", "windows", "linux"} {
		if !strings.Contains(string(release), "SHA256SUMS-${platform}.txt") || !strings.Contains(string(release), p) {
			t.Errorf("release.yml does not check that the %s build exists", p)
		}
	}
}

func TestReleasesAreDraftsTriggeredByTagsAndCheckTheVersion(t *testing.T) {
	w := loadWorkflows(t)["release.yml"]
	if _, ok := w.On["push"]; !ok || len(w.On) != 1 {
		t.Fatalf("release.yml should trigger only on tag pushes: %v", w.On)
	}
	if w.Jobs["build"].Uses != "./.github/workflows/ci.yml" {
		t.Error("the release should reuse CI's build, so what is released is what was tested")
	}
	data, _ := os.ReadFile(".github/workflows/release.yml")
	text := string(data)
	for what, ok := range map[string]bool{
		"creates a draft (a person publishes it)": strings.Contains(text, "--draft"),
		"verifies the tag":                        strings.Contains(text, "--verify-tag"),
		"checks the tag against build/config.yml": strings.Contains(text, "build/config.yml"),
		"marks suffixed tags as pre-releases":     strings.Contains(text, "--prerelease"),
	} {
		if !ok {
			t.Errorf("release.yml does not do this: %s", what)
		}
	}
	// Leaving draft is allowed only in the gated publish step, which
	// TestReleaseWorkflowPublishesOnlyThroughTheGate checks.
	if regexp.MustCompile(`--latest(=true|\s|\)|")`).MatchString(text) {
		t.Error("a release must never be marked as the latest release")
	}
	if _, err := os.Stat(".github/release-notes.md"); err != nil {
		t.Errorf("release.yml uses .github/release-notes.md: %v", err)
	}
}

func TestScriptsAreExecutableAndSyntacticallyValid(t *testing.T) {
	scripts, _ := filepath.Glob("scripts/*.sh")
	if len(scripts) < 2 {
		t.Fatalf("expected the CI scripts, found %v", scripts)
	}
	for _, s := range scripts {
		info, err := os.Stat(s)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&0o111 == 0 {
			t.Errorf("%s is not executable", s)
		}
		if out, err := exec.Command("bash", "-n", s).CombinedOutput(); err != nil {
			t.Errorf("%s has a syntax error: %v\n%s", s, err, out)
		}
	}
}

// A failed platform makes the release impossible, so the others should stop
// at once instead of running for many more minutes.
func TestBuildFailsFast(t *testing.T) {
	ff := loadWorkflows(t)["ci.yml"].Jobs["build"].Strategy.FailFast
	if ff == nil || !*ff {
		t.Error("the build matrix must set fail-fast: true so one failed platform stops the rest")
	}
}

// GitHub's Ubuntu mirror can stall for minutes on one file, which once ran a
// job for 20 minutes inside a bare `apt-get install`. Every Linux package
// install must go through the script that sets per-download timeouts and
// retries, and keeps the downloaded packages in a cache.
func TestLinuxPackagesInstallThroughTheRetryingScript(t *testing.T) {
	for file, w := range loadWorkflows(t) {
		for name, j := range w.Jobs {
			for _, st := range j.Steps {
				if strings.Contains(st.Run, "apt-get") {
					t.Errorf("%s job %q step %q calls apt-get directly; use scripts/install-linux-deps.sh", file, name, st.Name)
				}
			}
		}
	}
	data, err := os.ReadFile("scripts/install-linux-deps.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Acquire::Retries=", "Acquire::http::Timeout=", "Dir::Cache::archives="} {
		if !strings.Contains(string(data), want) {
			t.Errorf("scripts/install-linux-deps.sh must set %s", want)
		}
	}
	ci, _ := os.ReadFile(".github/workflows/ci.yml")
	if n := strings.Count(string(ci), "scripts/install-linux-deps.sh"); n < 4 { // 2 installs, 2 cache keys each save+restore
		t.Errorf("both jobs should install through the script and cache on it; found %d references", n)
	}
}
