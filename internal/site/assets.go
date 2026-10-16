package site

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	"kamski/internal/domain"
	"kamski/web"
)

func writeStaticFiles(d domain.Dataset, stage string) error {
	if err := fs.WalkDir(web.Files, "assets", func(path string, entry fs.DirEntry, err error) error {
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

	return nil
}
