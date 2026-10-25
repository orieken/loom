package factory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

// The pull request's description is the delivery summary: what was asked,
// what the reviewers concluded, what each stage did and cost, which gates a
// human approved, and where the full record is. The first real factory PR
// (I-4) said only that no summary had been written and pointed at a
// worktree that was deleted moments later — the reviewer had to go
// spelunking in run-state.json to learn any of this.

// proposalBody is the delivery summary for an order whose run completed.
// Every part is best-effort: a part that cannot be read says so in place,
// because a missing number must never read as a zero.
func (s *Supervisor) proposalBody(order *OrderState) string {
	var body strings.Builder
	writeOrigin(&body, order)
	writeRequest(&body, order)
	writeRun(&body, order)
	writeChangedFiles(&body, s.deps.Trees(order.Claim.Worktree), order)
	writeRecord(&body, order.ID)
	writeRunSummary(&body, order)
	return body.String()
}

func writeOrigin(body *strings.Builder, order *OrderState) {
	fmt.Fprintf(body, "Delivered by the loom factory from work order `%s` (`%s` at `%s`).\n",
		order.ID, order.Claim.OrderPath, shortCommit(order.Claim.SourceCommit))
}

func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

// writeRequest quotes the order's own Summary section, from the claim
// snapshot the run read: the request as it was when the work was done.
func writeRequest(body *strings.Builder, order *OrderState) {
	snapshot, err := os.ReadFile(filepath.Join(order.Claim.Worktree, ClaimPath(order.ID)))
	if err != nil {
		return
	}
	if summary := sectionBody(string(snapshot), "Summary"); summary != "" {
		fmt.Fprintf(body, "\n## Request\n\n%s\n", summary)
	}
}

// sectionBody is the text under a markdown "## heading", up to the next
// heading of the same or a higher level.
func sectionBody(markdown, heading string) string {
	var lines []string
	inside := false
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "###") {
			inside = strings.TrimSpace(strings.TrimLeft(line, "#")) == heading && strings.HasPrefix(line, "## ")
			continue
		}
		if inside {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func writeRun(body *strings.Builder, order *OrderState) {
	runState, err := loadRunState(order.Claim.Worktree, order.ID)
	if err != nil {
		fmt.Fprintf(body, "\nThe run's state could not be read, so its stages are not summarized: %v\n", err)
		return
	}
	if runState == nil {
		body.WriteString("\nThe run left no state, so its stages are not summarized.\n")
		return
	}
	writeResults(body, runState)
	writeStages(body, runState, order.Claim.Budget.MaxUSD)
	writeGates(body, runState)
}

// writeResults reports what the reviewing stages concluded, read from their
// typed state rather than from prose.
func writeResults(body *strings.Builder, runState *orchestrator.RunState) {
	var lines []string
	for _, stageID := range runState.StagesInSequence() {
		if line, ok := resultLine(stageID, runState.Stages[stageID]); ok {
			lines = append(lines, line)
		}
	}
	if len(lines) > 0 {
		fmt.Fprintf(body, "\n## Results\n\n%s\n", strings.Join(lines, "\n"))
	}
}

func resultLine(stageID string, record orchestrator.StageRecord) (string, bool) {
	if record.Status != orchestrator.StageStatusCompleted || record.StateKind == "" {
		return "", false
	}
	raw, err := os.ReadFile(record.ArtifactPath)
	if err != nil {
		return fmt.Sprintf("- `%s`: its state could not be read", stageID), true
	}
	document, err := state.Decode(state.Kind(record.StateKind), raw)
	if err != nil {
		return fmt.Sprintf("- `%s`: its state could not be decoded", stageID), true
	}
	return describeResult(stageID, document)
}

func describeResult(stageID string, document state.Validatable) (string, bool) {
	switch result := document.(type) {
	case *state.ReviewState:
		return fmt.Sprintf("- **Code review** (`%s`): %s", stageID, result.Verdict), true
	case *state.SecurityState:
		return fmt.Sprintf("- **Security review** (`%s`): %s", stageID, findingCounts(result.Findings)), true
	case *state.QAState:
		tests := result.TestResults
		return fmt.Sprintf("- **Tests** (`%s`): %d passed, %d failed, %d skipped",
			stageID, tests.Passed, tests.Failed, tests.Skipped), true
	}
	return "", false
}

func findingCounts(findings []state.SecurityFinding) string {
	if len(findings) == 0 {
		return "no findings"
	}
	bySeverity := map[state.Severity]int{}
	for _, finding := range findings {
		bySeverity[finding.Severity]++
	}
	var parts []string
	for _, severity := range []state.Severity{state.SeverityCritical, state.SeverityHigh, state.SeverityMedium,
		state.SeverityLow, state.SeverityInfo} {
		if count := bySeverity[severity]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, strings.ToLower(string(severity))))
		}
	}
	return fmt.Sprintf("%d finding(s): %s", len(findings), strings.Join(parts, ", "))
}

