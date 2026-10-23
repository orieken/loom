package intake

import (
	"fmt"

	"github.com/orieken/loom/internal/injectionscan"
)

// Check is everything a pickup decision needs from one file's contents:
// parse, validate the order, and scan an external order's body. It takes
// bytes rather than a path so the caller owns the filesystem.
func Check(source []byte, context Context) []Violation {
	document, err := Parse(source)
	if err != nil {
		return violation("frontmatter", "%v", err)
	}
	violations := Validate(document.Order, context)
	return append(violations, scanBody(document)...)
}

// scanBody runs the deterministic injection scan over an external order's
// spec. The scan catches naive injection only (see injectionscan), so a clean
// result is permission to read the order, not proof that it is safe.
func scanBody(document Document) []Violation {
	if !document.Order.IsExternal() {
		return nil
	}
	var violations []Violation
	for _, finding := range injectionscan.Scan(document.Body) {
		line := document.BodyLine + finding.Line - 1
		violations = append(violations, Violation{
			Field:   fmt.Sprintf("body:%d", line),
			Message: fmt.Sprintf("%s: %s", finding.Rule, finding.Excerpt),
		})
	}
	return violations
}
