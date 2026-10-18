package sandbox

import (
	"os"
	"path/filepath"
	"testing"
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
