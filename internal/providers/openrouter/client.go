package openrouter

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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
