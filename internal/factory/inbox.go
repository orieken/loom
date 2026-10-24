package factory

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
)

// The approvals the factory itself asks for, beside a run's own gates.
const (
	// GateCommit approves committing and pushing a completed run's changes —
	// approval gate #2. Nothing is auto-approved, so a human approves it.
	GateCommit = "commit"
	// GateOpenPR approves opening the proposed pull request — gate #5,
	// Always Human (roadmap L3.69).
	GateOpenPR = "open-pr"
)

// ItemKind is what an inbox item asks of a human.
type ItemKind string

// The kinds of inbox item.
const (
	ItemGate    ItemKind = "gate"
	ItemCommit  ItemKind = "commit"
	ItemOpenPR  ItemKind = "open-pr"
	ItemParked  ItemKind = "parked"
	ItemChanged ItemKind = "changed-since-claim"
)

// InboxItem is one thing waiting on a human (roadmap L3.67).
type InboxItem struct {
	Order string
	Kind  ItemKind
	// Gate is what `loom factory approve <order> <gate>` names. Empty for
	// an item that informs rather than asks.
	Gate  string
	Since time.Time
	// Digest is what an approval binds to. If it differs when the
	// supervisor acts, the approval is dropped and the item asked again.
	Digest  string
	Summary string
	Details []string
}

// IsApprovable reports whether the item asks for an approval.
func (i InboxItem) IsApprovable() bool { return i.Gate != "" }

// Inbox lists everything waiting on a human, across every order. It adds
// visibility, never a new way to approve: a run gate is still approved by
// `loom run --resume --approve`, which the supervisor runs on the human's
// behalf with the approval bound to what the inbox showed.
func Inbox(orders map[string]*OrderState, committed Intake, trees TreeOpener) ([]InboxItem, error) {
	var items []InboxItem
	for _, order := range Sorted(orders) {
		found, err := itemsFor(order, trees)
		if err != nil {
			return nil, fmt.Errorf("inbox for %s: %w", order.ID, err)
		}
		items = append(items, found...)
		if changed, ok := changedSinceClaim(order, committed); ok {
			items = append(items, changed)
		}
	}
	return items, nil
}

func itemsFor(order *OrderState, trees TreeOpener) ([]InboxItem, error) {
	switch order.Phase {
	case PhaseWaiting:
		item, err := gateItem(order)
		return []InboxItem{item}, err
	case PhaseCompleted:
		return commitItem(order, trees)
	case PhaseProposed:
		return []InboxItem{openPRItem(order)}, nil
	case PhaseParked:
		return []InboxItem{parkedItem(order)}, nil
	}
	return nil, nil
}

func gateItem(order *OrderState) (InboxItem, error) {
	state, err := loadRunState(order.Claim.Worktree, order.ID)
	if err != nil {
		return InboxItem{}, err
	}
	digest, stages := BindingDigest(state)
	return InboxItem{
		Order: order.ID, Kind: ItemGate, Gate: order.Gate, Since: order.Since, Digest: digest,
		Summary: fmt.Sprintf("run waiting at gate %s", order.Gate), Details: stages,
	}, nil
}

// BindingDigest fingerprints every completed stage's artifact — the set an
// approval binds to (roadmap L2.14) — and lists them.
func BindingDigest(state *orchestrator.RunState) (string, []string) {
	if state == nil {
		return "", nil
	}
	var stages []string
	for _, stageID := range state.StagesInSequence() {
		record := state.Stages[stageID]
		if record.Status == orchestrator.StageStatusCompleted && record.ArtifactSHA256 != "" {
			stages = append(stages, fmt.Sprintf("%s %s", stageID, record.ArtifactSHA256))
		}
	}
	return digestOf(stages...), stages
}

func commitItem(order *OrderState, trees TreeOpener) ([]InboxItem, error) {
	if order.Claim.Finish == "none" {
		return nil, nil
	}
	tree := trees(order.Claim.Worktree)
	digest, err := tree.Digest()
	if err != nil {
		return nil, err
	}
	changed, err := tree.ChangedPaths()
	if err != nil {
		return nil, err
	}
	return []InboxItem{{
		Order: order.ID, Kind: ItemCommit, Gate: GateCommit, Since: order.Since, Digest: digest,
		Summary: fmt.Sprintf("run completed — commit %d changed path(s) to %s and push", len(committable(changed)), order.Claim.Branch),
		Details: describeChanges(committable(changed), order.Claim.ScopePaths),
	}}, nil
}

// committable drops what the commit leaves out, so the inbox shows what the
// commit will actually contain.
func committable(paths []string) []string {
	var kept []string
	for _, changed := range paths {
		if !isExcluded(changed) {
			kept = append(kept, changed)
		}
	}
	return kept
}

func isExcluded(changed string) bool {
	for _, excluded := range commitExclusions {
		if changed == excluded || strings.HasPrefix(changed, excluded+"/") {
			return true
		}
	}
	return false
}

