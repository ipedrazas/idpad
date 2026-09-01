package model

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// MaxTagLen mirrors the varchar(60) columns on tags.
const MaxTagLen = 60

// Slugify folds a tag name to its identity: lower case, accents stripped, and
// every run of non-alphanumeric characters collapsed to a single hyphen. It is
// what makes "Machine Learning", "machine-learning" and "Máchine  Learning"
// the same tag rather than three.
//
// An empty result means the name carried no usable characters; callers reject
// it rather than storing a tag nothing can be looked up by.
func Slugify(name string) string {
	// NFD splits an accented rune into base + combining marks so the marks can
	// be dropped, leaving the ASCII-ish base letter behind.
	folded, _, err := transform.String(
		transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC),
		name,
	)
	if err != nil {
		folded = name
	}

	var b strings.Builder
	b.Grow(len(folded))
	pendingHyphen := false
	for _, r := range strings.ToLower(folded) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// The hyphen is only emitted once a following character earns it,
			// so leading and trailing separators never survive.
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
		default:
			pendingHyphen = true
		}
	}
	return b.String()
}

// NormaliseTagName trims a tag name and collapses its internal whitespace, so
// the stored display name matches what the slug was derived from.
func NormaliseTagName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}
