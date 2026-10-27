package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// The conversation is trimmed once it passes the high mark, down to the low
// mark in one go. Trimming rewrites the start of the conversation, which the
// engine then reads again in full instead of from its cache: doing it rarely,
// and by a good margin, keeps the cache useful.
const (
	highMark = 0.85
	lowMark  = 0.60
)

// What a message weighs, in characters, and an image whatever its bytes:
// engines count it in tokens by its pixels, not its encoding.
const imageSize = 4000

func size(messages []message) int {
	n := 0
	for _, m := range messages {
		n += len(m.Content) + len(m.ReasoningContent) + len(m.Images)*imageSize
		for _, c := range m.ToolCalls {
			n += len(c.Function.Arguments) + len(c.Function.Name)
		}
	}
	return n
}

// signature identifies a tool call by its name and arguments, whatever their
// spacing or key order.
func signature(name, arguments string) string {
	var v any
	if json.Unmarshal([]byte(arguments), &v) == nil {
		if data, err := json.Marshal(v); err == nil {
			arguments = string(data)
		}
	}
	return name + " " + arguments
}

// stub replaces a tool result taken out of the conversation, and says which
// call it was so the model knows what it no longer has.
func stub(name string) string {
	return fmt.Sprintf("[Résultat de %s retiré pour libérer du contexte. Refais l'appel seulement si tu en as encore besoin.]", name)
}

func isStub(content string) bool {
	return strings.HasPrefix(content, "[Résultat de ") && strings.Contains(content, "retiré pour libérer du contexte")
}

// fitContext trims the conversation once it passes highMark of the window,
// down to lowMark, losing first what is worth least: earlier thinking, then
// results a later identical call superseded, then the oldest results and
// images. The prompt, the task and the latest exchanges stay whole.
func fitContext(messages []message, contextTokens int, charsPerToken float64) []message {
	if contextTokens <= 0 || size(messages) <= int(float64(contextTokens)*highMark*charsPerToken) {
		return messages
	}
	budget := int(float64(contextTokens) * lowMark * charsPerToken)
	keepFrom := max(2, len(messages)-4)
	over := func() bool { return size(messages) > budget }

	// Which call each result answers, and the last time each call was made.
	calls := map[string]toolCall{}
	last := map[string]int{}
	for i, m := range messages {
		for _, c := range m.ToolCalls {
			calls[c.ID] = c
			last[signature(c.Function.Name, c.Function.Arguments)] = i
		}
	}
	trimResult := func(i int) {
		m := &messages[i]
		if m.Role != "tool" || isStub(m.Content) {
			return
		}
		m.Content = stub(calls[m.ToolCallID].Function.Name)
	}

	for i := 2; i < keepFrom && over(); i++ {
		messages[i].ReasoningContent = ""
	}
	for i := 2; i < keepFrom && over(); i++ {
		if m := messages[i]; m.Role == "tool" {
			c := calls[m.ToolCallID]
			if j, ok := last[signature(c.Function.Name, c.Function.Arguments)]; ok && j > i {
				trimResult(i)
			}
		}
	}
	for i := 2; i < keepFrom && over(); i++ {
		if m := &messages[i]; len(m.Images) > 0 {
			m.Images, m.Content = nil, m.Content+" [image retirée pour libérer du contexte]"
		}
		trimResult(i)
	}
	return messages
}
