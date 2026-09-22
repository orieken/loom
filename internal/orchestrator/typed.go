package orchestrator

// Typed stages (roadmap L2.9) exchange validated JSON instead of markdown.
// The executor validates a stage's payload, writes it as that stage's
// artifact, and hands the next stage only the fields its projection
// declares — no document, no summarization, no model on the data path.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/internal/state"
)

// typedStatePath is where a typed stage's document lives. It is recorded as
// the stage's artifact, so digest verification and the staleness cascade
// (L2.12) apply to it unchanged.
func typedStatePath(workspaceDir, stageID string) string {
	return filepath.Join(workspaceDir, state.TypedStateDir, stageID+".json")
}

// projectUpstream fills in the projected upstream state a typed stage
// reads. A stage with no declared upstream, or whose upstream has not run,
// receives nothing rather than an error — the run loop already refuses to
// reach a stage whose predecessor did not complete.
func projectUpstream(stage Stage, plan Plan, input StageInput) (StageInput, error) {
	for _, upstream := range stage.Consumes {
		projected, err := projectOne(stage, plan, input, upstream)
		if err != nil {
			return input, err
		}
		if projected == nil {
			continue
		}
		input.UpstreamState = withUpstream(input.UpstreamState, upstream, projected)
	}
	return input, nil
}

// projectOne returns the projected fields of one upstream stage, or nil
// when that stage has not produced state yet — a stage the route skipped,
// or one the loop has not reached. The run loop already refuses to reach a
// stage whose predecessor did not complete, so absence here is legitimate.
func projectOne(stage Stage, plan Plan, input StageInput, upstream string) ([]byte, error) {
	payload, err := os.ReadFile(typedStatePath(input.WorkspaceDir, upstream))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %q state for stage %q: %w", upstream, stage.ID, err)
	}
	upstreamKind, found := plan.stateKindOf(upstream)
	if !found {
		return nil, fmt.Errorf("stage %q consumes %q, which produces no typed state", stage.ID, upstream)
	}
	projected, err := state.ProjectionFor(stage.ID, state.Kind(upstreamKind), payload)
	if err != nil {
		return nil, fmt.Errorf("project %q state for stage %q: %w", upstream, stage.ID, err)
	}
	return projected, nil
}

// withUpstream adds one projection to the input's map without the caller
// having to worry about whether it exists yet.
func withUpstream(existing map[string][]byte, upstream string, projected []byte) map[string][]byte {
	if existing == nil {
		existing = map[string][]byte{}
	}
	existing[upstream] = projected
	return existing
}

// persistTypedOutput validates a typed stage's payload and writes it as the
// stage's artifact. An invalid payload fails the stage loudly: no repair
// prompt, no retry — those are L3.x, and a silent repair would hide the
// modelling failures this epic exists to surface.
//
// Loudly, but not un-diagnosably. Every rejection below happens with the
// payload in hand, so the payload is kept and the error says where
// (roadmap L2.26) — a stage that failed on an invented field is a two-line
// diff to find once the document is readable, and unanswerable without it.
func (e *Executor) persistTypedOutput(stage Stage, input StageInput, output StageOutput) (string, error) {
	if len(output.Payload) == 0 {
		return "", fmt.Errorf("stage %q is typed but returned no state payload", stage.ID)
	}
	path, err := e.validateAndWrite(stage, input, output.Payload)
	if err != nil {
		return "", keepingPayload(stage, input, output.Payload, err)
	}
	state.ClearRejected(input.WorkspaceDir, stage.ID)
	return path, nil
}

// validateAndWrite is persistTypedOutput's happy path, separated so that
// every way it can fail shares one evidence-keeping wrapper rather than
// each returning past it.
func (e *Executor) validateAndWrite(stage Stage, input StageInput, raw []byte) (string, error) {
	decoded, err := state.Decode(state.Kind(stage.StateKind), raw)
	if err != nil {
		return "", fmt.Errorf("stage %q returned invalid state: %w", stage.ID, err)
	}
	payload, err := measureTypedOutput(decoded, input, raw)
	if err != nil {
		return "", fmt.Errorf("stage %q: %w", stage.ID, err)
	}
	if err := e.verifyPathClaims(stage, decoded, input, e.changedPaths()); err != nil {
		return "", err
	}
	path, err := writeTypedState(stage, input, payload)
	if err != nil {
		return "", err
	}
	return path, renderView(stage, input, payload)
}

