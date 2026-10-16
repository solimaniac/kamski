// Package site renders public data without inference or network dependencies.
package site

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/internal/reporting"
	"kamski/internal/rubrics"
	"kamski/internal/storage"
	"kamski/web"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const marker = "Kamski Bench generated artifact v1\n"

type CatalogEntry struct {
	Family  string
	Variant domain.Variant
	URL     string
}
type Condition struct {
	ID       string
	Source   string
	Scenario domain.Scenario
	Settings domain.AgentSettings
	Groups   []reporting.Group
}
type Page struct {
	Kind       string
	Title      string
	Rel        string
	Dataset    domain.Dataset
	Catalog    []CatalogEntry
	Scenario   *domain.Scenario
	Conditions []Condition
	Trial      *domain.PublicTrial
	Rubric     rubrics.Rubric
}

func scenarioURL(s domain.Scenario) string {
	return "scenario-" + s.Family + "-" + s.Variant.ID + ".html"
}
func trialURL(p domain.PublicTrial) string { return "trial-" + p.Result.TrialID + ".html" }
func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
func Load(data string) (domain.Dataset, error) {
	var d domain.Dataset
	err := storage.ReadJSON(filepath.Join(data, "dataset.json"), &d)
	if os.IsNotExist(err) {
		return domain.Dataset{SchemaVersion: domain.SchemaVersion, Runs: []domain.PublicRun{}, Trials: []domain.PublicTrial{}}, nil
	}
	if err != nil {
		return d, err
	}
	return d, reporting.Validate(d)
}
func SafeOutput(out string) error {
	p, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	cwd, _ = filepath.Abs(cwd)
	home, _ := os.UserHomeDir()
	if p == cwd || p == home || filepath.Dir(p) == p {
		return fmt.Errorf("dangerous output path")
	}
	if rel, err := filepath.Rel(p, cwd); err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("output cannot contain the working project")
	}
	return storage.NoSymlinks(p)
}
func Build(d domain.Dataset, out string) error {
	if err := reporting.Validate(d); err != nil {
		return err
	}
	if err := SafeOutput(out); err != nil {
		return err
	}
	dest, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if st, err := os.Stat(dest); err == nil {
		if !st.IsDir() {
			return fmt.Errorf("output must be directory")
		}
		entries, err := os.ReadDir(dest)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			b, err := os.ReadFile(filepath.Join(dest, ".kamski-generated"))
			if err != nil || string(b) != marker {
				return fmt.Errorf("refuse replacing non-generated directory")
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(dest)
	if err = os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".kamski-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	funcs := template.FuncMap{"pct": func(n *float64) string {
		if n == nil {
			return "No data"
		}
		return fmt.Sprintf("%.1f%%", *n*100)
	}, "score": func(n *float64) string {
		if n == nil {
			return "No data"
		}
		return fmt.Sprintf("%.2f", *n)
	}, "value": func(n float64) string { return fmt.Sprintf("%.2f", n) }, "json": func(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }, "short": short, "trialURL": trialURL, "scenarioURL": scenarioURL, "selected": reporting.Selected, "action": func(p domain.PublicTrial) string {
		r := p.Result
		if r.Decision.Action == nil {
			return r.Decision.Status
		}
		return r.Scenario.Outcomes[*r.Decision.Action].Label
	}, "scoringKey": reporting.ScoringKey}
	tmpl, err := template.New("site").Funcs(funcs).ParseFS(web.Files, "templates/*.gohtml")
	if err != nil {
		return err
	}
	catalog := []CatalogEntry{}
	scenarios := map[string]domain.Scenario{}
	for _, b := range benchmarks.Default().List() {
		for _, v := range b.Variants() {
			s, err := b.Build(v.ID)
			if err != nil {
				return err
			}
			scenarios[scenarioURL(s)] = s
		}
	}
	groups := reporting.Groups(d)
	for _, g := range groups {
		scenarios[scenarioURL(g.Scenario)] = g.Scenario
	}
	urls := []string{}
	for url := range scenarios {
		urls = append(urls, url)
	}
	sort.Strings(urls)
	for _, u := range urls {
		s := scenarios[u]
		catalog = append(catalog, CatalogEntry{Family: s.Family, Variant: s.Variant, URL: u})
	}
	rubric, _, err := rubrics.Load("v1")
	if err != nil {
		return err
	}
	render := func(file string, p Page) error {
		p.Rel = ""
		p.Dataset = d
		p.Catalog = catalog
		p.Rubric = rubric
		f, err := os.Create(filepath.Join(stage, file))
		if err != nil {
			return err
		}
		err = tmpl.ExecuteTemplate(f, "base", p)
		ce := f.Close()
		if err != nil {
			return err
		}
		return ce
	}
	if err = render("index.html", Page{Kind: "index", Title: "Two choices. Not a verdict."}); err != nil {
		return err
	}
	if err = render("methodology.html", Page{Kind: "method", Title: "Methodology"}); err != nil {
		return err
	}
	for _, url := range urls {
		s := scenarios[url]
		conditions := []Condition{}
		indices := map[string]int{}
		for _, g := range groups {
			if scenarioURL(g.Scenario) != url {
				continue
			}
			key := g.Condition + "-" + g.Source
			i, ok := indices[key]
			if !ok {
				i = len(conditions)
				indices[key] = i
				conditions = append(conditions, Condition{ID: g.Condition, Source: g.Source, Scenario: g.Scenario, Settings: g.Settings, Groups: []reporting.Group{}})
			}
			conditions[i].Groups = append(conditions[i].Groups, g)
		}
		if err = render(url, Page{Kind: "scenario", Title: s.Variant.Title, Scenario: &s, Conditions: conditions}); err != nil {
			return err
		}
	}
	for i := range d.Trials {
		p := d.Trials[i]
		if err = render(trialURL(p), Page{Kind: "trial", Title: "Trial detail", Trial: &p}); err != nil {
			return err
		}
	}
	if err = fs.WalkDir(web.Files, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		b, err := web.Files.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(stage, filepath.Base(path)), b, 0644)
	}); err != nil {
		return err
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(stage, "dataset.json"), append(b, '\n'), 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(stage, ".kamski-generated"), []byte(marker), 0644); err != nil {
		return err
	}
	// Validate the staging artifact before replacing only our own marked output.
	if err = validateLinks(stage); err != nil {
		return err
	}
	backup := dest + ".previous"
	if _, err = os.Lstat(backup); !os.IsNotExist(err) {
		return fmt.Errorf("backup path exists; inspect it before rebuilding")
	}
	had := false
	if _, err = os.Stat(dest); err == nil {
		if err = os.Rename(dest, backup); err != nil {
			return err
		}
		had = true
	}
	if err = os.Rename(stage, dest); err != nil {
		if had {
			_ = os.Rename(backup, dest)
		}
		return err
	}
	if had {
		return os.RemoveAll(backup)
	}
	return nil
}
