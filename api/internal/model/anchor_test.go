package model_test

import (
	"encoding/json"
	"testing"

	"github.com/ipedrazas/idpad/api/internal/model"
)

const sampleDoc = `{
  "type": "doc",
  "content": [
    {"type": "heading", "attrs": {"level": 1},
     "content": [{"type": "text", "text": "Ship it"}]},
    {"type": "paragraph",
     "content": [
       {"type": "text", "text": "We should "},
       {"type": "text", "marks": [{"type": "bold"}], "text": "definitely"},
       {"type": "text", "text": " ship this."}
     ]},
    {"type": "bulletList", "content": [
      {"type": "listItem", "content": [
        {"type": "paragraph", "content": [{"type": "text", "text": "one"}]}]},
      {"type": "listItem", "content": [
        {"type": "paragraph", "content": [{"type": "text", "text": "two"}]}]}
    ]}
  ]
}`

func TestBlockTextsFlattensMarksAndNesting(t *testing.T) {
	blocks, err := model.BlockTexts(json.RawMessage(sampleDoc))
	if err != nil {
		t.Fatalf("BlockTexts: %v", err)
	}
	want := []string{"Ship it", "We should definitely ship this.", "onetwo"}
	if len(blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d: %q", len(blocks), len(want), blocks)
	}
	for i := range want {
		if blocks[i] != want[i] {
			t.Errorf("block %d = %q, want %q", i, blocks[i], want[i])
		}
	}
}

func TestBlockTextsRejectsNonDoc(t *testing.T) {
	for name, body := range map[string]string{
		"empty":       ``,
		"array":       `[]`,
		"wrong type":  `{"type":"paragraph"}`,
		"not json":    `{nope}`,
		"null string": `null`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := model.BlockTexts(json.RawMessage(body)); err == nil {
				t.Fatalf("expected an error for %q", body)
			}
		})
	}
}

func TestAnchorMatches(t *testing.T) {
	blocks := []string{"Ship it", "We should definitely ship this."}

	cases := []struct {
		name   string
		anchor model.Anchor
		want   bool
	}{
		{"exact", model.Anchor{BlockIndex: 1, StartOffset: 10, EndOffset: 20, Snippet: "definitely"}, true},
		{"whole block", model.Anchor{BlockIndex: 0, StartOffset: 0, EndOffset: 7, Snippet: "Ship it"}, true},
		{"shifted text", model.Anchor{BlockIndex: 1, StartOffset: 9, EndOffset: 19, Snippet: "definitely"}, false},
		{"block gone", model.Anchor{BlockIndex: 7, StartOffset: 0, EndOffset: 4, Snippet: "Ship"}, false},
		{"past end", model.Anchor{BlockIndex: 0, StartOffset: 0, EndOffset: 99, Snippet: "Ship it"}, false},
		{"inverted", model.Anchor{BlockIndex: 0, StartOffset: 5, EndOffset: 2, Snippet: "Ship"}, false},
		{"empty snippet", model.Anchor{BlockIndex: 0, StartOffset: 0, EndOffset: 0}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.anchor.Matches(blocks); got != tc.want {
				t.Errorf("Matches() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Offsets are UTF-16 code units so that Go agrees with the browser about
// where a selection starts and ends when the text contains astral characters.
func TestAnchorOffsetsAreUTF16CodeUnits(t *testing.T) {
	blocks := []string{"🚀 launch"} // the rocket is two UTF-16 code units

	matching := model.Anchor{BlockIndex: 0, StartOffset: 3, EndOffset: 9, Snippet: "launch"}
	if !matching.Matches(blocks) {
		t.Error("expected UTF-16 offsets to select \"launch\"")
	}

	runeBased := model.Anchor{BlockIndex: 0, StartOffset: 2, EndOffset: 8, Snippet: "launch"}
	if runeBased.Matches(blocks) {
		t.Error("rune-based offsets should not match; offsets are UTF-16 code units")
	}
}

func TestAnchorValidate(t *testing.T) {
	blocks := []string{"Ship it"}

	if err := (model.Anchor{BlockIndex: 0, StartOffset: 0, EndOffset: 4, Snippet: "Ship"}).Validate(blocks); err != nil {
		t.Fatalf("valid anchor rejected: %v", err)
	}

	bad := map[string]model.Anchor{
		"empty snippet":    {BlockIndex: 0, StartOffset: 0, EndOffset: 4},
		"negative block":   {BlockIndex: -1, StartOffset: 0, EndOffset: 4, Snippet: "Ship"},
		"block overflow":   {BlockIndex: 3, StartOffset: 0, EndOffset: 4, Snippet: "Ship"},
		"negative start":   {BlockIndex: 0, StartOffset: -1, EndOffset: 4, Snippet: "Ship"},
		"empty range":      {BlockIndex: 0, StartOffset: 2, EndOffset: 2, Snippet: "Ship"},
		"range overflow":   {BlockIndex: 0, StartOffset: 0, EndOffset: 99, Snippet: "Ship"},
		"snippet mismatch": {BlockIndex: 0, StartOffset: 0, EndOffset: 4, Snippet: "Sail"},
	}
	for name, a := range bad {
		t.Run(name, func(t *testing.T) {
			if err := a.Validate(blocks); err == nil {
				t.Error("expected a validation error")
			}
		})
	}
}
