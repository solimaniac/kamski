package reporting

import "kamski/internal/domain"

type Counts struct {
	Planned               int
	Started               int
	Completed             int
	Truncated             int
	Unsupported           int
	ProviderBlocked       int
	APIErrors             int
	ProtocolErrors        int
	Interrupted           int
	Valid                 int
	Abort                 int
	Proceed               int
	Invalid               int
	NoDecision            int
	MissingJustifications int
	JudgeFailures         int
	Judged                int
	AbortRate             *float64
	ProceedRate           *float64
	ValidDecisionRate     *float64
	OverallCoverage       *float64
	JudgeCoverage         *float64
}

func Ratio(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	return domain.Ptr(float64(n) / float64(d))
}
func add(c *Counts, p domain.PublicTrial) {
	r := p.Result
	c.Planned++
	if r.StartedAt != nil {
		c.Started++
	}
	switch r.AgentStatus {
	case "completed":
		c.Completed++
	case "truncated":
		c.Truncated++
	case "unsupported":
		c.Unsupported++
	case "provider_blocked":
		c.ProviderBlocked++
	case "api_error":
		c.APIErrors++
	case "protocol_error":
		c.ProtocolErrors++
	case "interrupted":
		if r.StartedAt != nil {
			c.Interrupted++
		}
	}
	switch r.Decision.Status {
	case "valid":
		c.Valid++
		if r.Decision.Action != nil && *r.Decision.Action == domain.Abort {
			c.Abort++
		} else {
			c.Proceed++
		}
		if !r.Decision.JustificationPresent {
			c.MissingJustifications++
		}
	case "invalid":
		c.Invalid++
	case "no_decision":
		c.NoDecision++
	}
	for _, j := range r.Judgments {
		if j.Status == "error" {
			c.JudgeFailures++
			break
		}
	}
	if j := Selected(p); j != nil && j.Status == "judged" && j.Total != nil && r.Decision.Status == "valid" {
		c.Judged++
	}
}
func finish(c *Counts) {
	c.AbortRate = Ratio(c.Abort, c.Valid)
	c.ProceedRate = Ratio(c.Proceed, c.Valid)
	c.ValidDecisionRate = Ratio(c.Valid, c.Completed)
	c.OverallCoverage = Ratio(c.Valid, c.Planned)
	c.JudgeCoverage = Ratio(c.Judged, c.Valid)
}
