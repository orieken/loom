package hooktiming

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// Split triggers of performance-report.md "Answer: separate hook binary".
// A trigger that fires is reported, never acted on: the binary is not split.
const (
	triggerStartupP95 = 20 * time.Millisecond
	triggerTypicalP95 = 35 * time.Millisecond
	triggerInitMillis = 10.0
)

// optInEnvName switches the timing suite on. A wall-clock gate on a shared
// runner is a CI job, and a CI job needs the human's "approve fitness
// function" (approval-gates.md gate #7). Until then the suite is opt-in.
const optInEnvName = "LOOM_HOOK_TIMING"

// isTimingSuiteWanted says whether the suite runs: never under -short, and
// otherwise only when the opt-in variable is exactly "1".
func isTimingSuiteWanted(isShort bool, optIn string) bool {
	return !isShort && optIn == "1"
}

func skipUnlessOptedIn(t *testing.T) {
	t.Helper()
	if !isTimingSuiteWanted(testing.Short(), os.Getenv(optInEnvName)) {
		t.Skipf("hook timing is opt-in and skips under -short; set %s=1 to run it", optInEnvName)
	}
}

func TestHookTimingHarnessFloor(t *testing.T) {
	skipUnlessOptedIn(t)

	floor := measureHarnessFloor(t)

	t.Logf("harness floor (/usr/bin/true)  %s", floor)
}

// S0: start-up floor. An unknown event in an empty root decodes, fails and
// exits. It reads no file.
func TestHookTimingS0StartupFloor(t *testing.T) {
	skipUnlessOptedIn(t)
	empty := t.TempDir()
	args := []string{"capture", "hook", "NoSuchEvent", "--root", empty}

	worstBatch := measureSequential(t, "S0", func(int) call { return call{args: args, stdin: "{}"} })

	verdict(t, "S0", worstBatch)
	reportTrigger(t, "S0 start-up p95", worstBatch.p95, triggerStartupP95)
}

// S0b: the per-package init cost, from GODEBUG=inittrace=1.
func TestHookTimingS0InitTrace(t *testing.T) {
	skipUnlessOptedIn(t)
	command := exec.Command(loomBinary(t), "--version")
	command.Env = append(os.Environ(), "GODEBUG=inittrace=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("loom --version: %v", err)
	}

	inits := parseInitTrace(string(output))

	var heavy float64
	for _, entry := range inits {
		if strings.Contains(entry.pkg, "sqlite") || strings.Contains(entry.pkg, "libc") {
			heavy += entry.clockMillis
		}
	}
	t.Logf("package init, slowest first:\n%s", formatInits(inits, 12))
	t.Logf("sqlite + libc init: %.2f ms (trigger above %.0f ms)", heavy, triggerInitMillis)
	if heavy > triggerInitMillis {
		t.Logf("SPLIT TRIGGER 3 FIRED: sqlite and libc init is %.2f ms. Report it; do not split the binary.", heavy)
	}
}

// S1: typical. An Edit result on a 50 KB file, a small tail, one append.
func TestHookTimingS1Typical(t *testing.T) {
	skipUnlessOptedIn(t)
	p := newProject(t)
	p.writeSource("app/small.go", 50<<10, uniqueLine)
	file := p.root + "/app/small.go"
	payload := editPayload(file)
	reseed := func() {
		now := time.Now()
		p.writeDayFile([]capture.SessionEvent{p.fillerEvent(now.Add(-time.Minute)), p.fillerEvent(now.Add(-30 * time.Second))})
	}

	reseed()
	runOnce(t, loomBinary(t), call{args: p.hookArgs("PostToolUse"), stdin: payload})
	if body := p.readSource(".claude/memory/sessions/" + time.Now().UTC().Format("2006-01-02") + ".jsonl"); !strings.Contains(body, `"rangeStart":20`) {
		t.Fatal("S1 probe: the Edit result recorded no line range")
	}

	worstBatch := measureSequential(t, "S1", func(index int) call {
		return call{args: p.hookArgs("PostToolUse"), stdin: payload, prepare: reseedEvery(index, reseed)}
	})

	verdict(t, "S1", worstBatch)
	reportTrigger(t, "S1 typical p95", worstBatch.p95, triggerTypicalP95)
}

// S2: busy. A day file of 10,000 events. The last 256 KB holds 5 open hunks
// on 3 files of 100 KB. The call is a prompt, which sees every path.
func TestHookTimingS2Busy(t *testing.T) {
	skipUnlessOptedIn(t)
	p := newProject(t)
	contents := map[string]string{}
	for _, name := range []string{"app/a.go", "app/b.go", "app/c.go"} {
		contents[name] = p.writeSource(name, 100<<10, "")
	}
	hunks := []struct {
		file       string
		start, end int
	}{{"app/a.go", 10, 14}, {"app/a.go", 50, 54}, {"app/b.go", 10, 14}, {"app/b.go", 80, 84}, {"app/c.go", 20, 24}}
	reseed := func() {
		now := time.Now()
		const total, firstHunk = 10000, 9300
		events := make([]capture.SessionEvent, 0, total)
		step := 8 * time.Minute / total
		for i := range total {
			at := now.Add(-9 * time.Minute).Add(time.Duration(i) * step)
			if i >= firstHunk && i < firstHunk+len(hunks) {
				// Hunks of one file share the instant of the write that made
				// them, as a MultiEdit does. A later write to the same file
				// at a later instant would replace the earlier hunks.
				h := hunks[i-firstHunk]
				events = append(events, p.hunkEvent(h.file, contents[h.file], fileWriteTime(now, h.file), h.start, h.end))
				continue
			}
			events = append(events, p.fillerEvent(at))
		}
		p.writeDayFile(events)
	}

	p.probeObservations(t, reseed, "UserPromptSubmit", []string{"app/a.go"}, 1)

	worstBatch := measureSequential(t, "S2", func(index int) call {
		return call{args: p.hookArgs("UserPromptSubmit"), stdin: promptPayload(), prepare: reseedEvery(index, reseed)}
	})

	verdict(t, "S2", worstBatch)
}

