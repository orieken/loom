package hooktiming

import (
	"os"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// Scenarios S4 and S5 of performance-report.md. They share the opt-in switch
// and the harness rules of the other scenarios (hook_timing_test.go).

const (
	expiredFileCount  = 3
	retentionAgeDays  = capture.DefaultRetentionDays
	staleStampAge     = 2 * time.Hour // older than capture.PurgeInterval
	freshStampAge     = time.Minute
	midnightWindow    = "window: 48h\n"
	todayTailBytes    = 100 << 10
	yesterdayTailSize = 300 << 10 // above the 256 KB tail budget, so the budget trips
	hoursPerDay       = 24
)

func stopPayload() string {
	return jsonOf(map[string]string{"session_id": "timing-session"})
}

// utcDay is the instant of 00:00 UTC of the day that holds the given instant.
func utcDay(at time.Time) time.Time { return at.UTC().Truncate(hoursPerDay * time.Hour) }

// seedOldDayFile writes one small filler event into the day file that is the
// given number of days old.
func (p *project) seedOldDayFile(ageDays int) {
	day := time.Now().AddDate(0, 0, -ageDays)
	p.writeDayFileFor(day, []capture.SessionEvent{p.fillerEvent(utcDay(day).Add(time.Hour))})
}

// seedRetentionStore writes the day files of S4: every age from 2 days up to
// the retention age stays, and the 3 older files are the ones the purge
// removes.
func (p *project) seedRetentionStore() {
	for age := 2; age <= retentionAgeDays; age++ {
		p.seedOldDayFile(age)
	}
	p.seedExpiredFiles()
}

func (p *project) seedExpiredFiles() {
	for i := range expiredFileCount {
		p.seedOldDayFile(retentionAgeDays + 1 + i)
	}
}

func (p *project) setPurgeStampAge(age time.Duration) {
	p.t.Helper()
	p.writeRaw(".claude/memory/capture.purge", "")
	when := time.Now().Add(-age)
	if err := os.Chtimes(p.layout.PurgeStampPath(), when, when); err != nil {
		p.t.Fatal(err)
	}
}

func (p *project) expiredFilesLeft() int {
	left := 0
	for i := range expiredFileCount {
		day := time.Now().AddDate(0, 0, -(retentionAgeDays + 1 + i))
		if _, err := os.Stat(p.layout.DayFile(day)); err == nil {
			left++
		}
	}
	return left
}

// S4a: Stop with the purge due. 94 day files, 3 of them past the retention
// age, and a stamp older than the purge interval. Every call purges, so every
// call first puts the 3 files and the stale stamp back.
func TestHookTimingS4StopWithPurgeDue(t *testing.T) {
	skipUnlessOptedIn(t)
	p := newProject(t)
	p.seedRetentionStore()
	reseed := func() {
		p.seedExpiredFiles()
		p.setPurgeStampAge(staleStampAge)
	}
	reseed()
	runOnce(t, loomBinary(t), call{args: p.hookArgs("Stop"), stdin: stopPayload()})
	if left := p.expiredFilesLeft(); left != 0 {
		t.Fatalf("S4 probe: %d expired day files are left, so the purge did not run", left)
	}

	worstBatch := measureSequential(t, "S4a", func(int) call {
		return call{args: p.hookArgs("Stop"), stdin: stopPayload(), prepare: reseed}
	})

	verdict(t, "S4a", worstBatch)
}

// S4b: Stop with the purge not due. The same store, a fresh stamp. The hook
// must not scan the directory.
func TestHookTimingS4StopWithPurgeNotDue(t *testing.T) {
	skipUnlessOptedIn(t)
	p := newProject(t)
	p.seedRetentionStore()
	p.setPurgeStampAge(freshStampAge)
	runOnce(t, loomBinary(t), call{args: p.hookArgs("Stop"), stdin: stopPayload()})
	if left := p.expiredFilesLeft(); left != expiredFileCount {
		t.Fatalf("S4 probe: %d expired day files are left, want %d: the purge ran when it was not due", left, expiredFileCount)
	}

	worstBatch := measureSequential(t, "S4b", func(int) call {
		return call{args: p.hookArgs("Stop"), stdin: stopPayload()}
	})

	verdict(t, "S4b", worstBatch)
}

// fillerCount returns how many filler events make about the given size.
func (p *project) fillerCount(size int) int {
	line, err := capture.MarshalLine(p.fillerEvent(time.Now()))
	if err != nil {
		p.t.Fatal(err)
	}
	return size / len(line)
}

// S5: the tail reads two day files. The call cannot run at 00:00 UTC on
// demand, so the fixture sets the Correction window to 48 hours. The cut-off
// then lies before the start of the UTC day, and the hook reads today's file
// and, with the budget left, yesterday's. The probe shows the hook finds the
// hunk that only yesterday's file holds.
func TestHookTimingS5MidnightTwoFileTail(t *testing.T) {
	skipUnlessOptedIn(t)
	p := newProject(t)
	p.writeRaw(".claude/capture.yaml", midnightWindow)
	contents := map[string]string{
		"app/a.go": p.writeSource("app/a.go", 100<<10, ""),
		"app/b.go": p.writeSource("app/b.go", 100<<10, ""),
	}
	midnight := utcDay(time.Now())
	yesterdayCount, todayCount := p.fillerCount(yesterdayTailSize), p.fillerCount(todayTailBytes)
	reseed := func() {
		yesterdayStart := midnight.Add(-time.Duration(yesterdayCount+1) * time.Millisecond)
		p.writeDayFileFor(midnight.Add(-time.Hour), p.eventsEndingInHunk(yesterdayStart, yesterdayCount, "app/b.go", contents))
		p.writeDayFileFor(midnight, p.eventsEndingInHunk(midnight.Add(time.Millisecond), todayCount, "app/a.go", contents))
	}

	p.probeObservations(t, reseed, "UserPromptSubmit", []string{"app/a.go", "app/b.go"}, 2)

	worstBatch := measureSequential(t, "S5", func(index int) call {
		return call{args: p.hookArgs("UserPromptSubmit"), stdin: promptPayload(), prepare: reseedEvery(index, reseed)}
	})

	verdict(t, "S5", worstBatch)
}

// eventsEndingInHunk makes the events of one day file: count filler events,
// one millisecond apart from the instant first, and then an agent write of
// lines 10-14 of the named file.
func (p *project) eventsEndingInHunk(first time.Time, count int, file string, contents map[string]string) []capture.SessionEvent {
	events := make([]capture.SessionEvent, 0, count+1)
	for i := range count {
		events = append(events, p.fillerEvent(first.Add(time.Duration(i)*time.Millisecond)))
	}
	hunkAt := first.Add(time.Duration(count) * time.Millisecond)
	return append(events, p.hunkEvent(file, contents[file], hunkAt, hunkFirstLine, hunkLastLine))
}
