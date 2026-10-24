package policy_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/policy"
)

// ADR-010 (4B): automation lives only in .policy.yaml files, which loom run
// evaluates and records. delivery-policy.yaml keeps the kill switch and the
// markdown pipeline's retry bound, and is validated strictly — a key that
// once switched on silent auto-approval must fail loudly, not be ignored.

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "delivery-policy.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigDefaultsWhenTheFileIsAbsent(t *testing.T) {
	config, err := policy.LoadConfig(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !config.PoliciesEvaluated() || config.ContractRetries() != policy.DefaultMaxContractRetries {
		t.Errorf("config = %+v, want evaluation on and %d retries", config, policy.DefaultMaxContractRetries)
	}
}

func loadKeptKeys(t *testing.T) policy.Config {
	t.Helper()
	path := writeConfig(t, "policiesEnabled: false\nmaxContractRetries: 5\n"+
		"workflowAuditsEnabled: false\norchestrationMode: runtime\n")
	config, err := policy.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return config
}

func TestLoadConfigReadsTheCoreKeys(t *testing.T) {
	config := loadKeptKeys(t)
	if config.PoliciesEvaluated() || config.ContractRetries() != 5 {
		t.Errorf("config = %+v, want policies off and 5 retries", config)
	}
}

func TestLoadConfigReadsTheOrchestrateKeys(t *testing.T) {
	config := loadKeptKeys(t)
	audits := config.WorkflowAuditsEnabled
	if audits == nil || *audits || config.OrchestrationMode != "runtime" {
		t.Errorf("config = %+v, want audits off and runtime orchestration", config)
	}
}

// The removed keys are named, with where automation went, so a project that
// set them learns what to do rather than seeing a parse error.
func TestLoadConfigRejectsEveryRemovedKeyByName(t *testing.T) {
	path := writeConfig(t, `version: "1.0"
mode: "policy-driven"
defaults:
  maxContractRetries: 3
gates:
  gitCommit:
    autoProceed: true
stages:
  phase1_discovery:
    autoProceedAnalyst: true
escalations:
  haltOnSecurityCritical: true
`)
	_, err := policy.LoadConfig(path)

	if !errors.Is(err, policy.ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
	for _, want := range []string{"`defaults`", "`escalations`", "`gates`", "`mode`", "`stages`", "`version`", "ADR-010", ".policy.yaml", "MIGRATION.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

// Keys the old skill read at the top level are removed there too.
func TestLoadConfigRejectsTopLevelAutoProceedKeys(t *testing.T) {
	for _, key := range []string{"maxDiffLines: 200", "requireGreenTests: true", "autoProceedPersistence: false"} {
		t.Run(key, func(t *testing.T) {
			_, err := policy.LoadConfig(writeConfig(t, key+"\n"))
			if !errors.Is(err, policy.ErrInvalidConfig) || !strings.Contains(err.Error(), "ADR-010") {
				t.Errorf("err = %v, want it rejected as removed by ADR-010", err)
			}
		})
	}
}

// A misspelt key is not silently ignored: the author believes it is set.
func TestLoadConfigRejectsAnUnknownKey(t *testing.T) {
	_, err := policy.LoadConfig(writeConfig(t, "policiesEnabld: false\n"))
	if !errors.Is(err, policy.ErrInvalidConfig) || !strings.Contains(err.Error(), "unknown key `policiesEnabld`") {
		t.Errorf("err = %v, want the unknown key named", err)
	}
}

func TestLoadConfigRejectsBadValues(t *testing.T) {
	for _, tc := range []struct{ name, content, want string }{
		{"negative retries", "maxContractRetries: -1\n", "maxContractRetries"},
		{"unknown orchestration mode", "orchestrationMode: turbo\n", "orchestrationMode"},
		{"not a mapping", "- a list\n", "mapping"},
		{"malformed yaml", "policiesEnabled: [unclosed\n", "delivery-policy.yaml"},
		{"wrong type", "policiesEnabled: sometimes\n", "delivery-policy.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := policy.LoadConfig(writeConfig(t, tc.content))
			if !errors.Is(err, policy.ErrInvalidConfig) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want ErrInvalidConfig mentioning %q", err, tc.want)
			}
		})
	}
}

func TestLoadConfigTreatsAnEmptyFileAsDefaults(t *testing.T) {
	config, err := policy.LoadConfig(writeConfig(t, ""))
	if err != nil || !config.PoliciesEvaluated() || config.ContractRetries() != policy.DefaultMaxContractRetries {
		t.Errorf("config = %+v, err = %v, want the defaults", config, err)
	}
}

// A file that exists but cannot be read is not the same as no file.
func TestLoadConfigReportsAnUnreadableFile(t *testing.T) {
	path := writeConfig(t, "")
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Mkdir(path, 0o755); err != nil { // a directory where the file belongs
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := policy.LoadConfig(path); err == nil {
		t.Error("LoadConfig read a directory as an empty config")
	}
}

// The shipped template must be valid under its own schema.
func TestTheShippedDeliveryPolicyTemplateIsValid(t *testing.T) {
	if _, err := policy.LoadConfig("../../shared/templates/delivery-policy.template.yaml"); err != nil {
		t.Errorf("the template fails its own validation: %v", err)
	}
}
