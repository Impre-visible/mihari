// Package tools holds what the agent can do, one file per tool, named after
// it. Each file registers its tool in init with agent.Register(agent.Define(...)).
//
// Adding a tool:
//  1. internal/tools/<name>.go: its argument struct and its Define call.
//  2. frontend/src/pages/components/chat/tools/<Name>.svelte (PascalCase of
//     the name): how the job's chat shows a call. Without it, the call shows
//     its raw arguments and result.
//
// Helpers several tools share live in shell.go, when.go, gitlab.go and web.go.
package tools
