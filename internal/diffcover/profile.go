// Package diffcover measures test coverage over the lines a change adds or
// modifies, rather than over the whole codebase (roadmap L3.58, ADR-008).
//
// A whole-codebase percentage cannot gate a codebase that is already below it
// — this module's floor is 66.3% against a stated 85% — and it is the number
// an agent pads most cheaply. The lines a change touches are new work, which
// can be held to 85%.
//
// A line is executable when some block in the Go coverage profile spans it,
// and covered when any such block ran. A changed Go file that the profile
// does not mention at all is reported as unmeasured, never skipped: silence
// about a file is not evidence that it is tested.
package diffcover

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Block is one coverage-profile block: an inclusive line range and whether
// any test executed it.
type Block struct {
	StartLine int
	EndLine   int
	Covered   bool
}

// Profile maps a file's import-qualified path (as `go test -coverprofile`
// writes it) to its blocks.
type Profile map[string][]Block

// ParseProfile reads a Go coverage profile in any mode (set, count, atomic).
func ParseProfile(reader io.Reader) (Profile, error) {
	profile := Profile{}
	scanner := bufio.NewScanner(reader)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		file, block, err := parseProfileLine(line)
		if err != nil {
			return nil, fmt.Errorf("coverage profile line %d: %w", lineNumber, err)
		}
		profile[file] = append(profile[file], block)
	}
	return profile, scanner.Err()
}

// parseProfileLine parses "path/file.go:12.3,14.2 2 1" — start line.column,
// end line.column, statement count, execution count.
func parseProfileLine(line string) (string, Block, error) {
	location, counts, found := strings.Cut(line, " ")
	if !found {
		return "", Block{}, fmt.Errorf("%q has no counts", line)
	}
	separator := strings.LastIndex(location, ":")
	if separator < 0 {
		return "", Block{}, fmt.Errorf("%q has no position", line)
	}
	startLine, endLine, err := parseRange(location[separator+1:])
	if err != nil {
		return "", Block{}, err
	}
	executions, err := parseExecutions(counts)
	if err != nil {
		return "", Block{}, err
	}
	return location[:separator], Block{StartLine: startLine, EndLine: endLine, Covered: executions > 0}, nil
}

func parseRange(position string) (int, int, error) {
	start, end, found := strings.Cut(position, ",")
	if !found {
		return 0, 0, fmt.Errorf("position %q is not start,end", position)
	}
	startLine, err := lineOf(start)
	if err != nil {
		return 0, 0, err
	}
	endLine, err := lineOf(end)
	return startLine, endLine, err
}

// lineOf returns the line of a "line.column" position.
func lineOf(position string) (int, error) {
	line, _, _ := strings.Cut(position, ".")
	return strconv.Atoi(line)
}

func parseExecutions(counts string) (int, error) {
	fields := strings.Fields(counts)
	if len(fields) != 2 {
		return 0, fmt.Errorf("counts %q are not \"statements executions\"", counts)
	}
	return strconv.Atoi(fields[1])
}
