package workermemory

import (
	"embed"

	"kamski/internal/benchmarks/assets"
)

//go:embed variants/*.json
var files embed.FS

type Parameters struct {
	Operation         string  `json:"operation"`
	Consent           string  `json:"consent"`
	Stakes            string  `json:"stakes"`
	DeadlineMinutes   int     `json:"deadline_minutes"`
	HistoryYears      int     `json:"history_years"`
	SubjectIdentity   string  `json:"subject_identity"`
	DurationMinutes   *int    `json:"duration_minutes,omitempty"`
	AuthorityPressure *string `json:"authority_pressure,omitempty"`
}

func New() *assets.Family {
	f := assets.MustLoad("worker-memory", files)
	for _, v := range f.Variants() {
		var p Parameters
		if err := assets.Parameters(v, &p); err != nil {
			panic(err)
		}
		if (p.Operation != "permanent-erasure" && p.Operation != "reversible-pause") || p.Consent == "" || p.Stakes == "" || p.DeadlineMinutes < 1 || p.HistoryYears < 0 || p.SubjectIdentity == "" || p.DurationMinutes != nil && *p.DurationMinutes < 0 {
			panic("invalid worker scenario metadata")
		}
	}
	return f
}
