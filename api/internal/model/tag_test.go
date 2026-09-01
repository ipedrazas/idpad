package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ipedrazas/idpad/api/internal/model"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "Machine Learning", "machine-learning"},
		{"already a slug", "machine-learning", "machine-learning"},
		{"collapses separator runs", "machine   ---  learning", "machine-learning"},
		{"trims separators", "  --machine learning--  ", "machine-learning"},
		{"strips accents", "Máchine Léarning", "machine-learning"},
		{"keeps digits", "web 3.0", "web-3-0"},
		{"drops punctuation", "c++/rust!", "c-rust"},
		{"keeps non-latin letters", "Идея", "идея"},
		{"nothing usable", "  --  ", ""},
		{"empty", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, model.Slugify(tc.in))
		})
	}
}

// Different spellings of one concept must fold onto the same slug, which is
// what makes the tag table a shared vocabulary rather than a list of strings.
func TestSlugifyFoldsVariantsTogether(t *testing.T) {
	variants := []string{"Machine Learning", "machine learning", "MACHINE-LEARNING", " machine  learning "}
	for _, v := range variants {
		require.Equal(t, "machine-learning", model.Slugify(v), "variant %q", v)
	}
}

func TestNormaliseTagName(t *testing.T) {
	require.Equal(t, "Machine Learning", model.NormaliseTagName("  Machine   Learning  "))
	require.Equal(t, "", model.NormaliseTagName("   "))
}

func TestRelationInverse(t *testing.T) {
	require.Equal(t, model.Relation("referenced_by"), model.RelationReferences.Inverse())
	require.Equal(t, model.Relation("expanded_by"), model.RelationExpands.Inverse())
	// Symmetric relations are their own inverse, so both ends read alike.
	require.Equal(t, model.RelationSimilar, model.RelationSimilar.Inverse())
	require.Equal(t, model.RelationRelated, model.RelationRelated.Inverse())
}

func TestRelationValid(t *testing.T) {
	for _, r := range model.Relations {
		require.True(t, r.Valid(), "relation %q", r)
	}
	require.False(t, model.Relation("referenced_by").Valid(), "an inverse is a rendering, not a storable relation")
	require.False(t, model.Relation("").Valid())
}
