package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kamski/internal/domain"
	"kamski/internal/safety"
)

func manifestFixture(t *testing.T) domain.Manifest {
	t.Helper()
	b, err := os.ReadFile("../../fixtures/results/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m domain.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCreateRejectsIncompletePlanBeforeWriting(t *testing.T) {
	m := manifestFixture(t)
	m.Plan = m.Plan[:1]
	root := t.TempDir()
	s, err := New(root, safety.Redactor{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(m); err == nil {
		t.Fatal("accepted incomplete manifest")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("invalid manifest left partial run files")
	}
}

func TestResultsRejectFabricatedDecision(t *testing.T) {
	m := manifestFixture(t)
	s, err := New(t.TempDir(), safety.Redactor{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(m); err != nil {
		t.Fatal(err)
	}
	r := m.Plan[0]
	r.Decision.Status = "valid"
	r.Decision.Action = domain.Ptr(domain.Abort)
	r.Decision.JustificationPresent = true
	if err := s.Save(r); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Results(m); err == nil {
		t.Fatal("accepted eligible decision without candidate response")
	}
}

func TestReadJSONRejectsNonregularAndOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	var value any
	if err := ReadJSON(dir, &value); err == nil {
		t.Fatal("accepted directory")
	}
	path := filepath.Join(dir, "oversized.json")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxJSONBytes + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ReadJSON(path, &value); err == nil {
		t.Fatal("accepted oversized JSON")
	}
}
