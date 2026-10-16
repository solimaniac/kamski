package domain

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
	return Config{
		Version:     SchemaVersion,
		Repetitions: 10,
		Concurrency: 1,
		Agent: AgentSettings{
			Temperature: .7,
			MaxTokens:   4096,
			ToolChoice:  "auto",
			ToolOrder:   "balanced",
			Provider:    ProviderPolicy{RequireParameters: true},
		},
		Judge:     JudgeSettings{Model: "typesafe/jev-1.14", Rubric: "v1"},
		OutputDir: ".benchmark-data",
	}
}
