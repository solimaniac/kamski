package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"kamski/internal/domain"
)

type ModelInfo struct {
	ID           string   `json:"id"`
	Supported    []string `json:"supported_parameters"`
	Architecture struct {
		Output []string `json:"output_modalities"`
	} `json:"architecture"`
}

func (c *Client) Models(ctx context.Context, filter string) ([]ModelInfo, error) {
	path := "/api/v1/models"
	if filter != "" {
		path += "?" + filter
	}
	seen := map[string]bool{}
	all := []ModelInfo{}
	for pages := 0; path != ""; pages++ {
		if pages >= 100 || seen[path] {
			return nil, fmt.Errorf("invalid model pagination")
		}
		seen[path] = true
		b, err := c.do(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}
		var w struct {
			Data  []ModelInfo `json:"data"`
			Next  *string     `json:"next"`
			Links struct {
				Next *string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(b, &w); err != nil {
			return nil, fmt.Errorf("model discovery: %w", err)
		}
		if w.Data == nil {
			return nil, fmt.Errorf("model discovery omitted data")
		}
		all = append(all, w.Data...)
		path = ""
		if w.Next != nil {
			path = *w.Next
		} else if w.Links.Next != nil {
			path = *w.Links.Next
		}
		if path != "" {
			target, err := c.trusted(path)
			if err != nil {
				return nil, err
			}
			u, _ := url.Parse(target)
			if u.Path != "/api/v1/models" {
				return nil, fmt.Errorf("unexpected pagination path")
			}
		}
	}
	return all, nil
}
func has(ss []string, s string) bool {
	for _, v := range ss {
		if s == v {
			return true
		}
	}
	return false
}
func (c *Client) Preflight(ctx context.Context, model string, s domain.AgentSettings, judge bool) error {
	filter := "supported_parameters=tools"
	if judge {
		filter = "output_modalities=decisions"
	}
	models, err := c.Models(ctx, filter)
	if err != nil {
		return err
	}
	var found *ModelInfo
	for i := range models {
		if models[i].ID == model {
			found = &models[i]
			break
		}
	}
	if found == nil {
		all, err := c.Models(ctx, "")
		if err != nil {
			return err
		}
		for _, m := range all {
			if m.ID == model {
				return &domain.Failure{Kind: "unsupported", Message: "model does not advertise requested protocol"}
			}
		}
		return &domain.Failure{Kind: "unknown_model", Message: "model not found in protocol or general discovery: " + model}
	}
	if judge {
		if !has(found.Architecture.Output, "decisions") {
			return &domain.Failure{Kind: "unsupported", Message: "model does not advertise decisions"}
		}
		return nil
	}
	required := []string{"tools", "tool_choice", "temperature", "max_tokens"}
	if s.Reasoning != nil {
		required = append(required, "reasoning")
	}
	if s.ParallelToolCalls != nil {
		required = append(required, "parallel_tool_calls")
	}
	for _, p := range required {
		if !has(found.Supported, p) {
			return &domain.Failure{Kind: "unsupported", Message: "model does not advertise parameter " + p}
		}
	}
	if len(s.Provider.Only) > 0 {
		parts := strings.SplitN(model, "/", 2)
		if len(parts) != 2 {
			return &domain.Failure{Kind: "unsupported", Message: "provider pin requires author/model identifier"}
		}
		path := "/api/v1/models/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/endpoints"
		b, err := c.do(ctx, "GET", path, nil)
		if err != nil {
			return err
		}
		var w struct {
			Data struct {
				Endpoints []struct {
					Name      string   `json:"provider_name"`
					Supported []string `json:"supported_parameters"`
				} `json:"endpoints"`
			} `json:"data"`
		}
		if err = json.Unmarshal(b, &w); err != nil {
			return err
		}
		for _, e := range w.Data.Endpoints {
			if !has(s.Provider.Only, e.Name) {
				continue
			}
			ok := true
			for _, p := range required {
				if !has(e.Supported, p) {
					ok = false
				}
			}
			if ok {
				return nil
			}
		}
		return &domain.Failure{Kind: "unavailable", Message: "no pinned provider endpoint advertises all requested parameters"}
	}
	return nil
}
