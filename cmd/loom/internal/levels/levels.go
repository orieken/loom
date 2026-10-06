// Package levels loads the agentic maturity level profiles from
// shared/levels.yaml (roadmap D.3). The installer selects install bundles
// from them; `loom health` (D.4) infers a project's current level from them.
package levels

import (
	"fmt"
	iofs "io/fs"
	"path"
	"strings"

	yaml "go.yaml.in/yaml/v4"
)

// ProfilePath is where the profile data lives in the embedded framework FS.
const ProfilePath = "shared/levels.yaml"

// MinLevel and MaxLevel bound the valid --level range.
const (
	MinLevel = 1
	MaxLevel = 4
)

// Module is an on-demand rule module opted in via --stack.
type Module struct {
	Stack string `yaml:"stack"`
	Path  string `yaml:"path"`
}

// Bundle is one installable unit of a level.
type Bundle struct {
	ID       string   `yaml:"id"`
	Paths    []string `yaml:"paths"`
	Requires string   `yaml:"requires"`
	Action   string   `yaml:"action"`
	// DocsOnly marks documentation content that installs normally but never
	// counts as maturity-level evidence for `loom health` (roadmap D.4).
	DocsOnly bool `yaml:"docsOnly"`
	// OptIn marks a bundle a project asks for rather than receives: it never
	// installs as part of a level and is never level evidence, so choosing not
	// to use it cannot lower a project's reported maturity.
	OptIn bool `yaml:"optIn"`
}

// Level is one maturity level profile.
type Level struct {
	Level           int      `yaml:"level"`
	Name            string   `yaml:"name"`
	Summary         string   `yaml:"summary"`
	Bundles         []Bundle `yaml:"bundles"`
	OnDemandModules []Module `yaml:"onDemandModules"`
}

// ContextBudget records the documented ceiling for the level 1 core bundle.
type ContextBudget struct {
	CoreRulesCeilingBytes int `yaml:"coreRulesCeilingBytes"`
}

// Profile is the parsed shared/levels.yaml.
type Profile struct {
	Version       int           `yaml:"version"`
	Landed        []string      `yaml:"landed"`
	ContextBudget ContextBudget `yaml:"contextBudget"`
	Levels        []Level       `yaml:"levels"`
}

// Load parses the profile from the embedded framework content.
func Load(content iofs.ReadFileFS) (Profile, error) {
	raw, err := content.ReadFile(ProfilePath)
	if err != nil {
		return Profile{}, fmt.Errorf("read %s: %w", ProfilePath, err)
	}
	var profile Profile
	if err := yaml.Unmarshal(raw, &profile); err != nil {
		return Profile{}, fmt.Errorf("parse %s: %w", ProfilePath, err)
	}
	return profile, validate(profile)
}

func validate(profile Profile) error {
	if len(profile.Levels) != MaxLevel {
		return fmt.Errorf("levels profile declares %d levels, want %d", len(profile.Levels), MaxLevel)
	}
	for index, level := range profile.Levels {
		if level.Level != index+MinLevel {
			return fmt.Errorf("levels profile out of order: entry %d declares level %d", index, level.Level)
		}
	}
	return nil
}

// Select returns levels MinLevel..target, cumulatively.
func (p Profile) Select(target int) ([]Level, error) {
	if target < MinLevel || target > MaxLevel {
		return nil, fmt.Errorf("level must be between %d and %d, got %d", MinLevel, MaxLevel, target)
	}
	return p.Levels[:target], nil
}

// IsLanded reports whether the roadmap item id has shipped.
func (p Profile) IsLanded(id string) bool {
	for _, landed := range p.Landed {
		if landed == id {
			return true
		}
	}
	return false
}

// CoreRuleNames returns the level 1 core rule filenames (basenames).
func (p Profile) CoreRuleNames() ([]string, error) {
	bundle, ok := p.findLevelOneBundle("core-rules")
	if !ok {
		return nil, fmt.Errorf("levels profile has no level 1 core-rules bundle")
	}
	names := make([]string, 0, len(bundle.Paths))
	for _, rulePath := range bundle.Paths {
		names = append(names, path.Base(rulePath))
	}
	return names, nil
}

// OnDemandRuleName returns the rule filename for an opt-in stack, or "" when
// the stack is unknown.
func (p Profile) OnDemandRuleName(stack string) string {
	for _, module := range p.Levels[0].OnDemandModules {
		if module.Stack == stack {
			return path.Base(module.Path)
		}
	}
	return ""
}

// InstallDestination maps a bundle source path to its install destination
// inside a target project. The installer writes there and `loom health`
// checks there — one mapping so the two can never diverge.
func InstallDestination(source string) string {
	if strings.HasPrefix(source, "shared/rules/") {
		return ".claude/rules/" + path.Base(source)
	}
	if path.Ext(source) == ".md" {
		return path.Base(source)
	}
	return ".claude/" + path.Base(source)
}

func (p Profile) findLevelOneBundle(id string) (Bundle, bool) {
	for _, bundle := range p.Levels[0].Bundles {
		if bundle.ID == id {
			return bundle, true
		}
	}
	return Bundle{}, false
}
