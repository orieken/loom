package factory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/orieken/loom/internal/intake"
)

// work is one thing the supervisor could start this tick.
type work struct {
	id       string
	priority int
	created  string
	// resume continues an existing claim; otherwise the order is claimed.
	resume bool
	start  func() error
}

// startWork starts what the free slots allow: work already in flight first,
// then new claims, each group by priority, then created, then id (review
// item I-8 — `created` has day granularity, so same-day orders would
// otherwise start in whatever order git lists them).
func (s *Supervisor) startWork(committed Intake) error {
	orders, err := s.Orders()
	if err != nil {
		return err
	}
	slots := s.config.MaxConcurrent - activeCount(orders)
	for _, item := range s.pendingWork(orders, committed) {
		if slots <= 0 {
			return nil
		}
		if err := item.start(); err != nil {
			return fmt.Errorf("start %s: %w", item.id, err)
		}
		slots--
	}
	return nil
}

func activeCount(orders map[string]*OrderState) int {
	active := 0
	for _, order := range orders {
		if order.IsActive() {
			active++
		}
	}
	return active
}

func (s *Supervisor) pendingWork(orders map[string]*OrderState, committed Intake) []work {
	var pending []work
	for _, order := range orders {
		if item, ok := s.continuation(order, committed); ok {
			pending = append(pending, item)
		}
	}
	for _, candidate := range committed.Orders {
		if s.isClaimable(candidate, orders) {
			pending = append(pending, s.claimWork(candidate, committed.Commit))
		}
	}
	sort.Slice(pending, func(i, j int) bool { return before(pending[i], pending[j]) })
	return pending
}

func before(left, right work) bool {
	if left.resume != right.resume {
		return left.resume
	}
	if left.priority != right.priority {
		return left.priority < right.priority
	}
	if left.created != right.created {
		return left.created < right.created
	}
	return left.id < right.id
}

// isClaimable: ready and valid, every dependency shipped, and either never
// claimed or parked and since edited — editing a parked order is how a human
// clears it (roadmap L3.68).
func (s *Supervisor) isClaimable(candidate Candidate, orders map[string]*OrderState) bool {
	if !candidate.IsClaimable() || !dependenciesShipped(candidate.Document.Order, orders) {
		return false
	}
	existing, claimed := orders[candidate.ID]
	if !claimed {
		return true
	}
	return existing.mayReclaim() && existing.Claim.OrderSHA256 != candidate.SHA256
}

// mayReclaim: a parked order is reclaimed as soon as it is edited; any other
// order not running only when a human asked for a re-run of the new version.
func (o *OrderState) mayReclaim() bool {
	if o.Phase == PhaseParked {
		return true
	}
	return o.RerunRequested && !o.IsActive() && o.Phase != PhaseShipped
}

func dependenciesShipped(order intake.Order, orders map[string]*OrderState) bool {
	for _, dependency := range order.DependsOn {
		if state, ok := orders[dependency]; !ok || state.Phase != PhaseShipped {
			return false
		}
	}
	return true
}

// continuation is the next step for an order already claimed, if it has
// one this tick. An order not committed as ready — on hold, back to draft,
// or deleted — is never resumed (ADR-010, 3A): a running leg finishes to its
// next halt and stays there.
func (s *Supervisor) continuation(order *OrderState, committed Intake) (work, bool) {
	approving, ok := s.continuationKind(order)
	if !ok || !committed.Orders[order.ID].IsReady() {
		return work{}, false
	}
	item := work{id: order.ID, priority: order.Claim.Priority, created: order.Claim.Created, resume: true}
	item.start = func() error { return s.relaunch(order, approving) }
	return item, true
}

// continuationKind says whether an order can continue now, and which gate
// the continuation approves, if any.
func (s *Supervisor) continuationKind(order *OrderState) (string, bool) {
	switch order.Phase {
	case PhaseClaimed:
		return "", true
	case PhaseRetrying:
		return "", !s.deps.Now().Before(order.NotBefore)
	case PhaseStopped:
		return "", order.ResumeRequested
	case PhaseWaiting:
		return order.Gate, order.Approval != nil && order.Approval.Gate == order.Gate
	case PhaseParked:
		return GateBudget, order.Approval != nil && order.Approval.Gate == GateBudget
	}
	return "", false
}

func (s *Supervisor) claimWork(candidate Candidate, commit string) work {
	order := candidate.Document.Order
	return work{id: candidate.ID, priority: order.Priority, created: order.Created,
		start: func() error { return s.claim(candidate, commit) }}
}

// claim records the claim, then prepares the worktree and snapshot, then
// launches. A crash between any two leaves the order claimed, and the next
// tick repeats the preparation — every step of it is idempotent.
func (s *Supervisor) claim(candidate Candidate, commit string) error {
	claim := s.claimFor(candidate, commit)
	if err := s.record(Entry{Event: EventClaimed, Order: candidate.ID, Claim: &claim}); err != nil {
		return err
	}
	return s.launch(&OrderState{ID: candidate.ID, Phase: PhaseClaimed, Claim: claim}, 1, "")
}

