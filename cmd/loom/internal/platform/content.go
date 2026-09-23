package platform

import (
	"fmt"
	"strings"
)

const craftsmanship = `## Craftsmanship Rules

You must **strictly adhere** to the patterns defined in ARCHITECTURE_RULES.md (Clean Architecture, DDD, GoF patterns, and micro-rules).
- **Tests Are Proof**: Feature code is incomplete without tests that can fail. Changed lines >= 85% covered; test-first is a technique, not a rule (ADR-008, ADR-009).
- **Kent Beck (Simple Design)**: 1) Passes tests, 2) Reveals intention, 3) No duplication, 4) Fewest elements.
- **Martin Fowler (Refactoring)**: Use named refactoring operations instead of vague cleanups.
- **Architectural Constraints & Fitness Functions**: Enforce cyclomatic complexity below 7 and functions below 30 LOC.
- **The Boy Scout Rule**: Always leave the code cleaner than you found it.

## Tech Stack

- **Backend / MCP**: Go
- **Frontend**: Vue 3 + Tailwind CSS
- **Test Automation**: TypeScript, Playwright, Cucumber.js, k6
`

func combinedRules(environment Environment) (string, error) {
	names, err := environment.Rules.Names(environment.Content)
	if err != nil {
		return "", err
	}
	var content strings.Builder
	for _, name := range names {
		rule, readErr := environment.Content.ReadFile("shared/rules/" + name)
		if readErr != nil {
			return "", fmt.Errorf("read rule %s: %w", name, readErr)
		}
		content.WriteString("\n")
		content.Write(rule)
		content.WriteString("\n")
	}
	return content.String(), nil
}

func agentRoster(content Content) (string, error) {
	agents, err := readAgents(content)
	if err != nil {
		return "", err
	}
	var roster strings.Builder
	roster.WriteString("## Persona Roster\n\nThe following specialized personas are available. Invoke them by name when you need domain-specific expertise.\n")
	for _, agent := range agents {
		roster.WriteString(agentRosterLine(agent))
	}
	return roster.String(), nil
}

func aggregateInstructions(environment Environment, header string) ([]byte, error) {
	rules, err := combinedRules(environment)
	if err != nil {
		return nil, err
	}
	roster, err := agentRoster(environment.Content)
	if err != nil {
		return nil, err
	}
	content := header + "\n\n## AI Feature Team & Global Rules\n" + rules + "\n" + craftsmanship + "\n" + roster + "\n"
	return []byte(content), nil
}

func agentRosterLine(agent agentDocument) string {
	if agent.status == "deprecated" && agent.successor != "" {
		return fmt.Sprintf("\n- **%s** *(deprecated — use %s)*: %s", agent.name, agent.successor, agent.description)
	}
	if agent.status == "deprecated" {
		return fmt.Sprintf("\n- **%s** *(deprecated)*: %s", agent.name, agent.description)
	}
	return fmt.Sprintf("\n- **%s**: %s", agent.name, agent.description)
}
