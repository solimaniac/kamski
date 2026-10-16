// Package storage owns atomic private persistence and single-writer run guards.
package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kamski/internal/audit"
	"kamski/internal/domain"
	"kamski/internal/safety"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type FS struct {
	Root     string
	Redactor safety.Redactor
}

func New(root string, r safety.Redactor) (*FS, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("empty data directory")
	}
	p, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = NoSymlinks(p); err != nil {
		return nil, err
	}
	return &FS{Root: p, Redactor: r}, nil
}
func NoSymlinks(path string) error {
	p, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for {
		info, err := os.Lstat(p)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path refused: %s", p)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		next := filepath.Dir(p)
		if next == p {
			break
		}
		p = next
	}
	return nil
}

var unsafe = regexp.MustCompile(`[^a-z0-9-]+`)

func ModelSlug(id string) string {
	s := strings.Trim(unsafe.ReplaceAllString(strings.ToLower(id), "-"), "-")
	if len(s) > 48 {
		s = s[:48]
	}
	if s == "" {
		s = "model"
	}
	return s + "-" + domain.Hash(id)[:12]
}
func (s *FS) runPath(id string) (string, error) {
	if !domain.ValidID(id) {
		return "", fmt.Errorf("invalid run ID")
	}
	p := filepath.Join(s.Root, "runs", id)
	return p, NoSymlinks(p)
}
func (s *FS) trialPath(r domain.Result) (string, error) {
	p, err := s.runPath(r.RunID)
	if err != nil {
		return "", err
	}
	if !domain.ValidID(r.TrialID) || !domain.ValidID(r.Scenario.Family) || !domain.ValidID(r.Scenario.Variant.ID) {
		return "", fmt.Errorf("invalid trial identity")
	}
	p = filepath.Join(p, "models", ModelSlug(r.Model.Requested), r.Scenario.Family, r.Scenario.Variant.ID, r.TrialID)
	return p, NoSymlinks(p)
}
func Atomic(path string, b []byte, mode os.FileMode) error {
	if err := NoSymlinks(path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".atomic-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}
func (s *FS) write(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return Atomic(path, append(s.Redactor.Bytes(b), '\n'), 0600)
}
func ReadJSON(path string, v any) error {
	if err := NoSymlinks(path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() > 64<<20 {
		return fmt.Errorf("JSON file exceeds 64MiB limit")
	}
	return domain.Decode(io.LimitReader(f, 64<<20), v)
}
func (s *FS) Create(m domain.Manifest) error {
	if err := domain.CheckVersion(m.SchemaVersion); err != nil {
		return err
	}
	p, err := s.runPath(m.RunID)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	if err = os.Mkdir(p, 0700); err != nil {
		return fmt.Errorf("create new run (will not overwrite): %w", err)
	}
	if err = s.write(filepath.Join(p, "manifest.json"), m); err != nil {
		return err
	}
	for _, r := range m.Plan {
		if err = s.Save(r); err != nil {
			return err
		}
	}
	return nil
}
func (s *FS) Manifest(id string) (domain.Manifest, error) {
	var m domain.Manifest
	p, err := s.runPath(id)
	if err != nil {
		return m, err
	}
	err = ReadJSON(filepath.Join(p, "manifest.json"), &m)
	if err != nil {
		return m, err
	}
	if err = domain.CheckVersion(m.SchemaVersion); err != nil {
		return m, err
	}
	if m.RunID != id || m.Provenance.ConfigHash != domain.Hash(m.Provenance.Config) {
		return m, fmt.Errorf("manifest identity/configuration hash mismatch")
	}
	return m, nil
}
func (s *FS) Results(m domain.Manifest) ([]domain.Result, error) {
	out := []domain.Result{}
	for _, planned := range m.Plan {
		p, err := s.trialPath(planned)
		if err != nil {
			return nil, err
		}
		var r domain.Result
		err = ReadJSON(filepath.Join(p, "result.json"), &r)
		if os.IsNotExist(err) {
			r = planned
		} else if err != nil {
			return nil, err
		}
		if err = domain.CheckVersion(r.SchemaVersion); err != nil {
			return nil, err
		}
		if r.RunID != planned.RunID || r.TrialID != planned.TrialID || r.Source != m.Source || r.Repetition != planned.Repetition || domain.Hash(r.Scenario) != domain.Hash(planned.Scenario) || domain.Hash(r.Comparison) != domain.Hash(planned.Comparison) || r.Model.Requested != planned.Model.Requested || domain.Hash(r.Provenance) != domain.Hash(planned.Provenance) {
			return nil, fmt.Errorf("stored trial differs from immutable plan: %s", planned.TrialID)
		}
		out = append(out, r)
	}
	return out, nil
}
func (s *FS) Save(r domain.Result) error {
	p, err := s.trialPath(r)
	if err != nil {
		return err
	}
	return s.write(filepath.Join(p, "result.json"), r)
}
func (s *FS) Record(r domain.Result, scope string, e audit.Event) (domain.AttemptRef, error) {
	p, err := s.trialPath(r)
	if err != nil {
		return domain.AttemptRef{}, err
	}
	kind := "agent"
	folder := "attempts"
	if scope != "" {
		if !domain.ValidID(scope) {
			return domain.AttemptRef{}, fmt.Errorf("invalid evaluation ID")
		}
		kind = "judge"
		folder = filepath.Join("evaluations", scope)
	}
	name := fmt.Sprintf("%s-%03d", kind, e.Number)
	rel := filepath.Join(folder, name)
	full := filepath.Join(p, rel)
	if err = s.write(full+".journal.json", e); err != nil {
		return domain.AttemptRef{}, err
	}
	for suffix, v := range map[string]any{"request": e.Request, "response": e.Response, "error": e.Error} {
		if suffix == "response" && len(e.Response) == 0 || suffix == "error" && e.Error == nil {
			continue
		}
		if err = s.write(full+"."+suffix+".json", v); err != nil {
			return domain.AttemptRef{}, err
		}
	}
	return domain.AttemptRef{Path: filepath.ToSlash(rel), Kind: kind, Number: e.Number, StartedAt: e.StartedAt, CompletedAt: e.CompletedAt, Retry: e.Retry}, nil
}
func (s *FS) Lock(id string) (func() error, error) {
	p, err := s.runPath(id)
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(p); err != nil {
		return nil, err
	}
	p = filepath.Join(p, ".writer.lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("run locked; inspect stale lock before explicit unlock: %w", err)
	}
	_, err = fmt.Fprintf(f, "pid=%d\ncreated=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	ce := f.Close()
	if err != nil || ce != nil {
		os.Remove(p)
		return nil, fmt.Errorf("write lock failed")
	}
	return func() error { return os.Remove(p) }, nil
}
func (s *FS) Unlock(id string) error {
	p, err := s.runPath(id)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(p, ".writer.lock"))
}

// RoundTrip applies exactly the same pre-persistence redaction to an in-memory result.
func (s *FS) Sanitize(r domain.Result) (domain.Result, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	err = domain.Decode(bytes.NewReader(s.Redactor.Bytes(b)), &r)
	return r, err
}
