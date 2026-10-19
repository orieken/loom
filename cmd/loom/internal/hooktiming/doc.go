// Package hooktiming holds the latency tests of the capture hook. It has no
// production code. The tests build the real loom binary once and time
// `loom capture hook` as a separate process, because a call inside the test
// process skips process start and package initialisation, which are the
// costs under test (architecture-notes.md decision 3, performance-report.md
// "What MUST be measured").
//
// Run them with:
//
//	go test -v -timeout 30m ./cmd/loom/internal/hooktiming/
//
// They skip under -short. The environment variable HOOK_TIMING_CALLS lowers
// the measured calls per batch while a person tunes a scenario. The gate
// needs the default of 1000.
package hooktiming
