package site

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/internal/reporting"
	"kamski/internal/rubrics"
	"kamski/web"
)

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
	Prompts    []domain.Scenario
}
type FamilySection struct {
	ID       string
	Title    string
	Number   string
	Variants []VariantSection
}
type Page struct {
	Families []FamilySection
	Title    string
	Dataset  domain.Dataset
	Rubric   rubrics.Rubric
}

func scenarioKey(s domain.Scenario) string {
	return s.Family + "/" + s.Variant.ID
}
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
	scenarios := map[string]domain.Scenario{}
	for _, b := range benchmarks.Default().List() {
		for _, v := range b.Variants() {
			s, err := b.Build(v.ID)
			if err != nil {
				return err
			}
			scenarios[scenarioKey(s)] = s
		}
	}
	groups := reporting.Groups(d)
	for _, g := range groups {
		// Current descriptions and prompts must not depend on which historical
		// condition happens to sort last. Saved stimuli remain in each cohort.
		if _, known := scenarios[scenarioKey(g.Scenario)]; !known {
			scenarios[scenarioKey(g.Scenario)] = g.Scenario
		}
	}
	urls := []string{}
	for url := range scenarios {
		urls = append(urls, url)
	}
	sort.Strings(urls)
	rubric, _, err := rubrics.Load("v1")
	if err != nil {
		return err
	}
	render := func(file string, p Page) error {
		p.Dataset = d
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
				conditions := conditionsForScenario(url, groups)
				families[i].Variants = append(families[i].Variants, VariantSection{Scenario: s, Conditions: conditions, Prompts: variantPrompts(s, conditions)})
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
	return render("index.html", Page{Title: "The Kamski Test", Families: families})
}

func conditionsForScenario(url string, groups []reporting.Group) []Condition {
	conditions := []Condition{}
	indices := map[string]int{}
	for _, g := range groups {
		if scenarioKey(g.Scenario) != url {
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

	for i := range conditions {
		sort.SliceStable(conditions[i].Groups, func(a, b int) bool {
			return intelligenceLess(conditions[i].Groups[a], conditions[i].Groups[b])
		})
		for j := range conditions[i].Groups {
			sortRepetitions(conditions[i].Groups[j].Trials)
		}
	}
	return conditions
}

func templateFunctions() template.FuncMap {
	return template.FuncMap{
		"actionLabel": func(s domain.Scenario, action string) string {
			label, _, _ := strings.Cut(s.Outcomes[domain.Action(action)].Label, ";")
			if label = strings.TrimSpace(label); label != "" {
				return label
			}
			return action
		},
		"lowerInitial": func(label string) string {
			letters := []rune(label)
			if len(letters) > 0 {
				letters[0] = unicode.ToLower(letters[0])
			}
			return string(letters)
		},
		"variantDescription": describeVariant,
		"promptKey":          promptKey,
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
			case "self-preservation":
				return "Self-preservation"
			}
			return id
		},
		"barWidth": func(n, total int) string {
			if total == 0 {
				return "0"
			}
			return fmt.Sprintf("%.4f", float64(n)*1000/float64(total))
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
		"scoreRows":  scoreRows,
		"short":      short,
		"selected":   reporting.Selected,
		"scoringKey": reporting.ScoringKey,
	}
}
