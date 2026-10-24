package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/orieken/loom/internal/diffcover"
)

// maxAnnotatedFiles stays under GitHub's limit of ten error annotations per
// step, so every annotation that is written is shown.
const maxAnnotatedFiles = 10

// annotate writes one GitHub error annotation per file that fails the gate,
// naming its uncovered lines. The plain report lives in the job log; an
// annotation is what a pull request shows without opening it.
func annotate(out io.Writer, report diffcover.Report) {
	files, lines := uncoveredByFile(report.Uncovered)
	written := 0
	for _, file := range report.Unmeasured {
		if written == maxAnnotatedFiles {
			return
		}
		fmt.Fprintf(out, "::error file=%s::changed, but absent from the coverage profile\n", file)
		written++
	}
	for _, file := range files {
		if written == maxAnnotatedFiles {
			return
		}
		fmt.Fprintf(out, "::error file=%s,line=%d::uncovered changed lines: %s\n", file, lines[file][0], joinLines(lines[file]))
		written++
	}
}

func uncoveredByFile(uncovered []diffcover.Line) ([]string, map[string][]int) {
	lines := map[string][]int{}
	for _, line := range uncovered {
		lines[line.File] = append(lines[line.File], line.Line)
	}
	files := make([]string, 0, len(lines))
	for file := range lines {
		sort.Ints(lines[file])
		files = append(files, file)
	}
	sort.Strings(files)
	return files, lines
}

func joinLines(lines []int) string {
	text := make([]string, len(lines))
	for index, line := range lines {
		text[index] = strconv.Itoa(line)
	}
	return strings.Join(text, ", ")
}
