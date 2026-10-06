package repodoc

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/state"
)

// Verify checks a documenter result against the fact sheet it was given and
// returns every violation, in a stable order. It is the first of two checks:
// a program can say a name does not exist, but only an auditor can say whether
// the facts cited actually support the claim (ADR-010, Phase 5).
//
// Names are checked only when the agent writes them in backticks — its prompt
// reserves backticks for names the fact sheet contains — so a name written
// bare is outside what this can see and is left to the auditor.
func Verify(sheet factsheet.FactSheet, doc state.RepoDocState, vocabulary Vocabulary) []Violation {
	facts := newFactIndex(sheet)
	var violations []Violation
	violations = append(violations, checkTags(doc, vocabulary)...)
	violations = append(violations, checkEntrypoints(doc, facts)...)
	violations = append(violations, checkClaimsAreEvidenced(doc)...)
	violations = append(violations, checkCitations(doc, facts)...)
	violations = append(violations, checkReads(doc, facts)...)
	violations = append(violations, checkNames(doc, facts)...)
	return violations
}

func checkTags(doc state.RepoDocState, vocabulary Vocabulary) []Violation {
	var violations []Violation
	for _, tag := range doc.Tags {
		if !vocabulary.Contains(tag) {
			violations = append(violations, Violation{SourceVerifier, CodeUnknownTag, "tags",
				fmt.Sprintf("%q is not in the allowed tag vocabulary; use one of the allowed tags or none", tag)})
		}
	}
	return violations
}

func checkEntrypoints(doc state.RepoDocState, facts factIndex) []Violation {
	var violations []Violation
	for _, name := range doc.KeyEntrypoints {
		if !facts.hasSymbolOrEntrypoint(name) {
			violations = append(violations, Violation{SourceVerifier, CodeUnknownEntrypoint, "keyEntrypoints",
				fmt.Sprintf("%q is neither an export nor an entrypoint in the fact sheet", name)})
		}
	}
	return violations
}

// claims lists every claim the document makes, with the field it sits in.
func claims(doc state.RepoDocState) []claim {
	var out []claim
	if doc.Purpose != "" {
		out = append(out, claim{"purpose", doc.Purpose})
	}
	for index, responsibility := range doc.Responsibilities {
		out = append(out, claim{fmt.Sprintf("responsibilities[%d]", index), responsibility})
	}
	for index, invariant := range doc.Invariants {
		out = append(out, claim{fmt.Sprintf("invariants[%d].statement", index), invariant.Statement})
	}
	return out
}

type claim struct {
	where string
	text  string
}

// checkClaimsAreEvidenced requires each claim to appear, verbatim, as the
// claim of an evidence entry: "every generated claim traces to a fact".
func checkClaimsAreEvidenced(doc state.RepoDocState) []Violation {
	evidenced := map[string]bool{}
	for _, item := range doc.Evidence {
		evidenced[strings.TrimSpace(item.Claim)] = true
	}
	var violations []Violation
	for _, made := range claims(doc) {
		if !evidenced[strings.TrimSpace(made.text)] {
			violations = append(violations, Violation{SourceVerifier, CodeUnevidencedClaim, made.where,
				fmt.Sprintf("the claim %q has no evidence entry whose claim is that exact text", made.text)})
		}
	}
	return violations
}

func checkCitations(doc state.RepoDocState, facts factIndex) []Violation {
	var violations []Violation
	for index, item := range doc.Evidence {
		for _, citation := range item.Fields {
			if problem := facts.checkCitation(citation); problem != nil {
				problem.Where = fmt.Sprintf("evidence[%d].fields", index)
				violations = append(violations, *problem)
			}
		}
	}
	for index, invariant := range doc.Invariants {
		for _, citation := range invariant.Evidence {
			if problem := checkInvariantEvidence(citation, doc, facts); problem != nil {
				problem.Where = fmt.Sprintf("invariants[%d].evidence", index)
				violations = append(violations, *problem)
			}
		}
	}
	return violations
}

var lineRangeCitation = regexp.MustCompile(`^(.+):(\d+)-(\d+)$`)

// checkInvariantEvidence accepts a fact-sheet citation, or a file line range
// that the document also records under filesRead.
func checkInvariantEvidence(citation string, doc state.RepoDocState, facts factIndex) *Violation {
	match := lineRangeCitation.FindStringSubmatch(citation)
	if match == nil || isFactRoot(match[1]) {
		return facts.checkCitation(citation)
	}
	start, _ := strconv.Atoi(match[2])
	end, _ := strconv.Atoi(match[3])
	if recordedRead(doc.FilesRead, facts, match[1], start, end) {
		return nil
	}
	return &Violation{Source: SourceVerifier, Code: CodeUnrecordedRead,
		Message: fmt.Sprintf("%q cites source lines the document does not record under filesRead", citation)}
}

func recordedRead(reads []state.DocFileRead, facts factIndex, file string, start, end int) bool {
	for _, read := range reads {
		if facts.sameFile(read.Path, file) && read.StartLine <= start && end <= read.EndLine {
			return true
		}
	}
	return false
}

func checkReads(doc state.RepoDocState, facts factIndex) []Violation {
	var violations []Violation
	for index, read := range doc.FilesRead {
		if !facts.hasFile(read.Path) {
			violations = append(violations, Violation{SourceVerifier, CodeReadOutsideDir, fmt.Sprintf("filesRead[%d]", index),
				fmt.Sprintf("%q is not a file of %s; the documenter may only read this directory's own files", read.Path, facts.sheet.Path)})
		}
	}
	return violations
}

var backticked = regexp.MustCompile("`([^`\n]+)`")

// hedgeWords mark a sentence that treats a heuristic edge as uncertain. They
// are matched as whole words: "unlikely" is not "likely", "impossible" is not
// "possible", and "dismay" does not contain "may".
var hedgeWords = regexp.MustCompile(`(?i)\b(?:approximate|approximately|possible|possibly|may|might|likely|unconfirmed|heuristic|appears to)\b`)

// checkNames verifies every backticked name in the prose claims against the
// fact sheet, and that a name known only through a heuristic edge is not
// stated as fact.
func checkNames(doc state.RepoDocState, facts factIndex) []Violation {
	var violations []Violation
	for _, made := range claims(doc) {
		for _, match := range backticked.FindAllStringSubmatch(made.text, -1) {
			if violation := facts.checkName(match[1], made); violation != nil {
				violations = append(violations, *violation)
			}
		}
	}
	return violations
}

func hasHedge(text string) bool { return hedgeWords.MatchString(text) }

func isFactRoot(root string) bool {
	for _, known := range state.FactFieldRoots() {
		if root == known {
			return true
		}
	}
	return false
}

func baseName(value string) string { return path.Base(value) }
