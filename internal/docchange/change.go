// Package docchange turns verified documenter results into one reviewable
// changeset and puts it through an approval gate before anything is written
// into the target repository (ADR-010, Phase 6).
//
// A changeset is staged, not applied: `plan` writes the final content of every
// file it would touch under .loom/changes/<id>/, and nothing else. A human
// approves it, and the approval binds to the digest of every staged file, so
// editing any of them resets it — the rule loom run's gates follow
// (roadmap L2.14). `apply` then refuses unless the staged files still match
// what was approved and the target files are still what they were when
// planned.
package docchange

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"path/filepath"

	"github.com/orieken/loom/internal/repograph"
)

// Artifact kinds.
const (
	KindSidecarReadme = "sidecar-readme"
	KindReadme        = "readme"
	KindDocComment    = "doc-comment"
	KindCodemap       = "codemap"
	KindIndex         = "index"
)

// Artifact actions.
const (
	ActionCreate    = "create"
	ActionUpdate    = "update"
	ActionUnchanged = "unchanged"
	// ActionConflict: a generated block was edited by hand, so it is not
	// overwritten. It is surfaced at the gate for a human to resolve.
	ActionConflict = "conflict"
	// ActionSkip: nothing was staged, for a stated reason.
	ActionSkip = "skip"
)

// Artifact is one file the changeset would write.
type Artifact struct {
	// Target is the repo-relative path that would be written.
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Action string `json:"action"`
	// Reason explains a conflict or a skip.
	Reason string `json:"reason,omitempty"`
	// BaseDigest is the target's digest when planned, empty if it did not
	// exist. Apply refuses if the target has since changed.
	BaseDigest string `json:"baseDigest,omitempty"`
	// Digest is the staged content's digest; empty unless something is staged.
	Digest string `json:"digest,omitempty"`
}

// Staged says the artifact has content that apply would write.
func (a Artifact) Staged() bool { return a.Action == ActionCreate || a.Action == ActionUpdate }

// OpenQuestion is something the documenter could not settle, shown at the
// gate so a human sees what the documents do not claim.
type OpenQuestion struct {
	Path     string `json:"path"`
	Question string `json:"question"`
}

// SkippedCandidate is a verified result that did not make it into the change.
type SkippedCandidate struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Options select the opt-in outputs.
type Options struct {
	InTreeReadmes bool `json:"inTreeReadmes"`
	DocComments   bool `json:"docComments"`
	NoCodemap     bool `json:"noCodemap"`
	// CodemapOnly plans just CODEMAP.md, from the graph and the index as they
	// are. It needs no documenter run: nothing here is model output.
	CodemapOnly bool `json:"codemapOnly"`
}

// Change is `.loom/changes/<id>/change.json`.
type Change struct {
	ID string `json:"id"`
	// RunID is the newest run the changeset draws on; Runs lists every one, newest
	// first. A directory takes its result from the newest run with a valid one.
	RunID     string   `json:"runId"`
	Runs      []string `json:"runs"`
	CreatedAt string   `json:"createdAt"`
	Options   Options  `json:"options"`
	// Documented counts the directories whose results are in this change.
	Documented    int                `json:"documented"`
	Artifacts     []Artifact         `json:"artifacts"`
	OpenQuestions []OpenQuestion     `json:"openQuestions"`
	Skipped       []SkippedCandidate `json:"skipped"`
}

// ChangesDir is where changesets live, under .loom/.
const ChangesDir = "changes"

// Dir is a changeset's directory.
func Dir(root, id string) string {
	return filepath.Join(root, repograph.OutputDir, ChangesDir, id)
}

// StagedPath is where an artifact's staged content is kept.
func StagedPath(root, id, target string) string {
	return filepath.Join(Dir(root, id), "files", filepath.FromSlash(target))
}

// Digest is the SHA-256 form used throughout: "sha256:<hex>".
func Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// readmeTarget is the in-tree README path for a directory.
func readmeTarget(dir string) string {
	if dir == "." {
		return "README.md"
	}
	return path.Join(dir, "README.md")
}

// sidecarTarget is where a directory's rendered document lives under .loom.
func sidecarTarget(dir string) string {
	if dir == "." {
		return path.Join(repograph.OutputDir, "docs", "README.md")
	}
	return path.Join(repograph.OutputDir, "docs", dir, "README.md")
}
