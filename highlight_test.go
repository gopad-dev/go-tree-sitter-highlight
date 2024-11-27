package highlight

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/tree-sitter/go-tree-sitter"
)

const (
	ResetStyle = "\x1b[m"

	WhiteStyle   = "\x1b[37m"
	GreenStyle   = "\x1b[32m"
	CyanStyle    = "\x1b[36m"
	RedStyle     = "\x1b[31m"
	MagentaStyle = "\x1b[35m"
	YellowStyle  = "\x1b[33m"
)

var theme = map[string]string{
	"markup.heading":   MagentaStyle,
	"markup.raw.block": CyanStyle,

	"punctuation.special":   GreenStyle,
	"punctuation.bracket":   WhiteStyle,
	"punctuation.delimiter": WhiteStyle,

	"attribute": YellowStyle,
	"variable":  WhiteStyle,
	"keyword":   MagentaStyle,
	"string":    GreenStyle,
	"property":  RedStyle,
	"function":  CyanStyle,
}

func loadLanguage(name string, captureNames []string) *Configuration {
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

	cfg.Configure(captureNames)
	return cfg
}

func TestHighlighter_Highlight(t *testing.T) {
	captureNames := make([]string, 0, len(theme))
	for name := range theme {
		captureNames = append(captureNames, name)
	}

	source, err := os.ReadFile("testdata/test.md")
	if err != nil {
		log.Fatalf("failed to read source file: %v", err)
	}

	cfg := loadLanguage("markdown", captureNames)
	if cfg == nil {
		t.Fatalf("failed to load markdown language")
	}

	highlighter := New()
	events := highlighter.Highlight(context.Background(), *cfg, source, func(name string) *Configuration {
		//return nil
		return loadLanguage(name, captureNames)
	})

	styles := []string{WhiteStyle}
	for event, err := range events {
		if err != nil {
			log.Panicf("failed to highlight source: %v", err)
		}

		switch e := event.(type) {
		case EventStart:
			fmt.Printf("START: %s", captureNames[e.Highlight])
			styles = append(styles, theme[captureNames[e.Highlight]])
		case EventEnd:
			styles = styles[:len(styles)-1]
		case EventSource:
			style := styles[len(styles)-1]
			print(style)
			print(string(source[e.StartByte:e.EndByte]))
			print(ResetStyle)
		}
	}
}
