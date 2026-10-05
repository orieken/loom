package repodoc

// Violation is one thing the deterministic verifier found wrong with a
// documenter result. It is typed because it is fed back into the documenter's
// next round: a finding the next round cannot act on is not worth recording.
type Violation struct {
	// Source is "verifier" for a Go check or "auditor" for a semantic one.
	Source string `json:"source"`
	// Code names the rule, stable enough to count and to test.
	Code string `json:"code"`
	// Where is the field of the document the problem is in.
	Where   string `json:"where"`
	Message string `json:"message"`
}

// Sources of a Violation.
const (
	SourceVerifier = "verifier"
	SourceAuditor  = "auditor"
)

// Codes the verifier emits.
const (
	CodeUnknownTag         = "unknown-tag"
	CodeUnknownName        = "unknown-name"
	CodeUnknownEntrypoint  = "unknown-entrypoint"
	CodeUnevidencedClaim   = "unevidenced-claim"
	CodeUnknownCitation    = "unknown-citation"
	CodeEmptyCitation      = "empty-citation"
	CodeUnrecordedRead     = "unrecorded-read"
	CodeReadOutsideDir     = "read-outside-directory"
	CodeApproximateAsFact  = "approximate-as-fact"
	CodeInvalidOutput      = "invalid-output"
	CodeAuditorBlockingFix = "auditor-blocking"
)
