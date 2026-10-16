package app

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"kamski/internal/domain"
	"kamski/internal/safety"
	"kamski/internal/storage"
)

func TestResumeRejectsInvalidOrChangedRecordedTransport(t *testing.T) {
	for _, tt := range []struct {
		name             string
		timeout, maxBody int64
		flags            []string
	}{
		{"zero timeout", 0, 1024, nil},
		{"overflowing timeout", 1<<63 - 1, 1024, nil},
		{"zero body bound", 1, 0, nil},
		{"excessive body bound", 1, 33 << 20, nil},
		{"different submillisecond timeout", 1, 1024, []string{"--timeout", "1500us"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, err := os.ReadFile("../../fixtures/results/manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var m domain.Manifest
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			m.Provenance.Transport = &domain.TransportSettings{Origin: "https://openrouter.ai", TimeoutMS: tt.timeout, MaxBodyBytes: tt.maxBody}
			for i := range m.Plan {
				m.Plan[i].Provenance = m.Provenance
			}
			dir := t.TempDir()
			s, err := storage.New(dir, safety.Redactor{})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Create(m); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"run", "--resume", m.RunID, "--out", dir, "--dry-run"}, tt.flags...)
			if err := Benchmark(context.Background(), args, io.Discard); err == nil {
				t.Fatal("accepted invalid or changed transport")
			}
		})
	}
}
