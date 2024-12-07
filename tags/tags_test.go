package tags

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func TestTagger_Tags(t *testing.T) {
	source, err := os.ReadFile("../testdata/test.go")
	require.NoError(t, err)

	tagsQuery, err := os.ReadFile("../testdata/go/tags.scm")
	require.NoError(t, err)

	localsQuery, err := os.ReadFile("../testdata/go/locals.scm")
	require.NoError(t, err)

	language := tree_sitter.NewLanguage(tree_sitter_go.Language())
	cfg, err := NewConfiguration(language, tagsQuery, localsQuery)
	require.NoError(t, err)

	tagsContext := New()

	ctx := context.Background()
	tags, _, err := tagsContext.Tags(ctx, *cfg, source)
	require.NoError(t, err)

	for tag, err := range tags {
		require.NoError(t, err)

		var kind string
		if tag.IsDefinition {
			kind = "def"
		} else {
			kind = "ref"
		}

		msg := fmt.Sprintf("%s\t | %s\t%s (%d, %d - %d, %d) `%s`",
			source[tag.NameRange.Start:tag.NameRange.End],
			cfg.SyntaxTypeName(tag.SyntaxTypeID),
			kind,
			tag.Span.Start.Row,
			tag.Span.Start.Column,
			tag.Span.End.Row,
			tag.Span.End.Column,
			source[tag.LineRange.Start:tag.LineRange.End],
		)
		if tag.Docs != "" {
			if len(tag.Docs) > 120 {
				msg += fmt.Sprintf("\t%q...", tag.Docs[:120])
			} else {
				msg += fmt.Sprintf("\t%q", tag.Docs)
			}
		}
		t.Log(msg)
	}
}
