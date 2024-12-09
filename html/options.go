package html

import (
	"fmt"
	"html"

	"go.gopad.dev/go-tree-sitter-highlight/tags"
)

// Theme is used to style the rendered code, line numbers & symbols.
type Theme struct {
	TabSize                        int
	Color                          string
	BackgroundColor                string
	LinesColor                     string
	LinesBackgroundColor           string
	SelectedLineBackgroundColor    string
	HighlightBackgroundColor       string
	SymbolKindBackgroundColor      string
	SymbolKindHoverBackgroundColor string
	CodeStyles                     map[string]string
}

func defaultOptions() Options {
	return Options{
		ClassNamePrefix: "hl-",
		IDPrefix:        "code-",
		ShowLineNumbers: true,
		ShowSymbols:     true,
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
	DebugTags       bool
	// TagIDCallback is a callback function that returns a unique HTML id for a tag. If nil, a default implementation is used.
	TagIDCallback   TagIDCallback
	CodeStyleSymbol map[string]string
	SymbolKindNames map[string]string
}

// TagIDCallback is a callback function that returns a unique HTML id for a tag.
type TagIDCallback func(tag tags.Tag, source []byte, syntaxNames []string) string

func defaultTagIDCallback(tag tags.Tag, source []byte, syntaxNames []string) string {
	return fmt.Sprintf("%s-%s", html.EscapeString(syntaxNames[tag.SyntaxTypeID]), html.EscapeString(tag.FullName(source)))
}