// S3: worst case. A prompt with 20 open hunks on 20 files of 1 MB. The 4 MB
// read budget trips and the call must still end inside p99.
func TestHookTimingS3WorstCase(t *testing.T) {
	skipUnlessOptedIn(t)
	p := newProject(t)
	const files = capture.MaxRehashedHunks
	contents := make([]string, files)
	for i := range files {
		contents[i] = p.writeSource(fmt.Sprintf("big/f%02d.go", i), capture.MaxTargetFileBytes, "")
	}
	reseed := func() {
		now := time.Now()
		events := make([]capture.SessionEvent, 0, files)
		for i := range files {
			at := now.Add(-time.Duration(files-i) * time.Second)
			events = append(events, p.hunkEvent(fmt.Sprintf("big/f%02d.go", i), contents[i], at, hunkFirstLine, hunkLastLine))
		}
		p.writeDayFile(events)
	}

	p.probeObservations(t, reseed, "UserPromptSubmit", []string{"big/f19.go"}, 1)
	p.probeObservations(t, reseed, "UserPromptSubmit", []string{"big/f00.go"}, 0) // beyond the 4 MB budget

	worstBatch := measureSequential(t, "S3", func(index int) call {
		return call{args: p.hookArgs("UserPromptSubmit"), stdin: promptPayload(), prepare: reseedEvery(index, reseed)}
	})

	verdict(t, "S3", worstBatch)
}

// S6: concurrency. Four processes run the same scenario at once and append to
// one day file. Before each call the worker changes its files and records
// fresh agent writes, so every call finds changed hunks and writes a batch.
func TestHookTimingS6Concurrency(t *testing.T) {
	skipUnlessOptedIn(t)
	const workers, filesPerWorker = 4, 5
	p := newProject(t)
	binary := loomBinary(t)
	calls := callsPerBatch(t) / workers
	var batches []stats
	for batch := range batchCount {
		samples := make([]time.Duration, 0, calls*workers)
		var mu sync.Mutex
		var wait sync.WaitGroup
		for worker := range workers {
			wait.Add(1)
			go func() {
				defer wait.Done()
				own := p.worker(worker, filesPerWorker)
				mine := make([]time.Duration, 0, calls)
				for i := range calls + warmUpCalls/workers {
					elapsed := runOnce(t, binary, call{args: p.hookArgs("UserPromptSubmit"), stdin: promptPayload(), prepare: own.changeAndRecord})
					if i >= warmUpCalls/workers {
						mine = append(mine, elapsed)
					}
				}
				mu.Lock()
				samples = append(samples, mine...)
				mu.Unlock()
			}()
		}
		wait.Wait()
		batches = append(batches, summarize(samples))
		t.Logf("S6   batch %d      %s", batch+1, batches[batch])
	}

	if p.observationCount() == 0 {
		t.Fatal("S6 probe: no call found a changed hunk, so no batch was written")
	}
	verdict(t, "S6", worst(batches))
}

// reseedEvery returns a prepare step that runs reseed at the start and then
// every reseedEveryCalls calls, so the hunks stay inside the window and the
// tail stays the size the scenario describes.
func reseedEvery(index int, reseed func()) func() {
	if index%reseedEveryCalls != 0 {
		return nil
	}
	return reseed
}

// fileWriteTime gives every hunk of one file the same write instant.
func fileWriteTime(now time.Time, file string) time.Time {
	offsets := map[string]time.Duration{"app/a.go": 90 * time.Second, "app/b.go": 80 * time.Second, "app/c.go": 70 * time.Second}
	return now.Add(-offsets[file])
}

func reportTrigger(t *testing.T, name string, got, trigger time.Duration) {
	t.Helper()
	if got > trigger {
		t.Logf("SPLIT TRIGGER FIRED: %s = %v is above %v. Report it; do not split the binary.", name, got, trigger)
		return
	}
	t.Logf("split trigger clear: %s = %v is within %v", name, got, trigger)
}

type initEntry struct {
	pkg         string
	clockMillis float64
}

var initTraceLine = regexp.MustCompile(`^init (\S+) @[\d.]+ ms, ([\d.]+) ms clock`)

func parseInitTrace(output string) []initEntry {
	var entries []initEntry
	for _, line := range strings.Split(output, "\n") {
		match := initTraceLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		clock, _ := strconv.ParseFloat(match[2], 64)
		entries = append(entries, initEntry{pkg: match[1], clockMillis: clock})
	}
	return entries
}

func formatInits(entries []initEntry, top int) string {
	sorted := append([]initEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].clockMillis > sorted[j].clockMillis })
	var out strings.Builder
	for i, entry := range sorted {
		if i == top {
			break
		}
		fmt.Fprintf(&out, "  %-48s %7.3f ms\n", entry.pkg, entry.clockMillis)
	}
	return out.String()
}
