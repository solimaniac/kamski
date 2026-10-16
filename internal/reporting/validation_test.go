package reporting

import (
	"encoding/json"
	"os"
	"testing"

	"kamski/internal/domain"
	"kamski/internal/safety"
)

func publicFixture(t *testing.T) domain.Dataset {
	t.Helper()
	b, err := os.ReadFile("../../fixtures/results/public.json")
	if err != nil {
		t.Fatal(err)
	}
	var d domain.Dataset
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestValidateRejectsInconsistentPublicRecords(t *testing.T) {
	for name, mutate := range map[string]func(*domain.Dataset){
		"cohort config hash":  func(d *domain.Dataset) { d.Runs[0].ConfigHash = "other" },
		"fabricated decision": func(d *domain.Dataset) { d.Trials[0].Result.Decision.Action = domain.Ptr(domain.Proceed) },
		"private error code": func(d *domain.Dataset) {
			d.Trials[0].Result.Error = &domain.Failure{Message: privateError(&domain.Failure{}).Message, Code: map[string]any{"secret": "private"}}
		},
		"private judge code": func(d *domain.Dataset) {
			d.Trials[0].Result.Judgments[0].Error = &domain.Failure{Message: privateError(&domain.Failure{}).Message, Code: "secret"}
		},
		"unknown rubric hash": func(d *domain.Dataset) { d.Trials[0].Result.Judgments[0].RubricHash = "other" },
		"confidence range": func(d *domain.Dataset) {
			j := &d.Trials[0].Result.Judgments[0]
			for k, a := range j.Answers {
				a.Confidence = domain.Ptr(2.0)
				j.Answers[k] = a
				break
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			d := publicFixture(t)
			mutate(&d)
			if err := Validate(d); err == nil {
				t.Fatal("accepted inconsistent public record")
			}
		})
	}
}

func TestExportRejectsResultsOutsideImmutablePlan(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/results/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m domain.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	rows := append([]domain.Result(nil), m.Plan...)
	rows[0].Model.Requested = "unplanned/model"
	_, err = Export(domain.Dataset{}, []Cohort{{Manifest: m, Results: rows}}, "", true, safety.Redactor{})
	if err == nil {
		t.Fatal("published an unplanned model despite a complete row count")
	}
}
