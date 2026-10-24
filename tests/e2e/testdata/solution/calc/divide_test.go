package calc

import "testing"

func TestDivide(t *testing.T) {
	if got, err := Divide(9, 3); err != nil || got != 3 {
		t.Errorf("Divide(9, 3) = %d, %v", got, err)
	}
}
