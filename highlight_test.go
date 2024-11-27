package highlight

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/tree-sitter/go-tree-sitter"
)

const (
	ResetStyle = "\x1b[m"

	GreenStyle   = "\x1b[32m"
	BlueStyle    = "\x1b[34m"
	MagentaStyle = "\x1b[35m"
)

var theme = map[string]string{
	"variable":       BlueStyle,
	"keyword":        MagentaStyle,
	"string":         GreenStyle,
	"markup.heading": MagentaStyle,
}

func loadLanguage(name string) *Configuration {
	lib, err := purego.Dlopen(filepath.Join("testdata", "grammars", name+".so"), purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		log.Printf("failed to load %q language: %v", name, err)
		return nil
	}

	var newLanguage func() uintptr
	purego.RegisterLibFunc(&newLanguage, lib, "tree_sitter_"+name)

	language := tree_sitter.NewLanguage(unsafe.Pointer(newLanguage()))

	highlightsQuery, err := os.ReadFile(filepath.Join("testdata", "queries", name, "highlights.scm"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("failed to read %q highlights query: %v", name, err)
		return nil
	}

	injectionsQuery, err := os.ReadFile(filepath.Join("testdata", "queries", name, "injections.scm"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("failed to read %q injections query: %v", name, err)
		return nil
	}

	cfg, err := NewConfiguration(language, name, highlightsQuery, injectionsQuery, nil)
	if err != nil {
		log.Printf("failed to create highlight config: %v", err)
		return nil
	}
	return cfg
}

func TestHighlighter_Highlight(t *testing.T) {
	source, err := os.ReadFile("testdata/test.md")
	if err != nil {
		log.Fatalf("failed to read source file: %v", err)
	}

	cfg := loadLanguage("markdown")
	if cfg == nil {
		t.Fatalf("failed to load markdown language")
	}

	captureNames := make([]string, len(theme))
	for name := range theme {
		captureNames = append(captureNames, name)
	}

	cfg.Configure(captureNames)

	highlighter := New()
	highlights := highlighter.Highlight(context.Background(), *cfg, source, loadLanguage)

	var (
		activeHighlights []Highlight
		usedCaptureNames []string
	)
	for event, err := range highlights {
		if err != nil {
			log.Panicf("failed to highlight source: %v", err)
		}
		switch e := event.(type) {
		case EventStart:
			log.Println("START:", captureNames[e.Highlight])
			activeHighlights = append(activeHighlights, e.Highlight)
		case EventEnd:
			activeHighlights = activeHighlights[:len(activeHighlights)-1]
		case EventSource:
			// var style string
			// if len(activeHighlights) > 0 {
			// 	activeHighlight := activeHighlights[len(activeHighlights)-1]
			// 	captureName := captureNames[activeHighlight]
			// 	usedCaptureNames = append(usedCaptureNames, captureName)
			// 	style = theme[captureName]
			// }
			// renderStyle(style, string(source[e.Start:e.End]))
		}
	}
	t.Logf("used capture names: %v", slices.Compact(usedCaptureNames))
}

func renderStyle(style string, source string) {
	if style == "" {
		print(source)
		return
	}
	print(style + source + ResetStyle)
}
