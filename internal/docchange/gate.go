package docchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/orieken/loom/internal/repograph"
)

// Errors a caller branches on.
var (
	// ErrNotApproved: apply needs a recorded, still-valid approval.
	ErrNotApproved = errors.New("the changeset is not approved")
	// ErrAlreadyApplied: a changeset is applied once.
	ErrAlreadyApplied = errors.New("the changeset was already applied")
)

// ResetError says the approval no longer holds because something it was bound
// to was edited. It names what, so the reviewer knows what to look at again.
type ResetError struct {
	Changed []string
}

func (e *ResetError) Error() string {
	return fmt.Sprintf("the approval was reset: %s changed since it was approved; review the changeset and approve it again", strings.Join(e.Changed, ", "))
}

// StaleTargetError says a file the changeset would overwrite is no longer what
// it was when planned, so writing the staged content would lose someone's edit.
type StaleTargetError struct {
	Target string
}

func (e *StaleTargetError) Error() string {
	return fmt.Sprintf("%s changed since the changeset was planned; run `loom docs plan` again", e.Target)
}

// Invalidation records why an approval stopped holding.
type Invalidation struct {
	At      string   `json:"at"`
	Changed []string `json:"changed"`
}

// Approval is `.loom/changes/<id>/approval.json`: a human's decision, bound to
// the digest of everything it covered.
type Approval struct {
	ChangeID   string            `json:"changeId"`
	ApprovedAt string            `json:"approvedAt"`
	Digests    map[string]string `json:"digests"`
	// Invalidated is set when a bound file was edited after approval. The
	// record is kept, naming what changed, rather than deleted.
	Invalidated *Invalidation `json:"invalidated,omitempty"`
}

// IsValid says the approval still holds.
func (a Approval) IsValid() bool { return a.Invalidated == nil }

// changeKey stands for change.json itself in an approval's digests.
const changeKey = "change.json"

// Approve records a human's approval of a changeset, binding it to the digest
// of change.json and of every staged file as they are on disk now. Nothing an
// agent returns can create one: this is called only from the CLI.
func Approve(root, id string, now time.Time) (Approval, error) {
	change, err := Load(root, id)
	if err != nil {
		return Approval{}, err
	}
	digests, missing := currentDigests(root, change)
	if len(missing) > 0 {
		return Approval{}, fmt.Errorf("cannot approve: staged file(s) missing: %s", strings.Join(missing, ", "))
	}
	approval := Approval{ChangeID: id, ApprovedAt: now.UTC().Format(time.RFC3339), Digests: digests}
	return approval, saveApproval(root, approval)
}

// currentDigests digests everything an approval binds to. A file that cannot
// be read is returned in missing and left out of the map, so a comparison with
// an approval sees it as changed.
func currentDigests(root string, change Change) (digests map[string]string, missing []string) {
	digests = map[string]string{}
	add := func(key, file string) {
		raw, err := os.ReadFile(file)
		if err != nil {
			missing = append(missing, key)
			return
		}
		digests[key] = Digest(raw)
	}
	add(changeKey, filepath.Join(Dir(root, change.ID), changeFile))
	for _, artifact := range change.Artifacts {
		if artifact.Staged() {
			add(artifact.Target, StagedPath(root, change.ID, artifact.Target))
		}
	}
	return digests, missing
}

func saveApproval(root string, approval Approval) error {
	raw, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return fmt.Errorf("encode approval: %w", err)
	}
	return repograph.WriteFileAtomically(filepath.Join(Dir(root, approval.ChangeID), approvalFile), append(raw, '\n'))
}

// LoadApproval reads a changeset's approval, if it has one.
func LoadApproval(root, id string) (Approval, bool, error) {
	raw, err := os.ReadFile(filepath.Join(Dir(root, id), approvalFile))
	if errors.Is(err, os.ErrNotExist) {
		return Approval{}, false, nil
	}
	if err != nil {
		return Approval{}, false, err
	}
	var approval Approval
	if err := json.Unmarshal(raw, &approval); err != nil {
		return Approval{}, false, fmt.Errorf("parse approval: %w", err)
	}
	return approval, true, nil
}

// checkApproval confirms the approval still binds to what is on disk. If not,
// it marks the approval invalid — keeping the record — and returns a
// ResetError naming every file that changed.
func checkApproval(root string, change Change, approval Approval, now time.Time) error {
	if !approval.IsValid() {
		return fmt.Errorf("%w: it was reset (%s changed) and must be approved again", ErrNotApproved, strings.Join(approval.Invalidated.Changed, ", "))
	}
	current, _ := currentDigests(root, change)
	changed := differing(approval.Digests, current)
	if len(changed) == 0 {
		return nil
	}
	approval.Invalidated = &Invalidation{At: now.UTC().Format(time.RFC3339), Changed: changed}
	if saveErr := saveApproval(root, approval); saveErr != nil {
		return saveErr
	}
	return &ResetError{Changed: changed}
}

// differing lists every key whose digest differs, or that exists on only one
// side — an added or removed staged file is a change too.
func differing(approved, current map[string]string) []string {
	seen := map[string]bool{}
	var changed []string
	for key, digest := range approved {
		seen[key] = true
		if current[key] != digest {
			changed = append(changed, key)
		}
	}
	for key := range current {
		if !seen[key] {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return changed
}
