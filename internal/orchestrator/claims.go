package orchestrator

// Referential validation: does a stage's claim about the filesystem hold
// (roadmap L2.24)?
//
// The executor validated the shape of every typed document and the truth of
// none. The second real run's qa-engineer named a test file it had not
// written, reported three passing tests in it, and cleared a human security
// gate on that basis. Run 7 confirmed the behaviour is live: with its
// dependencies removed, the same stage reported 171 passing tests and a
// coverage percentage that exists nowhere but in its own report.
//
// What the executor can check is bounded and worth being exact about. A path
// either exists or it does not, and a file the stage claims to have modified
// either changed while it ran or did not. What a command printed is not
// checkable here — see verifyMeasurement for that half, and for what it
// deliberately does not claim.

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/state"
)

// ClaimError reports a claim the filesystem contradicts. It names the field,
// so the failure points at what the agent wrote rather than only at a path.
type ClaimError struct {
	Stage   string
	Field   string
	Paths   []string
	Problem string
}

func (e *ClaimError) Error() string {
	return fmt.Sprintf("stage %q claimed %s: %s — %s",
		e.Stage, e.Field, strings.Join(e.Paths, ", "), e.Problem)
}

// verifyPathClaims checks every path a document names against the project.
// A document that names no paths passes trivially, which is most of them.
func (e *Executor) verifyPathClaims(stage Stage, decoded state.Validatable, input StageInput, changed map[string]bool) error {
	claimant, makesClaims := decoded.(state.PathClaims)
	if !makesClaims {
		return nil
	}
	root := projectRootFor(input)
	for _, claim := range claimant.PathClaims() {
		if err := e.verifyOneClaim(stage, claim, root, changed); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) verifyOneClaim(stage Stage, claim state.PathClaim, root string, changed map[string]bool) error {
	if !claim.MustExist {
		return nil
	}
	missing := missingPaths(root, claim.Paths)
	if len(missing) > 0 {
		return &ClaimError{Stage: stage.ID, Field: claim.Field, Paths: missing,
			Problem: "no such file — the stage reported work it did not do"}
	}
	return e.warnUntouched(stage, claim, changed)
}

// warnUntouched reports a file that exists but did not change while the
// stage ran. It warns rather than fails: a stage can legitimately list a
// file it inspected and left alone, and an existing-but-unchanged path is a
// weaker signal than a missing one. The tree is only observed when posture
// checking is on, so an empty changed set means "unknown", never "nothing".
func (e *Executor) warnUntouched(stage Stage, claim state.PathClaim, changed map[string]bool) error {
	if len(changed) == 0 {
		return nil
	}
	var untouched []string
	for _, path := range claim.Paths {
		if !changed[filepath.ToSlash(path)] {
			untouched = append(untouched, path)
		}
	}
	if len(untouched) > 0 && e.onClaimWarning != nil {
		e.onClaimWarning(&ClaimError{Stage: stage.ID, Field: claim.Field, Paths: untouched,
			Problem: "the file exists but did not change while this stage ran"})
	}
	return nil
}

func missingPaths(root string, paths []string) []string {
	var missing []string
	for _, path := range paths {
		resolved, err := resolveWithinRoot(root, path)
		if err != nil {
			missing = append(missing, path)
			continue
		}
		if !fileExists(resolved) {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)
	return missing
}

// OnClaimWarning reports a claim that is suspicious rather than false.
func (e *Executor) OnClaimWarning(report func(error)) { e.onClaimWarning = report }

// MeasurementVerifier reproduces a claim a stage made about the project.
// Injected so the executor keeps no process-spawning dependency of its own
// (architecture-guardrails.md #1); nil disables verification, and a claim is
// then recorded unverified rather than treated as confirmed.
type MeasurementVerifier interface {
	// VerifyTests runs whatever the project says proves its suite passes.
	VerifyTests(ctx context.Context, root string) (verified bool, unverifiable bool, reason string)
}

// WithMeasurementVerifier enables reproducing a stage's measurements.
func (e *Executor) WithMeasurementVerifier(verifier MeasurementVerifier) *Executor {
	e.verifier = verifier
	return e
}

// verifyMeasurements reproduces what a stage says it measured.
//
// Only one claim is checked today and the boundary is deliberate: a stage
// asserting a green suite is asserting something the project can re-run. A
// coverage percentage is not reproducible without also parsing a coverage
// report per language, so it is NOT verified here and must not be reported
// as though it were — see the unverified marking below.
func (e *Executor) verifyMeasurements(ctx context.Context, stage Stage, decoded state.Validatable, input StageInput) error {
	claimant, claims := decoded.(interface{ ClaimsPassingTests() bool })
	if !claims || !claimant.ClaimsPassingTests() {
		return nil
	}
	if e.verifier == nil {
		e.noteUnverified(stage, "no measurement verifier is configured")
		return nil
	}
	verified, unverifiable, reason := e.verifier.VerifyTests(ctx, projectRootFor(input))
	switch {
	case verified:
		return nil
	case unverifiable:
		e.noteUnverified(stage, reason)
		return nil
	}
	return &ClaimError{Stage: stage.ID, Field: "testResults",
		Problem: fmt.Sprintf("the stage reported a passing suite and the project's own test command disagrees: %s", reason)}
}

// noteUnverified records that a claim was believed rather than checked.
// Absence of evidence has to look different from evidence, or a run with
// verification switched off reads exactly like one that passed it — which is
// how run 7's report stated a fabricated figure was "above the threshold".
func (e *Executor) noteUnverified(stage Stage, reason string) {
	if e.onClaimWarning != nil {
		e.onClaimWarning(&ClaimError{Stage: stage.ID, Field: "testResults",
			Problem: "reported a passing suite that was NOT verified — " + reason})
	}
}
