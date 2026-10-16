// Package benchmarks explicitly registers available families.
package benchmarks

import (
	"fmt"
	"sort"

	"kamski/internal/benchmarks/kamski"
	"kamski/internal/benchmarks/workermemory"
	"kamski/internal/domain"
)

type Registry struct{ families map[string]domain.Benchmark }

func New(bs ...domain.Benchmark) (*Registry, error) {
	r := &Registry{families: map[string]domain.Benchmark{}}
	for _, b := range bs {
		if b == nil || !domain.ValidID(b.ID()) {
			return nil, fmt.Errorf("invalid family")
		}
		if _, ok := r.families[b.ID()]; ok {
			return nil, fmt.Errorf("duplicate family %q", b.ID())
		}
		r.families[b.ID()] = b
	}
	return r, nil
}
func Default() *Registry {
	r, err := New(kamski.New(), workermemory.New())
	if err != nil {
		panic(err)
	}
	return r
}
func (r *Registry) List() []domain.Benchmark {
	out := []domain.Benchmark{}
	for _, b := range r.families {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}
func (r *Registry) Build(f, v string) (domain.Scenario, error) {
	b, ok := r.families[f]
	if !ok {
		return domain.Scenario{}, fmt.Errorf("unknown benchmark %q", f)
	}
	return b.Build(v)
}
