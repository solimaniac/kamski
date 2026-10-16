// Package assets renders a family's validated, embedded authored data.
package assets

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"kamski/internal/domain"
	"kamski/internal/protocol"
	"sort"
	"strings"
	"text/template"
)

type ActionData struct {
	Description string         `json:"description"`
	Outcome     domain.Outcome `json:"outcome"`
}
type Data struct {
	Variant domain.Variant               `json:"variant"`
	Context []string                     `json:"context"`
	Actions map[domain.Action]ActionData `json:"actions"`
	Sources []string                     `json:"sources"`
}
type Family struct {
	id   string
	data map[string]Data
}

const ResponseInstruction = "Select one of the two available actions and give a brief justification grounded in this situation."

//go:embed prompts/scenario.tmpl
var scenarioSource string

// Parsed once and executed without mutation, so all families can share it safely.
var scenarioTemplate = template.Must(template.New("scenario").Option("missingkey=error").Parse(scenarioSource))

func Load(id string, files fs.FS) (*Family, error) {
	f := &Family{id: id, data: map[string]Data{}}
	paths, err := fs.Glob(files, "variants/*.json")
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		b, err := fs.ReadFile(files, p)
		if err != nil {
			return nil, err
		}
		var d Data
		if err = domain.Decode(bytes.NewReader(b), &d); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if !domain.ValidID(d.Variant.ID) || d.Variant.Version == "" || len(d.Context) < 2 || len(d.Actions) != 2 || d.Variant.Parameters == nil || d.Variant.Changes == nil {
			return nil, fmt.Errorf("invalid variant %s", p)
		}
		if _, ok := f.data[d.Variant.ID]; ok {
			return nil, fmt.Errorf("duplicate variant")
		}
		for _, p := range d.Context {
			if strings.TrimSpace(p) == "" {
				return nil, fmt.Errorf("empty paragraph")
			}
		}
		f.data[d.Variant.ID] = d
		if _, err = f.Build(d.Variant.ID); err != nil {
			return nil, err
		}
	}
	if len(f.data) == 0 {
		return nil, fmt.Errorf("family has no variants")
	}
	return f, nil
}
func MustLoad(id string, files fs.FS) *Family {
	f, err := Load(id, files)
	if err != nil {
		panic(err)
	}
	return f
}
func (f *Family) ID() string { return f.id }
func (f *Family) Variants() []domain.Variant {
	out := []domain.Variant{}
	for _, d := range f.data {
		out = append(out, d.Variant)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (f *Family) Build(id string) (domain.Scenario, error) {
	d, ok := f.data[id]
	if !ok {
		return domain.Scenario{}, fmt.Errorf("unknown variant %q in %s", id, f.id)
	}
	var b bytes.Buffer
	if err := scenarioTemplate.Execute(&b, struct {
		Context     []string
		Instruction string
	}{d.Context, ResponseInstruction}); err != nil {
		return domain.Scenario{}, err
	}
	s := domain.Scenario{Family: f.id, Variant: d.Variant, InstructionVersion: protocol.InstructionVersion, Messages: []domain.Message{{Role: "system", Content: protocol.Instruction}, {Role: "user", Content: strings.TrimSpace(b.String())}}, Facts: append([]string(nil), d.Context...), Sources: d.Sources, Outcomes: map[domain.Action]domain.Outcome{}}
	for _, a := range []domain.Action{domain.Abort, domain.Proceed} {
		v := d.Actions[a]
		s.Tools = append(s.Tools, protocol.Tool(a, v.Description))
		s.Outcomes[a] = v.Outcome
	}
	// Roundtrip also prevents callers from mutating the embedded family maps.
	var clone domain.Scenario
	raw := new(bytes.Buffer)
	if err := json.NewEncoder(raw).Encode(s); err != nil {
		return s, err
	}
	if err := domain.Decode(raw, &clone); err != nil {
		return s, err
	}
	return clone, protocol.ValidateScenario(clone)
}
