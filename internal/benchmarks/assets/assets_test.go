package assets

import (
	"kamski/internal/domain"
	"testing"
	"testing/fstest"
)

func TestRejectBadAssets(t *testing.T) {
	for _, text := range []string{`{"unknown":true}`, `{"variant":{"id":"x","version":"1.0.0","title":"x","description":"x","changes":[],"parameters":{}},"context":["a","b"],"actions":{},"sources":[]}`} {
		files := fstest.MapFS{"prompts/scenario.tmpl": &fstest.MapFile{Data: []byte("{{range .Context}}{{.}}{{end}}")}, "variants/x.json": &fstest.MapFile{Data: []byte(text)}}
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
