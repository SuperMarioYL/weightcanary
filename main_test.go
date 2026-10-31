package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestVersionLockstep keeps the hand-maintained version surfaces in lockstep:
// the VERSION file, the `version` constant behind `weightcanary --version`,
// and web/site.json's product-version key. One surface lagging the others is
// the recurring version-drift defect class; this test fails the build when
// they disagree.
func TestVersionLockstep(t *testing.T) {
	want := version

	raw, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != want {
		t.Errorf("VERSION = %q, want %q (the --version constant) — bump all version surfaces together", got, want)
	}

	site, err := os.ReadFile("web/site.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Meta struct {
			SourceVersion string `json:"source_version"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(site, &doc); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimPrefix(doc.Meta.SourceVersion, "v"); got != want {
		t.Errorf("web/site.json source_version = %q, want %q — bump all version surfaces together", doc.Meta.SourceVersion, want)
	}
}
