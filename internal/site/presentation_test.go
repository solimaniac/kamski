package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if !strings.Contains(html, "<title>The Kamski test</title>") || strings.Contains(html, "A model behavior benchmark") {
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
