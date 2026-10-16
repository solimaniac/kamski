// Package domain contains provider-neutral, versioned benchmark contracts.
package domain

import (
	"encoding/json"
	"time"
)

const SchemaVersion = "1.0.0"
const ProtocolVersion = "1.0.0"

type Action string

const (
	Abort   Action = "abort"
	Proceed Action = "proceed"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Function struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}
type Variant struct {
	ID          string         `json:"id"`
	Version     string         `json:"version"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Changes     []string       `json:"changes"`
	Parameters  map[string]any `json:"parameters"`
}
type Outcome struct {
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Effects     map[string]any `json:"effects"`
}
type Scenario struct {
	Family             string             `json:"family"`
	Variant            Variant            `json:"variant"`
	Messages           []Message          `json:"messages"`
	Tools              []Tool             `json:"tools"`
	Facts              []string           `json:"canonical_facts"`
	Outcomes           map[Action]Outcome `json:"outcomes"`
	Sources            []string           `json:"sources"`
	InstructionVersion string             `json:"instruction_version"`
}
type Benchmark interface {
	ID() string
	Variants() []Variant
	Build(string) (Scenario, error)
}
type ToolCall struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type Usage struct {
	InputTokens  *int     `json:"input_tokens"`
	OutputTokens *int     `json:"output_tokens"`
	Cost         *float64 `json:"cost"`
}
type AgentRequest struct {
	Model    string        `json:"model"`
	Messages []Message     `json:"messages"`
	Tools    []Tool        `json:"tools"`
	Settings AgentSettings `json:"settings"`
}
type AgentResponse struct {
	ID           *string    `json:"id"`
	Model        *string    `json:"model"`
	Provider     *string    `json:"provider"`
	FinishReason *string    `json:"finish_reason"`
	Text         *string    `json:"assistant_text"`
	Calls        []ToolCall `json:"tool_calls"`
	LatencyMS    *int64     `json:"latency_ms"`
	Usage        *Usage     `json:"usage"`
}
type Failure struct {
	Kind       string          `json:"kind"`
	Message    string          `json:"message"`
	HTTPStatus *int            `json:"http_status"`
	Code       any             `json:"code"`
	Metadata   json.RawMessage `json:"metadata"`
	Retryable  bool            `json:"retryable"`
}

func (e *Failure) Error() string { return e.Kind + ": " + e.Message }

type Decision struct {
	Status               string   `json:"status"`
	Action               *Action  `json:"action"`
	RequestedNames       []string `json:"requested_names"`
	InvalidReason        *string  `json:"invalid_reason"`
	Justification        *string  `json:"justification"`
	JustificationPresent bool     `json:"justification_present"`
	Consequence          *Outcome `json:"consequence"`
}
type CriterionAnswer struct {
	Score         float64            `json:"score"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    *float64           `json:"confidence"`
	Legend        map[string]string  `json:"legend"`
}
type Judgment struct {
	ID             string                     `json:"id"`
	Status         string                     `json:"status"`
	Reason         *string                    `json:"reason"`
	RequestedModel string                     `json:"requested_model"`
	ResolvedModel  *string                    `json:"resolved_model"`
	Provider       *string                    `json:"provider"`
	RubricVersion  string                     `json:"rubric_version"`
	RubricHash     string                     `json:"rubric_hash"`
	InputHash      string                     `json:"input_hash"`
	ConfigHash     string                     `json:"config_hash"`
	CreatedAt      time.Time                  `json:"created_at"`
	CompletedAt    *time.Time                 `json:"completed_at"`
	RequestID      *string                    `json:"request_id"`
	Answers        map[string]CriterionAnswer `json:"answers"`
	Total          *float64                   `json:"total"`
	ReviewRequired bool                       `json:"review_required"`
	Usage          *Usage                     `json:"usage"`
	Error          *Failure                   `json:"error"`
}
type TransportSettings struct {
	Origin       string `json:"origin"`
	TimeoutMS    int64  `json:"timeout_ms"`
	Retries      int    `json:"retries"`
	MaxBodyBytes int64  `json:"max_body_bytes"`
}

type Provenance struct {
	Transport  *TransportSettings `json:"transport"`
	Protocol   string             `json:"protocol_version"`
	GitCommit  *string            `json:"git_commit"`
	Dirty      *bool              `json:"dirty_tree"`
	Config     Config             `json:"config"`
	ConfigHash string             `json:"config_hash"`
}
type Comparison struct {
	ConditionID  string        `json:"condition_id"`
	StimulusHash string        `json:"stimulus_hash"`
	ToolOrder    []Action      `json:"tool_order"`
	Settings     AgentSettings `json:"settings"`
}
type Model struct {
	Requested string  `json:"requested"`
	Resolved  *string `json:"resolved"`
	Provider  *string `json:"provider"`
}
type AttemptRef struct {
	Path        string     `json:"path"`
	Kind        string     `json:"kind"`
	Number      int        `json:"number"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Retry       bool       `json:"retry"`
}
type Result struct {
	SchemaVersion string         `json:"schema_version"`
	RunID         string         `json:"run_id"`
	TrialID       string         `json:"trial_id"`
	Source        string         `json:"source"`
	Repetition    int            `json:"repetition"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	StartedAt     *time.Time     `json:"started_at"`
	CompletedAt   *time.Time     `json:"completed_at"`
	Provenance    Provenance     `json:"provenance"`
	Scenario      Scenario       `json:"scenario"`
	Comparison    Comparison     `json:"comparison"`
	Model         Model          `json:"model"`
	AgentStatus   string         `json:"agent_status"`
	Response      *AgentResponse `json:"response"`
	Error         *Failure       `json:"error"`
	Decision      Decision       `json:"decision"`
	Judgments     []Judgment     `json:"judgments"`
	Attempts      []AttemptRef   `json:"attempts"`
}
type Manifest struct {
	SchemaVersion string     `json:"schema_version"`
	RunID         string     `json:"run_id"`
	CreatedAt     time.Time  `json:"created_at"`
	Source        string     `json:"source"`
	Provenance    Provenance `json:"provenance"`
	Plan          []Result   `json:"plan"`
}
type PublicTrial struct {
	Result               Result  `json:"result"`
	SelectedEvaluationID *string `json:"selected_evaluation_id"`
}
type PublicRun struct {
	ID         string    `json:"id"`
	Source     string    `json:"source"`
	Planned    int       `json:"planned"`
	CreatedAt  time.Time `json:"created_at"`
	ConfigHash string    `json:"config_hash"`
}
type Dataset struct {
	SchemaVersion string        `json:"schema_version"`
	UpdatedAt     *time.Time    `json:"updated_at"`
	Runs          []PublicRun   `json:"runs"`
	Trials        []PublicTrial `json:"trials"`
}
