package cmd

import (
	"github.com/orieken/loom/internal/repograph"
	"github.com/orieken/loom/internal/repoquery"
	"github.com/orieken/loom/internal/state"
)

// blastRadiusResolverFor computes a context manifest's blast radius from the
// repo graph under root. The agent names the paths and how far to look; this
// is where the answer comes from, so a model never transcribes it (ADR-010,
// Phase 8). A repository with no graph yields an error, which the manifest
// records as "not resolved" instead of an empty radius.
func blastRadiusResolverFor(root string) state.BlastRadiusResolver {
	store := repoquery.NewStore(root, repograph.GitLister{})
	return func(seeds []string, depth int) (state.BlastResolution, error) {
		snapshot, err := store.Snapshot()
		if err != nil {
			return state.BlastResolution{}, err
		}
		impacted, err := snapshot.Impact(seeds, depth)
		if err != nil {
			return state.BlastResolution{}, err
		}
		resolution := state.BlastResolution{GraphSHA: snapshot.GraphSHA(), Fresh: snapshot.Fresh, Impacted: make([]state.BlastImpacted, 0, len(impacted))}
		for _, item := range impacted {
			resolution.Impacted = append(resolution.Impacted, state.BlastImpacted{Path: item.Path, Distance: item.Distance, Confidence: item.Confidence})
		}
		return resolution, nil
	}
}
