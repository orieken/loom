package intake

import (
	"errors"
	"strings"
	"testing"
)

const reagentMapSpecOf = "reagent-map-spec"

const (
	reagentMapSpecPath     = "docs/specs/reagent-map-spec/acceptance-criteria.json"
	reagentMapManifestPath = "docs/specs/reagent-map-spec/package.sha256"
	reagentMapExtraPath    = "docs/specs/reagent-map-spec/extra.txt"
)

// validReagentMapFixture builds the split the saturday-monorepo pilot used
// as its model: AC1 and AC3 (observable at both levels) each covered once at
// each level by a different slice, and AC14 (api only) covered bare by one
// slice. It returns the two slice orders and the backing files map, so a
// test can mutate either one in exactly one way before calling CheckSlices.
func validReagentMapFixture() ([]Document, map[string][]byte) {
	specBytes := []byte(`{"criteria":[
		{"id":"AC1","observableVia":"both"},
		{"id":"AC3","observableVia":"both"},
		{"id":"AC14","observableVia":"api"}
	]}`)
	extraBytes := []byte("reagent-map fixture notes\n")
	manifestBytes := []byte(
		sha256Hex(specBytes) + "  " + reagentMapSpecPath + "\n" +
			sha256Hex(extraBytes) + "  " + reagentMapExtraPath + "\n")
	digest := sha256Digest(manifestBytes)

	files := map[string][]byte{
		reagentMapSpecPath:     specBytes,
		reagentMapManifestPath: manifestBytes,
		reagentMapExtraPath:    extraBytes,
	}

	ui := Document{Order: Order{ID: "reagent-map-ui", Slice: &Slice{
		Of: reagentMapSpecOf, Spec: reagentMapSpecPath, Digest: digest, Manifest: reagentMapManifestPath,
		Covers: []string{"AC1:ui", "AC3:ui"},
	}}}
	api := Document{Order: Order{ID: "reagent-map-api", Slice: &Slice{
		Of: reagentMapSpecOf, Spec: reagentMapSpecPath, Digest: digest, Manifest: reagentMapManifestPath,
		Covers: []string{"AC1:api", "AC3:api", "AC14"},
	}}}
	return []Document{ui, api}, files
}

func anyViolationContains(violations []Violation, substr string) bool {
	for _, v := range violations {
		if strings.Contains(v.String(), substr) {
			return true
		}
	}
	return false
}

// TestCheckSlicesAcceptsTheReagentMapSplit proves AC9's passing fixture: no
// violation, and a summary naming the spec with its slice and level counts.
func TestCheckSlicesAcceptsTheReagentMapSplit(t *testing.T) {
	documents, files := validReagentMapFixture()
	violations, summaries := CheckSlices(documents, fakeFileReader(files))
	if len(violations) != 0 {
		t.Fatalf("violations = %v, want none", violations)
	}
	if len(summaries) != 1 {
		t.Fatalf("summaries = %v, want exactly one", summaries)
	}
	summary := summaries[0]
	if summary.Of != reagentMapSpecOf || summary.Slices != 2 || summary.Levels != 5 {
		t.Errorf("summary = %+v, want %s with 2 slices and 5 levels (AC1 x2, AC3 x2, AC14 x1)",
			summary, reagentMapSpecOf)
	}
}

// TestCheckSlicesReadsNothingWhenNoOrderSetsSlice proves the NFR: a batch
// with no slice block performs zero reads.
func TestCheckSlicesReadsNothingWhenNoOrderSetsSlice(t *testing.T) {
	calls := 0
	read := func(string) ([]byte, error) { calls++; return nil, errors.New("should not be called") }
	documents := []Document{{Order: Order{ID: "a"}}, {Order: Order{ID: "b"}}}

	violations, summaries := CheckSlices(documents, read)

	if len(violations) != 0 || len(summaries) != 0 {
		t.Fatalf("violations = %v, summaries = %v, want none", violations, summaries)
	}
	if calls != 0 {
		t.Errorf("read called %d times, want zero", calls)
	}
}

