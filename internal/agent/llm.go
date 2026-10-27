package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	// The model's thinking before this answer, sent back on later turns when
	// Config.KeepReasoning is set; "" otherwise.
	ReasoningContent string `json:"reasoning_content,omitempty"`
	// Data URLs sent along with Content, as OpenAI-style content parts.
	Images []string `json:"-"`
}

// MarshalJSON sends a message with images as a list of content parts, the
// only form that carries them.
func (m message) MarshalJSON() ([]byte, error) {
	type plain message
	if len(m.Images) == 0 {
		return json.Marshal(plain(m))
	}
	parts := []map[string]any{{"type": "text", "text": m.Content}}
	for _, url := range m.Images {
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": url}})
	}
	return json.Marshal(struct {
		plain
		Content []map[string]any `json:"content"`
	}{plain(m), parts})
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
	// Tokens the request took, to size the context, and the answer's.
	PromptTokens     int
	CompletionTokens int
}

// session is the same for every call of a job: remote APIs such as OpenCode Go
// route and cache prompts by conversation.
func chat(ctx context.Context, baseURL, model, session string, messages []message, tools []Tool) (completion, error) {
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
	req.Header.Set("X-Opencode-Session", session)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return completion{}, err
		}
		return completion{}, transientError{fmt.Errorf("moteur injoignable : %w", err)}
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		// Remote APIs and the proxy in front of them drop a request now and then.
		data, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		msg := fmt.Sprintf("le moteur a répondu %s", res.Status)
		if body := strings.TrimSpace(string(data)); body != "" {
			msg += " : " + body
		}
		return completion{}, transientError{fmt.Errorf("%s", msg)}
	}

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
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
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
	return completion{Message: msg, Reasoning: reasoning, PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens}, nil
}

// transientError is a failure of the engine worth trying again.
type transientError struct{ error }

func (e transientError) Unwrap() error { return e.error }
