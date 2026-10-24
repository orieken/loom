package calc

// The order's acceptance criteria, judged by tests the run never sees: the
// harness copies this file into a clone of the pushed branch only after the
// factory has finished.

import (
	"errors"
	"testing"
)

func TestAcceptanceDivideTruncatesTowardZero(t *testing.T) {
	cases := []struct{ a, b, want int }{{7, 2, 3}, {-7, 2, -3}, {6, -3, -2}, {0, 5, 0}}
	for _, tc := range cases {
		got, err := Divide(tc.a, tc.b)
		if err != nil || got != tc.want {
			t.Errorf("Divide(%d, %d) = %d, %v; want %d", tc.a, tc.b, got, err, tc.want)
		}
	}
}

func TestAcceptanceDivideByZeroIsASentinelError(t *testing.T) {
	got, err := Divide(1, 0)
	if !errors.Is(err, ErrDivideByZero) || got != 0 {
		t.Errorf("Divide(1, 0) = %d, %v; want 0, ErrDivideByZero", got, err)
	}
}

func TestAcceptanceAddIsUnchanged(t *testing.T) {
	if Add(2, 2) != 4 {
		t.Error("Add changed")
	}
}
