package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Impre-visible/mihari/internal/devmode"
)

type ChatResult struct {
	Text string `json:"text"`
	// What a thinking model wrote before its answer, when the engine returns it apart.
	Reasoning        string `json:"reasoning"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	DurationMs       int64  `json:"durationMs"`
}

// Chat sends message to the running engine, optionally behind the active
// profile's prompt as the system message, to check the model answers.
func (s *Service) Chat(message string, withProfilePrompt bool) (ChatResult, error) {
	if !devmode.Enabled {
		return ChatResult{}, devmode.ErrDisabled
	}
	s.mu.Lock()
	ready := s.snapshot.State == StateReady
	url, model := s.snapshot.URL, s.snapshot.Model
	var prompt string
	if s.current != nil {
		prompt = s.current.Prompt
	}
	s.mu.Unlock()
	if !ready {
		return ChatResult{}, errors.New("aucun moteur prêt : vérifie la page État")
	}

	messages := []map[string]string{}
	if withProfilePrompt && prompt != "" {
		messages = append(messages, map[string]string{"role": "system", "content": prompt})
	}
	messages = append(messages, map[string]string{"role": "user", "content": message})
	body, err := json.Marshal(map[string]any{"model": model, "messages": messages})
	if err != nil {
		return ChatResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	begin := time.Now()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ChatResult{}, err
	}
	defer res.Body.Close()

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				// llama-server says reasoning_content, Ollama says reasoning.
				ReasoningContent string `json:"reasoning_content"`
				Reasoning        string `json:"reasoning"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return ChatResult{}, fmt.Errorf("réponse illisible du moteur (%s) : %w", res.Status, err)
	}
	if res.StatusCode != http.StatusOK || len(out.Choices) == 0 {
		return ChatResult{}, fmt.Errorf("le moteur a répondu %s : %v", res.Status, out.Error)
	}
	msg := out.Choices[0].Message
	reasoning := msg.ReasoningContent
	if reasoning == "" {
		reasoning = msg.Reasoning
	}
	return ChatResult{
		Text:             msg.Content,
		Reasoning:        reasoning,
		PromptTokens:     out.Usage.PromptTokens,
		CompletionTokens: out.Usage.CompletionTokens,
		DurationMs:       time.Since(begin).Milliseconds(),
	}, nil
}
