package jobs

import (
	"context"
	"log"
	"slices"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/profiles"
	"github.com/Impre-visible/mihari/internal/projects"
)

// watchIssues looks for labelled issues every pollEvery while an issues
// profile is active and its engine ready.
func (s *Service) watchIssues(ctx context.Context) {
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		active := s.Scheduler.Active()
		if active.Ready && !active.Draining && active.Profile.Trigger == profiles.TriggerIssues && active.Profile.Label != "" {
			s.scanIssues(ctx, active.Profile.Label)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// scanIssues queues the issues of every enabled project that carry label,
// and those mihari::doing left by an interrupted run.
func (s *Service) scanIssues(ctx context.Context, label string) {
	config, err := projects.Load()
	if err != nil {
		log.Printf("load projects: %v", err)
		return
	}
	queuedAny := false
	for _, p := range config.Projects {
		if !p.Enabled {
			continue
		}
		client, err := gitlab.NewClient(p.ID)
		if err != nil {
			return
		}
		issues, err := client.OpenIssues(ctx, label, LabelDoing)
		if err != nil {
			log.Printf("issues of %s: %v", p.Path, err)
			continue
		}
		var members []gitlab.Member
		for _, issue := range issues {
			if s.hasIssueJob(p.ID, issue.IID) {
				continue
			}
			events, err := client.LabelEvents(ctx, issue.IID)
			if err != nil {
				continue
			}
			// Interrupted or not, the request is whoever put the profile's label.
			put, ok := gitlab.LastAdded(events, label)
			if !ok {
				continue
			}
			if members == nil {
				if members, err = gitlab.ProjectMembers(ctx, p.ID); err != nil {
					break
				}
			}
			if !p.Authorizes(put.User, members) {
				// Refused in silence, like a ping: the label says why.
				if err := client.Relabel(ctx, issue.IID, []string{LabelRefused}, []string{label, LabelDoing}); err != nil {
					log.Printf("refuse #%d: %v", issue.IID, err)
				}
				continue
			}
			s.enqueue(&job{
				Summary: Summary{
					Kind: agent.KindIssues, TargetType: gitlab.IssueTarget, Project: p.Path, ProjectID: p.ID,
					IID: issue.IID, Title: issue.Title, URL: issue.WebURL, Author: put.User,
				},
				label:   label,
				labelAt: put.At,
			})
			queuedAny = true
		}
	}
	if queuedAny {
		s.publish()
		s.poke()
	}
}

func (s *Service) hasIssueJob(projectID, iid int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.jobs, func(j *job) bool {
		return !j.finished() && j.ProjectID == projectID && j.TargetType == gitlab.IssueTarget && j.IID == iid
	})
}