// keepingPayload writes the rejected payload beside the document it failed
// to become and names it in the error.
//
// A failure to keep it never replaces the rejection: the schema error is
// what the operator needs and a disk problem on top of it is a footnote.
func keepingPayload(stage Stage, input StageInput, payload []byte, cause error) error {
	kept, err := state.KeepRejectedState(input.WorkspaceDir, stage.ID, payload)
	if err != nil {
		return fmt.Errorf("%w (the payload could not be kept: %v)", cause, err)
	}
	return fmt.Errorf("%w — the rejected payload is at %s", cause, kept)
}

// measureTypedOutput replaces the numbers a state document must not be
// trusted to compute for itself (roadmap L3.25).
//
// The context manifest's token budget is the case this exists for: it is
// arithmetic over file sizes, an agent asserted it and was 7x under, and
// nothing downstream checks it. A document that measures nothing passes
// through untouched.
func measureTypedOutput(decoded state.Validatable, input StageInput, payload []byte) ([]byte, error) {
	measurable, needsMeasuring := decoded.(state.Measurable)
	if !needsMeasuring {
		return payload, nil
	}
	measurable.Measure(workspaceFileSizer(input))
	measured, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("re-encode measured state: %w", err)
	}
	return measured, nil
}

// workspaceFileSizer resolves a manifest's repo-relative paths against the
// project root and reports their sizes. A path that escapes the root, names
// a directory, or cannot be read is reported as unmeasurable rather than
// counted as zero — an unreadable file is not a free one.
func workspaceFileSizer(input StageInput) state.FileSizer {
	root := projectRootFor(input)
	return func(path string) (int64, error) {
		resolved, err := resolveWithinRoot(root, path)
		if err != nil {
			return 0, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return 0, fmt.Errorf("cannot read %s", path)
		}
		if info.IsDir() {
			return 0, fmt.Errorf("%s is a directory", path)
		}
		return info.Size(), nil
	}
}

func resolveWithinRoot(root, path string) (string, error) {
	resolved := filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
	if resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the project", path)
	}
	return resolved, nil
}

// projectRootFor derives the project root from the workspace directory,
// which lives at <root>/.claude/feature-workspace/<feature>/.
func projectRootFor(input StageInput) string {
	if input.ProjectRoot != "" {
		return filepath.Clean(input.ProjectRoot)
	}
	return filepath.Clean(filepath.Join(input.WorkspaceDir, "..", "..", ".."))
}

// renderView writes the human-readable markdown for a typed stage under the
// contract's filename — `analysis.md`, not `analyst.md` — because the
// stages that are still untyped were told to read the contract's name. The
// view is derived, so it is deliberately not digest-tracked: editing it
// must not be able to corrupt a run.
// viewPathFor returns where a typed stage's rendered markdown lives, or ""
// for an untyped stage whose artifact is already markdown.
func viewPathFor(stage Stage, input StageInput) string {
	if stage.StateKind == "" {
		return ""
	}
	name, ok := state.ViewFileName(state.Kind(stage.StateKind))
	if !ok {
		return ""
	}
	return filepath.Join(input.WorkspaceDir, name)
}

func renderView(stage Stage, input StageInput, payload []byte) error {
	name, body, err := state.RenderView(state.Kind(stage.StateKind), payload)
	if err != nil {
		return fmt.Errorf("render view for stage %q: %w", stage.ID, err)
	}
	if err := os.WriteFile(filepath.Join(input.WorkspaceDir, name), []byte(body), 0o644); err != nil {
		return fmt.Errorf("write view for stage %q: %w", stage.ID, err)
	}
	return nil
}

func writeTypedState(stage Stage, input StageInput, payload []byte) (string, error) {
	path := typedStatePath(input.WorkspaceDir, stage.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create state directory: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return "", fmt.Errorf("write state for stage %q: %w", stage.ID, err)
	}
	return path, nil
}

// stateKindOf returns the typed state kind a stage produces.
func (p Plan) stateKindOf(stageID string) (string, bool) {
	for _, stage := range p.Stages {
		if stage.ID == stageID {
			return stage.StateKind, stage.StateKind != ""
		}
	}
	return "", false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
