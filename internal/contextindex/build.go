package contextindex

import "github.com/orieken/loom/internal/factsheet"

// Build brings an index up to date with the current fact sheets. A directory
// keeps whatever was recorded about it — purpose, tags, layer, the hash it was
// documented against — and has only its status recomputed. A new directory
// starts undocumented, and an entry for a directory that no longer exists is
// dropped. Nothing here consults a clock, so an unchanged tree rebuilds to the
// same bytes.
func Build(previous Index, sheets []factsheet.FactSheet) Index {
	entries := make(map[string]Entry, len(sheets))
	for _, sheet := range sheets {
		entry := normalize(previous.Entries[sheet.Path])
		entry.Status = StatusOf(entry.InputHash, sheet.InputHash)
		entries[sheet.Path] = entry
	}
	return Index{SchemaVersion: SchemaVersion, Entries: entries}
}

// normalize makes list fields non-nil so they serialize as [] rather than null.
func normalize(entry Entry) Entry {
	if entry.Tags == nil {
		entry.Tags = []string{}
	}
	if entry.OpenQuestions == nil {
		entry.OpenQuestions = []string{}
	}
	return entry
}
