package skilpel

import "time"

type Config struct {
	Root         string         `yaml:"root" json:"root"`
	Workspace    string         `yaml:"workspace" json:"workspace"`
	Baseline     bool           `yaml:"baseline" json:"baseline"`
	Provider     string         `yaml:"provider" json:"provider"`
	Target       string         `yaml:"target" json:"target"`
	Judge        string         `yaml:"judge" json:"judge"`
	BaseURL      string         `yaml:"baseUrl" json:"baseUrl"`
	APIKeyEnv    string         `yaml:"apiKeyEnv" json:"apiKeyEnv"`
	Skills       []string       `yaml:"skills" json:"skills"`
	EvalIDs      []string       `yaml:"evalIds" json:"evalIds"`
	MinPass      float64        `yaml:"minPass" json:"minPass"`
	MinDelta     float64        `yaml:"minDelta" json:"minDelta"`
	TargetParams map[string]any `yaml:"targetParams" json:"targetParams"`
	JudgeParams  map[string]any `yaml:"judgeParams" json:"judgeParams"`
}

type Skill struct {
	Name        string
	RelPath     string
	Dir         string
	SkillMD     string
	Description string
	Body        string
	Evals       []EvalCase
}

type EvalCase struct {
	ID             string
	Name           string
	Prompt         string
	ExpectedOutput string
	Files          []string
	Assertions     []string
	Params         map[string]any
}

type Mode string

const (
	WithSkill    Mode = "with_skill"
	WithoutSkill Mode = "without_skill"
)

type RunResult struct {
	Skill       string           `json:"skill"`
	SkillPath   string           `json:"skill_path"`
	EvalID      string           `json:"eval_id,omitempty"`
	EvalName    string           `json:"eval_name,omitempty"`
	EvalSlug    string           `json:"eval_slug"`
	ModeResults map[Mode]ModeRun `json:"modes"`
}

type ModeRun struct {
	Output   string       `json:"output"`
	Timing   Timing       `json:"timing"`
	Grading  Grading      `json:"grading"`
	Artifact string       `json:"artifact"`
	Prompt   PromptRecord `json:"prompt"`
}

type PromptRecord struct {
	System string `json:"system,omitempty"`
	User   string `json:"user"`
	Judge  string `json:"judge,omitempty"`
}

type Timing struct {
	DurationMS  int64 `json:"duration_ms"`
	TotalTokens int   `json:"total_tokens"`
}

type Grading struct {
	AssertionResults []AssertionResult `json:"assertion_results"`
	Summary          GradeSummary      `json:"summary"`
}

type AssertionResult struct {
	Text     string `json:"text"`
	Passed   bool   `json:"passed"`
	Evidence string `json:"evidence"`
}

type GradeSummary struct {
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Total    int     `json:"total"`
	PassRate float64 `json:"pass_rate"`
}

type Summary struct {
	Passed       int            `json:"passed"`
	Failed       int            `json:"failed"`
	Skills       []SkillSummary `json:"skills"`
	Workspace    string         `json:"workspace"`
	Gates        GateSummary    `json:"gates"`
	GateFailures []string       `json:"gate_failures,omitempty"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  time.Time      `json:"completed_at"`
}

type SkillSummary struct {
	Skill            string  `json:"skill"`
	RelPath          string  `json:"rel_path"`
	Evals            int     `json:"evals"`
	Passed           int     `json:"passed"`
	Failed           int     `json:"failed"`
	WithSkillPass    float64 `json:"with_skill_pass_rate"`
	WithoutSkillPass float64 `json:"without_skill_pass_rate,omitempty"`
	Delta            float64 `json:"delta,omitempty"`
	BenchmarkPath    string  `json:"benchmark_path"`
}

type GateSummary struct {
	MinPass  float64 `json:"min_pass"`
	MinDelta float64 `json:"min_delta"`
	Baseline bool    `json:"baseline"`
	Passed   bool    `json:"passed"`
}
