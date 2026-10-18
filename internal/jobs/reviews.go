package jobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/Impre-visible/mihari/internal/settings"
)

// The last commit the bot reviewed on each merge request, so that /review
// can look only at what came after. One small JSON file next to the config.

var reviewsMu sync.Mutex

func reviewsPath() (string, error) {
	config, err := settings.GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(config), "reviews.json"), nil
}

func reviewKey(projectID, iid int) string {
	return strconv.Itoa(projectID) + "!" + strconv.Itoa(iid)
}

func loadReviews() map[string]string {
	reviews := map[string]string{}
	path, err := reviewsPath()
	if err != nil {
		return reviews
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &reviews)
	}
	return reviews
}

// LastReviewed returns the commit of the bot's last review of a merge request, "" if none.
func LastReviewed(projectID, iid int) string {
	reviewsMu.Lock()
	defer reviewsMu.Unlock()
	return loadReviews()[reviewKey(projectID, iid)]
}

func markReviewed(projectID, iid int, sha string) {
	if sha == "" {
		return
	}
	reviewsMu.Lock()
	defer reviewsMu.Unlock()
	reviews := loadReviews()
	reviews[reviewKey(projectID, iid)] = sha
	path, err := reviewsPath()
	if err != nil {
		return
	}
	if data, err := json.Marshal(reviews); err == nil {
		_ = os.WriteFile(path, data, 0o600)
	}
}
