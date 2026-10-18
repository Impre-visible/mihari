package jobs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/projects"
	"github.com/Impre-visible/mihari/internal/sandbox"
	"github.com/Impre-visible/mihari/internal/scheduler"
)

// Labels an issues job leaves on its issue.
const (
	LabelDoing   = "mihari::doing"
	LabelDone    = "mihari::done"
	LabelFailed  = "mihari::failed"
	LabelRefused = "mihari::refused"
)

// run carries a job from clone to the last GitLab update, whatever happens.
func (s *Service) run(ctx context.Context, j *job, active scheduler.Active) {
	env, cleanup, err := s.prepare(ctx, j, active)
	defer cleanup()

	var outcome agent.Outcome
	if err == nil {
		inbox := make(chan agent.Event, 16)
		watchCtx, stopWatch := context.WithCancel(ctx)
		go s.watchComments(watchCtx, j, env, inbox)

		p := active.Profile
		outcome, err = agent.Run(ctx, agent.Config{
			URL: active.URL, Model: active.Model, MaxTurns: p.MaxTurns,
			Timeout: time.Duration(p.MaxMinutes) * time.Minute, ContextTokens: p.ContextPerJob,
		}, env, j.systemPrompt, j.taskPrompt, inbox, func(e agent.Event) { s.record(j, e) })
		stopWatch()
		s.mu.Lock()
		j.Turns = outcome.Turns
		s.mu.Unlock()
	}
	// The job's own time limit is internal to the loop; ctx ends only on Cancel or shutdown.
	s.finish(j, env, outcome, err, ctx.Err() != nil)
}

func (s *Service) prepare(ctx context.Context, j *job, active scheduler.Active) (*agent.Env, func(), error) {
	var cleanups []func()
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	target := j.target()

	client, err := gitlab.NewClient(j.ProjectID)
	if err != nil {
		return nil, cleanup, err
	}
	env := &agent.Env{Kind: j.Kind, GitLab: client, Target: target, Requester: j.Author}

	switch target.Type {
	case gitlab.MergeRequestTarget:
		if env.MR, err = client.MergeRequest(ctx, target.IID); err != nil {
			return nil, cleanup, err
		}
	case gitlab.IssueTarget:
		issue, err := client.Issue(ctx, target.IID)
		if err != nil {
			return nil, cleanup, err
		}
		env.Issue = &issue
	}

	if j.Kind == agent.KindIssues {
		// Visible to the team, and how a restart recognizes an interrupted issue.
		remove := []string{j.label, LabelDone, LabelFailed, LabelRefused}
		if err := client.Relabel(ctx, target.IID, []string{LabelDoing}, remove); err != nil {
			return env, cleanup, fmt.Errorf("pose du label %s : %w", LabelDoing, err)
		}
	} else {
		if id, err := client.React(ctx, target, "eyes"); err == nil {
			j.awardID = id
		} else {
			s.emit(j, "info", "GitLab", "Impossible d'ajouter 👀 : "+err.Error())
		}
		if j.todo.NoteID != 0 {
			s.findThread(ctx, j, env)
		}
	}

	config, err := projects.Load()
	if err != nil {
		return env, cleanup, err
	}
	dir, err := sandbox.NewWorkspace()
	if err != nil {
		return env, cleanup, err
	}
	cleanups = append(cleanups, func() { os.RemoveAll(dir) })
	repo := filepath.Join(dir, "repo")

	path, err := client.ProjectPath(ctx)
	if err != nil {
		return env, cleanup, err
	}

	// The branch pushes go to: the merge request's own, or a new one for an issue.
	var branch, base string
	switch {
	case target.Type == gitlab.MergeRequestTarget:
		mr := env.MR
		s.emit(j, "info", "Préparation", "Clone de "+path+" ("+mr.SourceBranch+")…")
		if mr.FromFork() {
			err = gitlab.Clone(ctx, path, mr.TargetBranch, repo)
			if err == nil {
				err = gitlab.CheckoutMergeRequest(ctx, repo, mr.IID)
			}
		} else {
			err = gitlab.Clone(ctx, path, mr.SourceBranch, repo)
			branch = mr.SourceBranch
		}
	default:
		if base, err = client.DefaultBranch(ctx); err != nil {
			return env, cleanup, err
		}
		s.emit(j, "info", "Préparation", "Clone de "+path+" ("+base+")…")
		if err = gitlab.Clone(ctx, path, base, repo); err == nil {
			if branch, err = freeBranch(ctx, client, env.Issue); err == nil {
				err = gitlab.NewBranch(ctx, repo, branch)
			}
		}
	}
	if err != nil {
		return env, cleanup, err
	}

	image := config.Image(j.ProjectID)
	s.emit(j, "info", "Préparation", "Démarrage du container "+image+"…")
	container, err := sandbox.Start(ctx, sandbox.Spec{Image: image, Workspace: repo})
	if err != nil {
		return env, cleanup, err
	}
	cleanups = append(cleanups, func() { _ = container.Close() })
	env.Runner = container

	if branch != "" {
		pushed := false
		env.Push = func(ctx context.Context, message string) (string, error) {
			sha, err := s.push(ctx, client, container, repo, branch, message, j.Author)
			if err == nil {
				pushed = true
			}
			return sha, err
		}
		if env.Issue != nil {
			issue := env.Issue
			env.OpenMR = func(ctx context.Context, title, description string) (gitlab.MergeRequest, error) {
				_, err := s.push(ctx, client, container, repo, branch, title, j.Author)
				if err != nil && !(pushed && strings.Contains(err.Error(), "aucun changement")) {
					if strings.Contains(err.Error(), "aucun changement") {
						return gitlab.MergeRequest{}, fmt.Errorf("aucune modification à proposer : change des fichiers avant d'ouvrir la MR")
					}
					return gitlab.MergeRequest{}, err
				}
				pushed = true
				var reviewers []int
				if id, err := client.UserID(ctx, j.Author); err == nil && j.Author != client.Bot {
					reviewers = []int{id}
				}
				description = strings.TrimSpace(description) + "\n\nPour #" + strconv.Itoa(issue.IID)
				mr, err := client.OpenMergeRequest(ctx, branch, base, title, description, reviewers)
				if err == nil {
					s.mu.Lock()
					j.OpenedMR = mr.WebURL
					s.mu.Unlock()
				}
				return mr, err
			}
		}
	}

	j.systemPrompt = systemPrompt(active.Profile.Prompt, config.Project(j.ProjectID).Instructions, readMihariMD(repo))
	j.taskPrompt = taskPrompt(j, env)
	return env, cleanup, nil
}

