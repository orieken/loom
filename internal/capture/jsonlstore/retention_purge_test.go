package jsonlstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

var purgeNow = time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

func daysAgo(days int) time.Time { return purgeNow.AddDate(0, 0, -days) }

// dayFileAt writes a day file of the given size, dated a number of days back.
func dayFileAt(t *testing.T, layout capture.Layout, days int, size int) string {
	t.Helper()
	path := layout.DayFile(daysAgo(days))
	writeFile(t, path, strings.Repeat("x", size))
	return path
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func purge(t *testing.T, layout capture.Layout, now time.Time, retention capture.Retention) error {
	t.Helper()
	return NewPurger(layout).PurgeIfDue(t.Context(), now, retention)
}

func TestPurgeRemovesFilesOlderThanTheRetentionAge(t *testing.T) {
	layout := newLayout(t)
	old91 := dayFileAt(t, layout, 91, 10)
	edge90 := dayFileAt(t, layout, 90, 10)
	recent := dayFileAt(t, layout, 5, 10)

	if err := purge(t, layout, purgeNow, capture.DefaultConfig().Retention); err != nil {
		t.Fatal(err)
	}

	if exists(old91) || !exists(edge90) || !exists(recent) {
		t.Fatalf("91 days removed = %v, 90 days kept = %v, 5 days kept = %v", !exists(old91), exists(edge90), exists(recent))
	}
}

func TestPurgeTrimsTheOldestFilesFirstWhenOverTheCap(t *testing.T) {
	layout := newLayout(t)
	retention := capture.Retention{MaxAgeDays: 90, MaxBytes: 45}
	var paths []string
	for _, days := range []int{9, 8, 7, 6, 5, 4, 3} {
		paths = append(paths, dayFileAt(t, layout, days, 10))
	}

	if err := purge(t, layout, purgeNow, retention); err != nil {
		t.Fatal(err)
	}

	// 70 bytes, cap 45: the three oldest go (40 bytes left); the fourth stays.
	for i, path := range paths {
		if want := i >= 3; exists(path) != want {
			t.Fatalf("file %d (%d days old) exists = %v, want %v", i, 9-i, exists(path), want)
		}
	}
}

func TestPurgeNeverRemovesTodayOrYesterdayEvenOverTheCapOrOverTheAge(t *testing.T) {
	layout := newLayout(t)
	retention := capture.Retention{MaxAgeDays: 0, MaxBytes: 1}
	today := dayFileAt(t, layout, 0, 100)
	yesterday := dayFileAt(t, layout, 1, 100)
	twoDays := dayFileAt(t, layout, 2, 100)

	if err := purge(t, layout, purgeNow, retention); err != nil {
		t.Fatal(err)
	}

	if !exists(today) || !exists(yesterday) || exists(twoDays) {
		t.Fatalf("today kept = %v, yesterday kept = %v, two days removed = %v", exists(today), exists(yesterday), !exists(twoDays))
	}
}

func TestPurgeProtectsTheUTCDaysNotTheLocalDays(t *testing.T) {
	layout := newLayout(t)
	late := time.Date(2026, 6, 1, 23, 59, 0, 0, time.FixedZone("UTC+14", 14*60*60))
	dayBefore := dayFileAt(t, layout, 2, 10) // 2026-05-30

	if err := purge(t, layout, late, capture.Retention{MaxAgeDays: 0, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}

	// late is 2026-06-01 09:59 UTC, so 05-31 and 06-01 are protected and 05-30 is not.
	if exists(dayBefore) {
		t.Fatal("the day before yesterday (UTC) must be purged")
	}
}

func TestPurgeLeavesForeignEntriesAlone(t *testing.T) {
	layout := newLayout(t)
	retention := capture.Retention{MaxAgeDays: 1, MaxBytes: 1}
	foreign := []string{"notes.txt", "2026-13-45.jsonl", "2020-01-01.json", "2020-01-02.jsonl.bak"}
	for _, name := range foreign {
		writeFile(t, filepath.Join(layout.SessionsDir(), name), "keep")
	}
	if err := os.Mkdir(filepath.Join(layout.SessionsDir(), "2020-01-03.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := purge(t, layout, purgeNow, retention); err != nil {
		t.Fatal(err)
	}

	for _, name := range append(foreign, "2020-01-03.jsonl") {
		if !exists(filepath.Join(layout.SessionsDir(), name)) {
			t.Fatalf("%s was removed, want it left alone", name)
		}
	}
}

func TestPurgeTouchesTheStampBeforeItDeletesAnything(t *testing.T) {
	layout := newLayout(t)
	// The sessions path is a file, so listing it fails after the stamp is set.
	writeFile(t, layout.SessionsDir(), "not a directory")

	err := purge(t, layout, purgeNow, capture.DefaultConfig().Retention)

	if err == nil {
		t.Fatal("want the listing error")
	}
	info, statErr := os.Stat(layout.PurgeStampPath())
	if statErr != nil || !info.ModTime().Equal(purgeNow) {
		t.Fatalf("stamp = %v (err %v), want it touched with the call's time", info, statErr)
	}
}

func TestPurgeRateLimit(t *testing.T) {
	tests := []struct {
		name      string
		later     time.Duration
		wantPurge bool
	}{
		{name: "30 minutes later", later: 30 * time.Minute, wantPurge: false},
		{name: "59 minutes later", later: 59 * time.Minute, wantPurge: false},
		{name: "exactly one hour later", later: time.Hour, wantPurge: true},
		{name: "two hours later", later: 2 * time.Hour, wantPurge: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newLayout(t)
			retention := capture.DefaultConfig().Retention
			if err := purge(t, layout, purgeNow, retention); err != nil {
				t.Fatal(err)
			}
			expired := dayFileAt(t, layout, 200, 10)

			if err := purge(t, layout, purgeNow.Add(tt.later), retention); err != nil {
				t.Fatal(err)
			}

			if exists(expired) == tt.wantPurge {
				t.Fatalf("expired file kept = %v, want purge = %v", exists(expired), tt.wantPurge)
			}
		})
	}
}

func TestPurgeMissingSessionsDirectoryIsAnEmptyStore(t *testing.T) {
	layout := newLayout(t)

	if err := purge(t, layout, purgeNow, capture.DefaultConfig().Retention); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !exists(layout.PurgeStampPath()) {
		t.Fatal("the stamp must be touched even for an empty store")
	}
}

func TestPurgeStopsOnACancelledContextBeforeDeleting(t *testing.T) {
	layout := newLayout(t)
	old := dayFileAt(t, layout, 200, 10)
	cancelled, cancel := cancelledContext(t)
	defer cancel()

	err := NewPurger(layout).PurgeIfDue(cancelled, purgeNow, capture.DefaultConfig().Retention)

	if err == nil || !exists(old) {
		t.Fatalf("err = %v, file kept = %v, want an error and no deletion", err, exists(old))
	}
}

func TestRemoveAllIgnoresMissingFilesAndReportsOtherErrors(t *testing.T) {
	dir := t.TempDir()
	missing := dayFile{path: filepath.Join(dir, "gone.jsonl")}
	notEmpty := filepath.Join(dir, "folder")
	writeFile(t, filepath.Join(notEmpty, "inner"), "x")

	if err := removeAll([]dayFile{missing}); err != nil {
		t.Fatalf("a missing file is not an error: %v", err)
	}
	if err := removeAll([]dayFile{{path: notEmpty}}); err == nil {
		t.Fatal("a real removal failure must be reported")
	}
}

func TestSelectForPurgeAgeBoundary(t *testing.T) {
	files := []dayFile{
		{path: "a", date: daysAgo(91).Truncate(24 * time.Hour), size: 1},
		{path: "b", date: daysAgo(90).Truncate(24 * time.Hour), size: 1},
	}

	got := selectForPurge(files, purgeNow, capture.Retention{MaxAgeDays: 90, MaxBytes: 1 << 20})

	if len(got) != 1 || got[0].path != "a" {
		t.Fatalf("selected %+v, want only the 91-day file", got)
	}
}
