package projects

import (
	"testing"

	"github.com/Impre-visible/mihari/internal/gitlab"
)

func TestAuthorizes(t *testing.T) {
	members := []gitlab.Member{
		{Username: "dev", AccessLevel: gitlab.AccessDeveloper},
		{Username: "reporter", AccessLevel: gitlab.AccessReporter},
		{Username: "Guest", AccessLevel: gitlab.AccessGuest},
		{Username: "owner", AccessLevel: gitlab.AccessOwner},
	}
	s := Settings{
		MinRole:      gitlab.AccessDeveloper,
		ExtraUsers:   []string{"guest", "left-the-project"},
		BlockedUsers: []string{"Owner"},
	}
	cases := map[string]bool{
		"dev":              true,
		"DEV":              true,
		"reporter":         false,
		"guest":            true,
		"left-the-project": false,
		"stranger":         false,
		"owner":            false,
	}
	for user, want := range cases {
		if got := s.Authorizes(user, members); got != want {
			t.Errorf("%s: got %v, want %v", user, got, want)
		}
	}
}

func TestDefaults(t *testing.T) {
	c := Config{DefaultImage: DefaultImage, Projects: []Settings{{ID: 1, Image: " node:22 ", Enabled: true}}}
	if c.Image(1) != "node:22" || c.Image(2) != DefaultImage {
		t.Errorf("images: %q %q", c.Image(1), c.Image(2))
	}
	if p := c.Project(2); p.Enabled || p.MinRole != gitlab.AccessDeveloper {
		t.Errorf("unknown project should be off with Developer: %+v", p)
	}
}
