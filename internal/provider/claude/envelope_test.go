package claude

// Tests against a CAPTURED REAL RESPONSE, not a hand-written approximation
// (roadmap L3.15).
//
// This mattered because of an asymmetric failure mode: a wrong field name in
// the decoder does not error, it decodes to the zero value, so usage would
// read as zero tokens and zero dollars. That is a wrong number wearing the
// appearance of a measurement — the exact defect the telemetry work exists
// to remove — and nothing downstream could tell it from a genuinely cheap
// stage.
//
// testdata/envelope-real.json is one real `claude -p --output-format json`
// response with only the session identifiers redacted. Every field name and
// shape is as the CLI produced it.

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
)

func realEnvelope(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/envelope-real.json")
	if err != nil {
		t.Fatalf("read captured envelope: %v", err)
	}
	return raw
}

func TestDecoderMatchesARealResponse(t *testing.T) {
	result, err := parseEnvelope(realEnvelope(t))
	if err != nil {
		t.Fatalf("the decoder cannot read a real CLI response: %v", err)
	}
	// Every field compared at once: a wrong name decodes to a zero value
	// rather than erroring, so each field needs its own captured expectation.
	// The prompt was cached, which is why input is 2 and cache creation is
	// 58,299 — the number a field-name mistake would silently lose.
	want := orchestrator.Usage{
		Model: "claude-sonnet-5", InputTokens: 2, OutputTokens: 4,
		CacheReadTokens: 0, CacheCreationTokens: 58299, CostUSD: 0.34986,
		// The captured response carries both: end_turn is the model's own
		// reason for stopping, completed is how the CLI's process ended. A
		// completion cut off at the token ceiling would differ in the first
		// and not the second (roadmap L3.43).
		FinishReason: "end_turn", TerminalReason: "completed",
	}
	if got := *result.usage(); got != want {
		t.Errorf("usage = %+v, want the captured %+v", got, want)
	}
	if result.Result != "ok" {
		t.Errorf("result = %q, want the agent's own output", result.Result)
	}
}

// The captured response carries twenty-one top-level fields, most of which
// loom does not read. A strict decode would turn every CLI release into a
// failed pipeline.
func TestARealResponseCarriesFieldsLoomIgnores(t *testing.T) {
	var everything map[string]json.RawMessage
	if err := json.Unmarshal(realEnvelope(t), &everything); err != nil {
		t.Fatalf("captured envelope is not JSON: %v", err)
	}
	if len(everything) < 15 {
		t.Fatalf("captured envelope has only %d fields — the fixture looks hand-written, "+
			"which would defeat the point of capturing it", len(everything))
	}
	if _, err := parseEnvelope(realEnvelope(t)); err != nil {
		t.Errorf("unknown fields broke the decode: %v", err)
	}
}

// The suspicion check. "Zero input tokens" alone is NOT the test: the real
// response above reported input_tokens = 2 with 58,299 cache-creation
// tokens, so a cached prompt legitimately shows a tiny input count.
// L3.22 / AC: an absent usage report is not the same fact as a zero one.
// exemplar: table-driven cases with descriptive names, boundary values on each
// input dimension, and one behaviour asserted per subtest.
func TestReportedDistinguishesUnmeasuredFromCheap(t *testing.T) {
	cases := []struct {
		name string
		json string
		want bool
	}{
		{"nothing at all", `{"type":"result","is_error":false,"result":"x"}`, false},
		{"explicit zeros", `{"type":"result","is_error":false,"result":"x",` +
			`"usage":{"input_tokens":0,"output_tokens":0},"total_cost_usd":0}`, false},
		{"tiny input, large cache creation", `{"type":"result","is_error":false,"result":"x",` +
			`"usage":{"input_tokens":2,"output_tokens":4,"cache_creation_input_tokens":58299}}`, true},
		{"cost but no tokens", `{"type":"result","is_error":false,"result":"x","total_cost_usd":0.01}`, true},
		{"cache reads only", `{"type":"result","is_error":false,"result":"x",` +
			`"usage":{"cache_read_input_tokens":900}}`, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := parseEnvelope([]byte(testCase.json))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := result.Reported(); got != testCase.want {
				t.Errorf("Reported() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestTheRealResponseCountsAsReported(t *testing.T) {
	result, err := parseEnvelope(realEnvelope(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !result.Reported() {
		t.Error("a real successful response was judged to have reported nothing")
	}
}
