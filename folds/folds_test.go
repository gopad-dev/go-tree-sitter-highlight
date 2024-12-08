package folds

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func TestFolder_Folds(t *testing.T) {
	source, err := os.ReadFile("../testdata/test.go")
	require.NoError(t, err)

	foldsQuery, err := os.ReadFile("../testdata/go/folds.scm")
	require.NoError(t, err)

	language := tree_sitter.NewLanguage(tree_sitter_go.Language())
	cfg, err := NewConfiguration(language, foldsQuery)
	require.NoError(t, err)

	foldsContext := New()

	ctx := context.Background()
	folds, err := foldsContext.Folds(ctx, *cfg, source)
	require.NoError(t, err)

	for fold, err := range folds {
		require.NoError(t, err)

		t.Logf("fold: %v", fold)
	}
}