// writeStages is every stage in run order, what it did and what it cost.
func writeStages(body *strings.Builder, runState *orchestrator.RunState, maxUSD float64) {
	body.WriteString("\n## Stages\n\n| Stage | Agent | Result | Cost |\n|---|---|---|---|\n")
	for _, stageID := range runState.StagesInSequence() {
		record := runState.Stages[stageID]
		fmt.Fprintf(body, "| `%s` | %s | %s | %s |\n", stageID, orDash(record.Agent), stageResult(record), stageCost(record))
	}
	fmt.Fprintf(body, "\n%s\n", spendLine(runState.TotalUsage().CostUSD, maxUSD))
}

func stageResult(record orchestrator.StageRecord) string {
	result := strings.ToLower(strings.ReplaceAll(string(record.Status), "_", " "))
	if record.SkipReason != "" {
		result += ": " + record.SkipReason
	}
	return strings.ReplaceAll(result, "|", "\\|")
}

func stageCost(record orchestrator.StageRecord) string {
	if record.Usage == nil {
		return "—"
	}
	return fmt.Sprintf("$%.2f", record.Usage.CostUSD)
}

func orDash(text string) string {
	if text == "" {
		return "—"
	}
	return text
}

func spendLine(totalUSD, maxUSD float64) string {
	if maxUSD <= 0 {
		return fmt.Sprintf("Measured spend: **$%.2f** (the order set no USD ceiling).", totalUSD)
	}
	return fmt.Sprintf("Measured spend: **$%.2f** of the order's $%.2f ceiling.", totalUSD, maxUSD)
}

// writeGates lists the run's gates as a human approved them. The commit
// and this pull request were approved in the factory inbox as well; the
// body is only ever used for a pull request a human approved.
func writeGates(body *strings.Builder, runState *orchestrator.RunState) {
	gates := make([]string, 0, len(runState.Approvals))
	for gate := range runState.Approvals {
		gates = append(gates, gate)
	}
	sort.Strings(gates)
	body.WriteString("\n## Gates\n\n")
	for _, gate := range gates {
		approval := runState.Approvals[gate]
		fmt.Fprintf(body, "- `%s`: approved by %s via %s at %s\n", gate, orDash(approval.Approver),
			approval.Method, approval.ApprovedAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	body.WriteString("- `commit` and `open-pr`: approved in the factory inbox, each bound to a digest\n")
}

// writeChangedFiles lists what the commit changed outside the delivery
// record, which has a section of its own.
func writeChangedFiles(body *strings.Builder, tree Tree, order *OrderState) {
	changed, err := tree.ChangedBetween(order.Claim.SourceCommit, order.Commit)
	if err != nil {
		fmt.Fprintf(body, "\n## Changed files\n\nThe changed files could not be listed: %v\n", err)
		return
	}
	var lines []string
	for _, path := range changed {
		if !strings.HasPrefix(path, DeliveryRecordDir(order.ID)+"/") {
			lines = append(lines, "- `"+path+"`")
		}
	}
	if len(lines) == 0 {
		lines = []string{"None outside the delivery record."}
	}
	fmt.Fprintf(body, "\n## Changed files\n\n%s\n", strings.Join(lines, "\n"))
}

func writeRecord(body *strings.Builder, id string) {
	fmt.Fprintf(body, "\n## Delivery record\n\nEvery stage's artifact, the run state and its event log are "+
		"committed in this pull request under `%s/`.\n", DeliveryRecordDir(id))
}

// writeRunSummary appends a summary the run wrote itself, when it wrote one.
func writeRunSummary(body *strings.Builder, order *OrderState) {
	for _, candidate := range []string{
		filepath.Join(order.Claim.Worktree, filepath.FromSlash(DeliveryRecordDir(order.ID)), "delivery-summary.md"),
		filepath.Join(order.Claim.Worktree, ".claude", "feature-workspace", order.ID, "delivery-summary.md"),
	} {
		if summary, err := os.ReadFile(candidate); err == nil {
			fmt.Fprintf(body, "\n## The run's own summary\n\n%s\n", strings.TrimSpace(string(summary)))
			return
		}
	}
}
