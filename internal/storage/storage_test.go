package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kamski/internal/domain"
	"kamski/internal/safety"
)

func TestAtomicAndSafePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "value.json")
	if err := Atomic(path, []byte(`{"n":0}`), 0600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	t.Cleanup(wg.Wait)
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		defer close(done)
		for i := 0; i < 40; i++ {
			b, _ := json.Marshal(map[string]int{"n": i})
			if err := Atomic(path, b, 0600); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for {
		b, err := os.ReadFile(path)
		if transientSharing(err) {
			time.Sleep(time.Millisecond)
			continue
		}
		if err != nil || !json.Valid(b) {
			t.Fatal("partial atomic result", err, string(b))
		}
		select {
		case <-done:
			wg.Wait()
			goto finished
		default:
		}
	}
finished:
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary files leaked")
	}
	fs, _ := New(dir, safety.Redactor{})
	if _, err := fs.runPath("../escape"); err == nil {
		t.Fatal("run traversal")
	}
	if _, err := fs.trialPath(domain.Result{RunID: "run", TrialID: "../escape"}); err == nil {
		t.Fatal("trial traversal")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Log("symlink check unavailable on host:", err)
	} else if NoSymlinks(filepath.Join(link, "child")) == nil {
		t.Fatal("symlink traversal accepted")
	}
}
func TestMissingResultNeverReplanned(t *testing.T) {
	var m domain.Manifest
	b, err := os.ReadFile("../../fixtures/results/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	fs, _ := New(t.TempDir(), safety.Redactor{})
	if err = fs.Create(m); err != nil {
		t.Fatal(err)
	}
	p, _ := fs.trialPath(m.Plan[0])
	if err = os.Remove(filepath.Join(p, "result.json")); err != nil {
		t.Fatal(err)
	}
	if _, err = fs.Results(m); err == nil {
		t.Fatal("lost output silently replanned")
	}
}
