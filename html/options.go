package html

import (
	"fmt"
	"html"

	"go.gopad.dev/go-tree-sitter-highlight/highlight"
	"go.gopad.dev/go-tree-sitter-highlight/tags"
)

/*
# black
palette = 0=#0D0D0D
palette = 8=#6D7070

# red
palette = 1=#FF301B
palette = 9=#FF4352

# green
palette = 2=#A0E521
palette = 10=#B8E466

# yellow
palette = 3=#FFC620
palette = 11=#FFD750

# blue
palette = 4=#1BA6FA
palette = 12=#1BA6FA

# purple
palette = 5=#8763B8
palette = 13=#A578EA

# aqua
palette = 6=#21DEEF
palette = 14=#73FBF1

# white
palette = 7=#EBEBEB
palette = 15=#FEFEF8
*/

func DefaultTheme() Theme {
	return Theme{
		TabSize: 4,
		Text0:   "#f8f8f2",
		Text1:   "#8A8A8A",

		Background0: "#212122",
		Background1: "#2b2b2b",
		Background2: "#43494a",
		Background3: "#363535",

		HighlightColor: "#534500",
		CodeStyles: map[string]string{
			"variable":              "color: #f8f8f2;",
			"variable.other.member": "color: #FF4352;",
			"function":              "color: #73FBF1;",
			"method":                "color: #73FBF1;",
			"string":                "color: #B8E466;",
			"type":                  "color: #FFD750;",
			"keyword":               "color: #A578EA;",
			"comment":               "color: #6D7070;",
			"comment.todo":          "color: #FEFEF8;",
		},
	}
}

// Theme is used to style the rendered code, line numbers & symbols.
type Theme struct {
	TabSize int

	Text0 string
	Text1 string

	Background0 string
	Background1 string
	Background2 string
	Background3 string

	HighlightColor string

	CodeStyles map[string]string
}

func defaultOptions() Options {
	return Options{
		ClassNamePrefix: "hl-",
		IDPrefix:        "code-",
		ShowLineNumbers: true,
		ShowSymbols:     true,
		SymbolsTitle:    "Symbols",
		DebugTags:       false,
		TagIDCallback:   defaultTagIDCallback,
		CodeStyleSymbol: map[string]string{
			"class":     "type",
			"struct":    "type",
			"function":  "function",
			"interface": "type",
			"method":    "function.method",
		},
		SymbolKindNames: map[string]string{
			"class":     "class",
			"struct":    "struct",
			"function":  "func",
			"interface": "interface",
			"method":    "method",
		},
	}
}

// Options are the options for the Renderer.
type Options struct {
	ClassNamePrefix string
	IDPrefix        string
	ShowLineNumbers bool
	ShowSymbols     bool
	SymbolsTitle    string
	DebugTags       bool
	// TagIDCallback is a callback function that returns a unique HTML id for a tag. If nil, a default implementation is used.
	TagIDCallback     TagIDCallback
	AttributeCallback AttributeCallback
	CodeStyleSymbol   map[string]string
	SymbolKindNames   map[string]string
}

// TagIDCallback is a callback function that returns a unique HTML id for a tag.
type TagIDCallback func(tag tags.Tag, source []byte, syntaxNames []string) string

func defaultTagIDCallback(tag tags.Tag, source []byte, syntaxNames []string) string {
	return fmt.Sprintf("%s-%s", html.EscapeString(syntaxNames[tag.SyntaxTypeID]), html.EscapeString(tag.FullName(source)))
}

// AttributeCallback is a callback function that returns the html element attributes for a highlight span.
// This can be anything from classes, ids, or inline styles.
type AttributeCallback func(h highlight.Highlight, languageName string, classNamePrefix string, captureNames []string) string

func defaultThemeAttributeCallback(h highlight.Highlight, languageName string, classNamePrefix string, captureNames []string) string {
	if h == highlight.DefaultHighlight {
		return ""
	}

	return fmt.Sprintf(`class="%s%s"`, classNamePrefix, escapeClassName(captureNames[h]))
}
