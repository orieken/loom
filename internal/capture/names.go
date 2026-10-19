package capture

import "strings"

// UnsafeNamePlaceholder replaces an argument key or tool name that fails the
// name check. A name can carry data, so an odd name is never stored.
const UnsafeNamePlaceholder = "?"

// IsSafeKeyName reports whether an argument key may be stored. The rule is
// [A-Za-z_][A-Za-z0-9_]{0,63}, written as a loop so no regular expression
// compiles at start-up.
func IsSafeKeyName(name string) bool {
	return isSafeName(name, "")
}

// IsSafeToolName reports whether a tool name may be stored. It also allows
// '-', '.' and ':' because tool names of extensions use them.
func IsSafeToolName(name string) bool {
	return isSafeName(name, "-.:")
}

// SafeToolName returns the tool name, or the placeholder when unsafe.
func SafeToolName(name string) string {
	if IsSafeToolName(name) {
		return name
	}
	return UnsafeNamePlaceholder
}

func isSafeName(name, extra string) bool {
	if name == "" || len(name) > MaxNameBytes {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isNameByte(name[i], i == 0, extra) {
			return false
		}
	}
	return true
}

func isNameByte(b byte, isFirst bool, extra string) bool {
	if isLetterByte(b) {
		return true
	}
	if isFirst {
		return false
	}
	return b >= '0' && b <= '9' || strings.IndexByte(extra, b) >= 0
}

func isLetterByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_'
}
