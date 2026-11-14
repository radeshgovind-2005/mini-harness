package fakemodel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Ollama talks to a local Ollama server (POST /api/chat, no streaming).
// It implements Model, so callers can swap it in for Scripted unchanged.
// Default tests never use it against a real server.
type Ollama struct {
	BaseURL string       // e.g. "http://localhost:11434"
	ModelID string       // e.g. "llama3.2"
	Client  *http.Client // nil means http.DefaultClient
}

var _ Model = (*Ollama)(nil)

type ollamaMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolCalls []struct {
		Function struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls,omitempty"`
}

// Complete sends the conversation and maps the reply to a Response.
func (o *Ollama) Complete(ctx context.Context, msgs []Message) (Response, error) {
	req := struct {
		Model    string          `json:"model"`
		Messages []ollamaMessage `json:"messages"`
		Stream   bool            `json:"stream"`
	}{Model: o.ModelID}
	for _, m := range msgs {
		req.Messages = append(req.Messages, ollamaMessage{Role: m.Role, Content: m.Content})
	}
	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := o.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("ollama: unexpected status %s", resp.Status)
	}
	var out struct {
		Message ollamaMessage `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("ollama: decode reply: %w", err)
	}
	r := Response{Text: out.Message.Content}
	for _, tc := range out.Message.ToolCalls {
		args := map[string]string{}
		for k, v := range tc.Function.Arguments {
			if s, ok := v.(string); ok {
				args[k] = s
			} else {
				args[k] = fmt.Sprint(v) // numbers, bools: keep a readable form
			}
		}
		r.ToolCalls = append(r.ToolCalls, ToolCall{Name: tc.Function.Name, Args: args})
	}
	return r, nil
}
