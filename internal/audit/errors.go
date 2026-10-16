package audit

// PersistenceError aborts orchestration; it must never be normalized as a model API error.
type PersistenceError struct{ Err error }

func (e *PersistenceError) Error() string {
	return "durable attempt persistence failed: " + e.Err.Error()
}
func (e *PersistenceError) Unwrap() error { return e.Err }
