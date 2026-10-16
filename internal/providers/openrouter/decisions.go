package openrouter

import (
	"context"
	"encoding/json"
	"kamski/internal/domain"
)

func (c *Client) Decide(ctx context.Context, r DecisionsRequest) (DecisionsResponse, json.RawMessage, error) {
	b, err := c.do(ctx, "POST", "/api/alpha/decisions", r)
	if err != nil {
		return DecisionsResponse{}, nil, err
	}
	var out DecisionsResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return out, b, &domain.Failure{Kind: "protocol_error", Message: "malformed Decisions response"}
	}
	return out, b, nil
}
