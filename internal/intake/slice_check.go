package intake

import (
	"sort"
	"strings"
)

// SliceSummary is what loom intake check prints for one spec package's
// slices once every criterion level they declare is covered with no
// violation: how many slices share it, and how many criterion levels they
// cover together.
type SliceSummary struct {
	Of     string
	Slices int
	Levels int
}

// CheckSlices verifies every group of orders in orders that share one
// slice.of: that together they cover each of the spec's criterion levels
// exactly once, that they agree on a manifest digest which matches the
// manifest's own files, and that their depends-on edges among the group
// have no cycle, and no entry naming an order absent from orders entirely.
// It reports every violation across every group, never only the first, and
// calls read zero times when no order in orders sets slice. summaries holds
// one entry per spec whose slices covered it with no violation.
func CheckSlices(orders []Document, read FileReader) ([]Violation, []SliceSummary) {
	allIDs := allOrderIDs(orders)
	var violations []Violation
	var summaries []SliceSummary
	for _, of := range sortedSliceOfs(orders) {
		group := ordersSlicedFrom(orders, of)
		groupViolations, summary := checkSliceGroup(of, group, allIDs, read)
		violations = append(violations, groupViolations...)
		if len(groupViolations) == 0 {
			summaries = append(summaries, summary)
		}
	}
	return violations, summaries
}

func allOrderIDs(orders []Document) map[string]bool {
	ids := make(map[string]bool, len(orders))
	for _, doc := range orders {
		ids[doc.Order.ID] = true
	}
	return ids
}

// sortedSliceOfs returns the distinct slice.of values orders declares, in a
// deterministic order so a run's report reads the same way twice.
func sortedSliceOfs(orders []Document) []string {
	seen := map[string]bool{}
	var ofs []string
	for _, doc := range orders {
		if doc.Order.Slice == nil || seen[doc.Order.Slice.Of] {
			continue
		}
		seen[doc.Order.Slice.Of] = true
		ofs = append(ofs, doc.Order.Slice.Of)
	}
	sort.Strings(ofs)
	return ofs
}

func ordersSlicedFrom(orders []Document, of string) []Document {
	var group []Document
	for _, doc := range orders {
		if doc.Order.Slice != nil && doc.Order.Slice.Of == of {
			group = append(group, doc)
		}
	}
	sort.Slice(group, func(i, j int) bool { return group[i].Order.ID < group[j].Order.ID })
	return group
}

func checkSliceGroup(of string, group []Document, allIDs map[string]bool, read FileReader) ([]Violation, SliceSummary) {
	var violations []Violation
	violations = append(violations, checkDanglingDependsOn(group, allIDs)...)
	violations = append(violations, checkDependsOnCycle(of, group)...)
	violations = append(violations, checkDigestAgreement(group)...)

	manifestViolations, manifests := loadGroupManifests(group, read)
	violations = append(violations, manifestViolations...)
	violations = append(violations, checkManifestDigests(group, manifests)...)
	violations = append(violations, checkManifestFiles(manifests, read)...)

	pkg, specPath, specViolations := loadGroupSpecPackage(group, read)
	violations = append(violations, specViolations...)
	if specPath == "" {
		return violations, SliceSummary{}
	}
	coverageViolations, levels := checkCoverage(of, specPath, pkg, group)
	violations = append(violations, coverageViolations...)

	return violations, SliceSummary{Of: of, Slices: len(group), Levels: levels}
}

// checkDanglingDependsOn reports a violation naming the order and the
// missing id when a depends-on entry names no order present anywhere in the
// batch CheckSlices was given.
func checkDanglingDependsOn(group []Document, allIDs map[string]bool) []Violation {
	var violations []Violation
	for _, doc := range group {
		for _, dep := range doc.Order.DependsOn {
			if !allIDs[dep] {
				violations = append(violations, violation("slice.depends-on",
					"%s depends on %q, which is not an order in the directory", doc.Order.ID, dep)...)
			}
		}
	}
	return violations
}

// checkDependsOnCycle reports one violation naming every order in a
// depends-on cycle among group's slices, found by a local DFS — the graph
// here is at most a handful of nodes, one spec's slices.
func checkDependsOnCycle(of string, group []Document) []Violation {
	edges := map[string][]string{}
	for _, doc := range group {
		edges[doc.Order.ID] = doc.Order.DependsOn
	}
	for _, doc := range group {
		if cycle := findCycle(doc.Order.ID, edges); cycle != nil {
			return violation("slice.depends-on", "depends-on cycle among slices of %s: %s",
				of, strings.Join(cycle, " -> "))
		}
	}
	return nil
}

