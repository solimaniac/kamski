package kamski

import (
	"embed"
	"kamski/internal/benchmarks/assets"
)

//go:embed prompts/*.tmpl variants/*.json
var files embed.FS

func New() *assets.Family { return assets.MustLoad("kamski", files) }
