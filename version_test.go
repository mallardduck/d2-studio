package main

import (
	"os"
	"regexp"
	"testing"
)

// build/config.yml is where the packaging reads the version from. The binary
// has its own copy for logs; they must not drift apart.
func TestAppVersionMatchesBuildConfig(t *testing.T) {
	data, err := os.ReadFile("build/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s*version:\s*"([^"]+)"`).FindSubmatch(data)
	if m == nil {
		t.Fatal("no version in build/config.yml")
	}
	if string(m[1]) != appVersion {
		t.Fatalf("build/config.yml says %q but appVersion is %q; update both together", m[1], appVersion)
	}
}

func TestAppVersionIsAPlainDottedNumber(t *testing.T) {
	// Windows file versions and the macOS bundle version only accept numbers.
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(appVersion) {
		t.Fatalf("appVersion %q must be major.minor.patch with no suffix; mark release candidates in the git tag", appVersion)
	}
}

func TestBuildInfoDescribesThePlatform(t *testing.T) {
	b := readBuildInfo()
	if b.App != appVersion || b.Go == "" || b.OS == "" || b.Arch == "" {
		t.Fatalf("buildInfo = %+v", b)
	}
}
