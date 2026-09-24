package telemetry

// Tool-call spans for the MCP server (roadmap L3.8, phase C).
//
// Three properties this file owes the rest of the system: a tool call's
// arguments never leak a secret into a span, never leak content into one,
// and never blow a span up with an unbounded payload. All three are
// enforced here rather than at call sites, because a call site that forgets
// is exactly the failure being prevented.
//
// Content minimisation is an allowlist, not a denylist (guardrail #9,
// roadmap L3.38). A value is recorded verbatim only when the tool declared
// that argument safe; everything else becomes a hash and a length. The
// inversion matters because the tool registry is consumer-extensible: a
// denylist of content-shaped names cannot cover an argument in someone
// else's tool that this package has never heard of, and the failure is
// silent.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// AttributeValueLimit caps how much of any single argument or result
// reaches a span. Values longer than this are truncated with a marker, so a
// reader can tell truncation from a genuinely short value.
const AttributeValueLimit = 512

// TruncationMarker is appended to any value the limit cut short.
const TruncationMarker = "…(truncated)"

// RedactedPlaceholder replaces a value whose key looks secret. The key is
// kept: knowing a token was passed is useful, knowing which token is not.
const RedactedPlaceholder = "[redacted]"

// SaltEnvVar names the environment variable holding the salt for argument
// hashes. Without it no hash is emitted at all — an unsalted hash of a short
// query is recoverable by brute force, so the choice is a salted hash or
// nothing, never a weak one.
const SaltEnvVar = "LOOM_TELEMETRY_SALT"

// HashPrefix marks an attribute value as a salted digest rather than content,
// so a reader never mistakes one for the other.
const HashPrefix = "sha256:"

// hashHexLength is how much of the digest is kept. 16 hex characters is ample
// to correlate repeated inputs and short enough to keep spans small.
const hashHexLength = 16

// secretKeyParts are substrings that make an argument name secret-shaped.
// Matching on the name rather than the value is deliberate — value-shaped
// detection misses the secrets that do not look like secrets, and a name
// match costs nothing when it is wrong.
var secretKeyParts = []string{
	"token", "secret", "password", "passwd", "credential",
	"apikey", "api_key", "authorization", "auth", "private_key", "session",
}

// ToolCall describes one MCP tool invocation. Arguments arrive already
// stringified so this package's signatures stay free of `any`, and so the
// caller owns how its own types render.
type ToolCall struct {
	Name      string
	Arguments map[string]string
	// SafeArguments names the arguments whose values may be recorded
	// verbatim, as declared by the tool itself via tools.SafeArguments.
	// Empty means every value is hashed, which is the safe default for a
	// tool that declared nothing.
	SafeArguments []string
}

// ToolResult is how a tool call ended. It carries the result's shape and
// never its text: a search tool's result body is the corpus it searched, and
// a preview of it is content the span has no business holding.
type ToolResult struct {
	// Bytes is the total size of the result text.
	Bytes int
	// Blocks is how many content blocks the tool returned. Zero blocks on a
	// successful call is the "found nothing" signal worth alerting on.
	Blocks int
	// IsError marks a tool that ran and reported failure, as distinct from
	// one that returned an error to the transport.
	IsError bool
	Err     error
	// ErrorKind is the failure's kind (validation, transient, …), empty on
	// success. A closed set, so it is safe as an attribute (guardrail #9).
	ErrorKind string
	// Attempts is how many times the tool ran; above one, it was retried
	// (roadmap L2.6). Zero means the caller did not count.
	Attempts int
	// Breaker is the tool's circuit breaker state before and after the call.
	// Both empty when the tool has no breaker.
	Breaker BreakerStates
}

// BreakerStates is a circuit breaker's state either side of one call.
type BreakerStates struct {
	Before, After string
}

// StartTool opens a span for one tool call beneath whatever is in ctx —
// the propagated stage span when TRACEPARENT survived the hop from
// `loom run`, and a fresh trace when it did not.
//
// It hangs off Session rather than off a tracer interface because a nil
// Session must be usable: the MCP server runs untraced far more often than
// traced, and its call sites should not each branch on that.
func (s *Session) StartTool(ctx context.Context, call ToolCall) (context.Context, *ToolSpan) {
	if s == nil {
		return ctx, nil
	}
	ctx, span := s.tracer.Start(ctx, "loom.tool "+call.Name,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(toolAttributes(call)...))
	return ctx, &ToolSpan{span: span}
}

// TraceIDs returns the trace and span IDs in ctx, or empty strings when it
// carries no recording span. It exists so log lines can carry correlation
// IDs without the logging package learning what OpenTelemetry is.
func TraceIDs(ctx context.Context) (traceID, spanID string) {
	context := trace.SpanContextFromContext(ctx)
	if !context.IsValid() {
		return "", ""
	}
	return context.TraceID().String(), context.SpanID().String()
}

