package runner

import (
	"errors"
	"time"

	"kamski/internal/audit"
	"kamski/internal/domain"
)

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}
func (r *Runner) save(t *domain.Result) error {
	t.UpdatedAt = r.now()
	clean, err := r.Store.Sanitize(*t)
	if err != nil {
		return err
	}
	*t = clean
	return r.Store.Save(*t)
}
func failure(err error) *domain.Failure {
	var f *domain.Failure
	if errors.As(err, &f) {
		return f
	}
	return &domain.Failure{Kind: "internal", Message: err.Error()}
}
func status(f *domain.Failure) string {
	switch f.Kind {
	case "unsupported", "unknown_model", "unavailable":
		return "unsupported"
	case "provider_blocked":
		return "provider_blocked"
	case "protocol_error":
		return "protocol_error"
	case "interrupted":
		return "interrupted"
	default:
		return "api_error"
	}
}
func (r *Runner) recorder(t *domain.Result, scope string) audit.Recorder {
	// Each retry sequence starts at 1 remotely, but private records remain append-only.
	offset := 0
	for _, a := range t.Attempts {
		if scope == "" && a.Kind == "agent" && a.Number > offset {
			offset = a.Number
		}
	}
	return func(e audit.Event) error {
		e.Number += offset
		ref, err := r.Store.Record(*t, scope, e)
		if err != nil {
			return err
		}
		found := false
		for i, a := range t.Attempts {
			if a.Path == ref.Path {
				t.Attempts[i] = ref
				found = true
				break
			}
		}
		if !found {
			t.Attempts = append(t.Attempts, ref)
		}
		return r.save(t)
	}
}
