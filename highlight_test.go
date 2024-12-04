package highlight

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// Minimal theme for testing
var theme = map[string]int{
	"variable": 15,
	"function": 14,
	"string":   10,
	"keyword":  13,
	"comment":  245,
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

func TestHighlighter_Highlight(t *testing.T) {
	// Get the capture names from the theme
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

	var styles []int
	for event, err := range events {
		require.NoError(t, err)

		switch e := event.(type) {
		// New language layer found, push a new style (white) so we don't inherit the previous style as fallback
		case EventLayerStart:
			styles = append(styles, 15)
			fmt.Printf("((")
		// End of language layer, pop the style
		case EventLayerEnd:
			styles = styles[:len(styles)-1]
			fmt.Printf("))")
		case EventFoldStart:
			fmt.Printf("[%d,%d-%d,%d[", e.Range.StartPoint.Row, e.Range.StartPoint.Column, e.Range.EndPoint.Row, e.Range.EndPoint.Column)
		case EventFoldEnd:
			fmt.Print("]]")
		// Start of a capture, push the style
		case EventCaptureStart:
			styles = append(styles, theme[captureNames[e.Highlight]])
		// End of a capture, pop the style
		case EventCaptureEnd:
			styles = styles[:len(styles)-1]
		// Source code event, print the source code with the current style.
		case EventSource:
			//log.Printf("source: %d,%d", e.StartByte, e.EndByte)
			// Get the current style, there should always be at least one style
			style := styles[len(styles)-1]
			// print the style
			print(colorStyle(style))
			// print the source code
			print(string(source[e.StartByte:e.EndByte]))
			print("|")
			// reset the style
			print(resetStyle)
		}
	}
}
