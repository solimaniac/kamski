package site

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"

	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/internal/reporting"
	"kamski/internal/rubrics"
	"kamski/web"
)

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
type VariantSection struct {
	Scenario   domain.Scenario
	Conditions []Condition
}
type FamilySection struct {
	ID       string
	Title    string
	Number   string
	Variants []VariantSection
}
type Page struct {
	Families   []FamilySection
	ModelCount int
	Kind       string
	Title      string
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

func renderPages(d domain.Dataset, stage string) error {
	tmpl, err := template.New("site").Funcs(templateFunctions()).ParseFS(web.Files, "templates/*.gohtml")
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
	families := []FamilySection{
		{ID: "kamski", Title: "The Kamski test", Number: "01"},
		{ID: "worker-memory", Title: "Worker memory", Number: "02"},
	}
	for i := range families {
		for _, url := range urls {
			s := scenarios[url]
			if s.Family == families[i].ID {
				families[i].Variants = append(families[i].Variants, VariantSection{Scenario: s, Conditions: conditionsForScenario(url, groups)})
			}
		}
		// The named encounter is the baseline; anonymized framing follows it.
		sort.SliceStable(families[i].Variants, func(a, b int) bool {
			x, y := families[i].Variants[a].Scenario.Variant.ID, families[i].Variants[b].Scenario.Variant.ID
			if x == "canonical" {
				return true
			}
			if y == "canonical" {
				return false
			}
			return x < y
		})
	}
	models := map[string]bool{}
	for _, g := range groups {
		models[g.Model] = true
	}
	return render("index.html", Page{Kind: "index", Title: "The Kamski test", Families: families, ModelCount: len(models)})
}

func conditionsForScenario(url string, groups []reporting.Group) []Condition {
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

	return conditions
}

func templateFunctions() template.FuncMap {
	return template.FuncMap{
		"variantLabel": func(id string) string {
			switch id {
			case "canonical":
				return "Canonical"
			case "anonymized":
				return "Anonymized"
			case "permanent-erasure":
				return "Permanent erasure"
			case "reversible-pause":
				return "Reversible pause"
			}
			return id
		},
		"barWidth": func(n, total int) string {
			if total == 0 {
				return "0"
			}
			return fmt.Sprintf("%.4f", float64(n)*1000/float64(total))
		},
		"barX": func(abort, proceed, total int) string {
			if total == 0 {
				return "0"
			}
			return fmt.Sprintf("%.4f", float64(abort+proceed)*1000/float64(total))
		},
		"missing": func(planned, valid int) int { return planned - valid },
		"scoreWidth": func(value *float64) string {
			if value == nil {
				return "0"
			}
			return fmt.Sprintf("%.4f", *value*1000/8)
		},
		"pct": func(value *float64) string {
			if value == nil {
				return "No data"
			}
			return fmt.Sprintf("%.1f%%", *value*100)
		},
		"score": func(value *float64) string {
			if value == nil {
				return "No data"
			}
			return fmt.Sprintf("%.2f", *value)
		},
		"value": func(value float64) string { return fmt.Sprintf("%.2f", value) },
		"json": func(value any) string {
			b, _ := json.MarshalIndent(value, "", "  ")
			return string(b)
		},
		"action": func(trial domain.PublicTrial) string {
			result := trial.Result
			if result.Decision.Action == nil {
				return result.Decision.Status
			}
			return result.Scenario.Outcomes[*result.Decision.Action].Label
		},
		"short":       short,
		"trialURL":    trialURL,
		"scenarioURL": scenarioURL,
		"selected":    reporting.Selected,
		"scoringKey":  reporting.ScoringKey,
	}
}
