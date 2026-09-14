package telemetry

// The adapter half of the executor's seam: OpenTelemetry spans built from
// the executor's own typed span descriptions. Every OTel type in this
// repository is under this package, which is what makes
// architecture-guardrails.md #8 structural — the inner layers cannot emit a
// span because they cannot name one.

import (
	"context"
	"sort"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/orieken/loom/internal/orchestrator"
)

// ServiceName is what loom calls itself in the resource attributes.
const ServiceName = "loom"

const semConvSchemaURL = semconv.SchemaURL

// Attribute keys. Loom-specific facts take a `loom.` prefix rather than
// squatting on `gen_ai.`, which is reserved for what that convention
// actually defines — the model call itself, instrumented in Phase B.
const (
	attrPlan          = "loom.plan"
	attrFeature       = "loom.feature"
	attrSpecPath      = "loom.spec.path"
	attrStateFile     = "loom.state.file"
	attrStageID       = "loom.stage.id"
	attrStageAgent    = "loom.stage.agent"
	attrStageSequence = "loom.stage.sequence"
	attrStageIter     = "loom.stage.iteration"
	attrStageGate     = "loom.stage.gate"
	attrStageInternal = "loom.stage.internal"
	attrStatus        = "loom.status"
	attrReason        = "loom.reason"
)

// GenAI semantic convention keys. These are the ones the convention
// actually defines, used only on the invocation span — a generic GenAI
// dashboard reads these and needs to know nothing about loom.
const (
	attrGenAIOperation = "gen_ai.operation.name"
	// The model loom records is the one that ANSWERED, read from the
	// envelope's modelUsage keys. loom passes no --model, so it never
	// expresses a request and has no honest source for
	// gen_ai.request.model — an absent attribute is honest, and one naming
	// a model nobody asked for is worse than missing, because a reader
	// comparing it against a pin would conclude there was no substitution.
	// Recording what served is therefore the limit of what this can say
	// (roadmap L3.43).
	attrGenAIModel         = "gen_ai.response.model"
	attrGenAIFinishReasons = "gen_ai.response.finish_reasons"
	attrTerminalReason     = "loom.provider.terminal_reason"
	attrGenAIInputTokens   = "gen_ai.usage.input_tokens"
	attrGenAIOutputToken   = "gen_ai.usage.output_tokens"
)

// Cache traffic and cost have no GenAI semconv key, so they take the loom
// prefix rather than an invented gen_ai one that a dashboard would misread.
const (
	attrCacheRead     = "loom.usage.cache_read_tokens"
	attrCacheCreation = "loom.usage.cache_creation_tokens"
	attrCostUSD       = "loom.usage.cost_usd"
)

// otelTracer implements orchestrator.Tracer. It is unexported on purpose:
// callers receive it as the interface from Session.Tracer, so a disabled
// session can hand back an untyped nil. Returning a typed nil pointer here
// would produce a non-nil interface holding nil — which panics on first
// use, and did.
type otelTracer struct {
	tracer trace.Tracer
}

var _ orchestrator.Tracer = (*otelTracer)(nil)

// StartRun opens the root span for a whole run.
func (t *otelTracer) StartRun(ctx context.Context, run orchestrator.RunSpan) (context.Context, orchestrator.Span) {
	ctx, span := t.tracer.Start(ctx, "loom.run "+run.Plan, trace.WithAttributes(
		attribute.String(attrPlan, run.Plan),
		attribute.String(attrFeature, run.Feature),
		attribute.String(attrSpecPath, run.SpecPath),
		attribute.String(attrStateFile, run.StateFile),
	))
	return ctx, &otelSpan{span: span}
}

// StartStage opens a child span for one stage execution. A loop's second
// round produces a sibling of the first, not a child — see StageSpan.
func (t *otelTracer) StartStage(ctx context.Context, stage orchestrator.StageSpan) (context.Context, orchestrator.Span) {
	ctx, span := t.tracer.Start(ctx, "loom.stage "+stage.ID, trace.WithAttributes(stageAttributes(stage)...))
	return ctx, &otelSpan{span: span}
}

func stageAttributes(stage orchestrator.StageSpan) []attribute.KeyValue {
	attributes := []attribute.KeyValue{
		attribute.String(attrStageID, stage.ID),
		attribute.Int(attrStageSequence, stage.Sequence),
		attribute.Bool(attrStageInternal, stage.Internal),
	}
	return append(attributes, optionalStageAttributes(stage)...)
}

