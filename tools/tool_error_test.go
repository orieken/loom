package tools_test

import (
	"testing"

	"github.com/orieken/loom/tools"
)

func TestOnlyATransientFailureIsRetryable(t *testing.T) {
	kinds := []tools.ErrorKind{tools.ErrorValidation, tools.ErrorNotFound, tools.ErrorPermission,
		tools.ErrorTransient, tools.ErrorCancelled, tools.ErrorInternal}
	for _, kind := range kinds {
		if got := tools.NewToolError(kind, "m").Retryable; got != (kind == tools.ErrorTransient) {
			t.Errorf("%s: retryable = %v", kind, got)
		}
	}
}

func TestAToolErrorReadsAsKindFieldAndMessage(t *testing.T) {
	plain := tools.NewToolError(tools.ErrorNotFound, "no corpus")
	if plain.Error() != "not_found: no corpus" {
		t.Errorf("Error() = %q", plain.Error())
	}
	named := plain.WithField("docsPath")
	if named.Error() != "not_found: docsPath: no corpus" || plain.Field != "" {
		t.Errorf("Error() = %q, and WithField must not change the original (field %q)", named.Error(), plain.Field)
	}
}

// The typed error and the content an MCP client reads are the same error.
func TestAToolErrorResultCarriesTheErrorTypedAndAsAnEnvelope(t *testing.T) {
	failure := tools.NewToolError(tools.ErrorValidation, "invalid arguments").WithField("query")
	failure.Violations = []tools.FieldViolation{{Field: "query", Problem: "is required"}}
	result := tools.NewToolErrorResult(failure)

	if !result.IsError || result.Error == nil || result.Error.Kind != tools.ErrorValidation {
		t.Fatalf("not a typed failure: %+v", result)
	}
	want := `{"error":{"kind":"validation","message":"invalid arguments","field":"query","retryable":false,` +
		`"violations":[{"field":"query","problem":"is required"}]}}`
	if got := result.Content[0].Text; got != want {
		t.Errorf("envelope:\n  %s\nwant:\n  %s", got, want)
	}
}

// NewErrorResult predates the taxonomy. Its text is unchanged for existing
// embedders, and it is classified, so no IsError result lacks a kind.
func TestTheUntypedErrorResultIsClassifiedAsInternal(t *testing.T) {
	result := tools.NewErrorResult("boom")
	if !result.IsError || result.Content[0].Text != "boom" {
		t.Errorf("text or flag changed: %+v", result)
	}
	if result.Error == nil || result.Error.Kind != tools.ErrorInternal || result.Error.Message != "boom" {
		t.Errorf("error = %+v, want internal \"boom\"", result.Error)
	}
}

func TestATextResultCarriesNoError(t *testing.T) {
	if result := tools.NewTextResult("ok"); result.IsError || result.Error != nil {
		t.Errorf("a success carries a failure: %+v", result)
	}
}
