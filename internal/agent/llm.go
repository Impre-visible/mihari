package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type completion struct {
	Message   message
	Reasoning string
	// Tokens the request took, to size the context.
	PromptTokens int
}

func chat(ctx context.Context, baseURL, model string, messages []message, tools []Tool) (completion, error) {
	specs := make([]map[string]any, len(tools))
	for i, t := range tools {
		specs[i] = map[string]any{"type": "function", "function": map[string]any{
			"name": t.Name(), "description": t.Description(), "parameters": t.Schema(),
		}}
	}
	body, err := json.Marshal(map[string]any{
		"model": model, "messages": messages, "tools": specs, "tool_choice": "auto",
	})
	if err != nil {
		return completion{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return completion{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return completion{}, fmt.Errorf("moteur injoignable : %w", err)
	}
	defer res.Body.Close()

	var out struct {
		Choices []struct {
			Message struct {
				Content          *string    `json:"content"`
				ToolCalls        []toolCall `json:"tool_calls"`
				ReasoningContent string     `json:"reasoning_content"`
				Reasoning        string     `json:"reasoning"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
		} `json:"usage"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return completion{}, fmt.Errorf("réponse illisible du moteur (%s) : %w", res.Status, err)
	}
	if res.StatusCode != http.StatusOK || len(out.Choices) == 0 {
		return completion{}, fmt.Errorf("le moteur a répondu %s : %v", res.Status, out.Error)
	}
	m := out.Choices[0].Message
	msg := message{Role: "assistant", ToolCalls: m.ToolCalls}
	if m.Content != nil {
		msg.Content = *m.Content
	}
	for i := range msg.ToolCalls {
		if msg.ToolCalls[i].ID == "" {
			// Some engines leave ids out; tool results still need one to refer to.
			msg.ToolCalls[i].ID = fmt.Sprintf("call_%d", i)
		}
		msg.ToolCalls[i].Type = "function"
	}
	reasoning := m.ReasoningContent
	if reasoning == "" {
		reasoning = m.Reasoning
	}
	return completion{Message: msg, Reasoning: reasoning, PromptTokens: out.Usage.PromptTokens}, nil
}