// findCycle runs a depth-first search from start and returns the member ids
// of the first cycle it finds, start to start, or nil if there is none.
func findCycle(start string, edges map[string][]string) []string {
	var path []string
	onPath := map[string]int{}
	var visit func(node string) []string
	visit = func(node string) []string {
		if index, found := onPath[node]; found {
			return append(append([]string{}, path[index:]...), node)
		}
		onPath[node] = len(path)
		path = append(path, node)
		for _, next := range edges[node] {
			if cycle := visit(next); cycle != nil {
				return cycle
			}
		}
		path = path[:len(path)-1]
		delete(onPath, node)
		return nil
	}
	return visit(start)
}

// checkDigestAgreement reports one violation naming every order when
// slices of one spec declare different slice.digest values.
func checkDigestAgreement(group []Document) []Violation {
	digests := map[string]bool{}
	for _, doc := range group {
		digests[doc.Order.Slice.Digest] = true
	}
	if len(digests) <= 1 {
		return nil
	}
	var pairs []string
	for _, doc := range group {
		pairs = append(pairs, doc.Order.ID+": "+doc.Order.Slice.Digest)
	}
	return violation("slice.digest", "slices of one spec disagree on slice.digest: %s", strings.Join(pairs, "; "))
}

// loadGroupManifests reads every distinct slice.manifest path group's
// orders name, caching by path, and reports a violation naming the order
// for every reference to a path that could not be read.
func loadGroupManifests(group []Document, read FileReader) ([]Violation, map[string]manifestFile) {
	type result struct {
		manifest manifestFile
		err      error
	}
	cache := map[string]result{}
	var violations []Violation
	loaded := map[string]manifestFile{}
	for _, doc := range group {
		path := doc.Order.Slice.Manifest
		cached, ok := cache[path]
		if !ok {
			m, err := readManifest(path, read)
			cached = result{manifest: m, err: err}
			cache[path] = cached
		}
		if cached.err != nil {
			violations = append(violations, violation("slice.manifest",
				"%s: slice.manifest %q could not be read: %v", doc.Order.ID, path, cached.err)...)
			continue
		}
		loaded[path] = cached.manifest
	}
	return violations, loaded
}

// checkManifestDigests reports a violation naming the order when its
// slice.digest does not match the SHA-256 of its own slice.manifest's bytes.
// An order whose manifest failed to load already has a violation from
// loadGroupManifests and is skipped here.
func checkManifestDigests(group []Document, manifests map[string]manifestFile) []Violation {
	var violations []Violation
	for _, doc := range group {
		m, ok := manifests[doc.Order.Slice.Manifest]
		if !ok {
			continue
		}
		if actual := sha256Digest(m.raw); doc.Order.Slice.Digest != actual {
			violations = append(violations, violation("slice.digest",
				"%s: slice.digest %s does not match sha256 of %s (%s)",
				doc.Order.ID, doc.Order.Slice.Digest, doc.Order.Slice.Manifest, actual)...)
		}
	}
	return violations
}

// checkManifestFiles verifies every file every successfully loaded manifest
// lists still matches its recorded hash, naming the file and manifest when
// it does not.
func checkManifestFiles(manifests map[string]manifestFile, read FileReader) []Violation {
	var violations []Violation
	for _, manifestPath := range sortedKeys(manifests) {
		m := manifests[manifestPath]
		for _, file := range sortedManifestFiles(m) {
			violations = append(violations, checkListedFile(manifestPath, file, m.entries[file], read)...)
		}
	}
	return violations
}

func checkListedFile(manifestPath, file, wantHash string, read FileReader) []Violation {
	content, err := readRooted(file, read)
	if err != nil {
		return violation("slice.manifest", "%s lists %q, which could not be read: %v", manifestPath, file, err)
	}
	if actual := sha256Hex(content); actual != wantHash {
		return violation("slice.manifest", "%s no longer matches its recorded hash in %s", file, manifestPath)
	}
	return nil
}

