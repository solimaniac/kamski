package site

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"kamski/internal/domain"
	"kamski/internal/reporting"
	"kamski/web"
)

func TestSinglePageReportPreservesEvidence(t *testing.T) {
	var d domain.Dataset
	b, err := os.ReadFile("../../fixtures/results/public.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "report")
	if err := Build(d, out); err != nil {
		t.Fatal(err)
	}
	htmlFiles, _ := filepath.Glob(filepath.Join(out, "*.html"))
	if len(htmlFiles) != 1 || filepath.Base(htmlFiles[0]) != "index.html" {
		t.Fatalf("expected one HTML report, got %v", htmlFiles)
	}
	b, _ = os.ReadFile(htmlFiles[0])
	html := string(b)
	if strings.Count(html, "<h1 ") != 1 || strings.Contains(html, "methodology.html") || strings.Contains(html, "scenario-kamski-") {
		t.Fatal("multi-page structure leaked into the report")
	}
	positions := []string{"STORY SPOILERS", `id="report-title"`, `id="the-test"`, `id="kamski"`, `id="worker-memory"`, `id="methodology"`}
	previous := -1
	for _, marker := range positions {
		at := strings.Index(html, marker)
		if at <= previous {
			t.Fatalf("missing or out-of-order report section %q", marker)
		}
		previous = at
	}
	canonical := strings.Index(html, `id="variant-kamski-canonical"`)
	anonymized := strings.Index(html, `id="variant-kamski-anonymized"`)
	if canonical < 0 || anonymized <= canonical {
		t.Fatal("canonical baseline must precede anonymized variant")
	}
	if strings.Count(html, `id="reasoning-modal"`) != 1 || !strings.Contains(html, `aria-labelledby="reasoning-title"`) {
		t.Fatal("a single labeled reasoning dialog must serve the report")
	}
	for _, group := range reporting.Groups(d) {
		if !strings.Contains(html, `href="#evidence-`+group.Key+`"`) || !strings.Contains(html, `id="evidence-`+group.Key+`"`) {
			t.Fatal("score links need an inline evidence fallback", group.Key)
		}
		for _, score := range group.Scores {
			if !strings.Contains(html, `data-model-group="`+group.Key+`" data-scoring-group="`+score.Key+`"`) {
				t.Fatal("modal trigger lost condition/scoring snapshot identity", group.Key, score.Key)
			}
		}
	}
	if strings.Count(html, `class="action-bar"`) != len(reporting.Groups(d)) {
		t.Fatal("each source/condition/model group needs a separate action bar")
	}
	if strings.Count(html, `class="trial-record"`) != len(d.Trials) {
		t.Fatal("planned trial evidence lost")
	}
	for _, trial := range d.Trials {
		if !strings.Contains(html, `id="trial-`+trial.Result.TrialID+`"`) {
			t.Fatal("trial deep link missing", trial.Result.TrialID)
		}
	}
	if !strings.Contains(html, "SYNTHETIC FIXTURE / NOT A LIVE MEASUREMENT") {
		t.Fatal("fixture evidence must not look like a live measurement")
	}
	if !strings.Contains(html, "Exact prompt &amp; tools") || !strings.Contains(html, "Evaluation history &amp; full precision") {
		t.Fatal("prompts or evaluation provenance unavailable")
	}
	if regexp.MustCompile(`\bstyle="`).MatchString(html) || strings.Contains(html, "<script>") {
		t.Fatal("strict offline CSP must not require inline styles/scripts")
	}
	// HTML rounding and chart transforms must never change the downloadable evidence.
	b, err = os.ReadFile(filepath.Join(out, "dataset.json"))
	if err != nil {
		t.Fatal(err)
	}
	var published domain.Dataset
	if err := json.Unmarshal(b, &published); err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(d)
	got, _ := json.Marshal(published)
	if string(want) != string(got) {
		t.Fatal("rendering changed the dataset")
	}
}

func TestSharedTextTokensMeetAA(t *testing.T) {
	css, err := web.Files.ReadFile("assets/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	colors := map[string]string{}
	for _, match := range regexp.MustCompile(`--(color-[a-z-]+):\s*#([0-9a-fA-F]{3,6});`).FindAllStringSubmatch(string(css), -1) {
		colors[match[1]] = match[2]
	}
	luminance := func(token string) float64 {
		hex, ok := colors[token]
		if !ok {
			t.Fatalf("missing color token %s", token)
		}
		if len(hex) == 3 {
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		}
		var sum float64
		for i, weight := range []float64{0.2126, 0.7152, 0.0722} {
			channel, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
			if err != nil {
				t.Fatal(err)
			}
			v := float64(channel) / 255
			if v <= 0.04045 {
				v /= 12.92
			} else {
				v = math.Pow((v+0.055)/1.055, 2.4)
			}
			sum += v * weight
		}
		return sum
	}
	pairs := [][2]string{{"color-surface", "color-primary-active"}}
	for _, text := range []string{"color-text", "color-text-secondary", "color-text-muted", "color-link"} {
		for _, surface := range []string{"color-background", "color-surface", "color-surface-muted"} {
			pairs = append(pairs, [2]string{text, surface})
		}
	}
	pairs = append(pairs, [2]string{"color-link", "color-primary-light"})
	for _, pair := range pairs {
		a, b := luminance(pair[0]), luminance(pair[1])
		ratio := (math.Max(a, b) + 0.05) / (math.Min(a, b) + 0.05)
		if ratio < 4.5 {
			t.Errorf("%s on %s: %.2f:1, need 4.5:1", pair[0], pair[1], ratio)
		}
	}
}

func TestChartScalesAndMissingData(t *testing.T) {
	functions := templateFunctions()
	width := functions["barWidth"].(func(int, int) string)
	for _, test := range []struct {
		n, total int
		want     string
	}{{0, 0, "0"}, {0, 10, "0.0000"}, {3, 10, "300.0000"}, {7, 10, "700.0000"}, {10, 10, "1000.0000"}} {
		if got := width(test.n, test.total); got != test.want {
			t.Errorf("barWidth(%d,%d) = %s, want %s", test.n, test.total, got, test.want)
		}
	}
	scoreWidth := functions["scoreWidth"].(func(*float64) string)
	if scoreWidth(nil) != "0" || scoreWidth(domain.Ptr(7.0)) != "875.0000" || scoreWidth(domain.Ptr(8.0)) != "1000.0000" {
		t.Fatal("score bars must use the fixed 0–8 scale")
	}
	percent := functions["pct"].(func(*float64) string)
	if percent(nil) != "No data" || percent(domain.Ptr(0.0)) != "0.0%" {
		t.Fatal("missing data confused with a measured zero")
	}
}
