package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/policy"
	"github.com/spf13/cobra"
)

// chdirToProjectWithConfig puts the test in a fresh project directory,
// optionally with a .claude/delivery-policy.yaml — loadPolicies reads
// policy.ConfigFile, which is a path relative to the working directory.
func chdirToProjectWithConfig(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	if content != "" {
		claudeDir := filepath.Join(dir, ".claude")
		if err := os.MkdirAll(claudeDir, 0o755); err != nil {
			t.Fatalf("mkdir .claude: %v", err)
		}
		path := filepath.Join(claudeDir, "delivery-policy.yaml")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write delivery-policy.yaml: %v", err)
		}
	}
	t.Chdir(dir)
}

func runLoadPolicies(t *testing.T) ([]policy.Policy, []policy.PreShipCheck, error) {
	t.Helper()
	command := &cobra.Command{}
	var out bytes.Buffer
	command.SetOut(&out)
	return loadPolicies(command, policy.DefaultDir)
}

func TestLoadPoliciesWithNoConfigReturnsNoPreShipChecks(t *testing.T) {
	chdirToProjectWithConfig(t, "")

	policies, checks, err := runLoadPolicies(t)

	if err != nil {
		t.Fatalf("loadPolicies: %v", err)
	}
	if policies != nil || checks != nil {
		t.Errorf("policies = %v, checks = %v, want both nil for a project with no config", policies, checks)
	}
}

// preShipChecks is a required check, not automation/approval delegation, so
// it must survive the policiesEnabled: false kill switch that disables
// everything else loadPolicies returns.
func TestLoadPoliciesReturnsPreShipChecksWhenThePoliciesKillSwitchIsOff(t *testing.T) {
	chdirToProjectWithConfig(t, "policiesEnabled: false\n"+
		"preShipChecks:\n  - name: coverage\n    command: scripts/pre-ship-check.sh\n")

	policies, checks, err := runLoadPolicies(t)

	if err != nil {
		t.Fatalf("loadPolicies: %v", err)
	}
	if policies != nil {
		t.Errorf("policies = %v, want nil — the kill switch disables automation", policies)
	}
	if len(checks) != 1 || checks[0].Name != "coverage" {
		t.Errorf("checks = %+v, want the one configured preShipCheck", checks)
	}
}

func TestLoadPoliciesReturnsPreShipChecksAlongsidePolicies(t *testing.T) {
	chdirToProjectWithConfig(t, "preShipChecks:\n  - name: coverage\n    command: scripts/pre-ship-check.sh\n")

	_, checks, err := runLoadPolicies(t)

	if err != nil {
		t.Fatalf("loadPolicies: %v", err)
	}
	if len(checks) != 1 {
		t.Errorf("checks = %+v, want one", checks)
	}
}

func TestLoadPoliciesFailsOnAnInvalidConfig(t *testing.T) {
	chdirToProjectWithConfig(t, "mode: policy-driven\n")

	_, _, err := runLoadPolicies(t)

	if !errors.Is(err, policy.ErrInvalidConfig) {
		t.Errorf("err = %v, want ErrInvalidConfig", err)
	}
}

// TestLoadPoliciesFailsWhenAPolicyFileFailsToParse covers the branch
// policy.Load itself returns an error on — a valid delivery-policy.yaml
// paired with a malformed *.policy.yaml in the policies directory.
func TestLoadPoliciesFailsWhenAPolicyFileFailsToParse(t *testing.T) {
	chdirToProjectWithConfig(t, "")
	policiesDir := filepath.Join(".", policy.DefaultDir)
	if err := os.MkdirAll(policiesDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", policiesDir, err)
	}
	badPolicy := filepath.Join(policiesDir, "broken.policy.yaml")
	if err := os.WriteFile(badPolicy, []byte("not: [valid\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", badPolicy, err)
	}

	policies, checks, err := runLoadPolicies(t)

	if err == nil {
		t.Fatal("loadPolicies succeeded; a malformed policy file must fail it")
	}
	if !strings.Contains(err.Error(), policy.DefaultDir) {
		t.Errorf("err %q does not name the policy directory", err)
	}
	if policies != nil || checks != nil {
		t.Errorf("policies = %v, checks = %v, want both nil on this failure path", policies, checks)
	}
}
