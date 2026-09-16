// Package policy loads and validates the delivery policies that decide
// whether a gate may proceed without a human (roadmap L2.16).
//
// It replaces a prompt. `policy-evaluator.md` specified a condition
// language whose evaluator was natural-language reasoning over YAML, with
// the kill-switch, the always-human list, and the conflict-resolution table
// all in the same prose the model might misread. An authorization decision
// — may this pipeline commit without a human — is the last thing that
// should be resolved by reading comprehension.
//
// There is deliberately no expression language here. The condition
// vocabulary is closed and small: about ten checks over facts the executor
// already holds. CEL would buy arbitrary boolean logic at the cost of a
// dependency, an evaluation-cost surface, and a gate decision written in a
// language most readers of a .policy.yaml will not know. Epic 82 reached
// the same conclusion for loop conditions. The trade is that adding a check
// is a code change — which is the right direction for a mechanism whose
// purpose is skipping human review.
package policy

import "fmt"

// GateID names a gate a policy can watch.
type GateID string

// The gates policies may target. These mirror the nine gates in
// shared/rules/approval-gates.md.
const (
	GateGitCommit             GateID = "git-commit"
	GateOutOfBoundaryWrite    GateID = "out-of-boundary-write"
	GateFitnessFunctionWiring GateID = "fitness-function-wiring"
	GateShipToFriday          GateID = "ship-to-friday"
	GateDBMigration           GateID = "db-migration"
	GateDBContractPhase       GateID = "db-contract-phase"
	GateExternalAPI           GateID = "external-api"
	GateDeploy                GateID = "deploy"
	GateTestRemoval           GateID = "test-removal"
)

// The executor's own gates (roadmap L2.13). These name stage progressions
// rather than the irreversible actions above, and they are the only gates
// `loom run` actually halts at — the two vocabularies had no overlap at
// all, so before these were eligible no valid policy could name a barrier
// that exists.
//
// GateConfirmShip needs care in the docs: it PRECEDES the ship, commit and
// deploy gates rather than being one of them. A policy on it does not
// authorise a deployment, and a reader who assumes otherwise has misread it.
const (
	GateConfirmDesign           GateID = "confirm-design"
	GateConfirmSecurity         GateID = "confirm-security"
	GateConfirmShip             GateID = "confirm-ship"
	GateConfirmUnresolvedReview GateID = "confirm-unresolved-review"
)

// alwaysHuman is the compiled kill-switch. It is a Go constant rather than
// a YAML field on purpose: a list that can be edited by whoever is trying
// to bypass it is not a control, and one that lives in prose a model reads
// is the defect this package removes.
//
// Each entry's reason is the one from approval-gates.md, so a rejection
// message says why rather than only that.
func alwaysHuman() map[GateID]string {
	return map[GateID]string{
		GateShipToFriday:    "external shared reporting metrics cannot be rolled back",
		GateDBMigration:     "modifies live stateful infrastructure; a partially applied run can corrupt data",
		GateDBContractPhase: "data destruction is irreversible",
		GateExternalAPI:     "third-party mutations have no guaranteed rollback path",
		GateDeploy:          "deployment failures can cause production downtime",
		// Gate #9 existed in the rule for days before it existed here, and
		// the gap was not benign: `unknown gate` is the message a typo gets,
		// so anyone reconciling the two lists would reasonably have added
		// this to eligible() instead. L2.19 (honour a policy decision at a
		// gate) rests its safety argument on always-human gates being
		// unreachable, and a test removal auto-approved by policy is the
		// exact one-way door gate #9 was created to hold.
		GateTestRemoval: "losing regression signal is silent, and what the test protected " +
			"is a fact no run state carries",
	}
}

// eligible lists the gates a policy may target.
func eligible() map[GateID]bool {
	gates := make(map[GateID]bool, len(EligibleGates()))
	for _, gate := range EligibleGates() {
		gates[gate] = true
	}
	return gates
}

// CheckGate reports why a gate may not be targeted, or nil when it may.
//
// This is a LOAD-time check, not an evaluation-time one. The schema used to
// say policies targeting an always-human gate are "silently ignored" —
// which means someone who writes a policy to auto-approve a deployment sees
// no error and concludes it worked. Silence is the wrong answer to a
// request that will never be honoured.
func CheckGate(gate GateID) error {
	if reason, blocked := alwaysHuman()[gate]; blocked {
		return fmt.Errorf("gate %q is always human and cannot be governed by a policy: %s "+
			"(shared/rules/approval-gates.md)", gate, reason)
	}
	if !eligible()[gate] {
		return fmt.Errorf("unknown gate %q — valid policy-eligible gates are %s", gate, eligibleList())
	}
	return nil
}

// EligibleGates returns the gates a policy may target, for error messages
// and documentation.
func EligibleGates() []GateID {
	return []GateID{
		GateGitCommit, GateOutOfBoundaryWrite, GateFitnessFunctionWiring,
		GateConfirmDesign, GateConfirmSecurity, GateConfirmShip, GateConfirmUnresolvedReview,
	}
}

func eligibleList() string {
	names := ""
	for index, gate := range EligibleGates() {
		if index > 0 {
			names += ", "
		}
		names += string(gate)
	}
	return names
}