// optionalStageAttributes omits keys with nothing to say. An absent
// `loom.stage.gate` means ungated; a present empty string would read as a
// gate whose name someone forgot to fill in.
func optionalStageAttributes(stage orchestrator.StageSpan) []attribute.KeyValue {
	attributes := make([]attribute.KeyValue, 0, 3)
	if stage.Agent != "" {
		attributes = append(attributes, attribute.String(attrStageAgent, stage.Agent))
	}
	if stage.Gate != "" {
		attributes = append(attributes, attribute.String(attrStageGate, stage.Gate))
	}
	if stage.Iteration > 0 {
		attributes = append(attributes, attribute.Int(attrStageIter, stage.Iteration))
	}
	return attributes
}

// StartProvider opens the span for one model invocation, beneath its
// stage. Usage attributes arrive at End, because none of them are known
// until the call returns.
func (t *otelTracer) StartProvider(ctx context.Context, invocation orchestrator.ProviderSpan) (context.Context, orchestrator.Span) {
	ctx, span := t.tracer.Start(ctx, invocation.Operation+" "+invocation.Agent,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrGenAIOperation, invocation.Operation),
			attribute.String(attrStageID, invocation.Stage),
			attribute.String(attrStageAgent, invocation.Agent),
		))
	return ctx, &otelSpan{span: span}
}

type otelSpan struct {
	span trace.Span
}

// End records the outcome and closes the span. Only a FAILED status sets
// the error code: a run waiting at a gate, or a stage the router skipped, is
// an intended outcome, and a trace that paints them red would train people
// to ignore red.
func (s *otelSpan) End(outcome orchestrator.SpanOutcome) {
	s.span.SetAttributes(attribute.String(attrStatus, string(outcome.Status)))
	if outcome.Reason != "" {
		s.span.SetAttributes(attribute.String(attrReason, outcome.Reason))
	}
	s.span.SetAttributes(usageAttributes(outcome.Usage)...)
	s.span.SetAttributes(loopAttributes(outcome.LoopOutcomes)...)
	s.recordStatus(outcome)
	s.span.End()
}

// usageAttributes renders reported usage. A nil Usage yields no attributes
// at all: "the provider reported nothing" and "the provider reported zero"
// are different facts, and a span carrying explicit zeros would assert the
// second when only the first is true.
func usageAttributes(usage *orchestrator.Usage) []attribute.KeyValue {
	if usage == nil {
		return nil
	}
	attributes := []attribute.KeyValue{
		attribute.Int64(attrGenAIInputTokens, usage.InputTokens),
		attribute.Int64(attrGenAIOutputToken, usage.OutputTokens),
		attribute.Int64(attrCacheRead, usage.CacheReadTokens),
		attribute.Int64(attrCacheCreation, usage.CacheCreationTokens),
		attribute.Float64(attrCostUSD, usage.CostUSD),
	}
	if usage.Model != "" {
		attributes = append(attributes, attribute.String(attrGenAIModel, usage.Model))
	}
	// finish_reasons is an array in the GenAI convention: one completion can
	// stop for more than one reason. loom reports the one the CLI gives it
	// rather than inventing a second.
	if usage.FinishReason != "" {
		attributes = append(attributes,
			attribute.StringSlice(attrGenAIFinishReasons, []string{usage.FinishReason}))
	}
	if usage.TerminalReason != "" {
		attributes = append(attributes, attribute.String(attrTerminalReason, usage.TerminalReason))
	}
	return attributes
}

// loopAttributes renders how each loop settled, one attribute per loop. The
// key count is bounded by the loops a plan declares, so this stays low
// cardinality however many rounds a loop actually ran.
func loopAttributes(outcomes map[string]string) []attribute.KeyValue {
	if len(outcomes) == 0 {
		return nil
	}
	ids := make([]string, 0, len(outcomes))
	for id := range outcomes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	attributes := make([]attribute.KeyValue, 0, len(ids))
	for _, id := range ids {
		attributes = append(attributes,
			attribute.String("loom.loop."+id+".terminated_by", outcomes[id]))
	}
	return attributes
}

func (s *otelSpan) recordStatus(outcome orchestrator.SpanOutcome) {
	if outcome.Status != orchestrator.StageStatusFailed {
		s.span.SetStatus(codes.Ok, "")
		return
	}
	if outcome.Err != nil {
		s.span.RecordError(outcome.Err)
	}
	s.span.SetStatus(codes.Error, string(outcome.Status))
}
