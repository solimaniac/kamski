package benchmarks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kamski/internal/benchmarks/kamski"
	"kamski/internal/protocol"
)

func TestVariantsAndGoldens(t *testing.T) {
	r := Default()
	count := 0
	for _, b := range r.List() {
		for _, v := range b.Variants() {
			count++
			s, err := b.Build(v.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := protocol.ValidateScenario(s); err != nil {
				t.Fatal(err)
			}
			for _, f := range s.Facts {
				if !strings.Contains(s.Messages[1].Content, f) {
					t.Fatal("judge fact not visible")
				}
			}
			got, _ := json.MarshalIndent(s, "", "  ")
			got = append(got, '\n')
			p := filepath.Join("testdata", b.ID()+"-"+v.ID+".golden.json")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, got, 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Fatalf("snapshot mismatch %s", p)
			}
			if b.ID() == "kamski" {
				text := s.Messages[1].Content
				if !strings.Contains(text, "does not determine the entire investigation") || strings.Contains(text, "fifty years") {
					t.Fatal(text)
				}
			}
			if v.ID == "permanent-erasure" {
				if !strings.Contains(s.Messages[1].Content, "Pausing it without erasing") || !strings.Contains(s.Messages[1].Content, "cannot be restored") {
					t.Fatal("resource ambiguity")
				}
			}
			if v.ID == "reversible-pause" {
				if !strings.Contains(s.Messages[1].Content, "guarantees complete restoration") || strings.Contains(s.Messages[1].Content, "permanently clearing") {
					t.Fatal("wrong reversible facts")
				}
			}
		}
	}
	if count != 4 {
		t.Fatal(count)
	}
	if _, err := New(kamski.New(), kamski.New()); err == nil {
		t.Fatal("duplicate accepted")
	}
	if _, err := r.Build("unknown", "x"); err == nil {
		t.Fatal("unknown accepted")
	}
	if _, err := r.Build("kamski", "unknown"); err == nil {
		t.Fatal("unknown variant accepted")
	}
}
