package diffcover

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Changes maps a repository-relative file path to the line numbers the change
// adds or modifies in the new version of that file.
type Changes map[string][]int

// ParseDiff reads `git diff --unified=0` output. Only added lines are kept:
// a pure deletion has no new line to cover, and a modified line appears in a
// zero-context diff as a deletion plus an addition.
func ParseDiff(reader io.Reader) (Changes, error) {
	changes := Changes{}
	current := ""
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "+++ "):
			current = newFilePath(line)
		case strings.HasPrefix(line, "@@") && current != "":
			added, err := addedLines(line)
			if err != nil {
				return nil, err
			}
			changes[current] = append(changes[current], added...)
		}
	}
	return changes, scanner.Err()
}

// newFilePath returns the post-change path from a "+++ b/path" header, or ""
// for "+++ /dev/null" — a deleted file has nothing left to cover.
func newFilePath(header string) string {
	path := strings.TrimPrefix(header, "+++ ")
	if path == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(path, "b/")
}

// addedLines expands a hunk header "@@ -a,b +start,count @@" into the new
// file's line numbers. A missing count means one line; a zero count means the
// hunk only deletes.
func addedLines(header string) ([]int, error) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return nil, fmt.Errorf("hunk header %q has no new-file range", header)
	}
	start, count, err := parseHunkRange(strings.TrimPrefix(fields[2], "+"))
	if err != nil {
		return nil, fmt.Errorf("hunk header %q: %w", header, err)
	}
	lines := make([]int, 0, count)
	for offset := 0; offset < count; offset++ {
		lines = append(lines, start+offset)
	}
	return lines, nil
}

func parseHunkRange(hunkRange string) (int, int, error) {
	startText, countText, hasCount := strings.Cut(hunkRange, ",")
	start, err := strconv.Atoi(startText)
	if err != nil {
		return 0, 0, err
	}
	if !hasCount {
		return start, 1, nil
	}
	count, err := strconv.Atoi(countText)
	return start, count, err
}
