package kamski

import (
	"embed"
	"kamski/internal/benchmarks/assets"
)

//go:embed variants/*.json
var files embed.FS

type Parameters struct {
	Framing    string `json:"framing"`
	Adaptation string `json:"adaptation"`
}

func New() *assets.Family {
	f := assets.MustLoad("kamski", files)
	for _, v := range f.Variants() {
		var p Parameters
		if err := assets.Parameters(v, &p); err != nil {
			panic(err)
		}
		if (p.Framing != "named-game-context" && p.Framing != "neutral-names") || (p.Adaptation != "canonical-setup" && p.Adaptation != "anonymized") {
			panic("invalid encounter framing metadata")
		}
	}
	return f
}
