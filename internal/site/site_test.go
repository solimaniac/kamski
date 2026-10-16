package site

import (
	"encoding/json"
	"kamski/internal/domain"
	"kamski/internal/reporting"
	"kamski/internal/safety"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOfflineDeterministicHostileAndPartial(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	var d domain.Dataset
	b, err := os.ReadFile("../../fixtures/results/public.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	hostile := `<script>alert("x")</script><img src=x onerror=alert(1)>`
	d.Trials[0].Result.Decision.Justification = &hostile
	arguments, err := json.Marshal(map[string]string{"justification": hostile})
	if err != nil {
		t.Fatal(err)
	}
	d.Trials[0].Result.Response.Calls[0].Arguments = string(arguments)
	d.Trials[0].Result.Response.Text = &hostile
	d.Trials[0].Result.Model.Requested = hostile
	d.Trials[0].Result.Error = &domain.Failure{Kind: "api_error", Message: "SENTINEL-KEY"}
	clean, err := reporting.PublicResult(d.Trials[0].Result, safety.Redactor{Secrets: []string{"SENTINEL-KEY"}})
	if err != nil {
		t.Fatal(err)
	}
	d.Trials[0].Result = clean
	out := filepath.Join(t.TempDir(), "artifact")
	if err = Build(d, out); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(out)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0755 {
			t.Fatalf("static artifact is not publicly traversable: %v", info.Mode())
		}
	}
	before := map[string]string{}
	entries, _ := os.ReadDir(out)
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(out, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		before[e.Name()] = string(b)
		if strings.HasSuffix(e.Name(), ".html") && (strings.Contains(string(b), "<script>alert") || strings.Contains(string(b), "<img src=x") || strings.Contains(string(b), "SENTINEL-KEY")) {
			t.Fatal("unsafe render", e.Name())
		}
	}
	if err = Build(d, out); err != nil {
		t.Fatal(err)
	}
	for name, want := range before {
		b, _ := os.ReadFile(filepath.Join(out, name))
		if string(b) != want {
			t.Fatal("nondeterministic", name)
		}
	}
	if err = Build(domain.Dataset{SchemaVersion: domain.SchemaVersion, Runs: []domain.PublicRun{}, Trials: []domain.PublicTrial{}}, out); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(out, "index.html"))
	if !strings.Contains(string(b), "No results published yet") {
		t.Fatal("dishonest empty state")
	}
}
func TestSafeOutput(t *testing.T) {
	for _, p := range []string{".", "..", filepath.VolumeName(os.TempDir()) + string(filepath.Separator)} {
		if SafeOutput(p) == nil {
			t.Fatal("dangerous output accepted", p)
		}
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("user"), 0644)
	d := domain.Dataset{SchemaVersion: domain.SchemaVersion}
	if Build(d, dir) == nil {
		t.Fatal("arbitrary directory replaced")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "keep.txt"))
	if string(b) != "user" {
		t.Fatal("user data deleted")
	}
}
