package openrouter

import (
	"encoding/json"
	"kamski/internal/domain"
)

// Wire DTOs stay here, not in the public result format.
type chatRequest struct {
	Model             string                `json:"model"`
	Messages          []domain.Message      `json:"messages"`
	Tools             []domain.Tool         `json:"tools"`
	ToolChoice        string                `json:"tool_choice"`
	Stream            bool                  `json:"stream"`
	Temperature       float64               `json:"temperature"`
	MaxTokens         int                   `json:"max_tokens"`
	Reasoning         map[string]any        `json:"reasoning,omitempty"`
	ParallelToolCalls *bool                 `json:"parallel_tool_calls,omitempty"`
	Provider          domain.ProviderPolicy `json:"provider"`
}
type wireUsage struct {
	PromptTokens     *int     `json:"prompt_tokens"`
	CompletionTokens *int     `json:"completion_tokens"`
	InputTokens      *int     `json:"input_tokens"`
	OutputTokens     *int     `json:"output_tokens"`
	Cost             *float64 `json:"cost"`
}

func (u *wireUsage) domain() *domain.Usage {
	if u == nil {
		return nil
	}
	in, out := u.InputTokens, u.OutputTokens
	if in == nil {
		in = u.PromptTokens
	}
	if out == nil {
		out = u.CompletionTokens
	}
	return &domain.Usage{InputTokens: in, OutputTokens: out, Cost: u.Cost}
}

type chatResponse struct {
	ID       *string    `json:"id"`
	Model    *string    `json:"model"`
	Provider *string    `json:"provider"`
	Usage    *wireUsage `json:"usage"`
	Choices  []struct {
		FinishReason *string `json:"finish_reason"`
		Message      *struct {
			Content          *string `json:"content"`
			Reasoning        *string `json:"reasoning"`
			ReasoningContent *string `json:"reasoning_content"`
			ReasoningDetails []struct {
				Type    string  `json:"type"`
				Text    *string `json:"text"`
				Summary *string `json:"summary"`
			} `json:"reasoning_details"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

// Alpha Decisions DTOs deliberately isolated and fixture-tested.
type ScoreQuestion struct {
	Type         string   `json:"type"`
	Instructions string   `json:"instructions"`
	Criteria     []string `json:"criteria"`
}
type DecisionsRequest struct {
	Model     string                   `json:"model"`
	State     any                      `json:"state"`
	Questions map[string]ScoreQuestion `json:"questions"`
	Provider  domain.ProviderPolicy    `json:"provider"`
}
type ScoreAnswer struct {
	Type          string              `json:"type"`
	Score         *float64            `json:"score"`
	Probabilities map[string]*float64 `json:"probabilities"`
	Confidence    *float64            `json:"confidence"`
	Legend        map[string]string   `json:"legend"`
}
type DecisionsResponse struct {
	ID       *string                `json:"id"`
	Model    *string                `json:"model"`
	Provider *string                `json:"provider"`
	Answers  map[string]ScoreAnswer `json:"answers"`
	Usage    *wireUsage             `json:"usage"`
}

func (r DecisionsResponse) UsageData() *domain.Usage { return r.Usage.domain() }

type errorEnvelope struct {
	Error *struct {
		Code     any             `json:"code"`
		Message  string          `json:"message"`
		Type     string          `json:"type"`
		Metadata json.RawMessage `json:"metadata"`
	} `json:"error"`
}
