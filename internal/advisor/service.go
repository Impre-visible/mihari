package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
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
	config, err := store.GetConfig()
	if err != nil {
		return "", err
	}
	var list []map[string]any
	if raw, ok := config["profiles"]; ok && raw != nil {
		data, _ := json.Marshal(raw)
		if err := json.Unmarshal(data, &list); err != nil {
			return "", fmt.Errorf("profils illisibles : %w", err)
		}
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
		target = map[string]any{
			"id":   strconv.FormatInt(time.Now().UnixMilli(), 36) + strconv.FormatInt(rand.Int64N(1<<30), 36),
			"name": r.Model, "prompt": "", "trigger": "ping", "label": "", "maxTurns": 50, "maxMinutes": 30,
		}
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
