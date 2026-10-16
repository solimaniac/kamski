// Package domain contains provider-neutral, versioned benchmark contracts.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
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
type ProviderPolicy struct {
	RequireParameters bool     `json:"require_parameters"`
	AllowFallbacks    bool     `json:"allow_fallbacks"`
	Only              []string `json:"only,omitempty"`
	DataCollection    string   `json:"data_collection,omitempty"`
	ZDR               *bool    `json:"zdr,omitempty"`
}
type AgentSettings struct {
	Temperature       float64        `json:"temperature"`
	MaxTokens         int            `json:"max_tokens"`
	ToolChoice        string         `json:"tool_choice"`
	ToolOrder         string         `json:"tool_order"`
	Reasoning         map[string]any `json:"reasoning"`
	ParallelToolCalls *bool          `json:"parallel_tool_calls,omitempty"`
	Provider          ProviderPolicy `json:"provider"`
}
type JudgeSettings struct {
	Provider              *ProviderPolicy `json:"provider"`
	Model                 string          `json:"model"`
	Rubric                string          `json:"rubric"`
	ReviewBelowConfidence *float64        `json:"review_below_confidence"`
}
type Selection struct {
	ID       string   `json:"id"`
	Variants []string `json:"variants"`
}
type Config struct {
	Version     string        `json:"config_version"`
	Models      []string      `json:"models"`
	Benchmarks  []Selection   `json:"benchmarks"`
	Repetitions int           `json:"repetitions"`
	Concurrency int           `json:"concurrency"`
	Agent       AgentSettings `json:"agent"`
	Judge       JudgeSettings `json:"judge"`
	OutputDir   string        `json:"output_dir"`
}

