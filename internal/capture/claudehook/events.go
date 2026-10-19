package claudehook

// RegisteredEvents lists the hook events that `loom install --capture`
// registers, in registration order. The names are UNVERIFIED (see the
// package comment). The installer and the health check read them from here,
// so a corrected name is a change in payload.go only.
func RegisteredEvents() []string {
	return []string{hookPreToolUse, hookPostToolUse, hookUserPromptSubmit, hookStop}
}
