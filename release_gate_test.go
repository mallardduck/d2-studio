package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// scripts/release-gate.sh decides whether a release may publish itself or must
// stay a draft. Publishing is hard to undo, so every way of saying "no" is
// tested, and there is exactly one way of saying "yes".

// release builds a dist/ and gates/ pair that passes every rule, for a test to
// then break one way.
type release struct {
	t           *testing.T
	dist, gates string
}

var allFiles = []string{
	"d2-studio-v0.1.0-rc.1-macos-universal.zip",
	"d2-studio-v0.1.0-rc.1-windows-amd64-setup.exe",
	"d2-studio-v0.1.0-rc.1-windows-amd64-portable.zip",
	"d2-studio-v0.1.0-rc.1-linux-amd64.tar.gz",
	"d2-studio-v0.1.0-rc.1-linux-amd64.deb",
	"SHA256SUMS.txt",
}

func newRelease(t *testing.T) *release {
	t.Helper()
	r := &release{t: t, dist: t.TempDir(), gates: t.TempDir()}
	for _, f := range allFiles {
		r.write(r.dist, f, "x")
	}
	for _, p := range []string{"macos", "windows", "linux"} {
		r.write(r.gates, "SMOKE-"+p+".txt", "success\n")
	}
	return r
}

func (r *release) write(dir, name, content string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *release) remove(dir, name string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(dir, name)); err != nil {
		r.t.Fatal(err)
	}
}

