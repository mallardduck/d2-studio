package snippets

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"d2studio/compiler"
)

var idPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

func TestSnippetsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range All() {
		if !idPattern.MatchString(s.ID) {
			t.Errorf("%q: ID must be lowercase letters, digits and dashes", s.ID)
		}
		if seen[s.ID] {
			t.Errorf("duplicate ID %q", s.ID)
		}
		seen[s.ID] = true
		if s.Title == "" || s.Description == "" || s.Category == "" {
			t.Errorf("%s: title, description and category are required", s.ID)
		}
		if !strings.HasSuffix(s.Code, "\n") {
			t.Errorf("%s: code must end with a newline", s.ID)
		}
		if refersToWeb := strings.Contains(s.Code, "https://") || strings.Contains(s.Code, "http://"); s.Remote != (refersToWeb && strings.Contains(s.Code, "icon:")) {
			t.Errorf("%s: Remote flag disagrees with the code", s.ID)
		}
	}
}

// Every example must compile, or the gallery would hand users broken D2.
func TestEverySnippetCompiles(t *testing.T) {
	svc, err := compiler.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range All() {
		t.Run(s.ID, func(t *testing.T) {
			svg, diags, err := svc.RenderSVG(context.Background(), s.Code)
			if err != nil {
				t.Fatal(err)
			}
			if len(diags) > 0 {
				t.Fatalf("diagnostics: %+v", diags)
			}
			if !strings.HasPrefix(svg, "<svg") {
				t.Fatalf("not an SVG: %.40q", svg)
			}
		})
	}
}

func TestByID(t *testing.T) {
	if _, ok := ByID("labels"); !ok {
		t.Fatal("labels should exist")
	}
	if _, ok := ByID("nope"); ok {
		t.Fatal("unknown ID should not be found")
	}
}

func TestAllReturnsACopy(t *testing.T) {
	a := All()
	a[0].Title = "changed"
	if All()[0].Title == "changed" {
		t.Fatal("All must not expose the package's slice")
	}
}
