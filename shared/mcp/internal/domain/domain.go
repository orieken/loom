// Package domain aliases loom's public transport-free tool abstraction.
// The canonical definitions live in github.com/orieken/loom/tools (public
// embedding API, roadmap D.2); this shim exists so internal packages keep
// their import path while sharing the exact same types with embedders.
// TestDomainImportsOnlyPublicAPI keeps anything else out.
package domain

import "github.com/orieken/loom/tools"

type (
	// Tool aliases tools.Tool.
	Tool = tools.Tool
	// ToolRequest aliases tools.ToolRequest.
	ToolRequest = tools.ToolRequest
	// ToolResult aliases tools.ToolResult.
	ToolResult = tools.ToolResult
	// ContentBlock aliases tools.ContentBlock.
	ContentBlock = tools.ContentBlock
	// ToolRegistration aliases tools.ToolRegistration.
	ToolRegistration = tools.ToolRegistration
	// Registry aliases tools.Registry.
	Registry = tools.Registry
	// RetryClass aliases tools.RetryClass.
	RetryClass = tools.RetryClass
	// PermissionScope aliases tools.PermissionScope.
	PermissionScope = tools.PermissionScope
	// ToolError aliases tools.ToolError.
	ToolError = tools.ToolError
	// ErrorKind aliases tools.ErrorKind.
	ErrorKind = tools.ErrorKind
	// FieldViolation aliases tools.FieldViolation.
	FieldViolation = tools.FieldViolation
)

const (
	// RetryNone aliases tools.RetryNone.
	RetryNone = tools.RetryNone
	// RetryIdempotent aliases tools.RetryIdempotent.
	RetryIdempotent = tools.RetryIdempotent
	// ScopeReadOnly aliases tools.ScopeReadOnly.
	ScopeReadOnly = tools.ScopeReadOnly
	// ScopeWorkspaceWrite aliases tools.ScopeWorkspaceWrite.
	ScopeWorkspaceWrite = tools.ScopeWorkspaceWrite
	// ErrorValidation aliases tools.ErrorValidation.
	ErrorValidation = tools.ErrorValidation
	// ErrorNotFound aliases tools.ErrorNotFound.
	ErrorNotFound = tools.ErrorNotFound
	// ErrorPermission aliases tools.ErrorPermission.
	ErrorPermission = tools.ErrorPermission
	// ErrorTransient aliases tools.ErrorTransient.
	ErrorTransient = tools.ErrorTransient
	// ErrorCancelled aliases tools.ErrorCancelled.
	ErrorCancelled = tools.ErrorCancelled
	// ErrorInternal aliases tools.ErrorInternal.
	ErrorInternal = tools.ErrorInternal
)

// NewTextResult aliases tools.NewTextResult.
func NewTextResult(text string) *ToolResult { return tools.NewTextResult(text) }

// NewErrorResult aliases tools.NewErrorResult.
func NewErrorResult(message string) *ToolResult { return tools.NewErrorResult(message) }

// NewRegistry aliases tools.NewRegistry.
func NewRegistry() *Registry { return tools.NewRegistry() }

// NewToolError aliases tools.NewToolError.
func NewToolError(kind ErrorKind, message string) ToolError { return tools.NewToolError(kind, message) }

// NewToolErrorResult aliases tools.NewToolErrorResult.
func NewToolErrorResult(toolError ToolError) *ToolResult { return tools.NewToolErrorResult(toolError) }
