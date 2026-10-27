package policy_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/policy"
)

func TestLoadConfigDecodesAPreShipCheckWithTheDefaultTimeout(t *testing.T) {
	path := writeConfig(t, "preShipChecks:\n  - name: coverage\n    command: scripts/pre-ship-check.sh\n")

	config, err := policy.LoadConfig(path)

	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(config.PreShipChecks) != 1 {
		t.Fatalf("PreShipChecks = %+v, want one entry", config.PreShipChecks)
	}
	check := config.PreShipChecks[0]
	if check.Name != "coverage" || check.Command != "scripts/pre-ship-check.sh" {
		t.Errorf("check = %+v, want name/command decoded", check)
	}
	if check.Timeout != 10*time.Minute {
		t.Errorf("check.Timeout = %v, want the 10-minute default", check.Timeout)
	}
}

func TestLoadConfigDecodesAnExplicitPreShipCheckTimeout(t *testing.T) {
	path := writeConfig(t, "preShipChecks:\n  - name: coverage\n    command: scripts/x.sh\n    timeout: 5m\n")

	config, err := policy.LoadConfig(path)

	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := config.PreShipChecks[0].Timeout; got != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", got)
	}
}

func TestLoadConfigWithNoPreShipChecksKeyDecodesToNone(t *testing.T) {
	config, err := policy.LoadConfig(writeConfig(t, "maxContractRetries: 2\n"))

	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(config.PreShipChecks) != 0 {
		t.Errorf("PreShipChecks = %+v, want none", config.PreShipChecks)
	}
}

func TestLoadConfigRejectsAPreShipCheckWithNoName(t *testing.T) {
	_, err := policy.LoadConfig(writeConfig(t, "preShipChecks:\n  - command: scripts/x.sh\n"))

	if !errors.Is(err, policy.ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
	for _, want := range []string{`"scripts/x.sh"`, "no name"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err %q does not mention %q", err, want)
		}
	}
}

func TestLoadConfigRejectsAPreShipCheckWithNoCommand(t *testing.T) {
	_, err := policy.LoadConfig(writeConfig(t, "preShipChecks:\n  - name: coverage\n"))

	if !errors.Is(err, policy.ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
	for _, want := range []string{`"coverage"`, "no command"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err %q does not mention %q", err, want)
		}
	}
}

func TestLoadConfigRejectsAPreShipCheckWithAnUnknownKey(t *testing.T) {
	path := writeConfig(t, "preShipChecks:\n  - name: coverage\n    command: scripts/x.sh\n    retries: 3\n")

	_, err := policy.LoadConfig(path)

	if !errors.Is(err, policy.ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
	for _, want := range []string{`"coverage"`, "unknown key \"retries\""} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err %q does not mention %q", err, want)
		}
	}
}

func TestLoadConfigRejectsAPreShipCheckWithAnUnparseableTimeout(t *testing.T) {
	path := writeConfig(t, "preShipChecks:\n  - name: coverage\n    command: scripts/x.sh\n    timeout: soon\n")

	_, err := policy.LoadConfig(path)

	if !errors.Is(err, policy.ErrInvalidConfig) || !strings.Contains(err.Error(), "soon") {
		t.Errorf("err = %v, want ErrInvalidConfig naming the bad timeout value", err)
	}
}

func TestLoadConfigRejectsAPreShipCheckWithNeitherNameNorCommand(t *testing.T) {
	_, err := policy.LoadConfig(writeConfig(t, "preShipChecks:\n  - timeout: 5m\n"))

	if !errors.Is(err, policy.ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
	for _, want := range []string{"at position 0", "no name"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err %q does not mention %q — an entry with neither name nor command must be named by its position", err, want)
		}
	}
}

func TestLoadConfigRejectsAPreShipChecksThatIsNotAList(t *testing.T) {
	_, err := policy.LoadConfig(writeConfig(t, "preShipChecks: not-a-list\n"))

	if !errors.Is(err, policy.ErrInvalidConfig) {
		t.Errorf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigRejectsAPreShipCheckEntryThatIsNotAMapping(t *testing.T) {
	_, err := policy.LoadConfig(writeConfig(t, "preShipChecks:\n  - - nested\n"))

	if !errors.Is(err, policy.ErrInvalidConfig) {
		t.Errorf("err = %v, want ErrInvalidConfig", err)
	}
}
