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

const ResetStyle = "\x1b[0m"

func ColorStyle(color int) string {
	if color == -1 {
		return ""
	}
	return fmt.Sprintf("\x1b[38;5;%dm", color)
}

var theme = map[string]int{
	"markup.heading":        14,
	"markup.raw.block":      14,
	"punctuation.bracket":   7,
	"punctuation.special":   7,
	"punctuation.delimiter": 15,
	"variable":              15,
	"function":              14,
	"string":                10,
	"attribute":             124,
	"keyword":               13,
	"comment":               245,
	"property":              9,
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
		return loadLanguage(name, captureNames)
	})

	var (
		styles    []int
		languages []string
	)
	for event, err := range events {
		if err != nil {
			log.Panicf("failed to highlight source: %v", err)
		}

		switch e := event.(type) {
		case EventLayerStart:
			styles = append(styles, 15)
			languages = append(languages, e.LanguageName)
		case EventLayerEnd:
			styles = styles[:len(styles)-1]
			languages = languages[:len(languages)-1]
		case EventCaptureStart:
			styles = append(styles, theme[captureNames[e.Highlight]])
		case EventCaptureEnd:
			styles = styles[:len(styles)-1]
		case EventSource:
			style := styles[len(styles)-1]
			print(ColorStyle(style))
			print(string(source[e.StartByte:e.EndByte]))
			print(ResetStyle)
		}
	}
}
