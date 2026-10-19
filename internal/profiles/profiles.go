// Package profiles reads the profiles and the schedule saved by the frontend,
// and tells which profile should be running at a given time.
package profiles

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/settings"
)

// Trigger says what starts a profile's jobs.
type Trigger string

const (
	// TriggerPing runs a job for each GitLab todo, up to MaxJobs at once.
	TriggerPing Trigger = "ping"
	// TriggerIssues works through the issues carrying Label, one job per project.
	TriggerIssues Trigger = "issues"
)

type Profile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
	// One of the engine.Kind values.
	Engine string `json:"engine"`
	// Its form depends on the engine: a .gguf path or Hugging Face repo, an Ollama tag…
	Model         string  `json:"model"`
	ContextPerJob int     `json:"contextPerJob"`
	MaxJobs       int     `json:"maxJobs"`
	Trigger       Trigger `json:"trigger"`
	Label         string  `json:"label"`
	// Appended as is to the engine's command line, split on whitespace.
	ExtraArgs string `json:"extraArgs"`
	// A job fails past this many model calls or minutes, unless the matching
	// Unlimited field lifts the limit.
	MaxTurns       int  `json:"maxTurns"`
	MaxMinutes     int  `json:"maxMinutes"`
	UnlimitedTurns bool `json:"unlimitedTurns"`
	UnlimitedTime  bool `json:"unlimitedTime"`
	// The model reads images, so jobs may hand it some (read_image).
	Vision bool `json:"vision"`
}

// TurnLimit returns the most model calls a job may make, 0 for no limit.
func (p Profile) TurnLimit() int {
	if p.UnlimitedTurns {
		return 0
	}
	return p.MaxTurns
}

// TimeLimit returns how long a job may run, 0 for no limit.
func (p Profile) TimeLimit() time.Duration {
	if p.UnlimitedTime {
		return 0
	}
	return time.Duration(p.MaxMinutes) * time.Minute
}

// Segment is a weekly time range. Days use time.Weekday numbering, and an End
// at or before Start runs past midnight into the next day.
type Segment struct {
	Profile string `json:"profile"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Days    []int  `json:"days"`
}

// Config is everything the scheduler needs from the settings.
type Config struct {
	Profiles []Profile
	Segments []Segment
	// Profile for every minute no segment covers; "" means idle.
	Fallback string
}

func Load() (Config, error) {
	raw, err := (&settings.Service{}).GetConfig()
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := decode(raw["profiles"], &c.Profiles); err != nil {
		return Config{}, fmt.Errorf("profils illisibles : %w", err)
	}
	if err := decode(raw["schedule"], &c.Segments); err != nil {
		return Config{}, fmt.Errorf("planning illisible : %w", err)
	}
	c.Fallback, _ = raw["schedule_fallback"].(string)
	for i := range c.Profiles {
		c.Profiles[i].normalize()
	}
	return c, nil
}

// decode turns the generic JSON value GetConfig returns into a typed one.
func decode(value any, out any) error {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// Profiles saved before these fields existed get the frontend's defaults.
func (p *Profile) normalize() {
	if p.Engine == "" {
		p.Engine = "llama-server"
	}
	if p.MaxJobs < 1 {
		p.MaxJobs = 1
	}
	if p.ContextPerJob < 1 {
		p.ContextPerJob = 32768
	}
	if p.Trigger != TriggerIssues {
		p.Trigger = TriggerPing
	}
	if p.MaxTurns < 1 {
		p.MaxTurns = 50
	}
	if p.MaxMinutes < 1 {
		p.MaxMinutes = 30
	}
}

func (c Config) Profile(id string) (Profile, bool) {
	for _, p := range c.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// Active returns the profile that should run at now, or false when none should.
func (c Config) Active(now time.Time) (Profile, bool) {
	const day, week = 24 * 60, 7 * 24 * 60
	minute := int(now.Weekday())*day + now.Hour()*60 + now.Minute()

	id := c.Fallback
	for _, s := range c.Segments {
		start, ok1 := minutes(s.Start)
		end, ok2 := minutes(s.End)
		if !ok1 || !ok2 {
			continue
		}
		length := end - start
		if length <= 0 {
			length += day
		}
		for _, d := range s.Days {
			from := d*day + start
			if ((minute-from)%week+week)%week < length {
				id = s.Profile
			}
		}
	}
	if id == "" {
		return Profile{}, false
	}
	return c.Profile(id)
}

func minutes(hhmm string) (int, bool) {
	h, m, ok := strings.Cut(hhmm, ":")
	if !ok {
		return 0, false
	}
	hours, err1 := strconv.Atoi(h)
	mins, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hours < 0 || hours > 23 || mins < 0 || mins > 59 {
		return 0, false
	}
	return hours*60 + mins, true
}
