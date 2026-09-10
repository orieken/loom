package claude

import (
	"slices"
	"strings"
	"testing"
)

// The two writing stages must actually be allowed to write. `claude -p`
// denies Write and Edit by default, and the provider passed no permission
// flags at all — the second real run's developer reported files it had
// modified against an empty git diff (roadmap L2.22).
func TestAWritingAgentIsAllowedToWrite(t *testing.T) {
	definition := []byte("---\nname: developer\ntools: Read, Write, Edit, MultiEdit, Bash, Glob, Grep\n---\n\nbody\n")

	allowed := allowedToolsFor(definition)

	for _, tool := range []string{"Write", "Edit", "MultiEdit"} {
		if !slices.Contains(allowed, tool) {
			t.Errorf("%q is absent from %v; the stage cannot write", tool, allowed)
		}
	}
}

// A reviewing agent declares no write tools, so it is granted none. The
// executor decides what a stage may do, and the declaration is the decision.
func TestAReviewingAgentIsNotGrantedWriteAccess(t *testing.T) {
	definition := []byte("---\nname: code-reviewer\ntools: Read, Glob, Grep, Bash\n---\n\nbody\n")

	allowed := allowedToolsFor(definition)

	for _, tool := range []string{"Write", "Edit", "MultiEdit"} {
		if slices.Contains(allowed, tool) {
			t.Errorf("%q was granted to a read-only agent: %v", tool, allowed)
		}
	}
}

// An agent that declares nothing has not asked to write. Inferring that it
// meant to would reinstate the silent failure the other way round.
func TestAnAgentDeclaringNoToolsGetsAReadOnlyPosture(t *testing.T) {
	for name, definition := range map[string]string{
		"no tools line":  "---\nname: x\nmodel_tier: default\n---\n\nbody\n",
		"no frontmatter": "You are an agent.\n",
	} {
		t.Run(name, func(t *testing.T) {
			allowed := allowedToolsFor([]byte(definition))
			if slices.Contains(allowed, "Write") {
				t.Errorf("an undeclared agent was granted Write: %v", allowed)
			}
			if len(allowed) == 0 {
				t.Error("an undeclared agent was granted nothing; it cannot even read")
			}
		})
	}
}

// A `tools:` line in the body is prose about tools, not a declaration.
func TestOnlyTheLeadingFrontmatterDeclaresTools(t *testing.T) {
	definition := []byte("---\nname: x\ntools: Read\n---\n\nSome prose.\n\ntools: Write, Edit\n")

	if allowed := allowedToolsFor(definition); slices.Contains(allowed, "Write") {
		t.Errorf("a tools: line in the body granted Write: %v", allowed)
	}
}

// The allowlist is the mechanism. bypassPermissions would make it
// decorative, so the posture must never reach for it.
func TestThePostureNeverBypassesPermissions(t *testing.T) {
	args := strings.Join(permissionArgs([]string{"Read", "Write"}), " ")

	if strings.Contains(args, "bypassPermissions") || strings.Contains(args, "dangerously") {
		t.Errorf("permission args bypass the allowlist: %s", args)
	}
	if !strings.Contains(args, "--allowed-tools Read,Write") {
		t.Errorf("permission args do not carry the allowlist: %s", args)
	}
}
