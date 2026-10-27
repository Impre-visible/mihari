package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Impre-visible/mihari/internal/projects"
)

func TestCIImage(t *testing.T) {
	cases := []struct {
		name, ci, image, job string
	}{
		{"test stage", `
image: alpine
stages: [build, test]
build: {stage: build, image: golang:1.26, script: [go build]}
unit-tests: {stage: test, image: golang:1.26, script: [go test ./...]}
lint: {stage: test, script: [golangci-lint run]}
`, "golang:1.26", "unit-tests"},
		{"default image and object form", `
default:
  image: {name: "node:22-bookworm"}
test:
  script: [npm test]
`, "node:22-bookworm", "test"},
		{"extends", `
.node: {image: "node:20"}
test:e2e: {extends: .node, stage: test, script: [npx playwright test]}
`, "node:20", "test:e2e"},
		{"only CI variables", `
test: {image: "$CI_REGISTRY_IMAGE/ci:latest", script: [make test]}
`, "", ""},
		{"no test job", `
deploy: {image: alpine, script: [./deploy.sh]}
`, "", ""},
	}
	for _, c := range cases {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, ".gitlab-ci.yml"), []byte(c.ci), 0o644)
		image, job := CIImage(dir)
		if image != c.image || job != c.job {
			t.Errorf("%s: got %q from %q, want %q from %q", c.name, image, job, c.image, c.job)
		}
	}
	if image, _ := CIImage(t.TempDir()); image != "" {
		t.Error("no CI file should give no image")
	}
}

func TestLanguageImage(t *testing.T) {
	repo := t.TempDir()
	c := projects.Config{DefaultImage: projects.DefaultImage}
	if image, source := PickImage(c, 1, repo); image != projects.DefaultImage || source != "image par défaut" {
		t.Errorf("no language: %s (%s)", image, source)
	}
	_ = os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module x\n"), 0o644)
	if image, source := PickImage(c, 1, repo); image != "golang:1" || source != "projet Go" {
		t.Errorf("Go project: %s (%s)", image, source)
	}
	// A default the user chose stays theirs.
	if image, _ := PickImage(projects.Config{DefaultImage: "alpine"}, 1, repo); image != "alpine" {
		t.Errorf("chosen default: %s", image)
	}
	// The CI's test image comes first.
	_ = os.WriteFile(filepath.Join(repo, ".gitlab-ci.yml"), []byte("test:\n  image: golang:1.22\n  script: [go test ./...]\n"), 0o644)
	if image, _ := PickImage(c, 1, repo); image != "golang:1.22" {
		t.Errorf("CI image: %s", image)
	}
}
