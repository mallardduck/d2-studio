// Command gennotices lists the third-party software D2 Studio ships and writes
// the notices a distribution needs.
//
//	go run ./tools/gennotices            # writes THIRD_PARTY_NOTICES.md
//	go run ./tools/gennotices -full F    # also writes every license text to F
//	go run ./tools/gennotices -check     # fails if any license is unrecognised
//
// It uses only the standard library and the local module cache: Go modules
// are the ones the build really links (go list -deps, for each platform, with
// the production tag), and the frontend's are the production dependencies in
// package-lock.json. A license it cannot identify is reported, never guessed.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type dep struct {
	Name, Version, License string
	Dir                    string // where the license text lives
}

func main() {
	full := flag.String("full", "", "also write all license texts to this file")
	check := flag.Bool("check", false, "write nothing; fail if a license is unrecognised")
	out := flag.String("o", "THIRD_PARTY_NOTICES.md", "summary output file")
	flag.Parse()

	goDeps, err := goModules([]string{"darwin", "linux", "windows"}, "d2studio")
	if err != nil {
		fatal(err)
	}
	jsDeps, err := npmProduction("frontend/package-lock.json", "frontend/node_modules")
	if err != nil {
		fatal(err)
	}

	var unknown []string
	for _, d := range append(append([]dep{}, goDeps...), jsDeps...) {
		if d.License == unrecognised || d.License == "" {
			unknown = append(unknown, d.Name+"@"+d.Version)
		}
	}
	if *check {
		if len(unknown) > 0 {
			fatal(fmt.Errorf("licenses not recognised for: %s", strings.Join(unknown, ", ")))
		}
		fmt.Printf("ok: %d Go modules and %d frontend packages, all licenses recognised\n", len(goDeps), len(jsDeps))
		return
	}

	if err := os.WriteFile(*out, []byte(summary(goDeps, jsDeps)), 0o644); err != nil {
		fatal(err)
	}
	if *full != "" {
		if err := os.WriteFile(*full, []byte(fullTexts(goDeps, jsDeps)), 0o644); err != nil {
			fatal(err)
		}
	}
	fmt.Printf("wrote %s (%d Go modules, %d frontend packages)\n", *out, len(goDeps), len(jsDeps))
	if len(unknown) > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: licenses not recognised for: %s\n", strings.Join(unknown, ", "))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gennotices:", err)
	os.Exit(1)
}

// goModules returns the modules linked into the app on any of the platforms,
// excluding the main module.
func goModules(platforms []string, mainModule string) ([]dep, error) {
	seen := map[string]dep{}
	for _, goos := range platforms {
		cmd := exec.Command("go", "list", "-deps", "-tags", "production", "-f",
			`{{with .Module}}{{.Path}}{{"\t"}}{{.Version}}{{"\t"}}{{.Dir}}{{end}}`, ".")
		cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		b, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("go list for %s: %v\n%s", goos, err, stderr.String())
		}
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Split(line, "\t")
			if len(f) != 3 || f[0] == "" || f[0] == mainModule {
				continue
			}
			if _, ok := seen[f[0]]; !ok {
				seen[f[0]] = dep{Name: f[0], Version: f[1], Dir: f[2], License: licensesIn(f[2])}
			}
		}
	}
	return sorted(seen), nil
}

// npmProduction returns the lockfile's non-dev packages with their licenses.
func npmProduction(lockfile, modulesDir string) ([]dep, error) {
	b, err := os.ReadFile(lockfile)
	if err != nil {
		return nil, err
	}
	deps, err := parseLockfile(b, modulesDir)
	if err != nil {
		return nil, err
	}
	return deps, nil
}

