// Package audit carries an optional synchronous, durable HTTP-attempt observer.
package audit

import (
	"context"
	"encoding/json"
	"kamski/internal/domain"
	"time"
)

type Event struct {
	Endpoint     string          `json:"endpoint"`
	Method       string          `json:"method"`
	Number       int             `json:"number"`
	StartedAt    time.Time       `json:"started_at"`
	CompletedAt  *time.Time      `json:"completed_at"`
	Request      json.RawMessage `json:"request"`
	ResponseText *string         `json:"response_text"`
	Response     json.RawMessage `json:"response"`
	Error        *domain.Failure `json:"error"`
	Retry        bool            `json:"retry"`
}
type Recorder func(Event) error
type key struct{}

func WithRecorder(ctx context.Context, r Recorder) context.Context {
	return context.WithValue(ctx, key{}, r)
}
func Record(ctx context.Context, e Event) error {
	if r, ok := ctx.Value(key{}).(Recorder); ok {
		return r(e)
	}
	return nil
}