// findThread locates the thread of the mention that started a ping job.
func (s *Service) findThread(ctx context.Context, j *job, env *agent.Env) {
	s.emit(j, "info", "GitLab", "Recherche du fil de la mention…")
	discussions, err := env.GitLab.Discussions(ctx, env.Target)
	if err != nil {
		return
	}
	if d, ok := gitlab.DiscussionOf(discussions, j.todo.NoteID); ok {
		s.mu.Lock()
		j.threadID = d.ID
		for _, n := range d.Notes {
			j.seen[n.ID] = true
		}
		s.mu.Unlock()
		env.ThreadID = d.ID
	}
}

// push commits the workspace as the bot, crediting who asked, and pushes it.
func (s *Service) push(ctx context.Context, client *gitlab.Client, c *sandbox.Container, repo, branch, message, requester string) (string, error) {
	c.Chown(ctx)
	bot, err := client.Identity(ctx, client.Bot)
	if err != nil {
		return "", err
	}
	if who, err := client.Identity(ctx, requester); err == nil && requester != client.Bot {
		message = strings.TrimSpace(message) + fmt.Sprintf("\n\nCo-authored-by: %s <%s>", who.Name, who.Email)
	}
	return gitlab.Push(ctx, repo, branch, message, bot)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// freeBranch names the branch of an issue's work, suffixed when an earlier
// attempt left one.
func freeBranch(ctx context.Context, client *gitlab.Client, issue *gitlab.Issue) (string, error) {
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(issue.Title), "-"), "-")
	if len(slug) > 40 {
		slug = strings.TrimRight(slug[:40], "-")
	}
	name := "mihari/issue-" + strconv.Itoa(issue.IID)
	if slug != "" {
		name += "-" + slug
	}
	candidate := name
	for n := 2; ; n++ {
		exists, err := client.BranchExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = name + "-" + strconv.Itoa(n)
	}
}

// watchComments passes new comments to the agent, from people allowed to call
// the bot: replies in the mention's thread for a ping, any comment on the
// issue for an issues job.
func (s *Service) watchComments(ctx context.Context, j *job, env *agent.Env, inbox chan<- agent.Event) {
	if j.Kind == agent.KindPing && env.ThreadID == "" {
		return
	}
	if j.Kind == agent.KindIssues {
		// What is already there is in the issue the agent reads.
		if discussions, err := env.GitLab.Discussions(ctx, env.Target); err == nil {
			s.markSeen(j, discussions)
		}
	}
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		var discussions []gitlab.Discussion
		if j.Kind == agent.KindIssues {
			discussions, _ = env.GitLab.Discussions(ctx, env.Target)
		} else if d, err := env.GitLab.Discussion(ctx, env.Target, env.ThreadID); err == nil {
			discussions = []gitlab.Discussion{d}
		}
		config, err := projects.Load()
		if err != nil {
			continue
		}
		var members []gitlab.Member
		for _, d := range discussions {
			for _, n := range d.Notes {
				s.mu.Lock()
				seen := j.seen[n.ID]
				j.seen[n.ID] = true
				s.mu.Unlock()
				if seen || n.Author == env.GitLab.Bot {
					continue
				}
				if members == nil {
					if members, err = gitlab.ProjectMembers(ctx, j.ProjectID); err != nil {
						continue
					}
				}
				if !config.Project(j.ProjectID).Authorizes(n.Author, members) {
					continue
				}
				select {
				case inbox <- agent.Event{At: time.Now(), Kind: "message", Title: "Nouveau message de @" + n.Author, Text: n.Body}:
				default:
				}
			}
		}
	}
}

