// Package tools - retriever.go
//
// Local retrieval adapter for M1 of mcp-expand. Per framework
// ADR-002 (Corpus-Aware Retrieval Strategy), the framework KI + ADR
// corpus is small enough that "LLM-as-retriever" is the correct
// approach — this adapter walks the corpus and surfaces matches with a
// lightweight lexical relevance score. The caller (an LLM invoking
// the search_ki MCP tool) does final semantic filtering on the ranked
// list this returns.
package tools

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Reference is a single retrieval hit — a pointer to canonical
// markdown, never a content copy (per ADR-002's principle "always
// verify retrieved knowledge against markdown").
type Reference struct {
	Title     string
	Path      string
	Summary   string
	Tags      []string
	Domain    string
	Relevance float64
}

// Retriever surfaces corpus items relevant to a query. Implementations
// vary by corpus per ADR-002's graduated retrieval strategy.
type Retriever interface {
	Retrieve(ctx context.Context, query string, tags []string, domain string) ([]Reference, error)
}

// maxResults caps the surfaced set so the calling LLM sees a
// manageable list rather than the entire corpus.
const maxResults = 25

// KICorpusRetriever walks Knowledge Items and Architecture Decision
// Records on disk and scores each by lexical query relevance.
type KICorpusRetriever struct {
	corpusPaths []string
}

// NewKICorpusRetriever wires the retriever with its corpus roots.
func NewKICorpusRetriever(corpusPaths []string) *KICorpusRetriever {
	return &KICorpusRetriever{corpusPaths: corpusPaths}
}

// Retrieve walks every configured corpus path, scores every markdown
// file, and returns the top-N by relevance descending.
func (r *KICorpusRetriever) Retrieve(ctx context.Context, query string, tags []string, domain string) ([]Reference, error) {
	if len(r.corpusPaths) == 0 || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	queryTokens := tokenize(query)
	var hits []Reference
	for _, root := range r.corpusPaths {
		found, err := hitsFromRoot(ctx, root, queryTokens, tags, domain)
		if err != nil {
			return nil, err
		}
		hits = append(hits, found...)
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Relevance > hits[j].Relevance })
	if len(hits) > maxResults {
		hits = hits[:maxResults]
	}
	return hits, nil
}

func shouldSkipEntry(entry os.DirEntry) bool {
	return entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "README.md"
}

// hitsFromRoot scores every corpus file directly under root, stopping with
// ctx's error once ctx is done (roadmap L2.2).
func hitsFromRoot(ctx context.Context, root string, queryTokens, tags []string, domain string) ([]Reference, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil
	}
	var hits []Reference
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if shouldSkipEntry(entry) {
			continue
		}
		ref, ok := parseCorpusFile(filepath.Join(root, entry.Name()))
		if !ok {
			continue
		}
		ref.Relevance = scoreRelevance(ref, queryTokens, tags, domain)
		if ref.Relevance > 0 {
			hits = append(hits, ref)
		}
	}
	return hits, nil
}

func parseCorpusFile(path string) (Reference, bool) {
	file, err := os.Open(path)
	if err != nil {
		return Reference{}, false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	if !scanner.Scan() || scanner.Text() != "---" {
		return Reference{}, false
	}
	ref := Reference{Path: path}
	summaryLines := scanFrontmatterAndBody(scanner, &ref)
	if ref.Title == "" {
		ref.Title = titleFromFilename(path)
	}
	ref.Summary = strings.Join(summaryLines, " ")
	return ref, true
}

func scanFrontmatterAndBody(scanner *bufio.Scanner, ref *Reference) []string {
	inFrontmatter := true
	var summaryLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if inFrontmatter {
			if line == "---" {
				inFrontmatter = false
				continue
			}
			assignFrontmatterField(ref, line)
			continue
		}
		summaryLines = appendSummaryLine(summaryLines, line)
	}
	return summaryLines
}

func appendSummaryLine(lines []string, line string) []string {
	if len(lines) >= 5 {
		return lines
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return lines
	}
	return append(lines, trimmed)
}

func assignFrontmatterField(ref *Reference, line string) {
	colonAt := strings.Index(line, ":")
	if colonAt <= 0 {
		return
	}
	key := strings.TrimSpace(line[:colonAt])
	value := strings.TrimSpace(line[colonAt+1:])
	switch key {
	case "name":
		ref.Title = value
	case "domain":
		ref.Domain = value
	case "tags":
		ref.Tags = parseTagsList(value)
	}
}

func parseTagsList(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		tag := strings.Trim(strings.TrimSpace(p), `"'`)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func titleFromFilename(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".md")
	return strings.ReplaceAll(base, "-", " ")
}

func scoreRelevance(ref Reference, queryTokens, requestedTags []string, requestedDomain string) float64 {
	tagSet := make(map[string]struct{}, len(ref.Tags))
	for _, t := range ref.Tags {
		tagSet[strings.ToLower(t)] = struct{}{}
	}
	score := scoreTagMatch(tagSet, requestedTags)
	if requestedDomain != "" && strings.EqualFold(requestedDomain, ref.Domain) {
		score += 1.5
	}
	titleLower := strings.ToLower(ref.Title)
	summaryLower := strings.ToLower(ref.Summary)
	for _, tok := range queryTokens {
		score += scoreTokenMatch(titleLower, summaryLower, tok)
	}
	return score
}

func scoreTagMatch(tagSet map[string]struct{}, requestedTags []string) float64 {
	var score float64
	for _, wanted := range requestedTags {
		if _, ok := tagSet[strings.ToLower(wanted)]; ok {
			score += 2.0
		}
	}
	return score
}

func scoreTokenMatch(titleLower, summaryLower, tok string) float64 {
	var score float64
	if strings.Contains(titleLower, tok) {
		score += 1.0
	}
	if strings.Contains(summaryLower, tok) {
		score += 0.3
	}
	return score
}

func tokenize(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		trimmed := strings.Trim(f, ".,;:!?()[]{}\"'`")
		if trimmed != "" {
			tokens = append(tokens, trimmed)
		}
	}
	return tokens
}
