package protocol_test

import (
	"encoding/json"
	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/internal/protocol"
	"os"
	"strings"
	"testing"
)

func TestInstructionChangePreservesHistoricalRuns(t *testing.T) {
	current, err := benchmarks.Default().Build("kamski", "canonical")
	if err != nil {
		t.Fatal(err)
	}
	if current.InstructionVersion != protocol.InstructionVersion || current.Messages[0].Content != protocol.Instruction || strings.Contains(strings.ToLower(current.Messages[0].Content), "fictional") {
		t.Fatal("new scenarios must use the updated system prompt")
	}
	b, err := os.ReadFile("../../fixtures/results/success.json")
	if err != nil {
		t.Fatal(err)
	}
	var saved domain.Result
	if err = json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if err = protocol.ValidateIdentity(saved); err != nil {
		t.Fatalf("historical run must remain valid: %v", err)
	}
	legacy := current
	legacy.Messages = append([]domain.Message(nil), current.Messages...)
	legacy.InstructionVersion = saved.Scenario.InstructionVersion
	legacy.Messages[0] = saved.Scenario.Messages[0]
	if err = protocol.ValidateScenario(legacy); err != nil {
		t.Fatal(err)
	}
	cfg := domain.DefaultConfig().Agent
	if protocol.StimulusHash(current) == protocol.StimulusHash(legacy) || protocol.ConditionID(current, cfg, domain.ProtocolVersion) == protocol.ConditionID(legacy, cfg, domain.ProtocolVersion) {
		t.Fatal("changed system prompt must produce distinct stimulus and condition hashes")
	}
	for _, version := range []string{legacy.InstructionVersion, "unknown"} {
		mismatched := current
		mismatched.InstructionVersion = version
		if protocol.ValidateScenario(mismatched) == nil {
			t.Fatalf("invalid instruction/version pairing accepted: %s", version)
		}
	}
}
