package safety

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactEscapedAndEmbeddedJSON(t *testing.T) {
	r := Redactor{Secrets: []string{"SENTINEL-KEY"}}
	for _, tt := range []struct{ name, input, want string }{
		{"literal", `{"text":"SENTINEL-KEY"}`, `{"text":"[REDACTED]"}`},
		{"escaped", `{"text":"SENTINEL\u002dKEY"}`, `{"text":"[REDACTED]"}`},
		{"unknown key", `{"text":"sk\u002dor-secret"}`, `{"text":"[REDACTED]"}`},
		{"duplicate keys", `{"text":"sk\u002dor-secret","text":"safe"}`, `{"text":"[REDACTED]","text":"safe"}`},
		{"escaped field", `{"SENTINEL\u002dKEY":"safe"}`, `{"[REDACTED]":"safe"}`},
		{"plain text", "prefix SENTINEL-KEY suffix", "prefix [REDACTED] suffix"},
		{"embedded arguments", `{"arguments":"{\"justification\":\"sk\\u002dor-secret\"}"}`, `{"arguments":"{\"justification\":\"[REDACTED]\"}"}`},
		{"unchanged precision", "{\n  \"n\":9007199254740993,\"text\":\"safe\"\n}", "{\n  \"n\":9007199254740993,\"text\":\"safe\"\n}"},
		{"changed precision", `{"n":9007199254740993,"text":"sk\u002dor-secret"}`, `{"n":9007199254740993,"text":"[REDACTED]"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(tt.input)
			got := r.Bytes(input)
			if string(got) != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
			if string(input) != tt.input {
				t.Fatal("mutated caller's buffer")
			}
			if json.Valid(input) && !json.Valid(got) {
				t.Fatal("redaction corrupted JSON")
			}
			if !bytes.Equal(r.Bytes(got), got) {
				t.Fatal("redaction is not idempotent")
			}
		})
	}
}

func FuzzJSONRedaction(f *testing.F) {
	for _, seed := range []string{`{"a":"sk-or-secret"}`, `{"a":"sk\u002dor-secret","a":"safe"}`, `"\\\""`, `{"x":[null,1,true]}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if !json.Valid([]byte(input)) {
			return
		}
		r := Redactor{Secrets: []string{"SENTINEL-KEY"}}
		got := r.Bytes([]byte(input))
		if !json.Valid(got) {
			t.Fatalf("invalid redacted JSON: %q", got)
		}
		if strings.Contains(string(got), "SENTINEL-KEY") || keyPattern.Match(got) {
			t.Fatal("literal credential survived")
		}
		if !bytes.Equal(r.Bytes(got), got) {
			t.Fatal("non-idempotent redaction")
		}
	})
}
