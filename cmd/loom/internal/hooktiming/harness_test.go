package hooktiming

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

// The harness rules of performance-report.md ("Hook harness rules").
const (
	warmUpCalls        = 50
	defaultBatchCalls  = 1000
	batchCount         = 3
	budgetP95          = 50 * time.Millisecond
	budgetP99          = 150 * time.Millisecond
	failFactor         = 2
	reseedEveryCalls   = 200
	timingCallsEnvName = "HOOK_TIMING_CALLS"
)

var (
	binaryOnce sync.Once
	binaryPath string
	binaryDir  string
	binaryErr  error
)

// TestMain removes the binary that the tests build once.
func TestMain(m *testing.M) {
	code := m.Run()
	if binaryDir != "" {
		os.RemoveAll(binaryDir)
	}
	os.Exit(code)
}

// loomBinary builds the real binary once. The tests never use `go run`.
func loomBinary(t *testing.T) string {
	t.Helper()
	binaryOnce.Do(func() {
		binaryDir, binaryErr = os.MkdirTemp("", "loom-hooktiming")
		if binaryErr != nil {
			return
		}
		binaryPath = filepath.Join(binaryDir, "loom")
		build := exec.Command("go", "build", "-o", binaryPath, "github.com/orieken/loom/cmd/loom")
		if output, err := build.CombinedOutput(); err != nil {
			binaryErr = fmt.Errorf("%w\n%s", err, output)
			return
		}
		// The first run of a new binary pays a one-time system scan on macOS.
		_ = exec.Command(binaryPath, "--version").Run()
	})
	if binaryErr != nil {
		t.Fatalf("build loom: %v", binaryErr)
	}
	return binaryPath
}

func callsPerBatch(t *testing.T) int {
	t.Helper()
	text := os.Getenv(timingCallsEnvName)
	if text == "" {
		return defaultBatchCalls
	}
	count, err := strconv.Atoi(text)
	if err != nil || count < 1 {
		t.Fatalf("%s = %q, want a positive number", timingCallsEnvName, text)
	}
	return count
}

// call is one process run: its arguments, its stdin and what to do before it.
type call struct {
	args    []string
	stdin   string
	prepare func() // runs before the timer starts
}

// runOnce times one process, from fork to wait. It fails the test on any
// non-zero exit, because the hook must always exit 0.
func runOnce(t *testing.T, binary string, c call) time.Duration {
	t.Helper()
	if c.prepare != nil {
		c.prepare()
	}
	command := exec.Command(binary, c.args...)
	if c.stdin != "" {
		command.Stdin = stringReader(c.stdin)
	}
	start := time.Now()
	err := command.Run()
	elapsed := time.Since(start)
	if err != nil {
		t.Errorf("%v exited with %v", c.args, err)
	}
	return elapsed
}

// stats are the percentiles of one batch.
type stats struct {
	p50, p95, p99, max time.Duration
}

func summarize(samples []time.Duration) stats {
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return stats{
		p50: nearestRank(sorted, 0.50), p95: nearestRank(sorted, 0.95),
		p99: nearestRank(sorted, 0.99), max: sorted[len(sorted)-1],
	}
}

// nearestRank is the smallest sample that covers fraction p of the batch.
func nearestRank(sorted []time.Duration, p float64) time.Duration {
	rank := int(math.Ceil(p*float64(len(sorted)))) - 1
	return sorted[max(rank, 0)]
}

func (s stats) String() string {
	return fmt.Sprintf("p50 %6.1f ms  p95 %6.1f ms  p99 %6.1f ms  max %6.1f ms",
		ms(s.p50), ms(s.p95), ms(s.p99), ms(s.max))
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// worst takes the worst batch for each percentile, because the gate is the
// worst batch and not the average.
func worst(batches []stats) stats {
	var result stats
	for _, b := range batches {
		result.p50 = max(result.p50, b.p50)
		result.p95 = max(result.p95, b.p95)
		result.p99 = max(result.p99, b.p99)
		result.max = max(result.max, b.max)
	}
	return result
}

// verdict applies the gate: fail at twice the budget, warn above the budget.
func verdict(t *testing.T, name string, worstBatch stats) {
	t.Helper()
	t.Logf("%-4s worst batch  %s", name, worstBatch)
	checks := []struct {
		label  string
		got    time.Duration
		budget time.Duration
	}{{"p95", worstBatch.p95, budgetP95}, {"p99", worstBatch.p99, budgetP99}}
	for _, check := range checks {
		switch {
		case check.got > failFactor*check.budget:
			t.Errorf("%s %s = %v is above %dx the budget of %v", name, check.label, check.got, failFactor, check.budget)
		case check.got > check.budget:
			t.Logf("WARN %s %s = %v is above the budget of %v (fails at %dx)", name, check.label, check.got, check.budget, failFactor)
		}
	}
}

// measureSequential runs one scenario: warm-ups, then three batches in a row.
func measureSequential(t *testing.T, name string, next func(index int) call) stats {
	t.Helper()
	binary := loomBinary(t)
	calls := callsPerBatch(t)
	for i := range warmUpCalls {
		runOnce(t, binary, next(i))
	}
	var batches []stats
	for batch := range batchCount {
		samples := make([]time.Duration, 0, calls)
		for i := range calls {
			samples = append(samples, runOnce(t, binary, next(batch*calls+i)))
		}
		batches = append(batches, summarize(samples))
		t.Logf("%-4s batch %d      %s", name, batch+1, batches[batch])
	}
	return worst(batches)
}

// measureHarnessFloor times /usr/bin/true the same way. Its time is the cost
// of fork, exec and wait that no hook can avoid.
func measureHarnessFloor(t *testing.T) stats {
	t.Helper()
	calls := callsPerBatch(t)
	for range warmUpCalls {
		runOnce(t, "/usr/bin/true", call{})
	}
	samples := make([]time.Duration, 0, calls)
	for range calls {
		samples = append(samples, runOnce(t, "/usr/bin/true", call{}))
	}
	return summarize(samples)
}
