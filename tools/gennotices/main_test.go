package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"MIT", "MIT License\n\nCopyright (c) 2020 X\n\nPermission is hereby granted, free of charge, to any person obtaining a copy of this software", "MIT"},
		{"MIT reflowed", "Permission is hereby  granted,\nfree of charge, to any person\n obtaining a copy of this", "MIT"},
		{"Apache", "Apache License\nVersion 2.0, January 2004\nhttp://www.apache.org/licenses/", "Apache-2.0"},
		{"MPL", "Mozilla Public License Version 2.0\n==================================", "MPL-2.0"},
		{"EPL", "# Eclipse Public License - v 2.0\n\nTHE ACCOMPANYING PROGRAM", "EPL-2.0"},
		{"BSD-3", "Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met: * Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer. * Neither the name of the copyright holder nor the names of its contributors", "BSD-3-Clause"},
		{"BSD-2", "Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met: 1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.", "BSD-2-Clause"},
		{"ISC modern", "Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted", "ISC"},
		{"ISC older wording", "Permission to use, copy, modify, and distribute this software for any purpose with or without fee is hereby granted", "ISC"},
		{"OFL", "This Font Software is licensed under the SIL Open Font License, Version 1.1.", "OFL-1.1"},
		{"Unlicense", "This is free and unencumbered software released into the public domain.", "Unlicense"},
		// chroma's COPYING: the MIT license, then the OFL for a font it bundles.
		{"MIT then OFL", "Permission is hereby granted, free of charge, to any person obtaining a copy ... This Font Software is licensed under the SIL Open Font License, Version 1.1.", "MIT + OFL-1.1"},
		{"BSD-3 is not also reported as BSD-2", "Redistribution and use in source and binary forms ... this list of conditions ... Neither the name of the copyright holder", "BSD-3-Clause"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := detect(c.text); got != c.want {
				t.Fatalf("detect = %q, want %q", got, c.want)
			}
		})
	}
}

// Wrongly calling a license permissive would be a compliance error, so
// anything unfamiliar must be reported, never guessed.
func TestDetectDoesNotGuess(t *testing.T) {
	for name, text := range map[string]string{
		"empty":            "",
		"whitespace":       "  \n\t ",
		"a custom license": "You may use this if you are nice to the author.",
		"GPL":              "GNU GENERAL PUBLIC LICENSE Version 3, 29 June 2007",
		"LGPL":             "GNU LESSER GENERAL PUBLIC LICENSE Version 2.1",
		"AGPL":             "GNU AFFERO GENERAL PUBLIC LICENSE Version 3",
		"just a copyright": "Copyright (c) 2024 Somebody. All rights reserved.",
	} {
		if got := detect(text); got != unrecognised {
			t.Errorf("%s was identified as %q; it must be reported as unrecognised", name, got)
		}
	}
}

func TestParseLockfile(t *testing.T) {
	lock := `{"packages": {
	  "": {"name": "app", "version": "1.0.0"},
	  "node_modules/react": {"version": "18.3.1", "license": "MIT"},
	  "node_modules/@codemirror/state": {"version": "6.7.6", "license": "MIT"},
	  "node_modules/vitest": {"version": "3.0.0", "license": "MIT", "dev": true},
	  "node_modules/a/node_modules/nested": {"version": "2.0.0", "license": "ISC"},
	  "node_modules/nolicense": {"version": "1.0.0"}
	}}`
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nolicense"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nolicense", "LICENSE"), []byte("Permission is hereby granted, free of charge, to any person obtaining a copy"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps, err := parseLockfile([]byte(lock), dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, d := range deps {
		got[d.Name] = d.License
	}
	want := map[string]string{"react": "MIT", "@codemirror/state": "MIT", "nested": "ISC", "nolicense": "MIT"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if _, dev := got["vitest"]; dev {
		t.Error("dev dependencies are not shipped and must not be listed")
	}
}

// elk-go ships its EPL, an Apache license and a BSD license as separate files.
func TestLicensesAreDetectedAcrossAllLicenseFilesButNotNotices(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"LICENSE":            "# Eclipse Public License - v 2.0",
		"LICENSE.apache-2.0": "Apache License Version 2.0 http://www.apache.org/licenses/",
		"LICENSE.v8-bsd":     "Redistribution and use in source and binary forms ... this list of conditions ... Neither the name of Google",
		"NOTICE.md":          "Permission is hereby granted, free of charge, to any person obtaining a copy (quoted in a notice)",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := licensesIn(dir); got != "Apache-2.0 + BSD-3-Clause + EPL-2.0" {
		t.Fatalf("licensesIn = %q; every license file counts, a NOTICE does not", got)
	}
}

func TestLicenseFilesIncludeEveryLicenseAndNoticeWithThePrimaryFirst(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"LICENSE":            "primary",
		"LICENSE.apache-2.0": "apache",
		"NOTICE.md":          "notice",
		"README.md":          "not a license",
		"main.go":            "package x",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := licenseFiles(dir)
	var names []string
	for _, f := range files {
		names = append(names, f.name)
	}
	if strings.Join(names, ",") != "LICENSE,LICENSE.apache-2.0,NOTICE.md" {
		t.Fatalf("files = %v", names)
	}
	if text, name := licenseFile(dir); name != "LICENSE" || text != "primary" {
		t.Fatalf("primary = %q %q", name, text)
	}
}

func TestSummaryFlagsUnrecognisedAndExplainsWeakCopyleft(t *testing.T) {
	out := summary(
		[]dep{{Name: "github.com/d2lang/d2", Version: "v0.9.0", License: "MPL-2.0"}, {Name: "github.com/d2lang/elk-go", Version: "v0.2.0", License: "Apache-2.0 + EPL-2.0"}, {Name: "example.com/odd", Version: "v1", License: unrecognised}},
		[]dep{{Name: "react", Version: "18.3.1", License: "MIT"}},
	)
	for _, want := range []string{
		"Mozilla Public License 2.0", "third_party/d2/THIRD_PARTY_NOTICES.txt", "| react | 18.3.1 | MIT |",
		"weak-copyleft", "source at https://github.com/d2lang/d2", "source at https://github.com/d2lang/elk-go", "**not recognised: see the package**",
	} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(want)) {
			t.Errorf("summary is missing %q", want)
		}
	}
}

func TestFullTextsShipEveryFileOfAMultiLicenseModule(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"LICENSE": "EPL text", "LICENSE.v8-bsd": "BSD text", "NOTICE.md": "notice text"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := fullTexts([]dep{{Name: "m", Version: "v1", License: "EPL-2.0", Dir: dir}}, nil)
	for _, want := range []string{"EPL text", "BSD text", "notice text", "--- LICENSE.v8-bsd ---"} {
		if !strings.Contains(out, want) {
			t.Errorf("full texts are missing %q", want)
		}
	}
	if !strings.Contains(fullTexts([]dep{{Name: "none", Version: "v1", License: unrecognised, Dir: t.TempDir()}}, nil), "no license file found") {
		t.Error("a package with no license file must say so")
	}
}
