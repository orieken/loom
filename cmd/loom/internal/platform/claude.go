package platform

import "fmt"

func installClaude(environment Environment) ([]string, error) {
	paths, err := installSources(environment, []sourceDestination{
		{"shared/agents", ".claude/agents"},
		{"shared/skills", ".claude/skills"},
	})
	if err != nil {
		return nil, err
	}
	rulePaths, err := installClaudeRules(environment)
	if err != nil {
		return nil, err
	}
	paths = append(paths, rulePaths...)
	return installClaudeProjectFiles(environment, paths)
}

func installClaudeRules(environment Environment) ([]string, error) {
	return installRuleDirectory(environment, ".claude/rules")
}

func installRuleDirectory(environment Environment, destination string) ([]string, error) {
	names, err := environment.Rules.Names(environment.Content)
	if err != nil {
		return nil, err
	}
	if !environment.Rules.isFiltered() {
		return installSources(environment, []sourceDestination{{"shared/rules", destination}})
	}
	if err := environment.Files.PrepareDirectory(destination); err != nil {
		return nil, fmt.Errorf("prepare filtered rules directory: %w", err)
	}
	for _, name := range names {
		_, installErr := environment.Files.Install("shared/rules/"+name, destination+"/"+name)
		if installErr != nil {
			return nil, fmt.Errorf("install filtered rule %s: %w", name, installErr)
		}
	}
	return []string{destination}, nil
}

// installClaudeProjectFiles seeds the two documents a project is expected to
// own and edit.
//
// They are copied only when absent, never linked (roadmap L3.26). Both hold
// project-specific content — design-principles.md §6 requires every domain
// term to match DOMAIN_DICTIONARY.md — so linking them to a shared cache
// silently replaces what the rules are checked against, and replacing an
// existing one discards work nobody asked to lose. CLAUDE.md has always been
// treated this way; these two are the same kind of file.
func installClaudeProjectFiles(environment Environment, paths []string) ([]string, error) {
	for _, pair := range []sourceDestination{
		{"shared/ARCHITECTURE_RULES.md", "ARCHITECTURE_RULES.md"},
		{"shared/DOMAIN_DICTIONARY.md", "DOMAIN_DICTIONARY.md"},
	} {
		installed, err := environment.Files.CopyIfMissing(pair.source, pair.destination)
		if err != nil {
			return nil, err
		}
		if installed {
			paths = append(paths, pair.destination)
		}
	}
	return installClaudeTemplates(environment, paths)
}

func installClaudeTemplates(environment Environment, paths []string) ([]string, error) {
	installed, err := environment.Files.CopyIfMissing("templates/claude-feature-team/CLAUDE.md", "CLAUDE.md")
	if err != nil {
		return nil, err
	}
	if installed {
		paths = append(paths, "CLAUDE.md")
	}
	installed, err = environment.Files.CopyIfMissing("templates/claude-feature-team/features", "features")
	if err != nil {
		return nil, err
	}
	if installed {
		paths = append(paths, "features")
	}
	return paths, nil
}
