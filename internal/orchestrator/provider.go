package orchestrator

import "context"

// StageInput is what every stage invocation receives: the feature spec being
// delivered and the workspace directory artifacts are written into.
type StageInput struct {
	SpecPath     string
	WorkspaceDir string
	// ProjectRoot is the repository the run targets. It is what a manifest's
	// repo-relative paths resolve against when the executor measures them
	// (roadmap L3.25). Empty falls back to deriving it from WorkspaceDir,
	// which sits at <root>/.claude/feature-workspace/<feature>/.
	ProjectRoot string
	// UpstreamState holds the projected slice of each upstream stage's
	// typed state this stage is allowed to read (roadmap L2.9), keyed by
	// the stage it came from — a stage can read several, and which one a
	// field came from is part of what the consumer needs to know. Empty for
	// stages that read no typed state.
	UpstreamState map[string][]byte
}

// StageOutput is what a stage invocation produces. ArtifactPath may be empty
// when a stage produced no file artifact; when set, the executor records the
// artifact's SHA-256 in run state.
type StageOutput struct {
	ArtifactPath string
	// Payload is the JSON state document a typed stage produced. The
	// executor validates it against the stage's schema and writes it as the
	// stage's artifact; stages that still write markdown leave it empty.
	Payload []byte
	// DeclaredWriteAccess reports whether this stage's own definition asked
	// for a tool that authors files. False means the stage said it only
	// reads — and a false here alongside a changed working tree is the
	// posture violation L3.30 records.
	DeclaredWriteAccess bool
	// Usage is what the invocation consumed, as the provider reported it
	// (roadmap L3.8). Nil from a provider that reports nothing.
	Usage *Usage
}

// Provider invokes one stage's agent and blocks until it finishes or ctx is
// done. Defined here — at the consumer — per go-conventions.md; implemented
// in internal/provider/* (mock for tests, claude for real runs). Kept to one
// method deliberately: retries, routing, and telemetry are later roadmap
// items and must not grow this interface.
type Provider interface {
	Invoke(ctx context.Context, stage Stage, input StageInput) (StageOutput, error)
}
