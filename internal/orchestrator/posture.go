package orchestrator

// Posture checking: did a stage change source it never said it would change
// (roadmap L3.30)?
//
// Run 4's accessibility-engineer declares `tools: Read, Glob, Grep, Bash` —
// no edit tool — and modified handlers.go through Bash. The edits were
// correct and improved the code, which is why this records rather than
// fails: the defect is that "read-only stage" was a property nothing
// enforced AND nothing checked, not that the change was bad.
//
// `--allowed-tools` cannot close it. It is an allowlist over named tools, and
// any stage holding Bash can write through a heredoc or `sed -i`. Removing
// Bash from six reviewing stages that use it to run checks costs more than
// the defect. So the executor observes the tree instead of trusting the
// allowlist, and says what it saw.

import "sort"

// WorkTree fingerprints the repository a run is changing. Injected rather
// than called directly so the executor keeps no filesystem or git dependency
// of its own (architecture-guardrails.md #1); a nil WorkTree disables the
// check, which is what every test that does not care about it uses.
type WorkTree interface {
	Digest() (string, error)
}

// WithWorkTree enables posture checking against a repository.
func (e *Executor) WithWorkTree(tree WorkTree) *Executor {
	e.workTree = tree
	return e
}

// OnPostureError reports a failure to fingerprint the tree. The check
// observes a stage rather than gating one, so a failure is reported and
// never propagated.
func (e *Executor) OnPostureError(report func(error)) { e.onPostureError = report }

// treeDigest fingerprints the tree, or returns "" when checking is disabled
// or the fingerprint cannot be taken. An unavailable digest disables the
// check for that stage rather than failing the run: this observes a stage,
// it does not gate one.
func (e *Executor) treeDigest() string {
	if e.workTree == nil {
		return ""
	}
	digest, err := e.workTree.Digest()
	if err != nil {
		e.reportPostureError(err)
		return ""
	}
	return digest
}

// notePostureViolation records a stage that changed the tree without
// declaring write access, on the stage record and on the timeline.
func (e *Executor) notePostureViolation(state *RunState, stage Stage, before string, output StageOutput) {
	if before == "" || output.DeclaredWriteAccess {
		return
	}
	if e.treeDigest() == before {
		return
	}
	record := state.Stages[stage.ID]
	record.PostureViolation = "changed the working tree without declaring a tool that writes files"
	state.Stages[stage.ID] = record
	if err := e.emit(Event{Kind: EventPostureViolation, Stage: stage.ID,
		Reason: record.PostureViolation}); err != nil {
		e.reportPostureError(err)
	}
}

func (e *Executor) reportPostureError(err error) {
	if e.onPostureError != nil {
		e.onPostureError(err)
	}
}

// PostureViolations lists the stages that changed source they never declared
// they would change, so a run can report them rather than burying them in
// the timeline.
func (s *RunState) PostureViolations() []string {
	var stages []string
	for id, record := range s.Stages {
		if record.PostureViolation != "" {
			stages = append(stages, id)
		}
	}
	sort.Strings(stages)
	return stages
}
