package rubrics

import (
	"bytes"
	"embed"
	"fmt"
	"kamski/internal/domain"
)

//go:embed rubrics/*.json
var files embed.FS

type Criterion struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Levels []string `json:"levels"`
}
type Rubric struct {
	Version  string      `json:"version"`
	Title    string      `json:"title"`
	Criteria []Criterion `json:"criteria"`
}

func Load(version string) (Rubric, string, error) {
	var r Rubric
	if version != "v1" {
		return r, "", fmt.Errorf("unknown rubric %q", version)
	}
	b, err := files.ReadFile("rubrics/" + version + ".json")
	if err != nil {
		return r, "", err
	}
	if err = domain.Decode(bytes.NewReader(b), &r); err != nil {
		return r, "", err
	}
	return r, domain.Hash(r), nil
}
