package diffcover

// RepositoryExclusions is this repository's explicit list of changed Go paths
// that are never measured — by coverage or by mutation. Every entry needs a
// reason; the list is not a pattern that can quietly grow.
var RepositoryExclusions = map[string]bool{
	// A separate module with its own go.mod: this module's coverage profile
	// can never contain its files, and mutation would run the wrong module's
	// tests. CI builds it in its own step.
	"examples/embedding/": true,
	// Source files `loom index` reads as input (ADR-010). They are never
	// compiled or run, and the Go ones sit in their own nested modules.
	"tests/fixtures/repograph/": true,
	// Test support: builds committed, indexed git repositories for tests of
	// code that reads the context index. Nothing in production imports it, so
	// no profile of this module's own tests can reach it.
	"internal/repoquery/fixture/": true,
}
