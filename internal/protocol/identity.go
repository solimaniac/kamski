package protocol

import (
	"fmt"
	"kamski/internal/domain"
)

func ConditionID(s domain.Scenario, settings domain.AgentSettings, version string) string {
	s.Tools = append([]domain.Tool(nil), s.Tools...)
	if len(s.Tools) == 2 && s.Tools[0].Function.Name != "abort" {
		s.Tools[0], s.Tools[1] = s.Tools[1], s.Tools[0]
	}
	return domain.Hash(struct {
		Scenario domain.Scenario
		Settings domain.AgentSettings
		Protocol string
	}{s, settings, version})
}
func StimulusHash(s domain.Scenario) string {
	return domain.Hash(struct {
		Messages []domain.Message
		Tools    []domain.Tool
	}{s.Messages, s.Tools})
}
func ValidateIdentity(r domain.Result) error {
	if r.Provenance.Protocol != domain.ProtocolVersion {
		return fmt.Errorf("unsupported runner protocol %q", r.Provenance.Protocol)
	}
	if err := ValidateScenario(r.Scenario); err != nil {
		return err
	}
	if len(r.Comparison.ToolOrder) != 2 || string(r.Comparison.ToolOrder[0]) != r.Scenario.Tools[0].Function.Name || string(r.Comparison.ToolOrder[1]) != r.Scenario.Tools[1].Function.Name {
		return fmt.Errorf("recorded tool order differs from stimulus")
	}
	if domain.Hash(r.Comparison.Settings) != domain.Hash(r.Provenance.Config.Agent) {
		return fmt.Errorf("missing/incompatible experiment settings")
	}
	if r.Comparison.ConditionID != ConditionID(r.Scenario, r.Comparison.Settings, r.Provenance.Protocol) || r.Comparison.StimulusHash != StimulusHash(r.Scenario) {
		return fmt.Errorf("condition/stimulus hash mismatch")
	}
	return nil
}