func (s *Supervisor) claimFor(candidate Candidate, commit string) Claim {
	order := candidate.Document.Order
	base := order.BaseBranch
	if base == "" {
		base = s.config.IntakeBranch
	}
	return Claim{
		SourceCommit: commit, OrderPath: candidate.Path, OrderSHA256: candidate.SHA256,
		Plan: order.Plan, Type: order.Type, Title: Title(candidate.Document.Body),
		Priority: order.Priority, Created: order.Created, BaseBranch: base, Finish: order.Finish,
		Budget: order.Budget, ScopePaths: order.Scope.Paths,
		Branch: BranchFor(candidate.ID), Worktree: s.config.Layout.WorktreePath(candidate.ID),
	}
}

// relaunch continues a claimed order: a fresh start if it never ran, a
// resume otherwise. Only a retry spends an attempt — and not one waiting out
// an unavailable provider; continuing after a gate, a stop or an accepted
// budget does not.
func (s *Supervisor) relaunch(order *OrderState, approving string) error {
	attempt := order.Attempt
	if (order.Phase == PhaseRetrying && !order.KeepsAttempt) || attempt == 0 {
		attempt++
	}
	if approving != "" {
		if dropped, err := s.dropStaleApproval(order); dropped || err != nil {
			return err
		}
	}
	return s.launch(order, attempt, approving)
}

// launch prepares the order's worktree and snapshot, starts `loom run`, and
// records the process.
func (s *Supervisor) launch(order *OrderState, attempt int, approving string) error {
	if err := s.prepare(order); err != nil {
		return err
	}
	resume, err := hasRunState(order)
	if err != nil {
		return err
	}
	process, err := s.deps.Launcher.Launch(LaunchRequest{
		Dir: order.Claim.Worktree, Args: s.runArgs(order, resume, approving),
		LogPath: s.config.Layout.LogPath(order.ID),
	})
	if err != nil {
		return err
	}
	s.processes[order.ID] = process
	s.watch(process)
	return s.record(Entry{Event: EventLaunched, Order: order.ID, Attempt: attempt, PID: process.PID(), Approving: approving})
}

// prepare makes the order's worktree on its branch and writes the claim
// snapshot the run reads as its spec (ADR-010, 1C): the intake file stays
// freely editable, and the run's digest guards exactly what it was given.
func (s *Supervisor) prepare(order *OrderState) error {
	claim := order.Claim
	if err := s.deps.Repository.AddWorktree(claim.Worktree, claim.Branch, claim.BaseBranch); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}
	if err := ignoredDirectory(filepath.Join(claim.Worktree, StateDir)); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(claim.Worktree, ClaimPath(order.ID))); err == nil && order.Phase != PhaseClaimed {
		return nil
	}
	return s.writeSnapshot(order)
}

func (s *Supervisor) writeSnapshot(order *OrderState) error {
	content, err := s.deps.Repository.ShowFile(order.Claim.SourceCommit, order.Claim.OrderPath)
	if err != nil {
		return fmt.Errorf("read %s at %s for the claim snapshot: %w", order.Claim.OrderPath, order.Claim.SourceCommit, err)
	}
	path := filepath.Join(order.Claim.Worktree, ClaimPath(order.ID))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create claims directory: %w", err)
	}
	return os.WriteFile(path, content, 0o644)
}

func hasRunState(order *OrderState) (bool, error) {
	_, err := os.Stat(RunStatePath(order.Claim.Worktree, order.ID))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// runArgs is the `loom run` command line for an order. The order's budget
// seeds the run's governor on every launch (roadmap L3.70), so an order
// re-claimed with a larger budget raises the run's ceiling with it.
func (s *Supervisor) runArgs(order *OrderState, resume bool, approving string) []string {
	claim := order.Claim
	args := []string{"run", "--spec", ClaimPath(order.ID), "--plan", claim.Plan, "--provider", s.config.Provider,
		"--max-usd", formatFloat(claim.Budget.MaxUSD), "--max-hours", formatFloat(claim.Budget.MaxHours)}
	if resume {
		args = append(args, "--resume")
	}
	return append(append(args, approvalArgs(order, approving)...), s.config.RunArgs...)
}

// approvalArgs carries a human's approval into the run: a run gate is
// approved by name; an accepted budget accepts the unmeasured spend on the
// approver's behalf, so the run records who accepted it.
func approvalArgs(order *OrderState, approving string) []string {
	switch approving {
	case "":
		return nil
	case GateBudget:
		return []string{"--accept-unmeasured", approverOf(order)}
	}
	return []string{"--approve", approving}
}

func approverOf(order *OrderState) string {
	if order.Approval == nil || order.Approval.ApprovedBy == "" {
		return "unknown"
	}
	return order.Approval.ApprovedBy
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
