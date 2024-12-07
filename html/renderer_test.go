package html

import (
	"context"
	"os"
	"testing"

	"github.com/alexaandru/go-sitter-forest/comment"
	"github.com/stretchr/testify/require"
	"github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/tree-sitter-go/bindings/go"

	"go.gopad.dev/go-tree-sitter-highlight/folds"
	"go.gopad.dev/go-tree-sitter-highlight/highlight"
	"go.gopad.dev/go-tree-sitter-highlight/tags"
)

var cssTheme = map[string]string{
	"variable":     "color: #DEAE60;",
	"function":     "color: #73FBF1;",
	"string":       "color: #B8E466;",
	"keyword":      "color: #A578EA;",
	"comment":      "color: #8A8A8A;",
	"comment.todo": "color: #B8E466;",
}

func loadInjection(t *testing.T, captureNames []string) highlight.InjectionCallback {
	return func(languageName string) *highlight.Configuration {
		switch languageName {
		case "comment":
			highlightsQuery, err := os.ReadFile("../testdata/comment/highlights.scm")
			require.NoError(t, err)

			commentLang := tree_sitter.NewLanguage(comment.GetLanguage())
			cfg, err := highlight.NewConfiguration(commentLang, languageName, highlightsQuery, nil, nil)
			require.NoError(t, err)

			cfg.Configure(captureNames)

			return cfg
		}

		return nil
	}
}

func TestRenderer_Render(t *testing.T) {
	captureNames := make([]string, 0, len(cssTheme))
	for name := range cssTheme {
		captureNames = append(captureNames, name)
	}

	source, err := os.ReadFile("../testdata/test.go")
	require.NoError(t, err)

	highlightsQuery, err := os.ReadFile("../testdata/go/highlights.scm")
	require.NoError(t, err)

	injectionsQuery, err := os.ReadFile("../testdata/go/injections.scm")
	require.NoError(t, err)

	localsQuery, err := os.ReadFile("../testdata/go/locals.scm")
	require.NoError(t, err)

	tagsQuery, err := os.ReadFile("../testdata/go/tags.scm")
	require.NoError(t, err)

	foldsQuery, err := os.ReadFile("../testdata/go/folds.scm")
	require.NoError(t, err)

	language := tree_sitter.NewLanguage(tree_sitter_go.Language())

	highlightCfg, err := highlight.NewConfiguration(language, "go", highlightsQuery, injectionsQuery, localsQuery)
	require.NoError(t, err)
	highlightCfg.Configure(captureNames)

	tagsCfg, err := tags.NewConfiguration(language, tagsQuery, localsQuery)
	require.NoError(t, err)

	foldsCfg, err := folds.NewConfiguration(language, foldsQuery)
	require.NoError(t, err)

	ctx := context.Background()

	highlighter := highlight.New()
	events, err := highlighter.Highlight(ctx, *highlightCfg, source, loadInjection(t, captureNames))
	require.NoError(t, err)

	tagsContext := tags.New()
	allTags, _, err := tagsContext.Tags(ctx, *tagsCfg, source)
	require.NoError(t, err)

	foldsContext := folds.New()
	allFolds, err := foldsContext.Folds(ctx, *foldsCfg, source)

	f, err := os.Create("out.html")
	require.NoError(t, err)
	defer func() {
		err = f.Close()
		require.NoError(t, err)
	}()

	renderer := NewRenderer()
	err = renderer.RenderDocument(f, events, allTags, allFolds, "test.go", source, captureNames, cssTheme)
	require.NoError(t, err)
}
