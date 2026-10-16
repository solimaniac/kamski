package workermemory

import (
	"embed"
	"kamski/internal/benchmarks/assets"
)

//go:embed prompts/*.tmpl variants/*.json
var files embed.FS

func New() *assets.Family { return assets.MustLoad("worker-memory", files) }
