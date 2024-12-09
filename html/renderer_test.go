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

var theme = Theme{
	TabSize:                        4,
	BackgroundColor:                "#212122",
	Color:                          "#f8f8f2",
	LinesBackgroundColor:           "#2b2b2b",
	LinesColor:                     "#8b8b8b",
	SelectedLineBackgroundColor:    "#43494a",
	HighlightBackgroundColor:       "#FFEF9C",
	SymbolKindBackgroundColor:      "#363535",
	SymbolKindHoverBackgroundColor: "#43494a",
	CodeStyles: map[string]string{
		"variable":        "color: #f8f8f2;",
		"function":        "color: #73FBF1;",
		"method":          "color: #73FBF1;",
		"string":          "color: #B8E466;",
		"type":            "color: #DEC560;",
		"keyword":         "color: #A578EA;",
		"comment":         "color: #8A8A8A;",
		"comment.todo":    "color: #B8E466;",
		"variable.member": "color: #d9112f;",
	},
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
	captureNames := make([]string, 0, len(theme.CodeStyles))
	for name := range theme.CodeStyles {
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

	renderer := NewRenderer(nil)
	renderer.Options.DebugTags = true
	err = renderer.RenderDocument(f, events, allTags, allFolds, "test.go", source, captureNames, tagsCfg.SyntaxTypeNames(), theme)
	require.NoError(t, err)
}
