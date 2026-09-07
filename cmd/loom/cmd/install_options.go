package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	frameworkfs "github.com/orieken/loom/cmd/loom/internal/fs"
	"github.com/orieken/loom/cmd/loom/internal/levels"
	"github.com/orieken/loom/cmd/loom/internal/platform"
)

type installRequest struct {
	target           string
	cache            string
	frameworkVersion string
	platforms        []string
	rules            platform.RuleSet
	level            int
	isForced         bool
	profile          levels.Profile
	isCopy           bool
	isDryRun         bool
	withConfig       bool
	withMCP          bool
}

func prepareInstall(flags installFlags, embeddedVersion string, content platform.Content) (installRequest, error) {
	target, err := frameworkfs.ResolveTarget(flags.target)
	if err != nil {
		return installRequest{}, err
	}
	level, profile, rules, err := selectLevelAndRules(flags, content)
	if err != nil {
		return installRequest{}, err
	}
	platforms, err := selectPlatforms(target, flags.platform)
	if err != nil {
		return installRequest{}, err
	}
	cache, err := frameworkCache(embeddedVersion)
	return installRequest{target, cache, embeddedVersion, platforms, rules, level, flags.isForced, profile, flags.isCopy, flags.isDryRun, flags.withConfig, flags.withMCP}, err
}

// selectLevelAndRules resolves the rule selection. Without --level the
// historic behavior is unchanged: all rules, or --stack filtering. With
// --level N, shared/levels.yaml decides the bundle: core rules plus any
// --stack opt-in modules.
func selectLevelAndRules(flags installFlags, content platform.Content) (int, levels.Profile, platform.RuleSet, error) {
	if flags.level == 0 {
		rules, err := platform.ParseRuleSet(flags.stack)
		return 0, levels.Profile{}, rules, err
	}
	profile, err := levels.Load(content)
	if err != nil {
		return 0, levels.Profile{}, platform.RuleSet{}, err
	}
	if _, err := profile.Select(flags.level); err != nil {
		return 0, levels.Profile{}, platform.RuleSet{}, err
	}
	rules, err := levelRuleSet(profile, flags.stack)
	return flags.level, profile, rules, err
}

func selectPlatforms(target, selected string) ([]string, error) {
	if selected == "" {
		return platform.Detect(target)
	}
	if !platform.IsKnown(selected) {
		return nil, fmt.Errorf("unknown platform %q (valid: %s)", selected, strings.Join(platform.Names(), ", "))
	}
	return []string{selected}, nil
}

func frameworkCache(embeddedVersion string) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache: %w", err)
	}
	return filepath.Join(cache, "loom", embeddedVersion), nil
}
