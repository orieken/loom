package patternspec

// Severity says whether an issue fails validation or only warns.
type Severity string

// The severities.
const (
	SeverityFailure Severity = "failure"
	SeverityWarning Severity = "warning"
)

// rootField is the field of an issue about the file as a whole.
const rootField = "(spec)"

// Issue is one problem with one pattern spec. Its message names fields,
// step ids and allowed values. It never repeats a value the spec holds,
// apart from the names the name rule compares.
type Issue struct {
	File     string
	Field    string
	Message  string
	Severity Severity
}

// IsFailure reports whether the issue fails validation.
func (issue Issue) IsFailure() bool { return issue.Severity == SeverityFailure }

// String renders "file: field: message".
func (issue Issue) String() string {
	return issue.File + ": " + issue.Field + ": " + issue.Message
}

func failure(field, message string) Issue {
	return Issue{Field: field, Message: message, Severity: SeverityFailure}
}

func warning(field, message string) Issue {
	return Issue{Field: field, Message: message, Severity: SeverityWarning}
}

// inFile stamps every issue with the file it belongs to.
func inFile(file string, issues []Issue) []Issue {
	stamped := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		issue.File = file
		stamped = append(stamped, issue)
	}
	return stamped
}

// failureCount counts the issues that fail validation.
func failureCount(issues []Issue) int {
	count := 0
	for _, issue := range issues {
		if issue.IsFailure() {
			count++
		}
	}
	return count
}
