package site

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kamski/internal/storage"
)

const marker = "Kamski Bench generated artifact v1\n"

func SafeOutput(out string) error {
	p, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	home, _ := os.UserHomeDir()
	if p == cwd || p == home || filepath.Dir(p) == p {
		return fmt.Errorf("dangerous output path")
	}
	if rel, err := filepath.Rel(p, cwd); err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("output cannot contain the working project")
	}
	return storage.NoSymlinks(p)
}
func validateDestination(dest string) error {
	if st, err := os.Stat(dest); err == nil {
		if !st.IsDir() {
			return fmt.Errorf("output must be directory")
		}
		entries, err := os.ReadDir(dest)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			b, err := os.ReadFile(filepath.Join(dest, ".kamski-generated"))
			if err != nil || string(b) != marker {
				return fmt.Errorf("refuse replacing non-generated directory")
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return nil
}

func installOutput(stage, dest string) error {
	backup := dest + ".previous"
	if _, err := os.Lstat(backup); !os.IsNotExist(err) {
		return fmt.Errorf("backup path exists; inspect it before rebuilding")
	}
	hadPrevious := false
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, backup); err != nil {
			return err
		}
		hadPrevious = true
	}
	if err := os.Rename(stage, dest); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(backup, dest); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("restore previous artifact from %s: %w", backup, restoreErr))
			}
		}
		return err
	}
	if hadPrevious {
		return os.RemoveAll(backup)
	}
	return nil
}
