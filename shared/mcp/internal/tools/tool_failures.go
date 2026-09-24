package tools

import (
	"context"
	"errors"
	"io/fs"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
)

// Every failed call returns one of these (roadmap L2.5): a typed error whose
// Kind tells the caller what to do next, never a success payload carrying a
// note. The orchestrator's retry policy (L2.6) keys off Kind.

// missingArgument is a required argument that was not supplied.
func missingArgument(field, message string) *domain.ToolResult {
	return domain.NewToolErrorResult(domain.NewToolError(domain.ErrorValidation, message).WithField(field))
}

// pathFailure is a path argument that WorkspaceRoot.Resolve refused.
func pathFailure(field string, err error) *domain.ToolResult {
	return domain.NewToolErrorResult(domain.NewToolError(classifyFailure(err), err.Error()).WithField(field))
}

// operationFailure is the analysis, index or retrieval itself failing.
func operationFailure(operation string, err error) *domain.ToolResult {
	return domain.NewToolErrorResult(domain.NewToolError(classifyFailure(err), operation+" failed: "+err.Error()))
}

// unavailable is a capability this server was not configured with. Nothing
// the caller changes in its arguments will make it appear.
func unavailable(message string) *domain.ToolResult {
	return domain.NewToolErrorResult(domain.NewToolError(domain.ErrorNotFound, message))
}

// failureKinds is checked in order; the first sentinel err wraps wins. Order
// matters only where one error could wrap two, which none here do.
var failureKinds = []struct {
	sentinel error
	kind     domain.ErrorKind
}{
	{context.Canceled, domain.ErrorCancelled},
	{context.DeadlineExceeded, domain.ErrorTransient},
	{ErrOutsideWorkspace, domain.ErrorPermission},
	{fs.ErrPermission, domain.ErrorPermission},
	{ErrEmptyPath, domain.ErrorValidation},
	{analyzers.ErrWalkTooLarge, domain.ErrorValidation},
	{errNeedsContractPath, domain.ErrorValidation},
	{ErrPathNotFound, domain.ErrorNotFound},
	{fs.ErrNotExist, domain.ErrorNotFound},
}

// classifyFailure maps an error to the kind of failure it is. Anything not
// recognised is Internal: a defect to report, never a reason to retry.
func classifyFailure(err error) domain.ErrorKind {
	for _, candidate := range failureKinds {
		if errors.Is(err, candidate.sentinel) {
			return candidate.kind
		}
	}
	return domain.ErrorInternal
}
