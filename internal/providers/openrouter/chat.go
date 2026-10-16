package openrouter

import (
	"context"
	"encoding/json"
	"time"

	"kamski/internal/domain"
)

func (c *Client) Complete(ctx context.Context, r domain.AgentRequest) (domain.AgentResponse, error) {
	p := chatRequest{
		Model:             r.Model,
		Messages:          r.Messages,
		Tools:             r.Tools,
		ToolChoice:        r.Settings.ToolChoice,
		Temperature:       r.Settings.Temperature,
		MaxTokens:         r.Settings.MaxTokens,
		Reasoning:         r.Settings.Reasoning,
		ParallelToolCalls: r.Settings.ParallelToolCalls,
		Provider:          r.Settings.Provider,
	}
	start := time.Now()
	b, err := c.do(ctx, "POST", "/api/v1/chat/completions", p)
	if err != nil {
		return domain.AgentResponse{}, err
	}
	var w chatResponse
	if err := json.Unmarshal(b, &w); err != nil {
		return domain.AgentResponse{}, &domain.Failure{Kind: "protocol_error", Message: "malformed chat response"}
	}
	if len(w.Choices) != 1 {
		return domain.AgentResponse{}, &domain.Failure{Kind: "protocol_error", Message: "expected exactly one candidate choice"}
	}
	ch := w.Choices[0]
	if ch.Message == nil {
		return domain.AgentResponse{}, &domain.Failure{Kind: "protocol_error", Message: "chat response missing assistant message"}
	}
	out := domain.AgentResponse{
		ID:           w.ID,
		Model:        w.Model,
		Provider:     w.Provider,
		FinishReason: ch.FinishReason,
		Text:         ch.Message.Content,
		LatencyMS:    domain.Ptr(time.Since(start).Milliseconds()),
		Usage:        w.Usage.domain(),
		Calls:        []domain.ToolCall{},
	}
	for _, t := range ch.Message.ToolCalls {
		out.Calls = append(out.Calls, domain.ToolCall{ID: t.ID, Type: t.Type, Name: t.Function.Name, Arguments: t.Function.Arguments})
	}
	return out, nil
}
