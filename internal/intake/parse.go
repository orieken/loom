package intake

import (
	"errors"
	"fmt"
	"io"
	"strings"

	yaml "go.yaml.in/yaml/v4"
)

// ErrNoFrontmatter reports a file that does not open with a `---` fence, so
// there is no work order to read.
var ErrNoFrontmatter = errors.New("no frontmatter: an intake file must start with a --- line")

// ErrUnclosedFrontmatter reports an opening fence with no closing one, which
// would otherwise swallow the whole spec into the order.
var ErrUnclosedFrontmatter = errors.New("frontmatter is never closed by a --- line")

const fence = "---"

// Parse splits an intake file into its order and its spec body. Unknown keys
// are rejected: a misspelt `prority` silently dropped is a priority the author
// believes they set and nobody reads.
func Parse(source []byte) (Document, error) {
	frontmatter, body, bodyLine, err := splitFrontmatter(string(source))
	if err != nil {
		return Document{}, err
	}
	order, err := decodeOrder(frontmatter)
	if err != nil {
		return Document{}, err
	}
	return Document{Order: order, Body: body, BodyLine: bodyLine}, nil
}

func decodeOrder(frontmatter string) (Order, error) {
	decoder := yaml.NewDecoder(strings.NewReader(frontmatter))
	decoder.KnownFields(true)
	var order Order
	err := decoder.Decode(&order)
	if errors.Is(err, io.EOF) {
		return Order{}, nil // empty frontmatter: every required field is reported missing
	}
	if err != nil {
		return Order{}, fmt.Errorf("frontmatter: %w", err)
	}
	return order, nil
}

// splitFrontmatter returns the text between the opening and closing fences,
// everything after the closing one, and the file line the body starts on.
func splitFrontmatter(text string) (string, string, int, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if strings.TrimSpace(lines[0]) != fence {
		return "", "", 0, ErrNoFrontmatter
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == fence {
			frontmatter := strings.Join(lines[1:index], "\n")
			return frontmatter, strings.Join(lines[index+1:], "\n"), index + 2, nil
		}
	}
	return "", "", 0, ErrUnclosedFrontmatter
}
