package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/engine"
	"github.com/Impre-visible/mihari/internal/settings"
)

// Service is exposed to the frontend through Wails bindings.
type Service struct {
	// Restarts the engine when the active profile changed; set before startup.
	Reload func()
}

type Overview struct {
	Hardware Hardware `json:"hardware"`
	Budget   Budget   `json:"budget"`
	// The engine recommendations use here, and whether it is installed.
	Engine engine.Info `json:"engine"`
}

// Machine describes the hardware and how much of it models may use.
func (s *Service) Machine() Overview {
	h := Detect(context.Background())
	kind := engine.LlamaServer
	if h.AppleSilicon() {
		kind = engine.VLLMMLX
	}
	var info engine.Info
	for _, e := range engine.Catalog() {
		if e.Kind == kind {
			info = e
		}
	}
	return Overview{Hardware: h, Budget: budgetOf(h), Engine: info}
}

// Recommend lists, best first, the models that fit jobs parallel jobs of
// contextPerJob tokens each.
func (s *Service) Recommend(jobs, contextPerJob int) ([]Recommendation, error) {
	if jobs < 1 || contextPerJob < 1024 {
		return nil, errors.New("il faut au moins 1 job et 1024 tokens de contexte")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	recs := Recommend(ctx, Detect(ctx), jobs, contextPerJob)
	if len(recs) == 0 && ctx.Err() == nil {
		// Either nothing fits, or Hugging Face did not answer.
		if _, err := engine.RemoteSize(ctx, engine.LlamaServer, Catalog[len(Catalog)-1].GGUF); err != nil {
			return nil, fmt.Errorf("tailles des modèles illisibles : %w", err)
		}
	}
	return recs, nil
}

// Apply writes a recommendation into a profile, or into a new one when
// profileID is "", and returns the profile's id. The prompt is left alone.
func (s *Service) Apply(profileID string, r Recommendation, jobs, contextPerJob int) (string, error) {
	store := &settings.Service{}
	list, err := storedProfiles(store)
	if err != nil {
		return "", err
	}

	var target map[string]any
	for _, p := range list {
		if p["id"] == profileID {
			target = p
		}
	}
	if target == nil {
		if profileID != "" {
			return "", errors.New("profil introuvable")
		}
		// Same shape and defaults as a profile made on the Profiles page.
		target = newProfile(r.Model)
		list = append(list, target)
	}
	target["engine"] = string(r.Engine)
	target["model"] = r.Ref
	target["maxJobs"] = jobs
	target["contextPerJob"] = contextPerJob
	target["extraArgs"] = r.ExtraArgs

	if err := store.Set("profiles", list); err != nil {
		return "", err
	}
	if s.Reload != nil {
		s.Reload()
	}
	return target["id"].(string), nil
}

func storedProfiles(store *settings.Service) ([]map[string]any, error) {
	config, err := store.GetConfig()
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if raw, ok := config["profiles"]; ok && raw != nil {
		data, _ := json.Marshal(raw)
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("profils illisibles : %w", err)
		}
	}
	return list, nil
}

// newProfile has the same shape and defaults as a profile made on the Profiles page.
func newProfile(name string) map[string]any {
	return map[string]any{
		"id":   strconv.FormatInt(time.Now().UnixMilli(), 36) + strconv.FormatInt(rand.Int64N(1<<30), 36),
		"name": name, "prompt": "", "trigger": "ping", "label": "", "maxTurns": 50, "maxMinutes": 30,
	}
}

// Onboarding is what the first launch shows of the machine.
type Onboarding struct {
	Hardware Hardware `json:"hardware"`
	// The Apple Silicon chip, such as "Apple M5 Ultra"; "" on any other machine.
	Chip string `json:"chip"`
	// The day and night setup for this machine; nil sends the user to the generic advisor.
	Preset *Preset     `json:"preset"`
	Engine engine.Info `json:"engine"`
}

// Onboarding detects the machine and the preset that suits it, if any.
func (s *Service) Onboarding() Onboarding {
	o := s.Machine()
	out := Onboarding{Hardware: o.Hardware, Engine: o.Engine}
	if c, ok := ParseChip(o.Hardware.CPU); ok && o.Hardware.AppleSilicon() {
		out.Chip = c.String()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if p, ok := presetFor(ctx, o.Hardware); ok {
		out.Preset = &p
	}
	return out
}

// Label of the issues the night profile turns into tests.
const NightLabel = "mihari::tests"

// ApplyPreset writes this machine's preset into a Jour and a Nuit profile, made
// or updated by name, and replaces the schedule: Jour on weekdays from 08:30
// to 17:00, Nuit every other hour. A prompt already set is kept.
func (s *Service) ApplyPreset(dayPrompt, nightPrompt string) (Preset, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	p, ok := presetFor(ctx, Detect(ctx))
	if !ok {
		return Preset{}, errors.New("aucun préréglage pour cette machine")
	}
	return p, s.applyPreset(p, dayPrompt, nightPrompt)
}

func (s *Service) applyPreset(p Preset, dayPrompt, nightPrompt string) error {
	store := &settings.Service{}
	list, err := storedProfiles(store)
	if err != nil {
		return err
	}
	ids := map[string]string{}
	for _, slot := range []struct {
		name, prompt, trigger, label string
		slot                         Slot
	}{
		{"Jour", dayPrompt, "ping", "", p.Day},
		{"Nuit", nightPrompt, "issues", NightLabel, p.Night},
	} {
		var target map[string]any
		for _, existing := range list {
			if name, _ := existing["name"].(string); strings.EqualFold(strings.TrimSpace(name), slot.name) {
				target = existing
			}
		}
		if target == nil {
			target = newProfile(slot.name)
			target["trigger"], target["label"] = slot.trigger, slot.label
			list = append(list, target)
		}
		if prompt, _ := target["prompt"].(string); strings.TrimSpace(prompt) == "" {
			target["prompt"] = slot.prompt
		}
		target["engine"] = string(p.Engine)
		target["model"] = slot.slot.Model
		target["maxJobs"] = slot.slot.Jobs
		target["contextPerJob"] = slot.slot.ContextPerJob
		target["extraArgs"] = p.ExtraArgs
		ids[slot.name] = target["id"].(string)
	}

	weekdays := []int{1, 2, 3, 4, 5}
	everyDay := []int{1, 2, 3, 4, 5, 6, 0}
	schedule := []map[string]any{
		{"profile": ids["Jour"], "start": "08:30", "end": "17:00", "days": weekdays},
		{"profile": ids["Nuit"], "start": "17:00", "end": "08:30", "days": everyDay},
	}
	if err := store.Set("profiles", list); err != nil {
		return err
	}
	if err := store.Set("schedule", schedule); err != nil {
		return err
	}
	// Weekend days, which no segment covers, also go to the night profile.
	if err := store.Set("schedule_fallback", ids["Nuit"]); err != nil {
		return err
	}
	if s.Reload != nil {
		s.Reload()
	}
	return nil
}
