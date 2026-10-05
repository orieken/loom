package repograph

import "strings"

// The strippers below let other packages read source the way the extractors
// do — comments and docstrings removed — without a second copy of the rules.

// StripJSComments removes comments and multi-line template literals from
// TypeScript or JavaScript source.
func StripJSComments(source string) string {
	source = jsBlockComment.ReplaceAllString(source, "")
	source = jsLineComment.ReplaceAllString(source, "$1")
	return jsTemplateLiteral.ReplaceAllStringFunc(source, blankMultilineTemplate)
}

// StripJavaComments removes comments from Java source.
func StripJavaComments(source string) string { return stripJavaComments(source) }

// StripPythonNoise removes docstrings, triple-quoted strings and comments from
// Python source.
func StripPythonNoise(source string) string {
	source = pyTripleDouble.ReplaceAllString(source, "")
	source = pyTripleSingle.ReplaceAllString(source, "")
	return pyComment.ReplaceAllString(source, "")
}

// IsTestFile reports whether a path follows its language's test-file
// convention. It is one rule, shared by the fact sheets.
func IsTestFile(file string) bool {
	base := file[strings.LastIndex(file, "/")+1:]
	for _, convention := range testConventions {
		if strings.HasSuffix(base, convention.suffix) {
			return convention.isTest(file, base)
		}
	}
	return isJSTest(file, base)
}

// testConventions is ordered: the more specific suffix must come first.
var testConventions = []struct {
	suffix string
	isTest func(file, base string) bool
}{
	{"_test.go", func(string, string) bool { return true }},
	{".go", func(string, string) bool { return false }},
	{".py", func(_, base string) bool {
		return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
	}},
	{".java", func(file, base string) bool {
		return strings.Contains(file, "src/test/") || hasAnySuffix(base, "Test.java", "Tests.java", "IT.java")
	}},
}

func hasAnySuffix(text string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(text, suffix) {
			return true
		}
	}
	return false
}

func isJSTest(file, base string) bool {
	if !hasExtension(base, tsExtensions...) {
		return false
	}
	return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.Contains(file, "__tests__/")
}
