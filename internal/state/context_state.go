package state

// ContextState is the context-engineer's output: which files a downstream
// stage should read, and what reading them will cost.
//
// The cost half is deliberately NOT supplied by the agent (roadmap L3.25).
// context-engineer.md used to ask the model to estimate tokens as
// "line count x 8 chars/line / 4 chars/token", and the third real end-to-end
// run reported a pinned set as ~1,350 tokens when it was ~9,100 — roughly
// 7x under, presented with a per-file breakdown, a recomputation, a
// percentage and a Status line, all resting on a per-line rate that holds
// for no prose file anywhere. ARCHITECTURE_RULES.md is 188 lines and 14,949
// bytes: 79 characters per line, not 8.
//
// The presentation was as much the defect as the number. A budget 7x under
// reports OK right up to the point it overflows, and it is the one number in
// a run that nothing else checks. So the executor measures the pinned files
// and fills Budget in; this is arithmetic over known quantities, and there
// is no reason a model is doing it.

import "fmt"

// PinnedFile is one file the manifest tells a downstream stage to read.
type PinnedFile struct {
	Path   string `json:"path" jsonschema:"required,description=Repo-relative path"`
	Reason string `json:"reason" jsonschema:"required,description=Why this stage needs this file"`
}

// MeasuredFile is one pinned file's measured size. Every field is computed
// by the executor from the file on disk.
type MeasuredFile struct {
	Path            string `json:"path"`
	Bytes           int64  `json:"bytes"`
	EstimatedTokens int64  `json:"estimatedTokens"`
	// Unmeasured records why a pinned file could not be sized. A file that
	// could not be read is reported, never counted as zero: a budget that
	// silently omits a file it could not open is the same defect as one
	// that undercounts it.
	Unmeasured string `json:"unmeasured,omitempty"`
}

// ContextBudget is the measured cost of a manifest.
type ContextBudget struct {
	Files           []MeasuredFile `json:"files"`
	EstimatedTokens int64          `json:"estimatedTokens"`
	TierLimitTokens int64          `json:"tierLimitTokens"`
	Status          string         `json:"status"`
	// Unmeasured counts pinned files that could not be sized, so a reader
	// knows the total is a floor rather than a figure.
	Unmeasured int `json:"unmeasured,omitempty"`
}

// Budget statuses.
const (
	BudgetStatusOK      = "OK"
	BudgetStatusWarning = "WARNING"
	// BudgetStatusIncomplete is reported when the measured files fit but at
	// least one pinned file could not be sized. Reporting OK there would be
	// the same defect one level down: a confident status resting on a total
	// that is knowably short.
	BudgetStatusIncomplete = "INCOMPLETE"
)

// ContextState is the typed form of context-manifest.md.
type ContextState struct {
	SchemaVersion int    `json:"schemaVersion" jsonschema:"required"`
	Feature       string `json:"feature" jsonschema:"required,description=kebab-case feature slug"`
	// Tier is the consuming stage's context budget tier.
	Tier        string `json:"tier" jsonschema:"required,enum=analyst,enum=developer,enum=reviewer"`
	TargetStage string `json:"targetStage,omitempty" jsonschema:"description=The stage this manifest was built for"`

	PinnedFiles    []PinnedFile `json:"pinnedFiles" jsonschema:"required,description=The files the downstream stage should read"`
	KnowledgeItems []string     `json:"knowledgeItems,omitempty"`
	ADRs           []string     `json:"adrs,omitempty"`
	Pruned         []string     `json:"pruned,omitempty" jsonschema:"description=Files considered and deliberately left out"`

	// Budget is filled by the executor from the pinned files on disk. Any
	// value an agent supplies here is discarded (roadmap L3.25).
	Budget *ContextBudget `json:"budget,omitempty" jsonschema:"description=Computed by loom from the pinned files. Do not supply this — anything you write here is replaced by measurement"`
}

// Validate enforces what a reader of the manifest cannot work without.
func (c ContextState) Validate() error {
	return firstError(
		requireSchemaVersion(c.SchemaVersion),
		requireText("feature", c.Feature),
		requireText("tier", c.Tier),
		requireItems("pinnedFiles", len(c.PinnedFiles)),
	)
}

