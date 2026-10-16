package site

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallOutputRestoresPreviousArtifactOnFailure(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "artifact")
	if err := os.Mkdir(dest, 0755); err != nil {
		t.Fatal(err)
	}
	previous := filepath.Join(dest, "previous.txt")
	if err := os.WriteFile(previous, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := installOutput(filepath.Join(root, "missing-stage"), dest); err == nil {
		t.Fatal("installed missing stage")
	}
	contents, err := os.ReadFile(previous)
	if err != nil || string(contents) != "keep" {
		t.Fatalf("previous artifact not restored: %q, %v", contents, err)
	}
	if _, err := os.Stat(dest + ".previous"); !os.IsNotExist(err) {
		t.Fatalf("backup not restored to destination: %v", err)
	}
}

func TestInstallOutputRefusesExistingBackup(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	dest := filepath.Join(root, "artifact")
	for _, dir := range []string{stage, dest, dest + ".previous"} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := installOutput(stage, dest); err == nil {
		t.Fatal("overwrote an existing backup")
	}
	for _, dir := range []string{stage, dest, dest + ".previous"} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("modified %s despite backup conflict: %v", dir, err)
		}
	}
}
