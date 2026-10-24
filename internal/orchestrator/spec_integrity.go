package orchestrator

// Spec integrity (review item I-1). Every stage derives from the spec, so
// the spec is verified like an artifact upstream of all of them: its digest
// is recorded when a run starts, a changed spec makes every completed stage
// stale on resume — which resets every approval bound to them through the
// L2.14 path — and a running process checks it before starting each stage.
//
// Before this, run state held the spec's path and never its content, so
// acceptance criteria could change mid-run while every approval derived
// from the old ones still stood.

import "fmt"

// SpecDigest is the SHA-256 of the spec at path, or "" when it cannot be
// read. The CLI refuses to start a run on a spec it cannot stat, so only
// library callers start without one; an empty digest has nothing to verify.
func SpecDigest(path string) string {
	if path == "" {
		return ""
	}
	digest, err := ArtifactSHA256(path)
	if err != nil {
		return ""
	}
	return digest
}

// SpecChangedError stops a run whose spec changed while it was in progress.
// Nothing is started against the new version: the stages already completed
// were derived from the old one, and resuming re-runs them all.
type SpecChangedError struct {
	Path string
}

func (e *SpecChangedError) Error() string {
	return fmt.Sprintf("spec %s changed while the run was in progress; nothing was started against "+
		"the new version — resume to re-run every completed stage against it", e.Path)
}

// specChanged reports whether the spec differs from the digest the run was
// bound to. A spec that can no longer be read counts as changed: content
// that vanished is not the content the run was started from.
func specChanged(state *RunState) bool {
	if state.SpecSHA256 == "" {
		return false
	}
	return SpecDigest(state.SpecPath) != state.SpecSHA256
}

// demoteForSpecChange marks every completed stage stale and re-binds the run
// to the spec as it now stands, so the re-run is verified against that.
func demoteForSpecChange(state *RunState) []StaleStage {
	stale := make([]StaleStage, 0)
	for _, stageID := range state.StagesInSequence() {
		if !state.IsStageCompleted(stageID) {
			continue
		}
		markStale(state, stageID, StaleReasonSpecChanged)
		stale = append(stale, StaleStage{StageID: stageID, Reason: StaleReasonSpecChanged})
	}
	state.SpecSHA256 = SpecDigest(state.SpecPath)
	return stale
}

// adoptSpecDigest binds a run whose state predates spec digests to the spec
// as it is now, and reports whether it did. There is no older digest to
// compare against, so nothing is demoted; edits from here on are caught.
func adoptSpecDigest(state *RunState) bool {
	if state.SpecSHA256 != "" {
		return false
	}
	state.SpecSHA256 = SpecDigest(state.SpecPath)
	return state.SpecSHA256 != ""
}

// checkSpecUnchanged is the mid-run check, made before a stage starts.
func checkSpecUnchanged(state *RunState) error {
	if specChanged(state) {
		return &SpecChangedError{Path: state.SpecPath}
	}
	return nil
}
