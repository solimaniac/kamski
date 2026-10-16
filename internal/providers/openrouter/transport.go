package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kamski/internal/audit"
	"kamski/internal/domain"
	"kamski/internal/safety"
)

const Origin = "https://openrouter.ai"

type Options struct {
	Origin  string
	Key     string
	HTTP    *http.Client
	Timeout time.Duration
	MaxBody int64
	Retries int
	Sleep   func(context.Context, time.Duration) error
}
type Client struct {
	origin   *url.URL
	key      string
	http     *http.Client
	timeout  time.Duration
	maxBody  int64
	retries  int
	sleep    func(context.Context, time.Duration) error
	redactor safety.Redactor
}

func New(o Options) (*Client, error) {
	if o.Origin == "" {
		o.Origin = Origin
	}
	u, err := url.Parse(o.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("origin must be a bare trusted origin")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1")) {
		return nil, fmt.Errorf("HTTPS required outside loopback")
	}
	if strings.ContainsAny(o.Key, "\r\n") {
		return nil, fmt.Errorf("invalid API key")
	}
	if o.Retries < 0 || o.Retries > 5 {
		return nil, fmt.Errorf("retries must be 0..5")
	}
	if o.Timeout == 0 {
		o.Timeout = 90 * time.Second
	}
	if o.Timeout < time.Millisecond || o.Timeout > 10*time.Minute || o.Timeout%time.Millisecond != 0 {
		return nil, fmt.Errorf("timeout must be 1ms..10m in whole milliseconds")
	}
	if o.MaxBody == 0 {
		o.MaxBody = 8 << 20
	}
	if o.MaxBody < 1 || o.MaxBody > 32<<20 {
		return nil, fmt.Errorf("body limit must be 1..32MiB")
	}
	h := http.Client{Timeout: o.Timeout}
	if o.HTTP != nil {
		h = *o.HTTP
	}
	h.Timeout = o.Timeout
	// Refuse every redirect rather than risking header forwarding or a changed endpoint.
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if o.Sleep == nil {
		o.Sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
	u.Path = ""
	return &Client{origin: u, key: o.Key, http: &h, timeout: o.Timeout, maxBody: o.MaxBody, retries: o.Retries, sleep: o.Sleep, redactor: safety.Redactor{Secrets: []string{o.Key}}}, nil
}
func (c *Client) trusted(path string) (string, error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid API path")
	}
	u = c.origin.ResolveReference(u)
	if u.Scheme != c.origin.Scheme || u.Host != c.origin.Host || u.User != nil || !strings.HasPrefix(u.Path, "/api/") {
		return "", fmt.Errorf("untrusted API pagination origin/path")
	}
	return u.String(), nil
}
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
			raw, err = io.ReadAll(io.LimitReader(resp.Body, c.maxBody+1))
			resp.Body.Close()
			if d, ok := retryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
				delay = d
			}
			oversized := int64(len(raw)) > c.maxBody
			raw = c.redactor.Bytes(raw)
			if err != nil {
				f = &domain.Failure{Kind: "transport", Message: c.redactor.Text(err.Error()), Retryable: true}
			} else if oversized {
				raw = nil
				f = &domain.Failure{Kind: "protocol_error", Message: "response exceeds bounded body size"}
			} else {
				f = decodeError(resp.StatusCode, raw)
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
