package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf16"
)

// ErrInvalidDoc is returned when a body is not a TipTap/ProseMirror document.
var ErrInvalidDoc = errors.New("body must be a ProseMirror document with type \"doc\"")

// docNode is the subset of the TipTap JSON shape the anchor math needs.
// Everything else in the document (marks, attrs, custom nodes) is preserved
// because the full JSON is stored verbatim; this type is only used to walk it.
type docNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []docNode `json:"content"`
}

// EmptyDoc is the body assigned to an idea created without one.
func EmptyDoc() json.RawMessage {
	return json.RawMessage(`{"type":"doc","content":[]}`)
}

// BlockTexts flattens a TipTap document into one plain-text string per
// top-level block, concatenating the text of every descendant text node in
// document order. Anchor offsets are expressed against these strings.
//
// Offsets count UTF-16 code units, not runes or bytes: the browser produces
// them from DOM/JS string indices, so matching that unit exactly is what keeps
// the Go validation and the editor decorations in agreement for text outside
// the BMP.
func BlockTexts(body json.RawMessage) ([]string, error) {
	if len(body) == 0 {
		return nil, ErrInvalidDoc
	}

	var doc docNode
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidDoc, err)
	}
	if doc.Type != "doc" {
		return nil, ErrInvalidDoc
	}

	blocks := make([]string, 0, len(doc.Content))
	for _, block := range doc.Content {
		blocks = append(blocks, flattenText(block))
	}
	return blocks, nil
}

// flattenText concatenates the text of node and all of its descendants.
func flattenText(node docNode) string {
	if node.Type == "text" {
		return node.Text
	}
	var out string
	for _, child := range node.Content {
		out += flattenText(child)
	}
	return out
}

// SliceUTF16 returns the substring of s between the given UTF-16 code unit
// offsets. ok is false when the range falls outside s or is inverted.
func SliceUTF16(s string, start, end int) (string, bool) {
	if start < 0 || end < start {
		return "", false
	}
	units := utf16.Encode([]rune(s))
	if end > len(units) {
		return "", false
	}
	return string(utf16.Decode(units[start:end])), true
}

// LenUTF16 returns the length of s in UTF-16 code units.
func LenUTF16(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// Matches reports whether the anchor still points at its snippet within the
// given flattened blocks. A false result means the anchored text was edited
// away and the thread should be shown as detached.
func (a Anchor) Matches(blocks []string) bool {
	if a.Snippet == "" {
		return false
	}
	if a.BlockIndex < 0 || a.BlockIndex >= len(blocks) {
		return false
	}
	got, ok := SliceUTF16(blocks[a.BlockIndex], a.StartOffset, a.EndOffset)
	if !ok {
		return false
	}
	return got == a.Snippet
}

// Validate checks that an anchor is internally consistent and that it selects
// exactly its snippet in the supplied body. It is the check applied when a
// thread is created, so a thread never starts life detached.
func (a Anchor) Validate(blocks []string) error {
	if a.Snippet == "" {
		return errors.New("anchor snippet must not be empty")
	}
	if a.BlockIndex < 0 {
		return errors.New("anchor block_index must not be negative")
	}
	if a.BlockIndex >= len(blocks) {
		return fmt.Errorf("anchor block_index %d is out of range for a body with %d blocks", a.BlockIndex, len(blocks))
	}
	if a.StartOffset < 0 {
		return errors.New("anchor start_offset must not be negative")
	}
	if a.EndOffset <= a.StartOffset {
		return errors.New("anchor end_offset must be greater than start_offset")
	}
	got, ok := SliceUTF16(blocks[a.BlockIndex], a.StartOffset, a.EndOffset)
	if !ok {
		return fmt.Errorf("anchor range [%d,%d) is out of range for block %d", a.StartOffset, a.EndOffset, a.BlockIndex)
	}
	if got != a.Snippet {
		return fmt.Errorf("anchor snippet %q does not match the text at [%d,%d) of block %d", a.Snippet, a.StartOffset, a.EndOffset, a.BlockIndex)
	}
	return nil
}
