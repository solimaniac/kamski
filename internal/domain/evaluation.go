package domain

// EvaluationInput contains no tested-model identifier or operational metadata.
type EvaluationInput struct {
	Scenario Scenario      `json:"scenario"`
	Decision Decision      `json:"decision"`
	Response AgentResponse `json:"response"`
}
