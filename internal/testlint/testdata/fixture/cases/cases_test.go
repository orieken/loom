package cases

import (
	"log/slog"
	"testing"

	"example.com/fixture/helpers"
	checks "example.com/fixture/helpers"
)

func add(a, b int) int { return a + b }

// --- must be flagged -------------------------------------------------------

func TestCallsButNeverChecks(t *testing.T) {
	add(1, 2)
}

func TestLogsInsteadOfFailing(t *testing.T) {
	if add(1, 2) != 3 {
		t.Log("wrong sum")
	}
}

// A logger's Error method is not a test failure. Matching on the method name
// alone would make this test look checked. The receiver is a named variable
// on purpose: the first version called slog.Default().Error, whose receiver
// is a call expression, and a mutant accepting any named receiver survived.
func TestLoggerErrorIsNotAFailure(t *testing.T) {
	logger := slog.Default()
	if add(1, 2) != 3 {
		logger.Error("wrong sum")
	}
}

func TestOnlySkips(t *testing.T) {
	t.Skip("not yet")
}

func TestHelperThatCannotFail(t *testing.T) {
	describe(t, add(1, 2))
}

func TestOtherPackageHelperThatCannotFail(t *testing.T) {
	helpers.Describe(t, add(1, 2))
}

// Proves aliased imports are resolved: unresolved, this would be assumed
// able to fail and pass silently.
func TestAliasedHelperThatCannotFail(t *testing.T) {
	checks.Describe(t, add(1, 2))
}

func TestSubtestThatCannotFail(t *testing.T) {
	t.Run("sum", func(t *testing.T) {
		add(1, 2)
	})
}

func TestRecursiveHelpersThatCannotFail(t *testing.T) {
	ping(t, 3)
}

// --- must pass --------------------------------------------------------------

func TestFailsDirectly(t *testing.T) {
	if add(1, 2) != 3 {
		t.Errorf("add(1, 2) = %d", add(1, 2))
	}
}

func TestFailsUnderAnotherName(tt *testing.T) {
	if add(1, 2) != 3 {
		tt.Fatal("wrong sum")
	}
}

func TestFailsInSubtest(t *testing.T) {
	t.Run("sum", func(sub *testing.T) {
		if add(1, 2) != 3 {
			sub.FailNow()
		}
	})
}

func TestFailsThroughHelper(t *testing.T) {
	requireSum(t, add(1, 2), 3)
}

func TestFailsThroughTwoHelpers(t *testing.T) {
	outer(t)
}

func TestFailsThroughOtherPackageHelper(t *testing.T) {
	helpers.RequireEqual(t, add(1, 2), 3)
}

func TestFailsThroughAliasedImport(t *testing.T) {
	checks.RequireEqual(t, add(1, 2), 3)
}

func TestFailsThroughRecursiveHelpers(t *testing.T) {
	pong(t, 3)
}

// Lenient by design: a method handed t cannot be resolved without type
// information, so it is assumed able to fail.
func TestMethodHelperIsAssumedAbleToFail(t *testing.T) {
	var checker helpers.Checker
	checker.Check(t, add(1, 2))
}

// --- not tests --------------------------------------------------------------

func TestMain(m *testing.M) { m.Run() }

func Testlowercase(t *testing.T) {}

// --- helpers ----------------------------------------------------------------

func describe(t *testing.T, sum int) {
	t.Helper()
	t.Logf("sum is %d", sum)
}

func requireSum(tb testing.TB, got, want int) {
	tb.Helper()
	if got != want {
		tb.Fatalf("got %d, want %d", got, want)
	}
}

func outer(t *testing.T) { requireSum(t, add(1, 2), 3) }

func ping(t *testing.T, remaining int) {
	if remaining > 0 {
		pingBack(t, remaining-1)
	}
}

func pingBack(t *testing.T, remaining int) { ping(t, remaining) }

func pong(t *testing.T, remaining int) {
	if remaining == 0 {
		t.Error("reached the bottom")
		return
	}
	pongBack(t, remaining-1)
}

func pongBack(t *testing.T, remaining int) { pong(t, remaining) }
