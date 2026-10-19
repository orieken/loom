package capture

// MinutesMethod names the estimate method for the report header
// (analysis criterion 14). The figure is an estimate, not a measurement.
const MinutesMethod = "fixed weight per kind (estimate)"

// Weights are the estimated human minutes per correction kind.
type Weights struct {
	HunkEdit         int
	Revert           int
	DeniedCall       int
	CorrectivePrompt int
}

// DefaultWeights returns the weights the human decided (architecture-notes
// decision 11): hunk-edit 2, revert 3, denied-call 1, corrective-prompt 1.
func DefaultWeights() Weights {
	return Weights{HunkEdit: 2, Revert: 3, DeniedCall: 1, CorrectivePrompt: 1}
}

// MinutesFor returns the weight of one kind. KindUnknown, and any kind that
// is not listed, weigh 0, so an unattributed change adds no minutes.
func (w Weights) MinutesFor(kind CorrectionKind) int {
	switch kind {
	case KindHunkEdit:
		return w.HunkEdit
	case KindRevert:
		return w.Revert
	case KindDeniedCall:
		return w.DeniedCall
	case KindCorrectivePrompt:
		return w.CorrectivePrompt
	}
	return 0
}

// MinutesEstimator multiplies a count by the weight of its kind.
type MinutesEstimator struct {
	weights Weights
}

// NewMinutesEstimator builds an estimator with the given weights.
func NewMinutesEstimator(weights Weights) MinutesEstimator {
	return MinutesEstimator{weights: weights}
}

// Method returns the method name the report header shows.
func (MinutesEstimator) Method() string { return MinutesMethod }

// EstimateKind returns count times the weight of the kind.
func (e MinutesEstimator) EstimateKind(kind CorrectionKind, count int) int {
	return count * e.weights.MinutesFor(kind)
}

// EstimateTotal sums the estimate over a count per kind.
func (e MinutesEstimator) EstimateTotal(counts map[CorrectionKind]int) int {
	total := 0
	for kind, count := range counts {
		total += e.EstimateKind(kind, count)
	}
	return total
}
