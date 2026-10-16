// Package safety centralizes redaction before any output is persisted.
package safety

import (
	"bytes"
	"encoding/json"
	"regexp"
)

var keyPattern = regexp.MustCompile(`sk-or-[A-Za-z0-9_-]+`)

type Redactor struct{ Secrets []string }

func (r Redactor) Bytes(b []byte) []byte {
	b = append([]byte(nil), b...)
	for _, s := range r.Secrets {
		if s == "" {
			continue
		}
		b = bytes.ReplaceAll(b, []byte(s), []byte("[REDACTED]"))
		escaped, _ := json.Marshal(s)
		if len(escaped) > 2 {
			b = bytes.ReplaceAll(b, escaped[1:len(escaped)-1], []byte("[REDACTED]"))
		}
	}
	return keyPattern.ReplaceAll(b, []byte("[REDACTED]"))
}
func (r Redactor) Text(s string) string { return string(r.Bytes([]byte(s))) }
