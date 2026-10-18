package sandbox

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Impre-visible/mihari/internal/projects"
	"gopkg.in/yaml.v3"
)

// Keys of .gitlab-ci.yml that are settings, not jobs.
var ciKeywords = map[string]bool{
	"default": true, "include": true, "stages": true, "variables": true, "workflow": true,
	"image": true, "services": true, "cache": true, "before_script": true, "after_script": true,
}

// CIImage returns the image the project's own test job runs in, read from
// the .gitlab-ci.yml of the clone at repo, and that job's name. It returns
// "" when there is no CI file, no test job, or an image only CI can resolve
// (one built from $CI_ variables).
func CIImage(repo string) (image, job string) {
	data, err := os.ReadFile(filepath.Join(repo, ".gitlab-ci.yml"))
	if err != nil {
		return "", ""
	}
	var doc map[string]any
	if yaml.Unmarshal(data, &doc) != nil {
		return "", ""
	}
	fallback := imageOf(doc["image"])
	if d, ok := doc["default"].(map[string]any); ok && imageOf(d["image"]) != "" {
		fallback = imageOf(d["image"])
	}

	// A job is a mapping with a script, not hidden (".template") and not a keyword.
	type candidate struct {
		name  string
		image string
		score int
	}
	var jobs []candidate
	for name, raw := range doc {
		spec, ok := raw.(map[string]any)
		if !ok || ciKeywords[name] || strings.HasPrefix(name, ".") || spec["script"] == nil {
			continue
		}
		img := imageOf(spec["image"])
		// One level of extends covers the common template pattern.
		if img == "" {
			if parent, ok := spec["extends"].(string); ok {
				if p, ok := doc[parent].(map[string]any); ok {
					img = imageOf(p["image"])
				}
			}
		}
		if img == "" {
			img = fallback
		}
		stage, _ := spec["stage"].(string)
		score := 0
		switch {
		case stage == "test" && strings.Contains(strings.ToLower(name), "test"):
			score = 3
		case stage == "test":
			score = 2
		case strings.Contains(strings.ToLower(name), "test"):
			score = 1
		}
		if score > 0 {
			jobs = append(jobs, candidate{name, img, score})
		}
	}
	// Best score first, then by name, so the choice is stable across runs.
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].score != jobs[j].score {
			return jobs[i].score > jobs[j].score
		}
		return jobs[i].name < jobs[j].name
	})
	for _, j := range jobs {
		if j.image != "" && !strings.Contains(j.image, "$") {
			return j.image, j.name
		}
	}
	return "", ""
}

// PickImage chooses the container image of a job on the project cloned at
// repo, and says where it comes from: the project's own setting, else its CI
// test job's, else the default image.
func PickImage(c projects.Config, projectID int, repo string) (image, source string) {
	if image := strings.TrimSpace(c.Project(projectID).Image); image != "" {
		return image, "réglage du projet"
	}
	if image, job := CIImage(repo); image != "" {
		return image, "job CI « " + job + " »"
	}
	return c.DefaultImage, "image par défaut"
}

// imageOf reads an image given as "node:22" or as {name: node:22}.
func imageOf(v any) string {
	switch img := v.(type) {
	case string:
		return strings.TrimSpace(img)
	case map[string]any:
		name, _ := img["name"].(string)
		return strings.TrimSpace(name)
	}
	return ""
}
