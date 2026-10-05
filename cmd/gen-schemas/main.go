// Command gen-schemas writes the generated schema artifacts: one JSON Schema
// per typed pipeline stage into shared/schemas/pipeline/, and the run event
// schema plus its documentation table into shared/schemas/telemetry/. Run it
// from the repo root after changing any struct in internal/state or the event
// vocabulary in internal/orchestrator; a test fails if the committed files
// drift from what the source generates.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/repograph"
	"github.com/orieken/loom/internal/state"
)

func main() {
	if err := generate(state.SchemaDir); err != nil {
		fmt.Fprintln(os.Stderr, "gen-schemas:", err)
		os.Exit(1)
	}
	if err := generateTelemetry(orchestrator.TelemetrySchemaDir); err != nil {
		fmt.Fprintln(os.Stderr, "gen-schemas:", err)
		os.Exit(1)
	}
	if err := generateRepoGraph(repograph.SchemaDir); err != nil {
		fmt.Fprintln(os.Stderr, "gen-schemas:", err)
		os.Exit(1)
	}
}

// generateRepoGraph writes the graph.json schema (ADR-010).
func generateRepoGraph(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create repograph schema directory: %w", err)
	}
	graph, err := repograph.GenerateSchema()
	if err != nil {
		return err
	}
	if err := writeGenerated(filepath.Join(dir, repograph.SchemaFile), graph); err != nil {
		return err
	}
	sheet, err := factsheet.GenerateSchema()
	if err != nil {
		return err
	}
	return writeGenerated(filepath.Join(dir, factsheet.SchemaFile), sheet)
}

// generateTelemetry writes the event schema and its documentation table
// from the single vocabulary in internal/orchestrator (roadmap L3.9).
func generateTelemetry(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create telemetry schema directory: %w", err)
	}
	schema, err := orchestrator.GenerateEventSchema()
	if err != nil {
		return err
	}
	files := map[string][]byte{
		orchestrator.RunEventSchemaFile: schema,
		orchestrator.RunEventTableFile:  orchestrator.GenerateEventTable(),
	}
	for name, content := range files {
		if err := writeGenerated(filepath.Join(dir, name), content); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerated(path string, content []byte) error {
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Println("wrote", path)
	return nil
}

func generate(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create schema directory: %w", err)
	}
	for _, schema := range state.StageSchemas() {
		if err := writeSchema(dir, schema); err != nil {
			return err
		}
	}
	return nil
}

func writeSchema(dir string, schema state.StageSchema) error {
	raw, err := schema.Generate()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, schema.FileName)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Println("wrote", path)
	return nil
}
