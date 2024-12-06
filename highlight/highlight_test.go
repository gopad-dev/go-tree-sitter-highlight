package highlight

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/alexaandru/go-sitter-forest/comment"
	"github.com/stretchr/testify/require"
	"github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// Minimal theme for testing
var theme = map[string]int{
	"variable":     15,
	"function":     14,
	"string":       10,
	"keyword":      13,
	"comment":      245,
	"comment.todo": 10,
}

// resetStyle resets the terminal color
const resetStyle = "\x1b[0m"

// colorStyle returns the ANSI escape sequence for the given ANSI color code
func colorStyle(color int) string {
	if color == -1 {
		return ""
	}
	return fmt.Sprintf("\x1b[38;5;%dm", color)
}

func loadInjection(t *testing.T, captureNames []string) InjectionCallback {
	return func(languageName string) *Configuration {
		switch languageName {
		case "go":
			highlightsQuery, err := os.ReadFile("../testdata/go/highlights.scm")
			require.NoError(t, err)

			injectionsQuery, err := os.ReadFile("../testdata/go/injections.scm")
			require.NoError(t, err)

			language := tree_sitter.NewLanguage(tree_sitter_go.Language())
			cfg, err := NewConfiguration(language, languageName, highlightsQuery, injectionsQuery, nil)
			require.NoError(t, err)

			cfg.Configure(captureNames)

			return cfg
		case "comment":
			highlightsQuery, err := os.ReadFile("../testdata/comment/highlights.scm")
			require.NoError(t, err)

			commentLang := tree_sitter.NewLanguage(comment.GetLanguage())
			cfg, err := NewConfiguration(commentLang, languageName, highlightsQuery, nil, nil)
			require.NoError(t, err)

			cfg.Configure(captureNames)

			return cfg
		}

		return nil
	}
}

func TestHighlighter_Highlight(t *testing.T) {
	// Get the capture names from the theme
	captureNames := make([]string, 0, len(theme))
	for name := range theme {
		captureNames = append(captureNames, name)
	}

	source, err := os.ReadFile("../testdata/test.go")
	require.NoError(t, err)

	cfg := loadInjection(t, captureNames)("go")

	highlighter := New()

	ctx := context.Background()
	events, err := highlighter.Highlight(ctx, *cfg, source, loadInjection(t, captureNames))
	require.NoError(t, err)

	var styles []int
	for event, err := range events {
		require.NoError(t, err)

		switch e := event.(type) {
		// New language layer found, push a new style (white) so we don't inherit the previous style as fallback
		case EventLayerStart:
			styles = append(styles, 15)
		// End of language layer, pop the style
		case EventLayerEnd:
			styles = styles[:len(styles)-1]
		// Start of a capture, push the style
		case EventCaptureStart:
			styles = append(styles, theme[captureNames[e.Highlight]])
		// End of a capture, pop the style
		case EventCaptureEnd:
			styles = styles[:len(styles)-1]
		// Source code event, print the source code with the current style.
		case EventSource:
			// Get the current style, there should always be at least one style
			style := styles[len(styles)-1]
			// print the style
			print(colorStyle(style))
			// print the source code
			print(string(source[e.StartByte:e.EndByte]))
			// reset the style
			print(resetStyle)
		}
	}
}
