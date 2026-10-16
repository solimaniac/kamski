package sitecli

import (
	"bytes"
	"encoding/json"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"kamski/internal/domain"
	"kamski/internal/safety"
	"kamski/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateWholeCohortAndOfflineBuild(t *testing.T) {
	var m domain.Manifest
	var success domain.Result
	for name, v := range map[string]any{"manifest": &m, "success": &success} {
		b, err := os.ReadFile("../../../fixtures/results/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, v); err != nil {
			t.Fatal(err)
		}
	}
	private := t.TempDir()
	public := t.TempDir()
	dest := filepath.Join(t.TempDir(), "artifact")
	fs, err := storage.New(private, safety.Redactor{Secrets: []string{"SENTINEL-KEY"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = fs.Create(m); err != nil {
		t.Fatal(err)
	}
	if err = fs.Save(success); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENROUTER_API_KEY", "SENTINEL-KEY")
	var log bytes.Buffer
	args := []string{"update", "--input", private, "--run", m.RunID, "--data", public, "--out", dest}
	if err = Run(args, &log); err == nil {
		t.Fatal("fixture publish not explicit")
	}
	args = append(args, "--allow-fixtures")
	if err = Run(args, &log); err != nil {
		t.Fatal(err)
	}
	schemaPath, _ := filepath.Abs("../../../schemas/public-dataset.schema.json")
	schema, err := jsonschema.Compile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(public, "dataset.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw any
	json.Unmarshal(b, &raw)
	if err = schema.Validate(raw); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dest, "scenario-worker-memory-permanent-erasure.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "Synthetic fixtures. Not live measurements.") || !strings.Contains(string(before), "7.00") {
		t.Fatal("demo labels/grade absent")
	}
	// Build needs neither raw directories nor credentials, including after deleting input.
	if err = os.RemoveAll(private); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENROUTER_API_KEY", "")
	if err = Run([]string{"build", "--data", public, "--out", dest}, &log); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(dest, "scenario-worker-memory-permanent-erasure.html"))
	if !bytes.Equal(before, after) {
		t.Fatal("offline nondeterministic")
	}
	// Opt-in local render-review artifact; never writes to tracked site/data.
	if demo := os.Getenv("RENDER_REVIEW_DIR"); demo != "" {
		if err = Run([]string{"build", "--data", public, "--out", demo}, &log); err != nil {
			t.Fatal(err)
		}
	}
}
