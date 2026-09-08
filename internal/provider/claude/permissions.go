package claude

// Permission posture for a stage (roadmap L2.22).
//
// `claude -p` denies every Write and Edit by default, and the provider used
// to invoke it with no permission flags at all. No stage could write a file.
// The second real end-to-end run's developer produced a complete
// implementation-notes.md naming the files it had modified, with a
// self-review checklist claiming the build passed, against an empty
// `git diff` — the pipeline's central promise did not hold on a default
// install, and it failed silently.
//
// What a stage may do is the executor's decision, not a host default. Every
// agent definition already declares its tools in frontmatter, so that
// declaration is the posture: it is versioned with the agent, reviewed with
// the agent, and cannot drift from it.

import (
	"strings"
)

// defaultAllowedTools is the posture for an agent that declares none. It is
// read-only on purpose: a stage whose definition says nothing about tools
// has not asked to write, and inferring that it meant to would put the
// silent-failure back the other way round.
func defaultAllowedTools() []string {
	return []string{"Read", "Glob", "Grep"}
}

// allowedToolsFor returns the tools a stage may use, read from the `tools:`
// line of its agent definition's YAML frontmatter.
func allowedToolsFor(definition []byte) []string {
	declared := frontmatterTools(string(definition))
	if len(declared) == 0 {
		return defaultAllowedTools()
	}
	return declared
}

// frontmatterTools reads `tools: A, B, C` from the leading `---` block. Only
// the leading block is scanned: a `tools:` line further down the document is
// prose about tools, not a declaration.
func frontmatterTools(definition string) []string {
	for _, line := range frontmatterLines(definition) {
		if value, found := strings.CutPrefix(line, "tools:"); found {
			return splitTools(value)
		}
	}
	return nil
}

func frontmatterLines(definition string) []string {
	if !strings.HasPrefix(definition, "---\n") {
		return nil
	}
	rest := definition[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil
	}
	return strings.Split(rest[:end], "\n")
}

func splitTools(value string) []string {
	tools := make([]string, 0, 8)
	for _, name := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			tools = append(tools, trimmed)
		}
	}
	return tools
}

// permissionArgs are the flags that carry the posture to `claude -p`.
//
// --allowed-tools is the whole mechanism: it pre-authorises exactly the
// declared tools and nothing else. --permission-mode acceptEdits removes the
// interactive confirmation that a headless run has nobody to answer; it does
// not widen the tool list, so a tool the agent never declared stays denied.
// bypassPermissions is deliberately not used — it would make the allowlist
// decorative.
func permissionArgs(allowed []string) []string {
	if len(allowed) == 0 {
		return nil
	}
	return []string{"--permission-mode", "acceptEdits", "--allowed-tools", strings.Join(allowed, ",")}
}
