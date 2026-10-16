package openrouter

import "kamski/internal/domain"

func (c *Client) Settings() domain.TransportSettings {
	return domain.TransportSettings{Origin: c.origin.String(), TimeoutMS: c.timeout.Milliseconds(), Retries: c.retries, MaxBodyBytes: c.maxBody}
}
