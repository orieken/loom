package diffcover

// RepositoryExclusions is this repository's explicit list of changed Go paths
// that are never measured — by coverage or by mutation. Every entry needs a
// reason; the list is not a pattern that can quietly grow.
var RepositoryExclusions = map[string]bool{
	// A separate module with its own go.mod: this module's coverage profile
	// can never contain its files, and mutation would run the wrong module's
	// tests. CI builds it in its own step.
	"examples/embedding/": true,
}
