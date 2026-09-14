package telemetry_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/telemetry"
)

// otlpFile is the subset of OTLP/JSON these tests assert on. Decoding into
// a struct rather than a map is the assertion that the shape is right.
type otlpFile struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []otlpAttr `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Scope struct {
				Name string `json:"name"`
			} `json:"scope"`
			Spans []otlpSpan `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
}

type otlpSpan struct {
	TraceID           string     `json:"traceId"`
	SpanID            string     `json:"spanId"`
	ParentSpanID      string     `json:"parentSpanId"`
	Name              string     `json:"name"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	EndTimeUnixNano   string     `json:"endTimeUnixNano"`
	Attributes        []otlpAttr `json:"attributes"`
	Status            struct {
		Code int `json:"code"`
	} `json:"status"`
}

type otlpAttr struct {
	Key   string `json:"key"`
	Value struct {
		StringValue *string  `json:"stringValue"`
		BoolValue   *bool    `json:"boolValue"`
		IntValue    *string  `json:"intValue"`
		DoubleValue *float64 `json:"doubleValue"`
	} `json:"value"`
}

// find looks an attribute up without failing. Asserting that something is
// absent needs a lookup that tolerates absence; attribute cannot, by design.
func (s otlpSpan) find(key string) *otlpAttr {
	for _, attr := range s.Attributes {
		if attr.Key == key {
			return &attr
		}
	}
	return nil
}

func (s otlpSpan) attribute(t *testing.T, key string) otlpAttr {
	t.Helper()
	for _, attr := range s.Attributes {
		if attr.Key == key {
			return attr
		}
	}
	t.Fatalf("span %q has no attribute %q", s.Name, key)
	return otlpAttr{}
}

// traceRun starts a session against a temp file, drives one run span with
// one stage span, and returns the decoded file.
func traceRun(t *testing.T, stage orchestrator.StageSpan, outcome orchestrator.SpanOutcome) []otlpSpan {
	t.Helper()
	path := filepath.Join(t.TempDir(), telemetry.TracesFileName)
	session, err := telemetry.Start(telemetry.Options{Version: "test-version", TraceFile: path})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	tracer := session.Tracer()
	ctx, runSpan := tracer.StartRun(context.Background(), orchestrator.RunSpan{Plan: "test-plan", Feature: "widgets"})
	_, stageSpan := tracer.StartStage(ctx, stage)
	stageSpan.End(outcome)
	runSpan.End(orchestrator.SpanOutcome{Status: orchestrator.StageStatusCompleted})
	if err := session.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	return decodeSpans(t, path)
}

func decodeSpans(t *testing.T, path string) []otlpSpan {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	spans := make([]otlpSpan, 0, 2)
	for _, line := range splitLines(content) {
		var decoded otlpFile
		if err := json.Unmarshal(line, &decoded); err != nil {
			t.Fatalf("trace file line is not valid OTLP/JSON: %v\nline: %s", err, line)
		}
		for _, resource := range decoded.ResourceSpans {
			assertResource(t, resource.Resource.Attributes)
			for _, scope := range resource.ScopeSpans {
				if scope.Scope.Name != telemetry.ScopeName {
					t.Errorf("scope name = %q, want %q", scope.Scope.Name, telemetry.ScopeName)
				}
				spans = append(spans, scope.Spans...)
			}
		}
	}
	return spans
}

func assertResource(t *testing.T, attributes []otlpAttr) {
	t.Helper()
	for _, attr := range attributes {
		if attr.Key == "service.name" && attr.Value.StringValue != nil && *attr.Value.StringValue == telemetry.ServiceName {
			return
		}
	}
	t.Errorf("resource attributes %v carry no service.name = %q", attributes, telemetry.ServiceName)
}

func splitLines(content []byte) [][]byte {
	lines := make([][]byte, 0, 2)
	for _, line := range splitOnNewline(content) {
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitOnNewline(content []byte) [][]byte {
	parts := make([][]byte, 0, 2)
	start := 0
	for i, b := range content {
		if b == '\n' {
			parts = append(parts, content[start:i])
			start = i + 1
		}
	}
	return append(parts, content[start:])
}

func findSpan(t *testing.T, spans []otlpSpan, name string) otlpSpan {
	t.Helper()
	for _, span := range spans {
		if span.Name == name {
			return span
		}
	}
	t.Fatalf("no span named %q in %d spans", name, len(spans))
	return otlpSpan{}
}

func TestTraceFileIsSpecCompliantOTLPJSON(t *testing.T) {
	spans := traceRun(t,
		orchestrator.StageSpan{ID: "developer", Agent: "developer", Sequence: 2},
		orchestrator.SpanOutcome{Status: orchestrator.StageStatusCompleted})

	stage := findSpan(t, spans, "loom.stage developer")
	// Hex, not base64 — the whole reason this encoder is hand-written.
	assertHexID(t, stage.TraceID, 16, "traceId")
	assertHexID(t, stage.SpanID, 8, "spanId")
	// int64 fields are strings in OTLP/JSON, per protojson's rule.
	if _, err := strconv.ParseInt(stage.StartTimeUnixNano, 10, 64); err != nil {
		t.Errorf("startTimeUnixNano %q is not a base-10 integer string: %v", stage.StartTimeUnixNano, err)
	}
	sequence := stage.attribute(t, "loom.stage.sequence")
	if sequence.Value.IntValue == nil || *sequence.Value.IntValue != "2" {
		t.Errorf("loom.stage.sequence = %+v, want intValue \"2\"", sequence.Value)
	}
}

func assertHexID(t *testing.T, id string, wantBytes int, label string) {
	t.Helper()
	decoded, err := hex.DecodeString(id)
	if err != nil {
		t.Fatalf("%s %q is not hex: %v", label, id, err)
	}
	if len(decoded) != wantBytes {
		t.Errorf("%s %q decodes to %d bytes, want %d", label, id, len(decoded), wantBytes)
	}
}

func TestStageSpanIsAChildOfTheRunSpan(t *testing.T) {
	spans := traceRun(t,
		orchestrator.StageSpan{ID: "analyst", Agent: "analyst"},
		orchestrator.SpanOutcome{Status: orchestrator.StageStatusCompleted})

	run := findSpan(t, spans, "loom.run test-plan")
	stage := findSpan(t, spans, "loom.stage analyst")
	if stage.ParentSpanID != run.SpanID {
		t.Errorf("stage parentSpanId = %q, want the run span's id %q", stage.ParentSpanID, run.SpanID)
	}
	if stage.TraceID != run.TraceID {
		t.Errorf("stage and run are in different traces: %q vs %q", stage.TraceID, run.TraceID)
	}
}

// Only a genuine failure is red. A stage the router skipped and a run
// waiting at a gate are intended outcomes; painting them as errors would
// train a reader to ignore the colour.
func TestOnlyFailedStatusRecordsATraceError(t *testing.T) {
	cases := []struct {
		name     string
		status   orchestrator.StageStatus
		wantCode int
	}{
		{"completed", orchestrator.StageStatusCompleted, 1},
		{"skipped", orchestrator.StageStatusSkipped, 1},
		{"waiting on a human", orchestrator.StageStatusWaitingApproval, 1},
		{"failed", orchestrator.StageStatusFailed, 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			spans := traceRun(t,
				orchestrator.StageSpan{ID: "developer"},
				orchestrator.SpanOutcome{Status: testCase.status})
			stage := findSpan(t, spans, "loom.stage developer")
			if stage.Status.Code != testCase.wantCode {
				t.Errorf("status code for %s = %d, want %d", testCase.status, stage.Status.Code, testCase.wantCode)
			}
			recorded := stage.attribute(t, "loom.status")
			if recorded.Value.StringValue == nil || *recorded.Value.StringValue != string(testCase.status) {
				t.Errorf("loom.status = %+v, want %q", recorded.Value, testCase.status)
			}
		})
	}
}

// An absent gate must be an absent key, not an empty string that reads like
// a gate whose name someone forgot to fill in.
func TestOptionalStageAttributesAreOmittedNotEmptied(t *testing.T) {
	spans := traceRun(t,
		orchestrator.StageSpan{ID: "analyst"},
		orchestrator.SpanOutcome{Status: orchestrator.StageStatusCompleted})

	stage := findSpan(t, spans, "loom.stage analyst")
	for _, attr := range stage.Attributes {
		switch attr.Key {
		case "loom.stage.gate", "loom.stage.agent", "loom.stage.iteration", "loom.reason":
			t.Errorf("attribute %q should be omitted when it has nothing to say, got %+v", attr.Key, attr.Value)
		}
	}
}

// Telemetry is opt-out locally and opt-in over the network. With both off
// there must be no session at all, so nothing is measured and no file is
// created.
func TestNoExporterConfiguredYieldsNoSession(t *testing.T) {
	t.Setenv(telemetry.EndpointEnvVar, "")
	session, err := telemetry.Start(telemetry.Options{Version: "test-version"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if session != nil {
		t.Fatal("Start returned a session with no exporter configured")
	}
	// The nil session must stay safe to use: this is what the CLI does.
	// Tracer returns the interface, which is the point of the check: a
	// concrete *otelTracer nil compares false to nil, but the same value
	// held as an interface compares true. The executor sees the interface,
	// and that gap is what panicked on the first real run.
	tracer := session.Tracer()
	if tracer != nil {
		t.Errorf("nil session Tracer() = %v, want an untyped nil so the executor disables tracing", tracer)
	}
	if err := session.Shutdown(context.Background()); err != nil {
		t.Errorf("nil session Shutdown: %v", err)
	}
}

func TestTraceFileForPutsTracesBesideRunState(t *testing.T) {
	got := telemetry.TraceFileFor("/tmp/workspace")
	want := filepath.Join("/tmp/workspace", telemetry.TracesFileName)
	if got != want {
		t.Errorf("TraceFileFor = %q, want %q", got, want)
	}
}

// Usage reaches the invocation span under the GenAI semantic convention's
// own keys, so a generic GenAI dashboard reads them without knowing
// anything about loom.
// traceInvocation drives a full run -> stage -> invocation nesting with the
// given usage, and returns every span in the resulting file.
func traceInvocation(t *testing.T, usage *orchestrator.Usage) []otlpSpan {
	t.Helper()
	path := filepath.Join(t.TempDir(), telemetry.TracesFileName)
	session, err := telemetry.Start(telemetry.Options{Version: "test-version", TraceFile: path})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	tracer := session.Tracer()
	ctx, run := tracer.StartRun(context.Background(), orchestrator.RunSpan{Plan: "test-plan"})
	ctx, stage := tracer.StartStage(ctx, orchestrator.StageSpan{ID: "developer"})
	_, invocation := tracer.StartProvider(ctx, orchestrator.ProviderSpan{
		Stage: "developer", Agent: "developer", Operation: orchestrator.GenAIOperationName,
	})
	invocation.End(orchestrator.SpanOutcome{Status: orchestrator.StageStatusCompleted, Usage: usage})
	stage.End(orchestrator.SpanOutcome{Status: orchestrator.StageStatusCompleted})
	run.End(orchestrator.SpanOutcome{Status: orchestrator.StageStatusCompleted})
	if err := session.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	return decodeSpans(t, path)
}

func TestProviderSpanCarriesGenAIUsageAttributes(t *testing.T) {
	spans := traceInvocation(t, &orchestrator.Usage{
		Model: "claude-opus-5", InputTokens: 1200, OutputTokens: 340,
		CacheReadTokens: 800, CacheCreationTokens: 64, CostUSD: 0.0425,
	})
	span := findSpan(t, spans, orchestrator.GenAIOperationName+" developer")
	assertIntAttribute(t, span, "gen_ai.usage.input_tokens", "1200")
	assertIntAttribute(t, span, "gen_ai.usage.output_tokens", "340")
	assertIntAttribute(t, span, "loom.usage.cache_read_tokens", "800")
	model := span.attribute(t, "gen_ai.request.model")
	if model.Value.StringValue == nil || *model.Value.StringValue != "claude-opus-5" {
		t.Errorf("gen_ai.request.model = %+v, want \"claude-opus-5\"", model.Value)
	}
	cost := span.attribute(t, "loom.usage.cost_usd")
	if cost.Value.DoubleValue == nil || *cost.Value.DoubleValue != 0.0425 {
		t.Errorf("loom.usage.cost_usd = %+v, want 0.0425", cost.Value)
	}
	// The invocation is a child of the stage, not of the run.
	if span.ParentSpanID != findSpan(t, spans, "loom.stage developer").SpanID {
		t.Error("invocation span is not a child of its stage span")
	}
}

func assertIntAttribute(t *testing.T, span otlpSpan, key, want string) {
	t.Helper()
	attr := span.attribute(t, key)
	if attr.Value.IntValue == nil || *attr.Value.IntValue != want {
		t.Errorf("%s = %+v, want intValue %q", key, attr.Value, want)
	}
}

// A span with no reported usage must carry no usage attributes at all.
// Explicit zeros would assert a measurement that was never taken.
func TestNoUsageReportedMeansNoUsageAttributes(t *testing.T) {
	spans := traceInvocation(t, nil)

	for _, attr := range findSpan(t, spans, orchestrator.GenAIOperationName+" developer").Attributes {
		if strings.HasPrefix(attr.Key, "gen_ai.usage.") || strings.HasPrefix(attr.Key, "loom.usage.") {
			t.Errorf("attribute %q present with no usage reported — absent and zero are different facts", attr.Key)
		}
	}
}