func (s *Service) markSeen(j *job, discussions []gitlab.Discussion) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range discussions {
		for _, n := range d.Notes {
			j.seen[n.ID] = true
		}
	}
}

// finish reports the outcome on GitLab, closes the todos and saves the job.
func (s *Service) finish(j *job, env *agent.Env, outcome agent.Outcome, runErr error, canceled bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	state, reason := StateSucceeded, ""
	switch {
	case canceled:
		state, reason = StateCanceled, "Job annulé depuis Mihari."
	case runErr != nil:
		state, reason = StateFailed, runErr.Error()
	case j.Kind == agent.KindIssues && (env == nil || env.OpenedMR == nil):
		// For an issue, the work is a merge request; anything else is a failure.
		state, reason = StateFailed, "aucune MR ouverte"
		if outcome.Summary != "" {
			reason += " : " + outcome.Summary
		}
	case !outcome.Success && j.Kind == agent.KindPing:
		state, reason = StateFailed, outcome.Summary
	}

	if env != nil && env.GitLab != nil {
		if j.Kind == agent.KindIssues {
			s.finishIssue(ctx, j, env, state, reason, outcome)
		} else {
			s.finishPing(ctx, j, env, state, reason, outcome)
		}
	}

	s.mu.Lock()
	ids := append([]int{}, j.todoIDs...)
	s.mu.Unlock()
	s.closeTodos(ids)

	s.mu.Lock()
	j.State, j.EndedAt, j.Error, j.Result = state, time.Now(), reason, outcome.Summary
	s.mu.Unlock()
	kind := "info"
	if state != StateSucceeded {
		kind = "error"
	}
	s.emit(j, kind, "Fin", strings.TrimSpace(stateLabel(state)+" "+reason))
	s.persist(j)
	s.publish()
	// A drained profile can switch now; the next queued job can start.
	s.Scheduler.Nudge()
	s.poke()
}

func (s *Service) finishPing(ctx context.Context, j *job, env *agent.Env, state State, reason string, outcome agent.Outcome) {
	// The model's summary is the answer when it forgot to post one.
	if state == StateSucceeded && !env.Replied && outcome.Summary != "" {
		s.post(ctx, env, outcome.Summary)
	}
	if state != StateSucceeded {
		msg := "❌ Je n'ai pas pu terminer : " + reason
		if state == StateCanceled {
			msg = "❌ " + reason
		}
		s.post(ctx, env, msg)
	}
	if j.awardID != 0 {
		_ = env.GitLab.Unreact(ctx, env.Target, j.awardID)
	}
	emoji := "white_check_mark"
	if state != StateSucceeded {
		emoji = "x"
	}
	_, _ = env.GitLab.React(ctx, env.Target, emoji)
}

func (s *Service) finishIssue(ctx context.Context, j *job, env *agent.Env, state State, reason string, outcome agent.Outcome) {
	label := LabelDone
	var msg string
	if state == StateSucceeded {
		msg = "✅ MR ouverte : " + env.OpenedMR.WebURL
		if outcome.Summary != "" {
			msg += "\n\n" + outcome.Summary
		}
	} else {
		label = LabelFailed
		msg = "❌ Je n'ai pas pu terminer : " + reason
		if env.OpenedMR != nil {
			msg += "\n\nMR ouverte en chemin : " + env.OpenedMR.WebURL
		}
	}
	if err := env.GitLab.Relabel(ctx, env.Target.IID, []string{label}, []string{LabelDoing}); err != nil {
		log.Printf("relabel #%d: %v", env.Target.IID, err)
	}
	if _, err := env.GitLab.Comment(ctx, env.Target, msg); err != nil {
		log.Printf("comment #%d: %v", env.Target.IID, err)
	}
}

func (s *Service) post(ctx context.Context, env *agent.Env, body string) {
	var err error
	if env.ThreadID != "" {
		_, err = env.GitLab.Reply(ctx, env.Target, env.ThreadID, body)
	} else {
		_, err = env.GitLab.Comment(ctx, env.Target, body)
	}
	if err != nil {
		log.Printf("post on %s: %v", env.Target.Ref(), err)
	}
}

func stateLabel(s State) string {
	switch s {
	case StateSucceeded:
		return "Terminé."
	case StateCanceled:
		return "Annulé."
	default:
		return "Échec :"
	}
}
