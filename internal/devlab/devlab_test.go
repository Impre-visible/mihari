package devlab

import (
	"strings"
	"testing"

	"github.com/Impre-visible/mihari/internal/gitlab"
)

func TestSpecs(t *testing.T) {
	specs, err := Specs()
	if err != nil || len(specs) < 2 {
		t.Fatal(len(specs), err)
	}
	if _, ok := specs[1].Files["go.mod"]; !ok || specs[1].Path != gitlab.TestPrefix+"factures" {
		t.Errorf("%s should have its go.mod: %v", specs[1].Path, specs[1].Files)
	}
	for _, s := range specs {
		if !strings.HasPrefix(s.Path, gitlab.TestPrefix) || len(s.Files) == 0 || s.Description == "" {
			t.Errorf("%s: incomplete", s.Path)
		}
		for name := range s.Files {
			if strings.HasSuffix(name, ".fixture") {
				t.Errorf("%s: %s keeps its suffix", s.Path, name)
			}
		}
		for _, mr := range s.MergeRequests {
			if len(mr.Files) == 0 || mr.Title == "" {
				t.Errorf("%s: merge request %q changes nothing", s.Path, mr.Branch)
			}
		}
		for _, is := range s.Issues {
			if len(is.Labels) == 0 {
				t.Errorf("%s: issue %q has no label for the night profile", s.Path, is.Title)
			}
		}
	}
}

func TestHas(t *testing.T) {
	list := []gitlab.TestProject{{Path: "bot/mihari-test-factures-k3x9"}}
	if !has(list, "mihari-test-factures") || has(list, "mihari-test-agenda") {
		t.Error("a suffixed project counts, another does not")
	}
}