// ToolSpan is an open tool-call span.
type ToolSpan struct {
	span trace.Span
}

// End closes the span with the tool's result.
func (s *ToolSpan) End(result ToolResult) {
	if s == nil {
		return
	}
	s.span.SetAttributes(
		attribute.Int("loom.tool.result.bytes", result.Bytes),
		attribute.Int("loom.tool.result.blocks", result.Blocks),
		attribute.Bool("loom.tool.is_error", result.IsError),
	)
	s.recordResilience(result)
	s.recordToolStatus(result)
	s.span.End()
}

// recordResilience records what L2.5 and L2.6 add to a call: the kind of
// failure, the attempts it took, and the breaker's state — with an event when
// this call moved the breaker, so an opening shows on the call that opened it.
func (s *ToolSpan) recordResilience(result ToolResult) {
	if result.ErrorKind != "" {
		s.span.SetAttributes(attribute.String("loom.tool.error.kind", result.ErrorKind))
	}
	if result.Attempts > 0 {
		s.span.SetAttributes(attribute.Int("loom.tool.attempts", result.Attempts))
	}
	if result.Breaker.After == "" {
		return
	}
	s.span.SetAttributes(attribute.String("loom.tool.breaker.state", result.Breaker.After))
	if result.Breaker.Before != result.Breaker.After {
		s.span.AddEvent("loom.tool.breaker.state_change", trace.WithAttributes(
			attribute.String("loom.tool.breaker.from", result.Breaker.Before),
			attribute.String("loom.tool.breaker.to", result.Breaker.After)))
	}
}

func (s *ToolSpan) recordToolStatus(result ToolResult) {
	if result.Err == nil && !result.IsError {
		s.span.SetStatus(codes.Ok, "")
		return
	}
	if result.Err != nil {
		s.span.RecordError(result.Err)
	}
	s.span.SetStatus(codes.Error, "tool call failed")
}

func toolAttributes(call ToolCall) []attribute.KeyValue {
	attributes := []attribute.KeyValue{attribute.String("loom.tool.name", call.Name)}
	safe := safeSet(call.SafeArguments)
	for _, key := range sortedKeys(call.Arguments) {
		attributes = append(attributes, argumentAttributes(key, call.Arguments[key], safe)...)
	}
	return attributes
}

// safeSet indexes the tool's declaration for lookup.
func safeSet(names []string) map[string]bool {
	safe := make(map[string]bool, len(names))
	for _, name := range names {
		safe[name] = true
	}
	return safe
}

// argumentAttributes renders one argument. A secret-shaped name is redacted
// whatever the tool declared — a tool cannot opt its own credentials into a
// span. An undeclared name yields a length, and a hash when a salt exists.
func argumentAttributes(key, value string, safe map[string]bool) []attribute.KeyValue {
	if IsSecretKey(key) {
		return []attribute.KeyValue{attribute.String("loom.tool.arg."+key, RedactedPlaceholder)}
	}
	if safe[key] {
		return []attribute.KeyValue{attribute.String("loom.tool.arg."+key, safeValue(value))}
	}
	attributes := []attribute.KeyValue{
		attribute.Int("loom.tool.arg."+key+".length", len([]rune(value))),
	}
	if digest := HashValue(value); digest != "" {
		attributes = append(attributes, attribute.String("loom.tool.arg."+key+".hash", digest))
	}
	return attributes
}

// HashValue returns a salted, truncated digest of value, or "" when no salt
// is configured. Exported so anything else deciding what not to record
// reaches the same answer.
func HashValue(value string) string {
	salt := os.Getenv(SaltEnvVar)
	if salt == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(salt + value))
	return HashPrefix + hex.EncodeToString(sum[:])[:hashHexLength]
}

// sortedKeys makes the attribute order deterministic, so two identical
// calls produce identical spans and a diff of two traces means something.
func sortedKeys(arguments map[string]string) []string {
	keys := make([]string, 0, len(arguments))
	for key := range arguments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// IsSecretKey reports whether an argument name looks like it carries a
// credential. Exported so the same judgement is available to anything else
// that has to decide what not to record.
func IsSecretKey(key string) bool {
	lowered := strings.ToLower(key)
	for _, part := range secretKeyParts {
		if strings.Contains(lowered, part) {
			return true
		}
	}
	return false
}

// safeValue truncates on a rune boundary, so a cut through a multi-byte
// character cannot put invalid UTF-8 into a span.
func safeValue(value string) string {
	if len(value) <= AttributeValueLimit {
		return value
	}
	runes := []rune(value)
	limit := AttributeValueLimit
	if limit > len(runes) {
		limit = len(runes)
	}
	return string(runes[:limit]) + TruncationMarker
}