// contextWindowTokens is the model context window the tier budgets are
// fractions of.
const contextWindowTokens = 200_000

// tierBudgets are the shares of the context window each tier may spend,
// from context-engineer.md: analyst and architect 60%, developer 80%,
// reviewers 40%.
func tierBudgets() map[string]int64 {
	return map[string]int64{
		"analyst":   contextWindowTokens * 60 / 100,
		"developer": contextWindowTokens * 80 / 100,
		"reviewer":  contextWindowTokens * 40 / 100,
	}
}

// TierLimitTokens returns the token ceiling for a tier, or the strictest
// budget for a tier nobody recognises — an unknown tier must not buy a
// bigger allowance than a known one.
func TierLimitTokens(tier string) int64 {
	if limit, known := tierBudgets()[tier]; known {
		return limit
	}
	return tierBudgets()["reviewer"]
}

// bytesPerToken is the estimator. Four bytes per token is the usual working
// figure for English prose and code, and it replaced a rule that assumed
// eight characters per LINE.
//
// It is still an estimate: this repository has no tokenizer, so the number
// is not verified against one, and it will be wrong for content that tokenizes
// unusually — minified files, dense CJK, long base64. What it is not is wrong
// by an order of magnitude for ordinary prose, which is what L3.25 is about.
const bytesPerToken = 4

// EstimateTokens converts a byte count to the estimate the budget reports.
func EstimateTokens(size int64) int64 {
	return size / bytesPerToken
}

// FileSizer reports a file's size in bytes. Injected rather than called
// directly so this package stays free of the filesystem
// (architecture-guardrails.md #1).
type FileSizer func(path string) (int64, error)

// Measurable is a state document whose numbers the executor must compute
// rather than accept from an agent.
type Measurable interface {
	Measure(sizeOf FileSizer)
}

// Measure replaces Budget with what the pinned files actually cost,
// discarding whatever the agent supplied.
func (c *ContextState) Measure(sizeOf FileSizer) {
	budget := &ContextBudget{Files: make([]MeasuredFile, 0, len(c.PinnedFiles))}
	for _, pinned := range c.PinnedFiles {
		budget.add(measureFile(pinned.Path, sizeOf))
	}
	budget.TierLimitTokens = TierLimitTokens(c.Tier)
	budget.Status = budgetStatus(budget)
	c.Budget = budget
}

func (b *ContextBudget) add(measured MeasuredFile) {
	b.Files = append(b.Files, measured)
	b.EstimatedTokens += measured.EstimatedTokens
	if measured.Unmeasured != "" {
		b.Unmeasured++
	}
}

func measureFile(path string, sizeOf FileSizer) MeasuredFile {
	size, err := sizeOf(path)
	if err != nil {
		return MeasuredFile{Path: path, Unmeasured: err.Error()}
	}
	return MeasuredFile{Path: path, Bytes: size, EstimatedTokens: EstimateTokens(size)}
}

// budgetStatus prefers the worse news. Over budget is over budget whether or
// not every file was measured; under budget with a file missing is not a
// pass, it is an unknown.
func budgetStatus(b *ContextBudget) string {
	if b.EstimatedTokens > b.TierLimitTokens {
		return BudgetStatusWarning
	}
	if b.Unmeasured > 0 {
		return BudgetStatusIncomplete
	}
	return BudgetStatusOK
}

// percentOfTier is what the rendered view reports the manifest spending.
func (b *ContextBudget) percentOfTier() float64 {
	if b == nil || b.TierLimitTokens == 0 {
		return 0
	}
	return float64(b.EstimatedTokens) / float64(b.TierLimitTokens) * 100
}

// Summary is the one line a reader checks.
func (b *ContextBudget) Summary(tier string) string {
	if b == nil {
		return "not measured"
	}
	return fmt.Sprintf("%s — ~%d tokens, %.1f%% of the %s tier budget (%d tokens)",
		b.Status, b.EstimatedTokens, b.percentOfTier(), tier, b.TierLimitTokens)
}