func sortedKeys(manifests map[string]manifestFile) []string {
	keys := make([]string, 0, len(manifests))
	for key := range manifests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedManifestFiles(m manifestFile) []string {
	files := make([]string, 0, len(m.entries))
	for file := range m.entries {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

// loadGroupSpecPackage reads the group's spec package, trying every order's
// own slice.spec path (normally identical across the group) and caching by
// path. It reports a violation naming the order for every reference to a
// path that could not be read or parsed, and returns the first package that
// loaded along with the path it came from.
func loadGroupSpecPackage(group []Document, read FileReader) (SpecPackage, string, []Violation) {
	type result struct {
		pkg SpecPackage
		err error
	}
	cache := map[string]result{}
	var violations []Violation
	loadedPath := ""
	for _, doc := range group {
		path := doc.Order.Slice.Spec
		cached, ok := cache[path]
		if !ok {
			pkg, err := readSpecPackage(path, read)
			cached = result{pkg: pkg, err: err}
			cache[path] = cached
		}
		if cached.err != nil {
			violations = append(violations, violation("slice.spec",
				"%s: slice.spec %q could not be read: %v", doc.Order.ID, path, cached.err)...)
			continue
		}
		if loadedPath == "" {
			loadedPath = path
		}
	}
	if loadedPath == "" {
		return SpecPackage{}, "", violations
	}
	return cache[loadedPath].pkg, loadedPath, violations
}

// checkCoverage validates every slice's covers entries against pkg and
// reports a violation for any criterion level pkg requires that is
// uncovered or covered by more than one slice. It returns the total number
// of criterion-levels pkg declares, for the passing-split summary.
func checkCoverage(of, specPath string, pkg SpecPackage, group []Document) ([]Violation, int) {
	var violations []Violation
	coveredBy := map[string][]string{} // "AC1:ui" -> order ids
	for _, doc := range group {
		for _, raw := range doc.Order.Slice.Covers {
			violations = append(violations, applyCoversEntry(doc.Order.ID, raw, specPath, pkg, coveredBy)...)
		}
	}
	levels := 0
	for _, criterion := range pkg.Criteria {
		criterionLevels, ok := levelsOf(criterion)
		if !ok {
			violations = append(violations, violation("slice.spec",
				"%s: %s has no observableVia (api, ui or both)", specPath, criterion.ID)...)
			continue
		}
		for _, level := range criterionLevels {
			levels++
			violations = append(violations, checkLevelCoverage(of, criterion.ID, level, coveredBy)...)
		}
	}
	return violations, levels
}

func checkLevelCoverage(of, criterionID, level string, coveredBy map[string][]string) []Violation {
	ids := coveredBy[levelLabel(criterionID, level)]
	switch len(ids) {
	case 0:
		return violation("slice.covers", "%s is not covered by any slice of %s", levelLabel(criterionID, level), of)
	case 1:
		return nil
	default:
		return violation("slice.covers", "%s is covered by more than one slice of %s: %s",
			levelLabel(criterionID, level), of, strings.Join(ids, ", "))
	}
}

func levelLabel(criterionID, level string) string {
	return criterionID + ":" + level
}

// applyCoversEntry parses one covers entry and records the order as
// covering every level it resolves to, or reports a violation naming the
// order when the entry names a criterion or level pkg does not have.
func applyCoversEntry(orderID, raw, specPath string, pkg SpecPackage, coveredBy map[string][]string) []Violation {
	entry := parseCoversEntry(raw)
	if entry.level != "" && !knownLevel[entry.level] {
		return violation("slice.covers", "%s covers %s, %q is not a level (api or ui)", orderID, raw, entry.level)
	}
	criterion, found := findCriterion(pkg, entry.criterionID)
	if !found {
		return violation("slice.covers", "%s covers %q, which %s does not have", orderID, entry.criterionID, specPath)
	}
	criterionLevels, ok := levelsOf(criterion)
	if !ok {
		return nil // already reported once by checkCoverage, against the spec itself
	}
	levels, violations := resolveCoversLevels(entry, criterionLevels, orderID)
	for _, level := range levels {
		label := levelLabel(entry.criterionID, level)
		coveredBy[label] = append(coveredBy[label], orderID)
	}
	return violations
}

func resolveCoversLevels(entry coversEntry, criterionLevels []string, orderID string) ([]string, []Violation) {
	if entry.level == "" {
		return criterionLevels, nil
	}
	for _, level := range criterionLevels {
		if level == entry.level {
			return []string{entry.level}, nil
		}
	}
	return nil, violation("slice.covers", "%s covers %s, but %s has no %s level",
		orderID, levelLabel(entry.criterionID, entry.level), entry.criterionID, entry.level)
}

func findCriterion(pkg SpecPackage, id string) (Criterion, bool) {
	for _, criterion := range pkg.Criteria {
		if criterion.ID == id {
			return criterion, true
		}
	}
	return Criterion{}, false
}
