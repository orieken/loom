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
// checkable here at all.
//
// A second half once re-ran the project's test suite to reproduce a stage's
// claim that it passed. It was retired 2026-09-10: runs 8 and 9 tested both
// available explanations for run 2's fabrication and neither reproduced it,
// so the executor was paying a full suite run per QA stage to guard a defect
// with no established cause. The path check below costs a stat and stays.

import (
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
	switch {
	case claim.MustExist:
		return e.verifyExistenceClaim(stage, claim, root, changed)
	case claim.MustNotExist:
		return verifyDeletionClaim(stage, claim, root, changed)
	default:
		return nil
	}
}

func (e *Executor) verifyExistenceClaim(stage Stage, claim state.PathClaim, root string, changed map[string]bool) error {
	missing := missingPaths(root, claim.Paths)
	if len(missing) > 0 {
		return &ClaimError{Stage: stage.ID, Field: claim.Field, Paths: missing,
			Problem: "no such file — the stage reported work it did not do"}
	}
	return e.warnUntouched(stage, claim, changed)
}

// verifyDeletionClaim checks a claim that a path is gone now and existed
// when the stage started. "Still exists" is a plain filesystem stat and
// always runs. "No such file was there to delete" needs git history, and
// runs only when changed is non-nil — a nil map means posture checking is
// off (no WorkTree configured, or the tree could not be read), and a
// non-nil map, even empty, means the tree was observed and the claimed path
// was not among what changed.
func verifyDeletionClaim(stage Stage, claim state.PathClaim, root string, changed map[string]bool) error {
	if present := existingPaths(root, claim.Paths); len(present) > 0 {
		return &ClaimError{Stage: stage.ID, Field: claim.Field, Paths: present,
			Problem: "the file still exists — the stage reported a deletion it did not do"}
	}
	if changed == nil {
		return nil
	}
	if unseen := unseenDeletions(claim.Paths, changed); len(unseen) > 0 {
		return &ClaimError{Stage: stage.ID, Field: claim.Field, Paths: unseen,
			Problem: "no such file was there to delete"}
	}
	return nil
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

// existingPaths reports which claimed paths are still on disk. A path that
// escapes the project root resolves to an error, which is treated the same
// as "not found on disk" — consistent with missingPaths above — and left
// for unseenDeletions to judge against the changed set.
func existingPaths(root string, paths []string) []string {
	var present []string
	for _, path := range paths {
		resolved, err := resolveWithinRoot(root, path)
		if err != nil {
			continue
		}
		if fileExists(resolved) {
			present = append(present, path)
		}
	}
	sort.Strings(present)
	return present
}

// unseenDeletions reports which claimed paths the posture check never saw
// change. Callers must only call this with a non-nil changed set — nil
// means "not observed", which carries no evidence either way.
func unseenDeletions(paths []string, changed map[string]bool) []string {
	var unseen []string
	for _, path := range paths {
		if !changed[filepath.ToSlash(path)] {
			unseen = append(unseen, path)
		}
	}
	sort.Strings(unseen)
	return unseen
}

// OnClaimWarning reports a claim that is suspicious rather than false.
func (e *Executor) OnClaimWarning(report func(error)) { e.onClaimWarning = report }
