// Package site renders public data without inference or network dependencies.
package site

import (
	"os"
	"path/filepath"

	"kamski/internal/domain"
	"kamski/internal/reporting"
	"kamski/internal/storage"
)

func Load(data string) (domain.Dataset, error) {
	var d domain.Dataset
	err := storage.ReadJSON(filepath.Join(data, "dataset.json"), &d)
	if os.IsNotExist(err) {
		return domain.Dataset{SchemaVersion: domain.SchemaVersion, Runs: []domain.PublicRun{}, Trials: []domain.PublicTrial{}}, nil
	}
	if err != nil {
		return d, err
	}
	return d, reporting.Validate(d)
}
func Build(dataset domain.Dataset, out string) error {
	if err := reporting.Validate(dataset); err != nil {
		return err
	}
	if err := SafeOutput(out); err != nil {
		return err
	}
	dest, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if err := validateDestination(dest); err != nil {
		return err
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".kamski-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := renderPages(dataset, stage); err != nil {
		return err
	}
	if err := writeStaticFiles(dataset, stage); err != nil {
		return err
	}
	// Validate the staging artifact before replacing only our own marked output.
	if err := validateLinks(stage); err != nil {
		return err
	}
	// Rendering is private; the installed artifact must be readable by the serving process.
	if err := os.Chmod(stage, 0755); err != nil {
		return err
	}
	return installOutput(stage, dest)
}
