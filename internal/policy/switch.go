package policy

// The global kill-switch.
//
// `policiesEnabled: false` has been documented in three places since v3.3 —
// approval-gates.md, shared/policies/README.md, and step 1 of the
// evaluator's own algorithm — and nothing read it. An off-switch that
// exists only in prose is the same defect as an evaluator that exists only
// in prose, and this package was written to remove one of those.
//
// It is implemented now, while nothing acts on a decision, precisely
// because that is the cheap moment. The change that starts honouring
// decisions inherits a working switch rather than having to build one under
// pressure.

import (
	"fmt"
	"os"
)

// ConfigFile is the project's delivery policy configuration.
const ConfigFile = ".claude/delivery-policy.yaml"

// Enabled reports whether policy evaluation should run at all. A missing
// file, an unreadable one, or a malformed one means enabled — the default
// has to be the behaviour the framework had before the switch existed, and
// evaluation changes nothing on its own.
//
// Note the asymmetry: only an explicit `policiesEnabled: false` disables.
// A typo cannot silently switch evaluation off, because a switch that turns
// itself off by accident is worse than one nobody set.
//
// This reader stays tolerant on purpose; LoadConfig is the strict one, and
// `loom run` calls it before anything else (ADR-010). Both decode the same
// Config, so there is one schema.
func Enabled(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	config, err := decodeConfig(raw)
	if err != nil {
		return true
	}
	return config.PoliciesEvaluated()
}

// DisabledNotice explains a skipped evaluation, for a caller that wants to
// say why nothing was evaluated rather than printing nothing.
func DisabledNotice(path string) string {
	return fmt.Sprintf("policy evaluation is off (%s sets policiesEnabled: false)", path)
}