func parseLockfile(data []byte, modulesDir string) ([]dep, error) {
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
			License any    `json:"license"`
			Dev     bool   `json:"dev"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	seen := map[string]dep{}
	for path, p := range lock.Packages {
		if path == "" || p.Dev {
			continue
		}
		name := path[strings.LastIndex(path, "node_modules/")+len("node_modules/"):]
		lic, _ := p.License.(string)
		dir := filepath.Join(modulesDir, name)
		if lic == "" {
			lic = licensesIn(dir)
		}
		seen[name+"@"+p.Version] = dep{Name: name, Version: p.Version, License: lic, Dir: dir}
	}
	return sorted(seen), nil
}

func sorted(m map[string]dep) []dep {
	out := make([]dep, 0, len(m))
	for _, d := range m {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Version < out[j].Version
	})
	return out
}

// licenseFile returns the primary license in dir: the file named exactly
// LICENSE (or similar) if there is one, else the first license-like file.
func licenseFile(dir string) (text, name string) {
	files := licenseFiles(dir)
	if len(files) == 0 {
		return "", ""
	}
	return files[0].text, files[0].name
}

type namedText struct{ name, text string }

// licenseFiles returns every license, copying and notice file in dir, with
// the primary license first. A module can carry several licenses (and NOTICE
// files that must travel with it), and all of them have to be distributed.
func licenseFiles(dir string) []namedText {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []namedText
	for _, e := range entries {
		n := strings.ToUpper(e.Name())
		if e.IsDir() || !(strings.HasPrefix(n, "LICENSE") || strings.HasPrefix(n, "LICENCE") ||
			strings.HasPrefix(n, "COPYING") || strings.HasPrefix(n, "UNLICENSE") || strings.HasPrefix(n, "NOTICE")) {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
			out = append(out, namedText{e.Name(), string(b)})
		}
	}
	// The primary license first: names without a suffix, and never a NOTICE.
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i].name) < rank(out[j].name) })
	return out
}

func rank(name string) int {
	n := strings.ToUpper(name)
	switch {
	case strings.HasPrefix(n, "NOTICE"):
		return 3
	case n == "LICENSE" || n == "LICENSE.TXT" || n == "LICENSE.MD" || n == "LICENCE" || n == "COPYING" || n == "UNLICENSE":
		return 0
	case strings.HasPrefix(n, "LICENSE") || strings.HasPrefix(n, "LICENCE") || strings.HasPrefix(n, "COPYING"):
		return 1
	}
	return 2
}

const unrecognised = "UNRECOGNISED"

// detect names every license a text contains, joined with " + " (a module can
// carry several: chroma's COPYING is MIT plus the OFL for a bundled font), or
// returns unrecognised. It looks for the distinctive wording of each license
// and prefers saying "unrecognised" to guessing.
func detect(text string) string {
	t := strings.Join(strings.Fields(strings.ToLower(text)), " ")
	has := func(s ...string) bool {
		for _, p := range s {
			if !strings.Contains(t, p) {
				return false
			}
		}
		return true
	}
	var found []string
	add := func(ok bool, name string) {
		if ok {
			found = append(found, name)
		}
	}
	add(has("apache license", "version 2.0", "apache.org/licenses"), "Apache-2.0")
	bsd3 := has("redistribution and use in source and binary forms", "neither the name of")
	add(bsd3, "BSD-3-Clause")
	add(!bsd3 && has("redistribution and use in source and binary forms", "this list of conditions"), "BSD-2-Clause")
	add(has("eclipse public license", "v 2.0"), "EPL-2.0")
	add(has("permission to use, copy, modify, and/or distribute this software for any purpose with or without fee"),
		"ISC") // modern wording; the older one is below
	if !contains(found, "ISC") {
		add(has("permission to use, copy, modify, and distribute this software for any purpose with or without fee is hereby granted"), "ISC")
	}
	add(has("permission is hereby granted, free of charge, to any person obtaining a copy"), "MIT")
	add(has("mozilla public license", "version 2.0"), "MPL-2.0")
	add(has("sil open font license"), "OFL-1.1")
	add(has("this is free and unencumbered software released into the public domain"), "Unlicense")
	if len(found) == 0 {
		return unrecognised
	}
	sort.Strings(found)
	return strings.Join(found, " + ")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// licensesIn detects the licenses across all the license files in dir
// (not NOTICE files, which are attributions and not licenses).
func licensesIn(dir string) string {
	var all strings.Builder
	for _, f := range licenseFiles(dir) {
		if rank(f.name) < 3 {
			all.WriteString(f.text + "\n")
		}
	}
	return detect(all.String())
}

func summary(goDeps, jsDeps []dep) string {
	var b strings.Builder
	b.WriteString("# Third-party notices\n\n")
	b.WriteString("D2 Studio is released under the [Mozilla Public License 2.0](LICENSE). It includes the software below.\n")
	b.WriteString("This file is generated by `go run ./tools/gennotices`; do not edit it by hand.\n\n")
	b.WriteString("## D2 and its bundled components\n\n")
	b.WriteString("D2 (https://github.com/d2lang/d2), MPL-2.0, does the diagram compiling, layout and PNG/PDF rendering. It bundles\n")
	b.WriteString("further components (layout engines, fonts and more) that have their own licenses; D2's own notices are in\n")
	b.WriteString("[third_party/d2/THIRD_PARTY_NOTICES.txt](third_party/d2/THIRD_PARTY_NOTICES.txt).\n\n")
	b.WriteString("## Components under weak-copyleft licenses\n\n")
	b.WriteString("These are used unmodified. Their licenses require that the source of the licensed files stays available, which it is at the\n")
	b.WriteString("addresses below. They do not apply to D2 Studio's own code, which is under the MPL-2.0 in any case.\n\n")
	for _, d := range goDeps {
		if strings.Contains(d.License, "MPL-2.0") || strings.Contains(d.License, "EPL-2.0") {
			fmt.Fprintf(&b, "- %s %s (%s): source at https://%s\n", d.Name, d.Version, d.License, d.Name)
		}
	}
	b.WriteString("\n")
	table := func(title string, deps []dep) {
		fmt.Fprintf(&b, "## %s\n\n| Package | Version | License |\n|---|---|---|\n", title)
		for _, d := range deps {
			lic := d.License
			if lic == unrecognised || lic == "" {
				lic = "**not recognised: see the package**"
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", d.Name, d.Version, lic)
		}
		b.WriteString("\n")
	}
	table("Go modules", goDeps)
	table("Frontend packages (production)", jsDeps)
	b.WriteString("The full license texts are distributed with each release as `THIRD_PARTY_LICENSES.txt`.\n")
	return b.String()
}

func fullTexts(goDeps, jsDeps []dep) string {
	var b strings.Builder
	b.WriteString("D2 Studio: third-party license texts\n====================================\n\n")
	for _, group := range []struct {
		title string
		deps  []dep
	}{{"Go modules", goDeps}, {"Frontend packages", jsDeps}} {
		fmt.Fprintf(&b, "%s\n%s\n\n", group.title, strings.Repeat("-", len(group.title)))
		for _, d := range group.deps {
			files := licenseFiles(d.Dir)
			fmt.Fprintf(&b, "=== %s %s (%s) ===\n", d.Name, d.Version, d.License)
			if len(files) == 0 {
				b.WriteString("(no license file found in the package)\n\n")
				continue
			}
			for _, f := range files {
				if len(files) > 1 {
					fmt.Fprintf(&b, "--- %s ---\n", f.name)
				}
				b.WriteString(strings.TrimSpace(f.text) + "\n\n")
			}
		}
	}
	return b.String()
}
