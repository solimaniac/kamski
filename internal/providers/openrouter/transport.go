package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kamski/internal/audit"
	"kamski/internal/domain"
)

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	target, err := c.trusted(path)
	if err != nil {
		return nil, err
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	for n := 1; n <= c.retries+1; n++ {
		if ctx.Err() != nil {
			return nil, &domain.Failure{Kind: "interrupted", Message: "request canceled before attempt"}
		}
		started := time.Now().UTC()
		e := audit.Event{Number: n, Endpoint: target, Method: method, StartedAt: started, Request: c.redactor.Bytes(payload)}
		if len(e.Request) == 0 {
			e.Request = json.RawMessage(`null`)
		}
		if err := audit.Record(ctx, e); err != nil {
			return nil, &audit.PersistenceError{Err: err}
		}
		callctx, cancel := context.WithTimeout(ctx, c.timeout)
		req, err := http.NewRequestWithContext(callctx, method, target, bytes.NewReader(payload))
		if err != nil {
			cancel()
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if c.key != "" {
			req.Header.Set("Authorization", "Bearer "+c.key)
		}
		resp, callErr := c.http.Do(req)
		var raw []byte
		var f *domain.Failure
		delay := time.Duration(1<<(n-1)) * time.Second
		if callErr != nil {
			f = &domain.Failure{Kind: "transport", Message: c.redactor.Text(callErr.Error()), Retryable: true}
		} else {
			raw, f = c.readResponse(resp)
			if d, ok := retryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
				delay = d
			}
		}
		cancel()
		// Cancellation can occur while reading headers OR the response body. Both
		// are ambiguous remote executions and require explicit consent to retry.
		if f != nil && ctx.Err() != nil {
			f.Kind = "interrupted"
			f.Retryable = false
		}
		e.CompletedAt = domain.Ptr(time.Now().UTC())
		e.Response = raw
		if !json.Valid(raw) {
			e.Response = nil
			if len(raw) > 0 {
				e.ResponseText = domain.Ptr(string(raw))
			}
		}
		e.Error = f
		e.Retry = f != nil && f.Retryable && n <= c.retries && ctx.Err() == nil && delay <= 2*time.Minute
		if err := audit.Record(ctx, e); err != nil {
			return nil, &audit.PersistenceError{Err: err}
		}
		if f == nil {
			return raw, nil
		}
		if !e.Retry {
			return nil, f
		}
		if err := c.sleep(ctx, delay); err != nil {
			return nil, &domain.Failure{Kind: "interrupted", Message: "retry wait interrupted; prior request may have been processed"}
		}
	}
	return nil, errors.New("unreachable retry state")
}

func (c *Client) readResponse(response *http.Response) ([]byte, *domain.Failure) {
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, c.maxBody+1))
	oversized := int64(len(raw)) > c.maxBody
	raw = c.redactor.Bytes(raw)
	if err != nil {
		return raw, &domain.Failure{Kind: "transport", Message: c.redactor.Text(err.Error()), Retryable: true}
	}
	if oversized {
		return nil, &domain.Failure{Kind: "protocol_error", Message: "response exceeds bounded body size"}
	}
	return raw, decodeError(response.StatusCode, raw)
}

// retryAfter bounds numeric values before converting to a Duration, which would
// otherwise overflow for large server-supplied delays and permit an early retry.
func retryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	seconds, err := strconv.ParseUint(value, 10, 64)
	if err == nil || errors.Is(err, strconv.ErrRange) {
		if seconds > 120 {
			return 2*time.Minute + time.Second, true // exceeds the automatic retry budget
		}
		return time.Duration(seconds) * time.Second, true
	}
	if t, err := http.ParseTime(value); err == nil {
		return max(0, t.Sub(now)), true
	}
	return 0, false
}