// TestCheckSlicesEvaluatesAGroupOfOneSlice covers the edge case where an
// order declares slice.of but shares it with nobody: the levels its own
// covers list omits are still reported uncovered.
func TestCheckSlicesEvaluatesAGroupOfOneSlice(t *testing.T) {
	documents, files := validReagentMapFixture()
	documents = documents[:1] // the ui slice alone: AC1:api, AC3:api, AC14 go uncovered
	violations, summaries := CheckSlices(documents, fakeFileReader(files))
	if len(summaries) != 0 {
		t.Fatalf("summaries = %v, want none (the split is incomplete)", summaries)
	}
	for _, want := range []string{"AC1:api", "AC3:api", "AC14:api"} {
		if !anyViolationContains(violations, want) {
			t.Errorf("violations = %v, want one naming %q uncovered", violations, want)
		}
	}
}

// TestCheckSlicesReportsEachViolationKind breaks the passing reagent-map
// fixture in exactly one way per case (AC9) and asserts the specific
// violation that break must produce. Each case fails if the check it
// exercises is removed, since nothing else in the fixture would report it.
func TestCheckSlicesReportsEachViolationKind(t *testing.T) {
	wrongDigest := "sha256:" + strings.Repeat("0", 64)
	for _, tc := range []struct {
		name   string
		mutate func(documents []Document, files map[string][]byte)
		want   string
	}{
		{
			"uncovered level",
			func(d []Document, _ map[string][]byte) {
				d[0].Order.Slice.Covers = []string{"AC1:ui"} // drops AC3:ui
			},
			"AC3:ui",
		},
		{
			"doubly covered level",
			func(d []Document, _ map[string][]byte) {
				d[1].Order.Slice.Covers = append(d[1].Order.Slice.Covers, "AC1:ui")
			},
			"reagent-map-ui",
		},
		{
			"unknown covers criterion",
			func(d []Document, _ map[string][]byte) {
				d[0].Order.Slice.Covers = append(d[0].Order.Slice.Covers, "AC99")
			},
			"AC99",
		},
		{
			"unknown covers level",
			func(d []Document, _ map[string][]byte) {
				d[0].Order.Slice.Covers = append(d[0].Order.Slice.Covers, "AC14:ui") // AC14 is api-only
			},
			"AC14",
		},
		{
			"covers level outside api or ui",
			func(d []Document, _ map[string][]byte) {
				d[0].Order.Slice.Covers = append(d[0].Order.Slice.Covers, "AC1:mobile")
			},
			"is not a level (api or ui)",
		},
		{
			"slice.digest disagreement",
			func(d []Document, _ map[string][]byte) {
				d[1].Order.Slice.Digest = wrongDigest
			},
			"disagree",
		},
		{
			"slice.digest does not match the manifest",
			func(d []Document, _ map[string][]byte) {
				d[0].Order.Slice.Digest = wrongDigest
				d[1].Order.Slice.Digest = wrongDigest // keep them agreeing, to isolate this check
			},
			"does not match sha256",
		},
		{
			"manifest-listed file hash mismatch",
			func(_ []Document, files map[string][]byte) {
				files[reagentMapExtraPath] = []byte("tampered\n")
			},
			"no longer matches its recorded hash",
		},
		{
			"manifest-listed file deleted",
			func(_ []Document, files map[string][]byte) {
				delete(files, reagentMapExtraPath)
			},
			"which could not be read",
		},
		{
			"depends-on cycle",
			func(d []Document, _ map[string][]byte) {
				d[0].Order.DependsOn = []string{"reagent-map-api"}
				d[1].Order.DependsOn = []string{"reagent-map-ui"}
			},
			"cycle",
		},
		{
			"dangling depends-on",
			func(d []Document, _ map[string][]byte) {
				d[0].Order.DependsOn = []string{"does-not-exist"}
			},
			"does-not-exist",
		},
		{
			"unreadable spec file",
			func(_ []Document, files map[string][]byte) {
				delete(files, reagentMapSpecPath)
			},
			"could not be read",
		},
		{
			"unreadable manifest file",
			func(_ []Document, files map[string][]byte) {
				delete(files, reagentMapManifestPath)
			},
			"could not be read",
		},
		{
			"slice.spec path leaves the project root",
			func(d []Document, _ map[string][]byte) {
				d[0].Order.Slice.Spec = "../../etc/passwd"
			},
			"leaves the project root",
		},
		{
			"slice.manifest path leaves the project root",
			func(d []Document, _ map[string][]byte) {
				d[0].Order.Slice.Manifest = "../../etc/shadow"
			},
			"leaves the project root",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			documents, files := validReagentMapFixture()
			tc.mutate(documents, files)

			violations, _ := CheckSlices(documents, fakeFileReader(files))

			if !anyViolationContains(violations, tc.want) {
				t.Errorf("violations = %v, want one containing %q", violations, tc.want)
			}
		})
	}
}

