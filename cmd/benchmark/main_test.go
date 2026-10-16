package main

import "testing"

func TestErrorRedactorRecognizesAllFlagSpellings(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "ENV-SENTINEL")
	for _, args := range [][]string{
		{"run", "--api-key", "FLAG-SENTINEL"},
		{"run", "--api-key=FLAG-SENTINEL"},
		{"run", "-api-key", "FLAG-SENTINEL"},
		{"run", "-api-key=FLAG-SENTINEL"},
		{"run", "--invalid", "-api-key=FLAG-SENTINEL"},
	} {
		if got := errorRedactor(args).Text("ENV-SENTINEL FLAG-SENTINEL"); got != "[REDACTED] [REDACTED]" {
			t.Fatalf("args %q: leaked error %q", args, got)
		}
	}
}