// describeChanges marks every path outside the order's scope, which is what
// a reviewer most needs to see before approving the commit.
func describeChanges(paths, scope []string) []string {
	described := make([]string, 0, len(paths))
	for _, changed := range paths {
		if len(scope) > 0 && !inScope(changed, scope) {
			changed += "  (outside scope.paths)"
		}
		described = append(described, changed)
	}
	sort.Strings(described)
	return described
}

// InScope reports whether a changed path falls inside an order's
// scope.paths, by the same rule the inbox uses to mark one that does not.
func InScope(changed string, scope []string) bool { return inScope(changed, scope) }

// inScope matches a path against scope globs. `dir/**` matches anything
// under dir; any other pattern is matched with path.Match.
func inScope(changed string, scope []string) bool {
	for _, pattern := range scope {
		if prefix, ok := strings.CutSuffix(pattern, "/**"); ok && strings.HasPrefix(changed, prefix+"/") {
			return true
		}
		if matched, _ := path.Match(pattern, changed); matched {
			return true
		}
	}
	return false
}

func openPRItem(order *OrderState) InboxItem {
	proposal := order.Proposal
	return InboxItem{
		Order: order.ID, Kind: ItemOpenPR, Gate: GateOpenPR, Since: order.Since, Digest: proposal.Digest(),
		Summary: fmt.Sprintf("open pull request %q (%s → %s)", proposal.Title, proposal.Head, proposal.Base),
	}
}

// Digest fingerprints the proposal, so an approval is for this text.
func (p *Proposal) Digest() string {
	if p == nil {
		return ""
	}
	return digestOf(p.Title, p.Body, p.Base, p.Head)
}

func parkedItem(order *OrderState) InboxItem {
	item := InboxItem{Order: order.ID, Kind: ItemParked, Since: order.Since,
		Summary: "parked — edit the order to re-run it: " + order.Reason}
	if order.Spend != nil {
		item.Details = spendLines(order.Spend)
	}
	return item
}

func spendLines(spend *Spend) []string {
	lines := []string{fmt.Sprintf("total $%.4f", spend.TotalUSD)}
	for _, stage := range spend.PerStage {
		if !stage.Measured {
			lines = append(lines, fmt.Sprintf("%s unmeasured", stage.Stage))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s $%.4f", stage.Stage, stage.CostUSD))
	}
	return lines
}

// changedSinceClaim tells a human the committed order no longer matches
// what is running (ADR-010, 1C). The run keeps the version it claimed until
// the human chooses `loom factory rerun`.
func changedSinceClaim(order *OrderState, committed Intake) (InboxItem, bool) {
	candidate, ok := committed.Orders[order.ID]
	settled := order.Phase == PhaseShipped || order.Phase == PhaseParked
	if !ok || settled || candidate.SHA256 == order.Claim.OrderSHA256 {
		return InboxItem{}, false
	}
	return InboxItem{Order: order.ID, Kind: ItemChanged, Since: order.Since,
		Summary: "the order changed since it was claimed — `loom factory rerun " + order.ID +
			"` re-runs the new version; doing nothing keeps the one that ran"}, true
}

func digestOf(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// ErrNothingToApprove reports an approval for something not in the inbox.
var ErrNothingToApprove = errors.New("nothing to approve")

// Approve records a human's approval of one inbox item, bound to the digest
// the item carries now. It refuses anything the inbox is not asking for.
func Approve(ledger *Ledger, items []InboxItem, order, gate, approver string) (Entry, error) {
	for _, item := range items {
		if item.Order == order && item.IsApprovable() && item.Gate == gate {
			entry := Entry{Event: EventApproved, Order: order, Gate: gate, Digest: item.Digest, ApprovedBy: approver}
			return entry, ledger.Append(entry)
		}
	}
	return Entry{}, fmt.Errorf("%w: %s is not waiting on %q (%s)", ErrNothingToApprove, order, gate, waitingOn(items, order))
}

func waitingOn(items []InboxItem, order string) string {
	var gates []string
	for _, item := range items {
		if item.Order == order && item.IsApprovable() {
			gates = append(gates, item.Gate)
		}
	}
	if len(gates) == 0 {
		return "it is waiting on nothing"
	}
	return "it is waiting on " + strings.Join(gates, ", ")
}

// dropStaleApproval refuses to act on a gate approval whose artifacts have
// changed since the human saw them. `loom run` would also refuse an approval
// its own digests contradict; this catches an edit made after the inbox
// was read and before the approval was given.
func (s *Supervisor) dropStaleApproval(order *OrderState) (bool, error) {
	state, err := loadRunState(order.Claim.Worktree, order.ID)
	if err != nil {
		return false, err
	}
	if digest, _ := BindingDigest(state); digest == order.Approval.Digest {
		return false, nil
	}
	return true, s.record(Entry{Event: EventApprovalDropped, Order: order.ID, Gate: order.Gate,
		Reason: "an artifact changed after the approval was given — review the inbox and approve again"})
}
