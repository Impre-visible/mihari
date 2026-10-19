package tools

import (
	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
)

// Conditions offering a tool to a job, for agent.When.

func hasMR(env *agent.Env) bool     { return env.MR.IID != 0 }
func hasTarget(env *agent.Env) bool { return env.Target.IID != 0 }
func reviewing(env *agent.Env) bool {
	return env.Target.Type == gitlab.MergeRequestTarget && hasMR(env)
}
func canPush(env *agent.Env) bool   { return env.Push != nil }
func canOpenMR(env *agent.Env) bool { return env.OpenMR != nil && env.OpenedMR == nil }

// threaded: someone is following the job in a thread, and their replies reach it.
func threaded(env *agent.Env) bool {
	return env.Kind == agent.KindPing && env.ThreadID != "" && hasTarget(env)
}
