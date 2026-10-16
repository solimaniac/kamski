// Package safety centralizes redaction before any output is persisted.
package safety

import (
	"bytes"
	"encoding/json"
	"regexp"
)

var keyPattern = regexp.MustCompile(`sk-or-[A-Za-z0-9_-]+`)

type Redactor struct{ Secrets []string }

// Bytes redacts both literal text and decoded JSON strings, including embedded
// JSON tool arguments. Byte replacement alone misses credentials represented
// using valid JSON escapes (for example, sk\\u002dor-...). Unchanged JSON keeps
// its original wire formatting; changed JSON is re-encoded without float loss.
func (r Redactor) Bytes(b []byte) []byte {
	if !json.Valid(b) {
		return r.raw(b)
	}
	// Inspect every string token, not a decoded map: duplicate object keys must
	// not hide an earlier credential, and numeric precision must not change.
	var out bytes.Buffer
	start := 0
	for i := 0; i < len(b); i++ {
		if b[i] != '"' {
			continue
		}
		end := i + 1
		for b[end] != '"' {
			if b[end] == '\\' {
				end++
			}
			end++
		}
		var value string
		// Validity was checked above, including string termination and escapes.
		_ = json.Unmarshal(b[i:end+1], &value)
		clean := r.Bytes([]byte(value))
		if string(clean) != value {
			out.Write(b[start:i])
			encoded, _ := json.Marshal(string(clean))
			out.Write(encoded)
			start = end + 1
		}
		i = end
	}
	if start == 0 {
		return r.raw(b)
	}
	out.Write(b[start:])
	return out.Bytes()
}

func (r Redactor) raw(b []byte) []byte {
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
func (r Redactor) Text(s string) string { return string(r.raw([]byte(s))) }
