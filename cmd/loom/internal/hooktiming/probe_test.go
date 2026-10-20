package hooktiming

import (
	"os"
	"strings"
	"testing"
	"time"
)

// A timing number is worth nothing when the scenario does not do the work it
// names. These probes run the built binary once on a changed fixture and
// count what it wrote, so the scenario is shown to see its hunks.

func (p *project) observationCount() int {
	body, err := os.ReadFile(p.layout.DayFile(time.Now()))
	if err != nil {
		p.t.Fatal(err)
	}
	return strings.Count(string(body), `"kind":"hunk-observed"`)
}

// changeLine replaces one line of a source file with a line of the same size
// (16 bytes), so a file at the size cap stays inside the cap. It returns the
// old content, so the probe can put it back.
func (p *project) changeLine(relative string, line int) string {
	before := p.readSource(relative)
	lines := strings.SplitAfter(before, "\n")
	lines[line-1] = "CHANGED-PROBE-1\n"
	p.writeRaw(relative, strings.Join(lines, ""))
	return before
}

// probeObservations seeds the day file, changes the named files, runs the
// hook once and checks the number of observations it wrote.
func (p *project) probeObservations(t *testing.T, reseed func(), event string, changed []string, want int) {
	t.Helper()
	reseed()
	originals := map[string]string{}
	for _, relative := range changed {
		originals[relative] = p.changeLine(relative, hunkFirstLine+1)
	}
	runOnce(t, loomBinary(t), call{args: p.hookArgs(event), stdin: promptPayload()})
	if got := p.observationCount(); got != want {
		failures, _ := os.ReadFile(p.layout.FailureLogPath())
		day, _ := os.ReadFile(p.layout.DayFile(time.Now()))
		lines := strings.Split(strings.TrimSpace(string(day)), "\n")
		t.Logf("last lines:\n%s", strings.Join(lines[max(0, len(lines)-8):], "\n"))
		t.Fatalf("probe wrote %d observations, want %d (failure log %q): the scenario does not do the work it names", got, want, failures)
	}
	for relative, content := range originals {
		p.writeRaw(relative, content)
	}
	reseed()
}
