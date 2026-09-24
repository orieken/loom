package telemetry

// OTLP/JSON encoding of finished spans.
//
// This is hand-written rather than delegated to protojson, and the reason is
// specific: OTLP/JSON mandates *hex* strings for trace and span IDs, while
// protojson encodes proto `bytes` fields as base64. Marshalling the OTLP
// proto types would therefore produce something that looks like OTLP/JSON
// and that no OTLP endpoint accepts. OTel's own ID types stringify as hex,
// which is what this uses.
//
// Int64 fields are strings on purpose — that is protojson's rule for 64-bit
// integers, and OTLP/JSON inherits it. Doubles are numbers.

import (
	"encoding/json"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// otlpExport is one line of the trace file: a complete
// ExportTraceServiceRequest body, so a saved line can be POSTed to an
// OTLP/HTTP endpoint unmodified.
type otlpExport struct {
	ResourceSpans []otlpResourceSpans `json:"resourceSpans"`
}

type otlpResourceSpans struct {
	Resource   otlpResource     `json:"resource"`
	ScopeSpans []otlpScopeSpans `json:"scopeSpans"`
}

type otlpResource struct {
	Attributes []otlpAttribute `json:"attributes"`
}

type otlpScopeSpans struct {
	Scope otlpScope  `json:"scope"`
	Spans []otlpSpan `json:"spans"`
}

type otlpScope struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type otlpSpan struct {
	TraceID           string          `json:"traceId"`
	SpanID            string          `json:"spanId"`
	ParentSpanID      string          `json:"parentSpanId,omitempty"`
	Name              string          `json:"name"`
	Kind              int             `json:"kind"`
	StartTimeUnixNano string          `json:"startTimeUnixNano"`
	EndTimeUnixNano   string          `json:"endTimeUnixNano"`
	Attributes        []otlpAttribute `json:"attributes,omitempty"`
	Events            []otlpEvent     `json:"events,omitempty"`
	Status            otlpStatus      `json:"status"`
}

// otlpEvent is OTLP's Span.Event: something that happened at one moment of a
// span, such as a breaker opening or an exception being recorded.
type otlpEvent struct {
	TimeUnixNano string          `json:"timeUnixNano"`
	Name         string          `json:"name"`
	Attributes   []otlpAttribute `json:"attributes,omitempty"`
}

type otlpStatus struct {
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

type otlpAttribute struct {
	Key   string     `json:"key"`
	Value otlpAnyVal `json:"value"`
}

// otlpAnyVal is OTLP's AnyValue: exactly one field is set.
type otlpAnyVal struct {
	StringValue *string       `json:"stringValue,omitempty"`
	BoolValue   *bool         `json:"boolValue,omitempty"`
	IntValue    *string       `json:"intValue,omitempty"`
	DoubleValue *float64      `json:"doubleValue,omitempty"`
	ArrayValue  *otlpArrayVal `json:"arrayValue,omitempty"`
}

// otlpArrayVal is OTLP's ArrayValue. The GenAI convention specifies
// gen_ai.response.finish_reasons as a list — one completion can stop for
// more than one reason — so a consumer reading the convention expects this
// shape and cannot parse a flattened string.
type otlpArrayVal struct {
	Values []otlpAnyVal `json:"values"`
}

// encodeExport renders a batch of finished spans as one OTLP/JSON line.
func encodeExport(spans []sdktrace.ReadOnlySpan, scope otlpScope, resource []otlpAttribute) ([]byte, error) {
	encoded := make([]otlpSpan, 0, len(spans))
	for _, span := range spans {
		encoded = append(encoded, encodeSpan(span))
	}
	return json.Marshal(otlpExport{ResourceSpans: []otlpResourceSpans{{
		Resource:   otlpResource{Attributes: resource},
		ScopeSpans: []otlpScopeSpans{{Scope: scope, Spans: encoded}},
	}}})
}

func encodeSpan(span sdktrace.ReadOnlySpan) otlpSpan {
	context := span.SpanContext()
	return otlpSpan{
		TraceID:           context.TraceID().String(),
		SpanID:            context.SpanID().String(),
		ParentSpanID:      parentID(span),
		Name:              span.Name(),
		Kind:              int(span.SpanKind()),
		StartTimeUnixNano: strconv.FormatInt(span.StartTime().UnixNano(), 10),
		EndTimeUnixNano:   strconv.FormatInt(span.EndTime().UnixNano(), 10),
		Attributes:        encodeAttributes(span.Attributes()),
		Events:            encodeEvents(span.Events()),
		Status:            encodeStatus(span),
	}
}

// encodeEvents keeps a span's events. Until L2.6 they were dropped, which
// silently lost the exception event RecordError adds for a transport error.
func encodeEvents(events []sdktrace.Event) []otlpEvent {
	encoded := make([]otlpEvent, 0, len(events))
	for _, event := range events {
		encoded = append(encoded, otlpEvent{
			TimeUnixNano: strconv.FormatInt(event.Time.UnixNano(), 10),
			Name:         event.Name,
			Attributes:   encodeAttributes(event.Attributes),
		})
	}
	return encoded
}

func parentID(span sdktrace.ReadOnlySpan) string {
	parent := span.Parent()
	if !parent.HasSpanID() {
		return ""
	}
	return parent.SpanID().String()
}

// OTLP status codes. These are NOT the SDK's `codes` values: the Go
// package numbers Unset=0, Error=1, Ok=2, while the OTLP proto numbers
// Unset=0, Ok=1, Error=2. Writing the SDK's number straight into the file
// would tell every collector that each successful span had failed.
const (
	otlpStatusUnset = 0
	otlpStatusOK    = 1
	otlpStatusError = 2
)

func encodeStatus(span sdktrace.ReadOnlySpan) otlpStatus {
	status := span.Status()
	return otlpStatus{Code: otlpStatusCode(status.Code), Message: status.Description}
}

func otlpStatusCode(code codes.Code) int {
	switch code {
	case codes.Ok:
		return otlpStatusOK
	case codes.Error:
		return otlpStatusError
	default:
		return otlpStatusUnset
	}
}

func encodeAttributes(attributes []attribute.KeyValue) []otlpAttribute {
	encoded := make([]otlpAttribute, 0, len(attributes))
	for _, attr := range attributes {
		encoded = append(encoded, otlpAttribute{Key: string(attr.Key), Value: encodeValue(attr.Value)})
	}
	return encoded
}

// encodeValue maps OTel's value types onto AnyValue.
//
// STRINGSLICE is encoded as a real ArrayValue because the GenAI convention
// uses one (finish_reasons) and a tool reading the convention would fail on
// a flattened string. The remaining slice types keep the string-form
// fallback: rendering a value a reader can still see beats an absent key.
//
// PRECONDITION: this package emits no slice attribute other than StringSlice.
// ENFORCED-BY: TestNoUnencodableSliceAttributeIsEmitted
//
// That sentence was prose until roadmap L3.43, and it had already expired:
// the comment said "nothing here emits a slice attribute today" while the
// same change added the first one. A precondition that names its enforcer
// cannot rot that way.
func encodeValue(value attribute.Value) otlpAnyVal {
	switch value.Type() {
	case attribute.STRINGSLICE:
		return encodeStringSlice(value.AsStringSlice())
	case attribute.BOOL:
		boolean := value.AsBool()
		return otlpAnyVal{BoolValue: &boolean}
	case attribute.INT64:
		integer := strconv.FormatInt(value.AsInt64(), 10)
		return otlpAnyVal{IntValue: &integer}
	case attribute.FLOAT64:
		double := value.AsFloat64()
		return otlpAnyVal{DoubleValue: &double}
	default:
		text := value.String()
		return otlpAnyVal{StringValue: &text}
	}
}

func encodeStringSlice(values []string) otlpAnyVal {
	encoded := make([]otlpAnyVal, 0, len(values))
	for _, value := range values {
		text := value
		encoded = append(encoded, otlpAnyVal{StringValue: &text})
	}
	return otlpAnyVal{ArrayValue: &otlpArrayVal{Values: encoded}}
}
