package hooktiming

import "testing"

func TestTimingSuiteRunsOnlyWhenOptedInAndNotShort(t *testing.T) {
	tests := []struct {
		name    string
		isShort bool
		optIn   string
		want    bool
	}{
		{"no variable", false, "", false},
		{"variable set to zero", false, "0", false},
		{"variable set to a word", false, "yes", false},
		{"variable set to one", false, "1", true},
		{"short wins over the variable", true, "1", false},
		{"short without the variable", true, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTimingSuiteWanted(tt.isShort, tt.optIn); got != tt.want {
				t.Fatalf("isTimingSuiteWanted(%v, %q) = %v, want %v", tt.isShort, tt.optIn, got, tt.want)
			}
		})
	}
}