// decide runs the gate and returns what it would publish and why.
func (r *release) decide(tag, auto string) (publish bool, reason string) {
	r.t.Helper()
	out, err := exec.Command("scripts/release-gate.sh", r.dist, r.gates, tag, auto).CombinedOutput()
	if err != nil {
		r.t.Fatalf("the gate failed instead of deciding: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "publish=") || !strings.HasPrefix(lines[1], "reason=") {
		r.t.Fatalf("expected exactly publish= and reason= lines, got %q", out)
	}
	switch lines[0] {
	case "publish=true":
		publish = true
	case "publish=false":
	default:
		r.t.Fatalf("unexpected publish line %q", lines[0])
	}
	return publish, strings.TrimPrefix(lines[1], "reason=")
}

func TestGatePublishesOnlyWhenEverythingIsInOrder(t *testing.T) {
	r := newRelease(t)
	publish, reason := r.decide("v0.1.0-rc.1", "true")
	if !publish {
		t.Fatalf("a complete, passing pre-release with the switch on should publish: %s", reason)
	}
}

func TestGateNeverPublishesAStableRelease(t *testing.T) {
	r := newRelease(t)
	for _, tag := range []string{"v0.1.0", "v1.0.0", "v10.20.30"} {
		if publish, reason := r.decide(tag, "true"); publish {
			t.Errorf("%s: a stable release must always be a draft (%s)", tag, reason)
		} else if !strings.Contains(reason, "stable") {
			t.Errorf("%s: the reason should say why: %q", tag, reason)
		}
	}
}

// Only the exact value "true" turns it on; anything else, including values that
// merely look affirmative, leaves a draft.
func TestGateRequiresTheSwitchToBeExactlyTrue(t *testing.T) {
	r := newRelease(t)
	for _, auto := range []string{"", "false", "TRUE", "True", "1", "yes", "on", "true ", " true", "true;", "truee"} {
		if publish, _ := r.decide("v0.1.0-rc.1", auto); publish {
			t.Errorf("AUTO_PUBLISH_PRERELEASES=%q must not publish", auto)
		}
	}
	if _, reason := r.decide("v0.1.0-rc.1", ""); !strings.Contains(reason, "AUTO_PUBLISH_PRERELEASES") {
		t.Errorf("the reason should name the switch: %q", reason)
	}
}

func TestGateRequiresEveryReleaseFile(t *testing.T) {
	for _, f := range allFiles {
		r := newRelease(t)
		r.remove(r.dist, f)
		publish, reason := r.decide("v0.1.0-rc.1", "true")
		if publish {
			t.Errorf("published without %s", f)
		}
		if !strings.Contains(reason, "missing") {
			t.Errorf("without %s the reason should say files are missing: %q", f, reason)
		}
	}
}

func TestGateDoesNotNeedTheAppImage(t *testing.T) {
	r := newRelease(t) // has no AppImage
	if publish, reason := r.decide("v0.1.0-rc.1", "true"); !publish {
		t.Fatalf("the AppImage is optional: %s", reason)
	}
}

func TestGateRequiresASmokeTestThatPassedOnEveryPlatform(t *testing.T) {
	for _, platform := range []string{"macos", "windows", "linux"} {
		// A failed, cancelled or skipped test, or garbage, is not a pass.
		for _, result := range []string{"failure", "cancelled", "skipped", "", "passed", "success but"} {
			r := newRelease(t)
			r.write(r.gates, "SMOKE-"+platform+".txt", result+"\n")
			publish, reason := r.decide("v0.1.0-rc.1", "true")
			if publish {
				t.Errorf("published although the %s smoke test said %q", platform, result)
			}
			if !strings.Contains(reason, platform) {
				t.Errorf("the reason should name %s: %q", platform, reason)
			}
		}
		// A platform that recorded nothing has not passed either.
		r := newRelease(t)
		r.remove(r.gates, "SMOKE-"+platform+".txt")
		if publish, reason := r.decide("v0.1.0-rc.1", "true"); publish || !strings.Contains(reason, "not recorded") {
			t.Errorf("a missing %s result must block publishing: %v %q", platform, publish, reason)
		}
	}
}

func TestGateTreatsAnEmptyOrMissingDistAsADraft(t *testing.T) {
	r := &release{t: t, dist: t.TempDir(), gates: t.TempDir()}
	if publish, _ := r.decide("v0.1.0-rc.1", "true"); publish {
		t.Fatal("nothing was built; there is nothing to publish")
	}
}

func TestGateIsStrictAboutHowItIsCalled(t *testing.T) {
	if out, err := exec.Command("scripts/release-gate.sh").CombinedOutput(); err == nil {
		t.Fatalf("calling it with no arguments should be an error, got %q", out)
	}
	if err := exec.Command("scripts/release-gate.sh", t.TempDir(), t.TempDir()).Run(); err == nil {
		t.Fatal("a missing tag should be an error")
	}
}

// ---- how the workflows use it ----

func readWorkflow(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(".github/workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The decision must really come from the gate script, not merely mention it:
// the gate step has to run it, and the step that creates the release has to be
// driven by that step's output.
func TestReleaseStepsAreWiredToTheGate(t *testing.T) {
	var gate, create *step
	steps := loadWorkflows(t)["release.yml"].Jobs["release"].Steps
	for i := range steps {
		switch {
		case steps[i].ID == "gate":
			gate = &steps[i]
		case strings.Contains(steps[i].Run, "gh release create"):
			create = &steps[i]
		}
	}
	if gate == nil || create == nil {
		t.Fatalf("release job needs a step with id 'gate' and one running 'gh release create' (gate=%v create=%v)", gate != nil, create != nil)
	}
	if !strings.Contains(gate.Run, "scripts/release-gate.sh dist gates") {
		t.Errorf("the gate step must run the gate script, but runs: %q", gate.Run)
	}
	if !strings.Contains(gate.Run, "$GITHUB_OUTPUT") {
		t.Error("the gate step must write the script's answer to $GITHUB_OUTPUT")
	}
	if got := gate.Env["AUTO_PUBLISH"]; got != "${{ vars.AUTO_PUBLISH_PRERELEASES }}" {
		t.Errorf("the gate must read the repository variable, got %q", got)
	}
	if got := create.Env["PUBLISH"]; got != "${{ steps.gate.outputs.publish }}" {
		t.Errorf("the release step must be driven by the gate's answer, got %q", got)
	}
	for i, st := range steps {
		if st.ID == "gate" && i > 0 {
			for _, later := range steps[:i] {
				if strings.Contains(later.Run, "gh release create") {
					t.Error("the release is created before the gate has decided")
				}
			}
		}
	}
}

func TestReleaseWorkflowPublishesOnlyThroughTheGate(t *testing.T) {
	text := readWorkflow(t, "release.yml")
	for what, ok := range map[string]bool{
		"runs the gate script":                       strings.Contains(text, "scripts/release-gate.sh"),
		"reads the repository variable":              strings.Contains(text, "vars.AUTO_PUBLISH_PRERELEASES"),
		"acts on the gate's answer":                  strings.Contains(text, "steps.gate.outputs.publish"),
		"keeps the smoke results out of the release": strings.Contains(text, "SMOKE-") && strings.Contains(text, "gates"),
		"never makes an automatic release 'latest'":  strings.Contains(text, "--prerelease --latest=false"),
		"drafts by default":                          strings.Contains(text, "flags+=(--draft)"),
	} {
		if !ok {
			t.Errorf("release.yml does not: %s", what)
		}
	}
	// The only path that omits --draft must be the one guarded by PUBLISH == true.
	create := text[strings.Index(text, "Create the release"):]
	publishBranch := regexp.MustCompile(`(?s)if \[ "\$\{PUBLISH\}" = "true" \]; then(.*?)else`).FindStringSubmatch(create)
	if publishBranch == nil || !strings.Contains(publishBranch[1], "--prerelease --latest=false") || strings.Contains(publishBranch[1], "--draft") {
		t.Error("the publish branch must add only --prerelease --latest=false, and be guarded by the gate's answer")
	}
}

func TestCIRecordsTheSmokeResultForTheGate(t *testing.T) {
	text := readWorkflow(t, "ci.yml")
	if !strings.Contains(text, "id: smoke") || !strings.Contains(text, "steps.smoke.outcome") || !strings.Contains(text, "dist/SMOKE-") {
		t.Error("ci.yml must record the smoke test's outcome in dist/SMOKE-<platform>.txt so the release gate can read it")
	}
	// The result is written after the checksums are made, so it is not a released file.
	if strings.Index(text, "Record the smoke test result") < strings.Index(text, "Collect the release files") {
		t.Error("the smoke result must be recorded after packaging (which recreates dist/)")
	}
}

func TestGateFilesMatchWhatPackagingProduces(t *testing.T) {
	// The gate's file patterns and package-ci.sh's names must agree, or a
	// healthy release would never be allowed to publish itself.
	gate, err := os.ReadFile("scripts/release-gate.sh")
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := os.ReadFile("scripts/package-ci.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-macos-universal.zip", "-windows-amd64-setup.exe", "-windows-amd64-portable.zip", "-linux-amd64.tar.gz", "-linux-amd64.deb"} {
		if !strings.Contains(string(gate), suffix) {
			t.Errorf("release-gate.sh does not look for %s", suffix)
		}
	}
	for _, want := range []string{`macos-universal`, `-setup.exe`, `-portable.zip`, `.tar.gz`, `.deb`} {
		if !strings.Contains(string(pkg), want) {
			t.Errorf("package-ci.sh no longer produces %q, which the release gate requires", want)
		}
	}
}
