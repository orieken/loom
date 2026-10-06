package cmd

import (
	"fmt"

	"github.com/orieken/loom/cmd/loom/internal/levels"
)

// levelProbe gathers the mechanical evidence behind a maturity level claim.
// Defined here at the consumer; healthLevelProbe implements it against the
// target project, tests substitute fakes.
type levelProbe interface {
	pathBundleInstalled(bundle levels.Bundle) (bool, string)
	mcpServerAnswering() (bool, string)
	telemetryStreamPresent() (bool, string)
	policiesPresent() (bool, string)
	repoIndexPresent() (bool, string)
}

type levelEvidence struct {
	passed bool
	detail string
}

// levelAssessment is one level's evidence set. Bundles gated on unlanded
// roadmap items are listed in gated, not evidence — you cannot install what
// has not shipped. Doc-only bundles contribute nothing: documentation
// presence never confers a level.
type levelAssessment struct {
	level    int
	name     string
	evidence []levelEvidence
	gated    []string
}

func (assessment levelAssessment) isAttained() bool {
	if len(assessment.evidence) == 0 {
		return false
	}
	for _, item := range assessment.evidence {
		if !item.passed {
			return false
		}
	}
	return true
}

func assessLevels(profile levels.Profile, probe levelProbe) []levelAssessment {
	assessments := make([]levelAssessment, 0, len(profile.Levels))
	for _, level := range profile.Levels {
		assessments = append(assessments, assessLevel(profile, level, probe))
	}
	return assessments
}

func assessLevel(profile levels.Profile, level levels.Level, probe levelProbe) levelAssessment {
	assessment := levelAssessment{level: level.Level, name: level.Name}
	for _, bundle := range level.Bundles {
		if bundle.DocsOnly || bundle.OptIn {
			continue
		}
		if bundle.Requires != "" && !profile.IsLanded(bundle.Requires) {
			assessment.gated = append(assessment.gated, fmt.Sprintf("%s: gated on roadmap item %s (not landed)", bundle.ID, bundle.Requires))
			continue
		}
		passed, detail := checkBundle(bundle, probe)
		assessment.evidence = append(assessment.evidence, levelEvidence{passed: passed, detail: detail})
	}
	return assessment
}

func checkBundle(bundle levels.Bundle, probe levelProbe) (bool, string) {
	checks := map[string]func() (bool, string){
		"mcp-config": probe.mcpServerAnswering,
		"telemetry":  probe.telemetryStreamPresent,
		"policies":   probe.policiesPresent,
		"repo-index": probe.repoIndexPresent,
	}
	if bundle.Action == "" {
		return probe.pathBundleInstalled(bundle)
	}
	check, known := checks[bundle.Action]
	if !known {
		return false, fmt.Sprintf("%s: no health check exists for action %q yet", bundle.ID, bundle.Action)
	}
	return check()
}

// maturityReport is the inferred level plus the gap checklist to the next one.
type maturityReport struct {
	level             int
	name              string
	passing           []string
	nextLevel         int
	gaps              []string
	nextIsUnreachable bool
}

// inferMaturity applies the inference rule: report the highest level whose
// entire evidence set passes; a level with an empty evidence set (everything
// gated or doc-only) is unattainable, which caps the ladder honestly.
func inferMaturity(assessments []levelAssessment) maturityReport {
	report := maturityReport{}
	for _, assessment := range assessments {
		if !assessment.isAttained() {
			break
		}
		report.level = assessment.level
		report.name = assessment.name
		for _, item := range assessment.evidence {
			report.passing = append(report.passing, item.detail)
		}
	}
	fillMaturityGaps(&report, assessments)
	return report
}

func fillMaturityGaps(report *maturityReport, assessments []levelAssessment) {
	if report.level >= len(assessments) {
		return
	}
	next := assessments[report.level]
	report.nextLevel = next.level
	for _, item := range next.evidence {
		if !item.passed {
			report.gaps = append(report.gaps, item.detail)
		}
	}
	report.gaps = append(report.gaps, next.gated...)
	report.nextIsUnreachable = len(next.evidence) == 0
}

func (check *healthCheck) verifyMaturity() {
	profile, err := levels.Load(check.content)
	if err != nil {
		check.warn("maturity assessment skipped: " + err.Error())
		return
	}
	probe := healthLevelProbe{target: check.request.target, manifestPaths: manifestPaths(check.manifest.Platforms)}
	check.output.maturity(inferMaturity(assessLevels(profile, probe)))
}
