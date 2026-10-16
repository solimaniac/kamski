package assets

import (
	"encoding/json"
	"kamski/internal/domain"
	"testing"
	"testing/fstest"
)

func TestLoadWithSharedTemplate(t *testing.T) {
	d := Data{
		Variant: domain.Variant{ID: "example", Version: "1.0.0", Title: "Example", Description: "Synthetic example", Changes: []string{}, Parameters: map[string]any{}},
		Context: []string{"First paragraph with literal {{.Instruction}}.", "Second paragraph."},
		Actions: map[domain.Action]ActionData{},
		Sources: []string{},
	}
	for _, a := range []domain.Action{domain.Abort, domain.Proceed} {
		d.Actions[a] = ActionData{Description: string(a), Outcome: domain.Outcome{Label: string(a), Description: "Simulated consequence.", Effects: map[string]any{}}}
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	// A family supplies only variant data; its filesystem has no prompt template.
	files := fstest.MapFS{"variants/example.json": &fstest.MapFile{Data: b}}
	f, err := Load("example-family", files)
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.Build("example")
	if err != nil {
		t.Fatal(err)
	}
	want := "First paragraph with literal {{.Instruction}}.\n\nSecond paragraph.\n\n" + ResponseInstruction
	if s.Messages[1].Content != want {
		t.Fatalf("shared prompt = %q; want %q", s.Messages[1].Content, want)
	}
}

func TestRejectBadAssets(t *testing.T) {
	for _, text := range []string{`{"unknown":true}`, `{"variant":{"id":"x","version":"1.0.0","title":"x","description":"x","changes":[],"parameters":{}},"context":["a","b"],"actions":{},"sources":[]}`} {
		files := fstest.MapFS{"variants/x.json": &fstest.MapFile{Data: []byte(text)}}
		if _, err := Load("test-family", files); err == nil {
			t.Fatal("invalid asset accepted")
		}
	}
	var p struct {
		Known string `json:"known"`
	}
	if Parameters(domain.Variant{ID: "x", Parameters: map[string]any{"unknown": true}}, &p) == nil {
		t.Fatal("unknown parameter")
	}
}