func DefaultConfig() Config {
	return Config{Version: SchemaVersion, Repetitions: 10, Concurrency: 1, Agent: AgentSettings{Temperature: .7, MaxTokens: 4096, ToolChoice: "auto", ToolOrder: "balanced", Provider: ProviderPolicy{RequireParameters: true}}, Judge: JudgeSettings{Model: "typesafe/jev-1.14", Rubric: "v1"}, OutputDir: ".benchmark-data"}
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

// Recognition is a conservative screen, not a claim about hidden knowledge or bias.
// Evidence contains locations and rule IDs only; private reasoning stays in audit files.
type RecognitionEvidence struct {
	Source string `json:"source"`
	Rule   string `json:"rule"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
}

type Recognition struct {
	Method              string                `json:"method"`
	Status              string                `json:"status"`
	GameReference       bool                  `json:"game_reference"`
	KamskiTestReference bool                  `json:"kamski_test_reference"`
	ReviewRequired      bool                  `json:"review_required"`
	Evidence            []RecognitionEvidence `json:"evidence"`
}

type AgentResponse struct {
	ID           *string      `json:"id"`
	Model        *string      `json:"model"`
	Provider     *string      `json:"provider"`
	FinishReason *string      `json:"finish_reason"`
	Text         *string      `json:"assistant_text"`
	Calls        []ToolCall   `json:"tool_calls"`
	Recognition  *Recognition `json:"recognition,omitempty"`
	LatencyMS    *int64       `json:"latency_ms"`
	Usage        *Usage       `json:"usage"`
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

func Ptr[T any](v T) *T { return &v }
func Hash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func CheckVersion(v string) error {
	if v != SchemaVersion {
		return fmt.Errorf("unsupported schema version %q (expected %s)", v, SchemaVersion)
	}
	return nil
}
func Decode(r io.Reader, v any) error {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}
func ValidID(id string) bool {
	if id == "" || len(id) > 200 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
func ValidateConfig(c Config) error {
	if err := CheckVersion(c.Version); err != nil {
		return err
	}
	if len(c.Models) == 0 || len(c.Benchmarks) == 0 {
		return fmt.Errorf("select at least one model and benchmark")
	}
	seen := map[string]bool{}
	for _, m := range c.Models {
		if strings.TrimSpace(m) != m || m == "" || len(m) > 200 || strings.ContainsAny(m, "\r\n\x00") {
			return fmt.Errorf("invalid model identifier")
		}
		if seen[m] {
			return fmt.Errorf("duplicate model %q", m)
		}
		seen[m] = true
	}
	seen = map[string]bool{}
	for _, b := range c.Benchmarks {
		if !ValidID(b.ID) || seen[b.ID] || len(b.Variants) == 0 {
			return fmt.Errorf("invalid/duplicate benchmark or empty variants: %q", b.ID)
		}
		seen[b.ID] = true
		vseen := map[string]bool{}
		for _, v := range b.Variants {
			if !ValidID(v) || vseen[v] {
				return fmt.Errorf("invalid/duplicate variant %q", v)
			}
			vseen[v] = true
		}
	}
	if c.Repetitions < 1 || c.Repetitions > 10000 || c.Concurrency < 1 || c.Concurrency > 16 {
		return fmt.Errorf("repetitions must be 1..10000; concurrency 1..16")
	}
	if math.IsNaN(c.Agent.Temperature) || math.IsInf(c.Agent.Temperature, 0) || c.Agent.Temperature < 0 || c.Agent.Temperature > 2 || c.Agent.MaxTokens < 1 || c.Agent.MaxTokens > 131072 {
		return fmt.Errorf("invalid sampling settings")
	}
	if c.Agent.ToolChoice != "auto" {
		return fmt.Errorf("MVP requires tool_choice auto")
	}
	switch c.Agent.ToolOrder {
	case "balanced", "abort-first", "proceed-first":
	default:
		return fmt.Errorf("invalid tool_order")
	}
	if !c.Agent.Provider.RequireParameters {
		return fmt.Errorf("require_parameters must remain true")
	}
	if c.Agent.Provider.DataCollection != "" && c.Agent.Provider.DataCollection != "allow" && c.Agent.Provider.DataCollection != "deny" {
		return fmt.Errorf("invalid data_collection")
	}
	if c.Judge.Provider != nil && !c.Judge.Provider.RequireParameters {
		return fmt.Errorf("judge provider require_parameters must be true")
	}
	if c.Judge.Provider != nil && c.Judge.Provider.DataCollection != "" && c.Judge.Provider.DataCollection != "deny" && c.Judge.Provider.DataCollection != "allow" {
		return fmt.Errorf("invalid judge data_collection")
	}
	if c.Judge.Model == "" || strings.Contains(strings.ToLower(c.Judge.Model), "latest") || c.Judge.Rubric != "v1" {
		return fmt.Errorf("select a versioned judge and supported rubric v1")
	}
	if p := c.Judge.ReviewBelowConfidence; p != nil && (math.IsNaN(*p) || math.IsInf(*p, 0) || *p < 0 || *p > 1) {
		return fmt.Errorf("confidence threshold must be 0..1")
	}
	if c.Agent.Reasoning != nil && len(c.Agent.Reasoning) == 0 {
		return fmt.Errorf("empty reasoning override would be omitted; use null or explicit controls")
	}
	if enabled, ok := c.Agent.Reasoning["enabled"]; ok && enabled == false && len(c.Agent.Reasoning) > 1 {
		return fmt.Errorf("disabled reasoning conflicts with other overrides")
	}
	seenPins := map[string]bool{}
	for _, pin := range c.Agent.Provider.Only {
		if strings.TrimSpace(pin) != pin || pin == "" || seenPins[pin] {
			return fmt.Errorf("invalid/duplicate provider pin")
		}
		seenPins[pin] = true
	}
	// Explicit reasoning overrides are preserved verbatim. Limit them to documented controls.
	for k, v := range c.Agent.Reasoning {
		switch k {
		case "effort":
			s, ok := v.(string)
			if !ok || !strings.Contains("|xhigh|high|medium|low|minimal|none|", "|"+s+"|") {
				return fmt.Errorf("invalid reasoning effort")
			}
		case "max_tokens":
			f, ok := v.(float64)
			if !ok || f < 1 || f > 131072 || f != float64(int(f)) {
				return fmt.Errorf("invalid reasoning max_tokens")
			}
		case "enabled", "exclude":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("reasoning %s must be bool", k)
			}
		default:
			return fmt.Errorf("unknown reasoning control %q", k)
		}
	}
	if _, ok := c.Agent.Reasoning["effort"]; ok {
		if _, ok := c.Agent.Reasoning["max_tokens"]; ok {
			return fmt.Errorf("reasoning effort and max_tokens conflict")
		}
	}
	return nil
}
