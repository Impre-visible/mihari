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

// pinged: someone asked for the job and follows it, in its thread or in the
// comments of the merge request or issue, and their replies reach it.
func pinged(env *agent.Env) bool {
	return env.Kind == agent.KindPing && hasTarget(env)
}
