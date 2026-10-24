package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/internal/factory"
	"github.com/orieken/loom/internal/intake"
	"github.com/orieken/loom/internal/orchestrator"
)

// Verify judges a delivered order: the run's state, its budget, the pushed
// branch, the diff's scope, and — only now, in a fresh clone — the hidden
// acceptance tests.
func Verify(config Config) (Report, error) {
	config = config.withDefaults()
	report, err := loadReport(config)
	if err != nil {
		return Report{}, err
	}
	order, err := readOrder(config.Fixture)
	if err != nil {
		return Report{}, err
	}
	judgeRunState(config, &report, order)
	judgeBranch(config, &report, order)
	return report, saveReport(config, report)
}

func readOrder(fixture string) (intake.Order, error) {
	source, err := os.ReadFile(filepath.Join(fixture, "order.md"))
	if err != nil {
		return intake.Order{}, err
	}
	document, err := intake.Parse(source)
	return document.Order, err
}

// judgeRunState reads the run as the factory archived it when it shipped.
func judgeRunState(config Config, report *Report, order intake.Order) {
	archived := filepath.Join(factory.Layout{Root: config.project()}.RunArchivePath(order.ID), orchestrator.RunStateFileName)
	state, err := orchestrator.NewStateStore(archived).Load()
	if err == nil && state == nil {
		err = errors.New("the shipped run left no archived state")
	}
	if err != nil {
		report.record("the run's state was archived", err)
		return
	}
	report.record("every stage settled", everyStageSettled(state))
	report.record("every gate was approved explicitly", gatesApprovedByHand(state, report.Approved))
	report.record("the run stayed within the order's budget", withinBudget(state, order.Budget))
}

func everyStageSettled(state *orchestrator.RunState) error {
	if len(state.Stages) == 0 {
		return errors.New("the run recorded no stages")
	}
	for _, stageID := range state.StagesInSequence() {
		if !state.IsStageSettled(stageID) {
			return fmt.Errorf("stage %s is %s", stageID, state.Stages[stageID].Status)
		}
	}
	if state.BudgetHalt != nil {
		return fmt.Errorf("the run was halted by its budget before %s", state.BudgetHalt.BeforeStage)
	}
	return nil
}

// gatesApprovedByHand: each run gate the harness approved is recorded as an
// approval made with --approve and bound to digests — the path a human takes,
// not one any agent output could reach.
func gatesApprovedByHand(state *orchestrator.RunState, approved []string) error {
	gates := runGates(approved)
	if len(gates) == 0 {
		return errors.New("no run gate was approved; the run never stopped for a human")
	}
	for _, gate := range gates {
		if approval, ok := state.Approvals[gate]; !ok || !isHandApproval(approval) {
			return fmt.Errorf("gate %s: approval %+v is not a valid --approve bound to digests", gate, approval)
		}
	}
	return nil
}

// runGates drops the factory's own approvals, which are not run gates.
func runGates(approved []string) []string {
	var gates []string
	for _, gate := range approved {
		if gate != factory.GateCommit && gate != factory.GateOpenPR {
			gates = append(gates, gate)
		}
	}
	return gates
}

func isHandApproval(approval orchestrator.Approval) bool {
	return approval.Method == orchestrator.ApprovalMethodFlag && approval.IsValid() && len(approval.ArtifactDigests) > 0
}

func withinBudget(state *orchestrator.RunState, budget intake.Budget) error {
	if spent := state.TotalUsage().CostUSD; spent > budget.MaxUSD {
		return fmt.Errorf("spent $%.4f of $%.2f", spent, budget.MaxUSD)
	}
	return nil
}

// judgeBranch clones the pushed branch and checks what it carries.
func judgeBranch(config Config, report *Report, order intake.Order) {
	branch := factory.BranchFor(order.ID)
	if _, err := run(config.Workdir, config.PassTimeout, "git", "clone", "-q", "--branch", branch, config.origin(), config.clone()); err != nil {
		report.record("the branch was pushed", err)
		return
	}
	report.record("the branch was pushed", nil)
	report.record("the diff stays inside scope.paths", diffInScope(config, order))
	report.record("the hidden acceptance tests pass", acceptanceTestsPass(config))
}

func diffInScope(config Config, order intake.Order) error {
	output, err := run(config.clone(), config.PassTimeout, "git", "diff", "--name-only", "origin/main...HEAD")
	if err != nil {
		return err
	}
	changed := strings.Fields(output)
	if len(changed) == 0 {
		return errors.New("the branch changes nothing")
	}
	var outside []string
	for _, path := range changed {
		if !factory.InOrderScope(path, order.ID, order.Scope.Paths) {
			outside = append(outside, path)
		}
	}
	if len(outside) > 0 {
		return fmt.Errorf("outside %v: %v", order.Scope.Paths, outside)
	}
	return nil
}

// acceptanceTestsPass copies the hidden tests into the clone — the first
// time they exist anywhere the factory could have reached — and runs them.
func acceptanceTestsPass(config Config) error {
	if err := copyTree(config.Hidden, config.clone()); err != nil {
		return fmt.Errorf("copy the hidden tests: %w", err)
	}
	_, err := run(config.clone(), config.PassTimeout, config.Acceptance[0], config.Acceptance[1:]...)
	return err
}
