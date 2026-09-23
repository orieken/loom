package diffcover_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/diffcover"
)

func TestParseProfileReadsBlocksInEveryMode(t *testing.T) {
	for _, mode := range []string{"set", "count", "atomic"} {
		t.Run(mode, func(t *testing.T) {
			profile, err := diffcover.ParseProfile(strings.NewReader(
				"mode: " + mode + "\n" +
					"example.com/m/a.go:3.14,5.2 2 7\n" +
					"example.com/m/a.go:6.1,6.20 1 0\n"))
			if err != nil {
				t.Fatalf("ParseProfile: %v", err)
			}
			want := []diffcover.Block{
				{StartLine: 3, EndLine: 5, Covered: true},
				{StartLine: 6, EndLine: 6, Covered: false},
			}
			if got := profile["example.com/m/a.go"]; !reflect.DeepEqual(got, want) {
				t.Errorf("blocks = %+v, want %+v", got, want)
			}
		})
	}
}

func TestParseProfileRejectsMalformedLines(t *testing.T) {
	for name, line := range map[string]string{
		"no counts":        "example.com/m/a.go:3.1,5.2",
		"no position":      "a.go 1 1",
		"no end":           "example.com/m/a.go:3.1 1 1",
		"bad line":         "example.com/m/a.go:x.1,5.2 1 1",
		"bad end line":     "example.com/m/a.go:3.1,y.2 1 1",
		"one count":        "example.com/m/a.go:3.1,5.2 1",
		"bad execution no": "example.com/m/a.go:3.1,5.2 1 z",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := diffcover.ParseProfile(strings.NewReader("mode: set\n" + line + "\n")); err == nil {
				t.Errorf("accepted %q — a profile line that cannot be read must not be silently dropped", line)
			}
		})
	}
}
