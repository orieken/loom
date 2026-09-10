package state

// Path claims: which fields of a state document name files, so the executor
// can check whether the files are there (roadmap L2.24).
//
// Typed validation checks the SHAPE of a claim and never whether it is true.
// The second real end-to-end run's qa-engineer returned
// `testFilesCreated: ["internal/server/server_test.go"]` with
// `testResults: {passed: 3}` and 100% coverage. The file did not exist and no
// test had run. The payload was schema-valid, so the stage completed, the run
// cleared the confirm-security gate, and two more stages ran on top of it.
//
// A path either exists or it does not. That is the cheapest true thing the
// executor can check about a claim, and nothing was checking it.

// PathClaims is implemented by a state document that names files it wrote.
// Returning them as labelled groups lets a failure say which field lied
// rather than only which path is missing.
type PathClaims interface {
	// PathClaims returns each path-naming field and what it holds.
	PathClaims() []PathClaim
}

// PathClaim is one field of a document and the paths it names.
type PathClaim struct {
	// Field is the JSON name, so a failure names what the agent wrote.
	Field string
	Paths []string
	// MustExist is false for a field that may legitimately name something
	// absent — a deleted file, for instance. Nothing uses that yet, and the
	// flag exists so a future field can opt out explicitly rather than by
	// being forgotten.
	MustExist bool
}

// PathClaims reports the files an implementation says it wrote.
func (i ImplementationState) PathClaims() []PathClaim {
	return []PathClaim{
		{Field: "filesCreated", Paths: i.FilesCreated, MustExist: true},
		{Field: "filesModified", Paths: i.FilesModified, MustExist: true},
	}
}

// PathClaims reports the test files QA says it wrote. This is the field the
// second real run fabricated.
func (q QAState) PathClaims() []PathClaim {
	return []PathClaim{
		{Field: "testFilesCreated", Paths: q.TestFilesCreated, MustExist: true},
		{Field: "testFilesModified", Paths: q.TestFilesModified, MustExist: true},
	}
}

// PathClaims reports the files a security review says it touched.
func (s SecurityState) PathClaims() []PathClaim {
	return []PathClaim{
		{Field: "filesModified", Paths: s.FilesModified, MustExist: true},
	}
}
