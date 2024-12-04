package highlight

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/tree-sitter-go/bindings/go"
)

var cssTheme = map[string]string{
	"variable": "color: #DEAE60;",
	"function": "color: #73FBF1;",
	"string":   "color: #B8E466;",
	"keyword":  "color: #A578EA;",
	"comment":  "color: #8A8A8A;",
}

func TestHTMLRender_Render(t *testing.T) {
	captureNames := make([]string, 0, len(theme))
	for name := range theme {
		captureNames = append(captureNames, name)
	}

	source, err := os.ReadFile("testdata/test.go")
	require.NoError(t, err)

	language := tree_sitter.NewLanguage(tree_sitter_go.Language())

	highlightsQuery, err := os.ReadFile("testdata/highlights.scm")
	require.NoError(t, err)

	foldsQuery, err := os.ReadFile("testdata/folds.scm")
	require.NoError(t, err)

	cfg, err := NewConfiguration(language, "go", highlightsQuery, nil, nil, foldsQuery)
	require.NoError(t, err)

	cfg.Configure(captureNames)

	highlighter := New()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	events := highlighter.Highlight(ctx, *cfg, source, func(name string) *Configuration {
		return nil
	})

	f, err := os.Create("out.html")
	require.NoError(t, err)
	defer func() {
		err = f.Close()
		require.NoError(t, err)
	}()

	htmlRender := NewHTMLRender()
	err = htmlRender.RenderDocument(f, events, "test.go", source, captureNames, cssTheme)
	require.NoError(t, err)
}
