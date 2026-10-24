package diffcover

// RepositoryExclusions is this repository's explicit list of changed Go paths
// that are never measured — by coverage or by mutation. Every entry needs a
// reason; the list is not a pattern that can quietly grow.
var RepositoryExclusions = map[string]bool{
	// A separate module with its own go.mod: this module's coverage profile
	// can never contain its files, and mutation would run the wrong module's
	// tests. CI builds it in its own step.
	"examples/embedding/": true,
	// Compiled only for GOOS=windows. Coverage and mutation both run on
	// Linux, so no profile can contain these files; `GOOS=windows go vet
	// ./internal/factory/` type-checks them. Each holds the Windows half of a
	// function whose POSIX half is measured.
	"internal/factory/lock_windows.go":    true,
	"internal/factory/process_windows.go": true,
}