// TestCheckCoverageReportsCriterionWithNoObservableVia covers the edge case
// where a spec criterion has no observableVia at all (as opposed to one
// outside {api, ui, both}): checkCoverage must report it against the spec
// itself and must not count it toward the passing-split level total.
func TestCheckCoverageReportsCriterionWithNoObservableVia(t *testing.T) {
	pkg := SpecPackage{Criteria: []Criterion{{ID: "AC_NOVIA"}}}

	violations, levels := checkCoverage("some-spec", "spec.json", pkg, nil)

	if levels != 0 {
		t.Errorf("levels = %d, want 0", levels)
	}
	if !anyViolationContains(violations, "AC_NOVIA") {
		t.Errorf("violations = %v, want one naming AC_NOVIA", violations)
	}
}

// TestApplyCoversEntrySkipsCriterionAlreadyReportedByCheckCoverage covers the
// case where a covers entry names a criterion with no observableVia:
// checkCoverage has already reported that defect against the spec itself, so
// applyCoversEntry must not report it a second time. Using a level-suffixed
// entry ("AC_NOVIA:ui") distinguishes this from simply falling through to
// resolveCoversLevels, which would otherwise report its own, redundant
// "has no ui level" violation for the same defect.
func TestApplyCoversEntrySkipsCriterionAlreadyReportedByCheckCoverage(t *testing.T) {
	pkg := SpecPackage{Criteria: []Criterion{{ID: "AC_NOVIA"}}}
	coveredBy := map[string][]string{}

	violations := applyCoversEntry("order-1", "AC_NOVIA:ui", "spec.json", pkg, coveredBy)

	if len(violations) != 0 {
		t.Errorf("violations = %v, want none (already reported by checkCoverage)", violations)
	}
	if len(coveredBy) != 0 {
		t.Errorf("coveredBy = %v, want nothing recorded for an unresolvable criterion", coveredBy)
	}
}

// TestCheckSlicesReportsEveryBrokenSpecIndependently proves CheckSlices
// reports violations from every broken group in the batch, not only the
// first one it finds.
func TestCheckSlicesReportsEveryBrokenSpecIndependently(t *testing.T) {
	first, firstFiles := validReagentMapFixture()
	first[0].Order.Slice.Covers = []string{"AC1:ui"} // drops AC3:ui from the first spec

	secondSpecBytes := []byte(`{"criteria":[{"id":"AC1","observableVia":"api"}]}`)
	second := []Document{{Order: Order{ID: "other-spec-slice", Slice: &Slice{
		Of: "other-spec", Spec: "docs/specs/other-spec/acceptance-criteria.json",
		Digest:   "sha256:" + strings.Repeat("1", 64), // deliberately wrong: isolates this spec's own break
		Manifest: "docs/specs/other-spec/package.sha256",
	}}}}

	files := map[string][]byte{"docs/specs/other-spec/acceptance-criteria.json": secondSpecBytes}
	for path, content := range firstFiles {
		files[path] = content
	}

	documents := append(first, second...)
	violations, _ := CheckSlices(documents, fakeFileReader(files))

	if !anyViolationContains(violations, "AC3:ui") {
		t.Errorf("violations = %v, want the first spec's gap reported", violations)
	}
	if !anyViolationContains(violations, "other-spec-slice") {
		t.Errorf("violations = %v, want the second spec's own violation reported too", violations)
	}
}
