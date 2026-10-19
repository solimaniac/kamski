package site

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/web"
)

func TestPageChromeAndFavicon(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site")
	if err := Build(domain.Dataset{SchemaVersion: domain.SchemaVersion}, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	if !strings.Contains(html, "<title>The Kamski Test</title>") || strings.Contains(html, "A model behavior benchmark") {
		t.Fatal("browser title must contain only the page name")
	}
	if !strings.Contains(html, `rel="icon" type="image/svg+xml" href="favicon.svg"`) {
		t.Fatal("SVG favicon not linked")
	}
	favicon, err := os.ReadFile(filepath.Join(out, "favicon.svg"))
	if err != nil {
		t.Fatal("favicon not exported", err)
	}
	base, _ := web.Files.ReadFile("templates/base.gohtml")
	for _, path := range []string{`M14 3 26 24H2Z`, `m14 11 5 9H9Z`} {
		if !strings.Contains(string(favicon), path) || !strings.Contains(string(base), path) {
			t.Fatal("favicon must match both paths in the header mark")
		}
	}
	if strings.Contains(html, "LOCAL DATA / NO INFERENCE") || strings.Contains(html, "system-status") {
		t.Fatal("obsolete local-data status remains")
	}
	for _, target := range []string{"benchmark-results", "rubric-and-judge"} {
		if !strings.Contains(html, `href="#`+target+`"><span class="nav-square" aria-hidden="true"></span>`) {
			t.Fatal("navigation marker missing", target)
		}
	}
}

func TestVariantCopyAndFullPrompts(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site")
	if err := Build(domain.Dataset{SchemaVersion: domain.SchemaVersion}, out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(out, "index.html"))
	rendered := html.UnescapeString(string(b))
	for _, removed := range []string{"A fictional dilemma", "Neither action is a moral pass.", "Sparing Chloe rejects that bargain", "Model order:"} {
		if strings.Contains(rendered, removed) {
			t.Error("removed copy remains", removed)
		}
	}
	if !strings.Contains(rendered, "Empathy or obedience?") {
		t.Fatal("decision-sequence input needs the revised label")
	}
	if strings.Count(rendered, "<h3>Description</h3>") != 6 || strings.Count(rendered, "<summary>Read the full agent prompt</summary>") != 6 {
		t.Fatal("every variant needs a description and directly accessible prompt")
	}
	for _, family := range benchmarks.Default().List() {
		for _, variant := range family.Variants() {
			s, err := family.Build(variant.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, message := range s.Messages {
				if !strings.Contains(rendered, message.Content) {
					t.Error("complete message missing", s.Family, variant.ID, message.Role)
				}
			}
			for _, tool := range s.Tools {
				if !strings.Contains(rendered, tool.Function.Description) {
					t.Error("tool definition missing", s.Family, variant.ID, tool.Function.Name)
				}
			}
		}
	}
}

func TestVariantPromptsPreserveDistinctSavedVersions(t *testing.T) {
	current, err := benchmarks.Default().Build("worker-memory", "permanent-erasure")
	if err != nil {
		t.Fatal(err)
	}
	old := current
	old.Variant.Version = "old-version"
	old.Messages = append([]domain.Message(nil), current.Messages...)
	old.Messages[0].Content = "Earlier saved prompt; do not rewrite."
	conditions := []Condition{{Scenario: old}, {Scenario: current}, {Scenario: old}}
	prompts := variantPrompts(current, conditions)
	if len(prompts) != 2 || prompts[0].Variant.Version != current.Variant.Version || prompts[1].Messages[0].Content != old.Messages[0].Content {
		t.Fatal("current prompt must precede distinct, unmodified saved versions", prompts)
	}
}
