package app

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"kamski/internal/domain"
)

func gitProvenance() domain.Provenance {
	p := domain.Provenance{}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				p.GitCommit = domain.Ptr(setting.Value)
			case "vcs.modified":
				p.Dirty = domain.Ptr(setting.Value == "true")
			}
		}
		if p.GitCommit != nil {
			return p
		}
	}
	// go run may lack VCS build stamps. Do not attribute a compiled binary to
	// an unrelated repository merely because the operator launched it there.
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return p
	}
	mod, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(root)), "go.mod"))
	if err != nil || !strings.HasPrefix(strings.ReplaceAll(string(mod), "\r\n", "\n"), "module kamski\n") {
		return p
	}
	b, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err == nil {
		p.GitCommit = domain.Ptr(strings.TrimSpace(string(b)))
	}
	b, err = exec.Command("git", "status", "--porcelain").Output()
	if err == nil {
		p.Dirty = domain.Ptr(len(b) > 0)
	}
	return p
}
func newRunID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "run-" + time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}
