package main

import (
	"encoding/xml"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Most of build/ is generated from build/config.yml by
// `wails3 task common:update:build-assets`, which overwrites it. These checks
// make sure the hand-made parts that make ".d2 files open in D2 Studio" work
// on every platform survive such a regeneration.

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func configMIME(t *testing.T) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^\s*mimeType:\s*(\S+)`).FindStringSubmatch(read(t, "build/config.yml"))
	if m == nil {
		t.Fatal("build/config.yml declares no mimeType for the .d2 file association")
	}
	return m[1]
}

func TestConfigDeclaresTheD2FileAssociation(t *testing.T) {
	cfg := read(t, "build/config.yml")
	if !regexp.MustCompile(`(?m)^\s*-\s*ext:\s*d2\s*$`).MatchString(cfg) {
		t.Fatal("build/config.yml must declare the d2 file association")
	}
}

func TestMacBundleDeclaresD2DocumentType(t *testing.T) {
	plist := read(t, "build/darwin/Info.plist")
	if !strings.Contains(plist, "CFBundleDocumentTypes") || !strings.Contains(plist, "<string>d2</string>") {
		t.Fatal("build/darwin/Info.plist does not declare the .d2 document type")
	}
	if !strings.Contains(plist, "<string>D2 Studio</string>") || strings.Contains(plist, "My Product") {
		t.Fatal("the bundle must be named D2 Studio")
	}
}

func TestWindowsInstallerAssociatesD2Files(t *testing.T) {
	tools := read(t, "build/windows/nsis/wails_tools.nsh")
	if !strings.Contains(tools, `APP_ASSOCIATE "d2"`) {
		t.Fatal("the Windows installer does not associate .d2 files")
	}
	if !strings.Contains(read(t, "build/windows/nsis/project.nsi"), "wails.associateFiles") {
		t.Fatal("the installer script never runs the association step")
	}
	if !strings.Contains(tools, `$\"%1$\"`) {
		t.Fatal("the association must pass the file as an argument ('%1')")
	}
}

func TestLinuxDesktopEntryAdvertisesAndReceivesD2Files(t *testing.T) {
	task := read(t, "build/linux/Taskfile.yml")
	mime := configMIME(t)
	for what, want := range map[string]string{
		"the MIME type from config.yml":  mime,
		"%f, so a chosen file is passed": "%f",
		"a display name of D2 Studio":    "D2 Studio",
	} {
		if !strings.Contains(task, want) {
			t.Errorf("build/linux/Taskfile.yml's desktop entry lacks %s (%q); regenerating the build assets may have overwritten it", what, want)
		}
	}
	if strings.Contains(task, "%u") {
		t.Errorf("the desktop entry passes a URL (%%u); a file manager sends a file (%%f)")
	}
}

func TestLinuxPackageInstallsTheMIMEMappingAndHasOurLicense(t *testing.T) {
	nfpm := read(t, "build/linux/nfpm/nfpm.yaml")
	if !strings.Contains(nfpm, "/usr/share/mime/packages/") || !strings.Contains(nfpm, "d2-studio-mime.xml") {
		t.Error("the Linux package must install the shared-MIME mapping for *.d2")
	}
	if !strings.Contains(nfpm, `license: "MPL-2.0"`) {
		t.Error("the package must declare the MPL-2.0 license")
	}
	if strings.Contains(nfpm, "wails.io") || strings.Contains(nfpm, `license: "MIT"`) {
		t.Error("the package still carries template values")
	}
}

func TestSharedMIMEMappingMatchesTheConfig(t *testing.T) {
	var doc struct {
		Types []struct {
			Type  string `xml:"type,attr"`
			Globs []struct {
				Pattern string `xml:"pattern,attr"`
			} `xml:"glob"`
		} `xml:"mime-type"`
	}
	if err := xml.Unmarshal([]byte(read(t, "build/linux/d2-studio-mime.xml")), &doc); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
	if len(doc.Types) != 1 || doc.Types[0].Type != configMIME(t) {
		t.Fatalf("types = %+v, want one of %q", doc.Types, configMIME(t))
	}
	if len(doc.Types[0].Globs) != 1 || doc.Types[0].Globs[0].Pattern != "*.d2" {
		t.Fatalf("globs = %+v", doc.Types[0].Globs)
	}
}

func TestNoMobileScaffoldingIsLeftBehind(t *testing.T) {
	for _, dir := range []string{"build/ios", "build/android"} {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("%s exists; mobile is out of scope. The update task removes it, so was it run by hand?", dir)
		}
	}
}

func TestEveryPlatformCarriesTheProductName(t *testing.T) {
	for path, want := range map[string]string{
		"build/windows/info.json":    `"ProductName": "D2 Studio"`,
		"build/linux/nfpm/nfpm.yaml": `vendor: "LucidInternets"`,
	} {
		if !strings.Contains(read(t, path), want) {
			t.Errorf("%s should contain %s", path, want)
		}
	}
}

// The Windows installer bundles files from build/windows (the file-type icon
// for .d2 files, the installer's own icon). If one is missing the NSIS build
// fails, and only on the Windows runner, so check the references here.
func TestWindowsInstallerFilesExist(t *testing.T) {
	ref := regexp.MustCompile(`(?m)^\s*(?:File\s+"|!define MUI_U?N?ICON\s+")\.\.\\([^"\\]+)"`)
	checked := 0
	for _, script := range []string{"build/windows/nsis/project.nsi", "build/windows/nsis/wails_tools.nsh"} {
		for _, m := range ref.FindAllStringSubmatch(read(t, script), -1) {
			checked++
			if _, err := os.Stat("build/windows/" + m[1]); err != nil {
				t.Errorf("%s bundles %q, but build/windows/%s does not exist", script, m[1], m[1])
			}
		}
	}
	if checked < 3 { // the installer icon, the uninstaller icon and the .d2 icon
		t.Fatalf("found only %d bundled files in the installer scripts; has their format changed?", checked)
	}
}
