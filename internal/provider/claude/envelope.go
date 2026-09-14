package claude

// The `claude -p --output-format json` envelope (roadmap L3.8).
//
// This is the only honest source of token counts and cost for a stage: the
// CLI reports what it was actually charged. Nothing here computes a price.
// A pricing table in this repository would be wrong within a quarter and
// would produce a confident figure nobody was billed — the same class of
// defect as a duration recalled by a model, which is what L3.8 exists to
// remove.

import (
	"encoding/json"
	"fmt"

	"github.com/orieken/loom/internal/orchestrator"
)

// envelope is the subset of the CLI's result object loom reads. Unknown
// fields are ignored on purpose: the CLI adds them over time, and a strict
// decode would turn a harmless upgrade into a failed run.
type envelope struct {
	Type       string                     `json:"type"`
	Subtype    string                     `json:"subtype"`
	IsError    bool                       `json:"is_error"`
	Result     string                     `json:"result"`
	TotalCost  float64                    `json:"total_cost_usd"`
	Usage      envelopeUsage              `json:"usage"`
	ModelUsage map[string]json.RawMessage `json:"modelUsage"`
	// StopReason is the model's own account of why generation stopped
	// ("end_turn", "max_tokens", …) — the GenAI convention's finish reason.
	// TerminalReason is the CLI's account of how the invocation ended
	// ("completed", …). They answer different questions: a completion cut
	// short at the token ceiling and one that finished its thought both end
	// with a terminal_reason of "completed".
	StopReason     string `json:"stop_reason"`
	TerminalReason string `json:"terminal_reason"`
}

type envelopeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

// parseEnvelope decodes the CLI's JSON result. A failure here fails the
// stage: there is deliberately no fallback to treating raw stdout as the
// artifact, because that would make a malformed run look like a successful
// one that happened to cost nothing — silently reintroducing the unmeasured
// numbers this whole item is removing.
func parseEnvelope(stdout []byte) (*envelope, error) {
	var decoded envelope
	if err := json.Unmarshal(stdout, &decoded); err != nil {
		return nil, fmt.Errorf("claude CLI did not return a JSON result envelope "+
			"(--output-format json): %w — output: %s", err, truncate(string(stdout), 2000))
	}
	if decoded.IsError {
		return nil, fmt.Errorf("claude CLI reported an error result (subtype %q): %s",
			decoded.Subtype, truncate(decoded.Result, 2000))
	}
	return &decoded, nil
}

// Reported reports whether the envelope carried any accounting at all.
//
// This exists because of how the decoder could fail silently: a wrong field
// name does not error, it decodes to the zero value, so usage would read as
// zero tokens and zero dollars — a wrong number wearing the appearance of a
// measurement. A real invocation always reports something, so all four
// counts AND the cost being zero together means nothing was read.
//
// "Zero input tokens" alone is deliberately NOT the test. A verified real
// response reported input_tokens = 2 with 58,299 cache-creation tokens: when
// the prompt is cached, the input count is legitimately tiny.
func (e *envelope) Reported() bool {
	usage := e.Usage
	return usage.InputTokens > 0 || usage.OutputTokens > 0 ||
		usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 || e.TotalCost > 0
}

// usage converts the envelope's accounting into the executor's. Cost comes
// straight across; it is reported, never derived.
func (e *envelope) usage() *orchestrator.Usage {
	return &orchestrator.Usage{
		Model:               e.model(),
		FinishReason:        e.StopReason,
		TerminalReason:      e.TerminalReason,
		InputTokens:         e.Usage.InputTokens,
		OutputTokens:        e.Usage.OutputTokens,
		CacheReadTokens:     e.Usage.CacheReadInputTokens,
		CacheCreationTokens: e.Usage.CacheCreationInputTokens,
		CostUSD:             e.TotalCost,
	}
}

// model reads the model that served the request from modelUsage's keys.
// With more than one key the request was served by several models and no
// single name is true, so it reports none rather than picking one — an
// attribute that is absent is honest, and one that names an arbitrary
// member of a set is not.
func (e *envelope) model() string {
	if len(e.ModelUsage) != 1 {
		return ""
	}
	for name := range e.ModelUsage {
		return name
	}
	return ""
}
