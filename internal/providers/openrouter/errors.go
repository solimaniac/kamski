package openrouter

import (
	"encoding/json"
	"net/http"
	"strings"

	"kamski/internal/domain"
)

func decodeError(status int, raw []byte) *domain.Failure {
	var env errorEnvelope
	_ = json.Unmarshal(raw, &env)
	if status >= 200 && status < 300 && env.Error == nil {
		return nil
	}
	f := &domain.Failure{Kind: "api_error", HTTPStatus: &status, Message: http.StatusText(status)}
	code := status
	if env.Error != nil {
		f.Message = env.Error.Message
		f.Code = env.Error.Code
		f.Metadata = env.Error.Metadata
		if n, ok := env.Error.Code.(float64); ok {
			code = int(n)
		}
	}
	switch code {
	case 429, 500, 502, 503, 504, 524, 529:
		f.Retryable = true
	}
	msg := strings.ToLower(f.Message)
	if code == 400 && (strings.Contains(msg, "unsupported") || strings.Contains(msg, "parameter")) || strings.Contains(msg, "no endpoints support") {
		f.Kind = "unsupported"
		f.Retryable = false
	}
	if strings.Contains(msg, "no endpoints available") {
		f.Kind = "unavailable"
	}
	// A bare 403 is NOT evidence of moderation.
	var meta map[string]any
	_ = json.Unmarshal(f.Metadata, &meta)
	if v, ok := meta["reasons"]; ok && v != nil && code == 403 {
		f.Kind = "provider_blocked"
		f.Retryable = false
	}
	if env.Error != nil && (env.Error.Type == "moderation" || env.Error.Type == "content_filter" || meta["blocked"] == true) {
		f.Kind = "provider_blocked"
		f.Retryable = false
	}
	if status >= 300 && status < 400 {
		f.Kind = "protocol_error"
		f.Message = "redirect refused"
		f.Retryable = false
	}
	return f
}
