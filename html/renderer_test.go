package html

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alexaandru/go-sitter-forest/comment"
	"github.com/stretchr/testify/require"
	"github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/tree-sitter-go/bindings/go"

	"go.gopad.dev/go-tree-sitter-highlight/highlight"
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
		case "go":
			highlightsQuery, err := os.ReadFile("../testdata/go/highlights.scm")
			require.NoError(t, err)

			injectionsQuery, err := os.ReadFile("../testdata/go/injections.scm")
			require.NoError(t, err)

			language := tree_sitter.NewLanguage(tree_sitter_go.Language())
			cfg, err := highlight.NewConfiguration(language, languageName, highlightsQuery, injectionsQuery, nil)
			require.NoError(t, err)

			cfg.Configure(captureNames)

			return cfg
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

	cfg := loadInjection(t, captureNames)("go")

	highlighter := highlight.New()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	events, err := highlighter.Highlight(ctx, *cfg, source, loadInjection(t, captureNames))
	require.NoError(t, err)

	f, err := os.Create("out.html")
	require.NoError(t, err)
	defer func() {
		err = f.Close()
		require.NoError(t, err)
	}()

	renderer := NewRenderer()
	err = renderer.RenderDocument(f, events, "test.go", source, captureNames, cssTheme)
	require.NoError(t, err)
}
