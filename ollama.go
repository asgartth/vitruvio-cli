package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Cliente OpenAI-compatível (/v1/chat/completions) — funciona com Ollama E llama.cpp
// (llama-server), então o "swap" de engine é só trocar --host.

type ToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // OpenAI: JSON como STRING
	} `json:"function"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// listarModelos consulta os modelos disponíveis na engine (/v1/models).
func listarModelos(host string) []string {
	cli := &http.Client{Timeout: 10 * time.Second}
	r, err := cli.Get(host + "/v1/models")
	if err != nil {
		return nil
	}
	defer r.Body.Close()
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.NewDecoder(r.Body).Decode(&resp) != nil {
		return nil
	}
	var ids []string
	for _, d := range resp.Data {
		ids = append(ids, d.ID)
	}
	return ids
}

// chatTools: uma rodada não-streaming com ferramentas disponíveis.
func chatTools(host, model string, msgs []Message, tools []any) (Message, Usage, error) {
	return chatToolsCtx(context.Background(), host, model, msgs, tools)
}

// chatToolsCtx: idem, mas cancelável por contexto (ESC no TUI aborta a requisição).
func chatToolsCtx(ctx context.Context, host, model string, msgs []Message, tools []any) (Message, Usage, error) {
	payload := map[string]any{"model": model, "messages": msgs, "stream": false}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", host+"/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return Message{}, Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 15 * time.Minute}
	r, err := cli.Do(req)
	if err != nil {
		return Message{}, Usage{}, err
	}
	defer r.Body.Close()
	var resp struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
		Error any   `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		return Message{}, Usage{}, err
	}
	if len(resp.Choices) == 0 {
		return Message{}, Usage{}, fmt.Errorf("resposta vazia da engine (%v)", resp.Error)
	}
	return resp.Choices[0].Message, resp.Usage.norm(), nil
}
