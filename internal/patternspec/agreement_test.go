package patternspec

import (
	"os"
	"path/filepath"
	"testing"
)

// The committed schema is what VS Code and Cursor load, through the
// settings templates. Every fixture gets the same structural verdict from
// it as from loom health: a structure fixture fails both, and a contract
// fixture or a valid spec passes the schema and leaves the rest to Validate.
// @issue F2 @ac the Go validator and the JSON Schema agree on structure
func TestTheCommittedSchemaAgreesWithTheValidatorOnEveryFixture(t *testing.T) {
	committed, err := os.ReadFile(filepath.Join("..", "..", SchemaDir, SchemaFile))
	if err != nil {
		t.Fatalf("read the committed schema: %v (run go run ./cmd/gen-schemas)", err)
	}
	editorSchema, err := CompileSchema(committed)
	if err != nil {
		t.Fatalf("the committed schema does not compile: %v", err)
	}
	for _, path := range append(fixturePaths(t, "valid"), fixturePaths(t, "invalid")...) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			assertSchemaAgreement(t, editorSchema, path)
		})
	}
}

func assertSchemaAgreement(t *testing.T, editorSchema interface {
	Validate(any) error
}, path string) {
	t.Helper()
	instance, _, isParsed := decodeDocument(readFixture(t, path))
	if !isParsed || len(versionIssues(instance)) > 0 {
		return // a document-layer fixture never reaches either schema
	}
	isStructureFixture := filepath.Base(filepath.Dir(path)) == "invalid" && expectationOf(t, path).layer == "structure"
	editorRejects := editorSchema.Validate(instance) != nil
	validatorRejects := len(structuralIssues(instance)) > 0
	if editorRejects != validatorRejects || editorRejects != isStructureFixture {
		t.Errorf("editor rejects: %v, loom rejects: %v, structure fixture: %v", editorRejects, validatorRejects, isStructureFixture)
	}
}
