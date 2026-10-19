package projects

import "testing"

func TestMemory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if Memory(7) != "" {
		t.Fatal("no notes yet")
	}
	if err := SetMemory(7, "  - 2026-10-19 Les tests se lancent avec make test.\n"); err != nil {
		t.Fatal(err)
	}
	if m := Memory(7); m != "- 2026-10-19 Les tests se lancent avec make test." {
		t.Fatalf("notes %q", m)
	}
	if err := SetMemory(7, ""); err != nil || Memory(7) != "" {
		t.Errorf("clearing: %v", err)
	}
}
