package projects

import (
	"strings"
	"testing"
)

func TestRemember(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if Memory(7) != "" {
		t.Fatal("no notes yet")
	}
	_ = Remember(7, "Les tests se lancent avec make test.")
	_ = Remember(7, "  Les tests   se lancent avec make test.  ")
	_ = Remember(7, "Le code est en Go 1.26.")
	m := Memory(7)
	if strings.Count(m, "make test") != 1 || !strings.Contains(m, "Go 1.26") || strings.Count(m, "\n") != 1 {
		t.Fatalf("notes %q", m)
	}
	for i := range memoryLines + 5 {
		_ = Remember(7, "note "+strings.Repeat("x", i))
	}
	if n := len(strings.Split(Memory(7), "\n")); n != memoryLines {
		t.Errorf("%d lines kept, want %d", n, memoryLines)
	}
	if err := SetMemory(7, ""); err != nil || Memory(7) != "" {
		t.Errorf("clearing: %v", err)
	}
}
