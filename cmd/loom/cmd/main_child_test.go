package cmd

import (
	"os"
	"testing"
)

// actAsLoomEnv makes this test binary behave as the loom binary. The factory
// launches os.Executable() for every run; in an in-process test that is this
// binary, so a child started with the variable set runs loom instead of the
// tests (and in-process tests can measure the factory's own lines).
const actAsLoomEnv = "LOOM_TEST_ACT_AS_LOOM"

func TestMain(m *testing.M) {
	if os.Getenv(actAsLoomEnv) == "1" {
		Execute(nil, nil)
		os.Exit(0)
	}
	code := m.Run()
	if builtBinaryDir != "" {
		os.RemoveAll(builtBinaryDir)
	}
	os.Exit(code)
}
