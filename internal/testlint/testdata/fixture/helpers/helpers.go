package helpers

import "testing"

// RequireEqual fails the test when got differs from want.
func RequireEqual(tb testing.TB, got, want int) {
	tb.Helper()
	if got != want {
		tb.Errorf("got %d, want %d", got, want)
	}
}

// Describe only logs; it cannot fail a test.
func Describe(tb testing.TB, value int) {
	tb.Logf("value is %d", value)
}

// Checker is a helper reached through a method, which the lint cannot
// resolve without type information.
type Checker struct{}

// Check logs only — the lint still assumes it can fail, by design.
func (Checker) Check(tb testing.TB, value int) {
	tb.Logf("value is %d", value)
}
