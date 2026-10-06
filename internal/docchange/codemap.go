package docchange

import (
	"fmt"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/factsheet"
)

// maxFanIn and maxEntrypoints bound the two ranked sections.
const (
	maxFanIn       = 10
	maxEntrypoints = 40
	// codemapDepth is how deep the directory table goes.
	codemapDepth = 2
)

// codemapBody renders CODEMAP.md's content from the fact sheets (the graph,
// per directory) and the index. It has no clock and no model: the same inputs
// give the same text. A directory with no purpose says it is undocumented
// rather than carrying a placeholder to be filled in.
func codemapBody(sheets []factsheet.FactSheet, index contextindex.Index) string {
	var out strings.Builder
	out.WriteString("# CODEMAP\n\nEntry-point map for this repository, generated from its import graph and the context index. ")
	out.WriteString("Read it first to orient, then search for specifics.\n\n")
	writeEntrypoints(&out, sheets)
	writeFanIn(&out, sheets, index)
	writeLayers(&out, index)
	writeDirectories(&out, sheets, index)
	return out.String()
}

func writeEntrypoints(out *strings.Builder, sheets []factsheet.FactSheet) {
	var found []string
	for _, sheet := range sheets {
		for _, entry := range sheet.Entrypoints {
			found = append(found, joinPath(sheet.Path, entry))
		}
	}
	sort.Strings(found)
	out.WriteString("## Entrypoints\n\n")
	if len(found) == 0 {
		out.WriteString("_None detected._\n\n")
		return
	}
	for index, entry := range found {
		if index == maxEntrypoints {
			fmt.Fprintf(out, "- … and %d more\n", len(found)-maxEntrypoints)
			break
		}
		fmt.Fprintf(out, "- `%s`\n", entry)
	}
	out.WriteString("\n")
}

func joinPath(dir, name string) string {
	if dir == factsheet.RootPath {
		return name
	}
	return dir + "/" + name
}

// writeFanIn lists the directories the most others import: the ones a change
// is most likely to ripple through.
func writeFanIn(out *strings.Builder, sheets []factsheet.FactSheet, index contextindex.Index) {
	ranked := make([]factsheet.FactSheet, 0, len(sheets))
	for _, sheet := range sheets {
		if len(sheet.Dependents) > 0 {
			ranked = append(ranked, sheet)
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if len(ranked[i].Dependents) != len(ranked[j].Dependents) {
			return len(ranked[i].Dependents) > len(ranked[j].Dependents)
		}
		return ranked[i].Path < ranked[j].Path
	})
	out.WriteString("## Highest fan-in\n\nDirectories imported by the most others (exact edges only).\n\n")
	if len(ranked) == 0 {
		out.WriteString("_No import edges found._\n\n")
		return
	}
	for position, sheet := range ranked {
		if position == maxFanIn {
			break
		}
		fmt.Fprintf(out, "- `%s` — %d dependents — %s\n", sheet.Path, len(sheet.Dependents), purposeOf(index, sheet.Path))
	}
	out.WriteString("\n")
}

func purposeOf(index contextindex.Index, dir string) string {
	if purpose := index.Entries[dir].Purpose; purpose != "" {
		return purpose
	}
	return "_undocumented_"
}

// writeLayers groups the documented directories by the layer the documenter
// assigned. Directories with none are left out: a layer is an inference, and
// an absent one is not a layer.
func writeLayers(out *strings.Builder, index contextindex.Index) {
	byLayer := map[string][]string{}
	for dir, entry := range index.Entries {
		if entry.Layer != "" {
			byLayer[entry.Layer] = append(byLayer[entry.Layer], dir)
		}
	}
	out.WriteString("## Layers\n\n")
	if len(byLayer) == 0 {
		out.WriteString("_No layers inferred yet._\n\n")
		return
	}
	layers := make([]string, 0, len(byLayer))
	for layer := range byLayer {
		layers = append(layers, layer)
	}
	sort.Strings(layers)
	for _, layer := range layers {
		sort.Strings(byLayer[layer])
		fmt.Fprintf(out, "- **%s**: %s\n", layer, "`"+strings.Join(byLayer[layer], "`, `")+"`")
	}
	out.WriteString("\n")
}

func writeDirectories(out *strings.Builder, sheets []factsheet.FactSheet, index contextindex.Index) {
	out.WriteString("## Directories\n\n| Directory | Purpose |\n|---|---|\n")
	for _, sheet := range sheets {
		if sheet.Path == factsheet.RootPath || strings.Count(sheet.Path, "/")+1 > codemapDepth {
			continue
		}
		fmt.Fprintf(out, "| `%s/` | %s |\n", sheet.Path, strings.ReplaceAll(purposeOf(index, sheet.Path), "|", "\\|"))
	}
}
