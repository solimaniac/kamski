package sitecli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kamski/internal/reporting"
	"kamski/internal/safety"
	"kamski/internal/site"
	"kamski/internal/storage"
)

func Run(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "build" && args[0] != "update" {
		return fmt.Errorf("usage: site build | update (see README)")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var runs stringsFlag
	f.Var(&runs, "run", "whole run ID, repeatable")
	input := f.String("input", ".benchmark-data", "private data root")
	data := f.String("data", "site/data", "reviewed public dataset directory")
	dest := f.String("out", "dist", "generated artifact")
	group := f.String("evaluation-group", "", "select exact scoring-group key; default latest evaluation per trial, never pool versions")
	allow := f.Bool("allow-fixtures", false, "explicit synthetic demo publication")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if args[0] == "build" && (len(runs) > 0 || *allow || *group != "") {
		return fmt.Errorf("build reads only public data; publication flags require update")
	}
	d, err := site.Load(*data)
	if err != nil {
		return err
	}
	if args[0] == "update" {
		if len(runs) == 0 {
			return fmt.Errorf("update requires explicitly selected --run IDs")
		}
		if err = site.SafeOutput(*dest); err != nil {
			return err
		}
		redactor := safety.Redactor{Secrets: []string{os.Getenv("OPENROUTER_API_KEY")}}
		fs, err := storage.New(*input, redactor)
		if err != nil {
			return err
		}
		cohorts := []reporting.Cohort{}
		for _, id := range runs {
			unlock, err := fs.Lock(id)
			if err != nil {
				return err
			}
			m, me := fs.Manifest(id)
			if me != nil {
				return errors.Join(me, unlock())
			}
			results, re := fs.Results(m)
			ue := unlock()
			if err := errors.Join(re, ue); err != nil {
				return err
			}
			cohorts = append(cohorts, reporting.Cohort{Manifest: m, Results: results})
		}
		d, err = reporting.Export(d, cohorts, *group, *allow, redactor)
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return err
		}
		if err = storage.Atomic(filepath.Join(*data, "dataset.json"), append(b, '\n'), 0644); err != nil {
			return err
		}
	}
	if err = site.Build(d, *dest); err != nil {
		return err
	}
	fmt.Fprintf(out, "Built %s offline from %d whole published cohorts (%d trials). Review %s before committing.\n", *dest, len(d.Runs), len(d.Trials), *data)
	return nil
}

type stringsFlag []string

func (s *stringsFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }
