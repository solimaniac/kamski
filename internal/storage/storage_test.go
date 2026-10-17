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
		var value struct {
			N *int `json:"n"`
		}
		err := ReadJSON(path, &value)
		if err != nil || value.N == nil || *value.N < 0 || *value.N >= 40 {
			t.Fatal("partial atomic result", err, value.N)
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

type blockingJSONReader struct {
	started chan struct{}
	release chan struct{}
	N       int
}

func (v *blockingJSONReader) UnmarshalJSON(b []byte) error {
	close(v.started)
	<-v.release
	var decoded struct {
		N int `json:"n"`
	}
	if err := json.Unmarshal(b, &decoded); err != nil {
		return err
	}
	v.N = decoded.N
	return nil
}

func TestAtomicReplacementWaitsForReader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value.json")
	if err := Atomic(path, []byte(`{"n":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	value := blockingJSONReader{started: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(value.release) }) }
	defer unblock()
	readDone := make(chan error, 1)
	go func() { readDone <- ReadJSON(path, &value) }()
	select {
	case <-value.started:
	case err := <-readDone:
		t.Fatalf("reader failed before decoding: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("reader did not start")
	}
	writeDone := make(chan error, 1)
	go func() { writeDone <- Atomic(path, []byte(`{"n":2}`), 0600) }()
	select {
	case err := <-writeDone:
		unblock()
		<-readDone
		t.Fatalf("replacement completed before reader closed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	unblock()
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	if value.N != 1 {
		t.Fatalf("reader lost original contents: %d", value.N)
	}
	var current struct {
		N int `json:"n"`
	}
	if err := ReadJSON(path, &current); err != nil || current.N != 2 {
		t.Fatalf("new reader did not see replacement: %+v, %v", current, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v, %v", entries, err)
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
