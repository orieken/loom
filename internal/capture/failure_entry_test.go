package capture

import (
	"strings"
	"testing"
	"time"
)

var entryAt = time.Date(2026, 10, 19, 12, 34, 56, 789_000_000, time.UTC)

// The closed set is pinned. A new cause is a deliberate change of this list,
// of FailureCauses and of the failure-log documentation together.
func TestTheFailureCauseSetIsClosed(t *testing.T) {
	want := []FailureCause{"decode", "salt", "config", "write", "stall", "panic", "flag"}

	got := FailureCauses()

	if len(got) != len(want) {
		t.Fatalf("causes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] || !got[i].IsKnown() {
			t.Fatalf("cause %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTheCauseConstantsAreInTheClosedSet(t *testing.T) {
	for _, cause := range []FailureCause{CauseDecode, CauseSalt, CauseConfig, CauseWrite, CauseStall, CausePanic, CauseFlag} {
		if !cause.IsKnown() {
			t.Errorf("%q is not in FailureCauses", cause)
		}
	}
}

func TestACauseOutsideTheSetIsNotKnown(t *testing.T) {
	for _, text := range []string{"", "Stall", "stall ", "sdt", "disk full: /home/me/x"} {
		if FailureCause(text).IsKnown() {
			t.Errorf("%q must not be a known cause", text)
		}
	}
}

func TestFailureEntryLineFormat(t *testing.T) {
	line := NewFailureEntry(entryAt, CauseConfig).Line()

	if line != "2026-10-19T12:34:56.789Z config\n" {
		t.Fatalf("line = %q", line)
	}
}

func TestFailureEntryLineHasAFixedWidthTimeForAnyInstant(t *testing.T) {
	early := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	prefix, _, _ := strings.Cut(NewFailureEntry(early, CauseSalt).Line(), " ")

	if prefix != "2026-01-02T03:04:05.000Z" || len(prefix) != len(FailureTimeLayout) {
		t.Fatalf("time = %q, want a fixed width of %d", prefix, len(FailureTimeLayout))
	}
}

func TestFailureEntryNeverCarriesFreeText(t *testing.T) {
	canary := FailureCause("CANARY /secret/path")

	line := NewFailureEntry(entryAt, canary).Line()

	if strings.Contains(line, "CANARY") || line != "2026-10-19T12:34:56.789Z write\n" {
		t.Fatalf("line = %q: an unknown cause must become write", line)
	}
}

func TestParseFailureLineRoundTrips(t *testing.T) {
	for _, cause := range FailureCauses() {
		line := strings.TrimSuffix(NewFailureEntry(entryAt, cause).Line(), "\n")

		entry, isParsed := ParseFailureLine(line)

		if !isParsed || entry.Cause != cause || !entry.At.Equal(entryAt) {
			t.Errorf("%q parsed to %+v, %v", line, entry, isParsed)
		}
	}
}

func TestParseFailureLineRejectsWhatLineCannotWrite(t *testing.T) {
	for _, line := range []string{
		"", "t", "sdt", "2026-10-19T12:34:56.789Z", "2026-10-19T12:34:56.789Z ",
		"2026-10-19T12:34:56.789Z unknown", "2026-10-19T12:34:56.789Z stall extra",
		"2026-10-19 12:34:56 stall", "not-a-time stall",
	} {
		if entry, isParsed := ParseFailureLine(line); isParsed {
			t.Errorf("%q parsed to %+v", line, entry)
		}
	}
}
