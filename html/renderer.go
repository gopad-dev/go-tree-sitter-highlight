package html

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"iter"
	"slices"
	"unicode/utf8"

	"github.com/tree-sitter/go-tree-sitter"

	"go.gopad.dev/go-tree-sitter-highlight/folds"
	"go.gopad.dev/go-tree-sitter-highlight/highlight"
	"go.gopad.dev/go-tree-sitter-highlight/internal/teeiter"
	"go.gopad.dev/go-tree-sitter-highlight/tags"
)

// AttributeCallback is a callback function that returns the html element attributes for a highlight span.
// This can be anything from classes, ids, or inline styles.
type AttributeCallback func(h highlight.Highlight, languageName string) string

type Theme struct {
	TabSize                     int
	BackgroundColor             string
	Color                       string
	LineNumbersBackgroundColor  string
	LineNumberColor             string
	SelectedLineBackgroundColor string
	HighlightBackgroundColor    string
}

// NewRenderer returns a new Renderer.
func NewRenderer() *Renderer {
	return &Renderer{
		ClassNamePrefix: "hl-",
		IDPrefix:        "code-",
		LineNumbers:     true,
		Symbols:         true,
		Theme: Theme{
			TabSize:                     4,
			BackgroundColor:             "#212122",
			Color:                       "#f8f8f2",
			LineNumbersBackgroundColor:  "#2b2b2b",
			LineNumberColor:             "#8b8b8b",
			SelectedLineBackgroundColor: "#43494a",
			HighlightBackgroundColor:    "#FFEF9C",
		},
		DebugTags: false,
	}
}

// Renderer is a renderer that outputs HTML.
type Renderer struct {
	ClassNamePrefix string
	IDPrefix        string
	LineNumbers     bool
	Symbols         bool
	Theme           Theme
	DebugTags       bool
}

func (r *Renderer) addText(w io.Writer, line int, source []byte, hs []highlight.Highlight, languages []string, callback AttributeCallback) error {
	for len(source) > 0 {
		c, l := utf8.DecodeRune(source)
		source = source[l:]

		if c == utf8.RuneError || c == '\r' {
			continue
		}

		if c == '\n' {
			for range len(hs) - 1 {
				if err := r.endHighlight(w); err != nil {
					return err
				}
			}

			if _, err := w.Write([]byte("</span>")); err != nil {
				return err
			}

			if _, err := w.Write([]byte(string(c))); err != nil {
				return err
			}

			line++

			if _, err := fmt.Fprintf(w, "<span id=\"L%d\" class=\"%sl\">", line, r.ClassNamePrefix); err != nil {
				return err
			}

			nextLanguage, closeLanguage := iter.Pull(slices.Values(languages))
			defer closeLanguage()

			languageName, _ := nextLanguage()
			for i, h := range hs {
				if i == 0 {
					continue
				}
				if err := r.startHighlight(w, h, languageName, callback); err != nil {
					return err
				}
				if h == highlight.DefaultHighlight {
					languageName, _ = nextLanguage()
				}
			}

			continue
		}

		if _, err := w.Write([]byte(string(c))); err != nil {
			return err
		}
	}

	return nil
}

func (r *Renderer) startHighlight(w io.Writer, h highlight.Highlight, languageName string, callback AttributeCallback) error {
	if _, err := fmt.Fprintf(w, "<span"); err != nil {
		return err
	}

	var attributes string
	if callback != nil {
		attributes = callback(h, languageName)
	}

	if len(attributes) > 0 {
		if _, err := w.Write([]byte(" ")); err != nil {
			return err
		}
		if _, err := w.Write([]byte(attributes)); err != nil {
			return err
		}
	}

	_, err := w.Write([]byte(">"))
	return err
}

func (r *Renderer) endHighlight(w io.Writer) error {
	_, err := w.Write([]byte("</span>"))
	return err
}

type Fold struct {
	Range tree_sitter.Range
}

// Render renders the code to the writer with spans for each highlight capture.
// The [AttributeCallback] is used to generate the classes or inline styles for each span.
func (r *Renderer) Render(w io.Writer, events iter.Seq2[highlight.Event, error], allTags iter.Seq2[tags.Tag, error], allFolds iter.Seq2[folds.Fold, error], source []byte, callback AttributeCallback) error {
	nextTag, closeTag := iter.Pull2(allTags)
	defer closeTag()

	currentTag, err, currentTagOk := nextTag()
	if err != nil {
		return fmt.Errorf("error while rendering: %w", err)
	}

	var (
		highlights []highlight.Highlight
		languages  []string
		line       int
	)

	line++
	if _, err := fmt.Fprintf(w, "<span id=\"L%d\" class=\"%sl\">", line, r.ClassNamePrefix); err != nil {
		return err
	}

	defer func() {
		_, _ = w.Write([]byte("</span>"))
	}()

	for event, err := range events {
		if err != nil {
			return fmt.Errorf("error while rendering: %w", err)
		}

		switch e := event.(type) {
		case highlight.EventLayerStart:
			highlights = append(highlights, highlight.DefaultHighlight)
			languages = append(languages, e.LanguageName)
		case highlight.EventLayerEnd:
			highlights = highlights[:len(highlights)-1]
			languages = languages[:len(languages)-1]
		case highlight.EventCaptureStart:
			highlights = append(highlights, e.Highlight)
			language := languages[len(languages)-1]
			if err = r.startHighlight(w, e.Highlight, language, callback); err != nil {
				return fmt.Errorf("error while starting highlight: %w", err)
			}
		case highlight.EventCaptureEnd:
			highlights = highlights[:len(highlights)-1]
			if err = r.endHighlight(w); err != nil {
				return fmt.Errorf("error while ending highlight: %w", err)
			}
		case highlight.EventSource:
			var text []byte
			for i := e.StartByte; i < e.EndByte; i++ {
				if i > currentTag.NameRange.Start {
					for {
						currentTag, err, currentTagOk = nextTag()
						if err != nil {
							return err
						}
						if !currentTagOk {
							break
						}
						if e.StartByte <= currentTag.NameRange.Start {
							break
						}
					}
				}

				if i == currentTag.NameRange.Start {
					name := html.EscapeString(currentTag.Name(source))
					renderName := name
					if r.DebugTags {
						if currentTag.IsDefinition {
							renderName = "#" + name
						} else {
							renderName = "@" + name
						}
					}

					if currentTag.IsDefinition {
						name = fmt.Sprintf("<a id=\"%s%s\" class=\"%sdef\" href=\"#%s%s\">%s</a>", r.IDPrefix, name, r.ClassNamePrefix, r.IDPrefix, name, renderName)
					} else {
						name = fmt.Sprintf("<a class=\"%sref\" href=\"#%s%s\">%s</a>", r.ClassNamePrefix, r.IDPrefix, name, renderName)
					}
					text = append(text, []byte(name)...)
					i = currentTag.NameRange.End - 1
					continue
				}
				text = append(text, []byte(html.EscapeString(string(source[i])))...)
			}

			if err = r.addText(w, line, text, highlights, languages, callback); err != nil {
				return fmt.Errorf("error while writing source: %w", err)
			}

			line += bytes.Count(source[e.StartByte:e.EndByte], []byte("\n"))
		}
	}

	return nil
}

// RenderCSS renders the css classes for a theme to the writer.
func (r *Renderer) RenderCSS(w io.Writer, theme map[string]string) error {
	if r.LineNumbers {
		if _, err := fmt.Fprintf(w, ".%shl{background-color:%s;color:%s;tab-size:%d;}\n"+
			".%slns {float: left;padding-left:0.5rem;padding-right: 0.5rem;background-color:%s;color:%s;}\n"+
			".%scode {display:inline-block;}\n"+
			".%scode summary {list-style-type: none;}\n"+
			".%scode summary::before {content:\"🞂\";position:absolute;}\n"+
			".%scode details[open] summary::before{content:\"🞃\";}\n"+
			".%ssymbols {float:right;width:10rem;}\n"+
			".%ssymbols li {list-style-type: none;}\n"+
			".%ssymbols li a {color:unset;text-decoration:none;}\n"+
			".%sref{color:unset;text-decoration:none;}\n"+
			".%sref:hover {text-decoration:underline;}\n"+
			".%sdef{color:unset;text-decoration:none;}\n"+
			".%sdef:hover {text-decoration:underline;}\n"+
			".%sdef:target{background-color:%s}\n"+
			".%sln {display:inline-block;text-align:right;text-decoration:none;user-select:none;color:unset;}\n"+
			".%sln:hover {text-decoration:underline;}\n"+
			".%sln:focus {outline: none;}\n"+
			".%sl {display:inline-block;width:100%%;padding-left:0.5rem;padding-right:0.5rem;}\n"+
			".%sl:before {content:\"\\200b\";user-select:none;}\n"+
			".%sl:target {background-color:%s;}\n",
			r.ClassNamePrefix, r.Theme.BackgroundColor, r.Theme.Color, r.Theme.TabSize,
			r.ClassNamePrefix, r.Theme.LineNumbersBackgroundColor, r.Theme.LineNumberColor,
			r.ClassNamePrefix, r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix, r.Theme.HighlightBackgroundColor,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix, r.Theme.SelectedLineBackgroundColor,
		); err != nil {
			return err
		}
	}

	for name, style := range theme {
		_, err := fmt.Fprintf(w, ".%s%s {%s}\n", r.ClassNamePrefix, name, style)
		if err != nil {
			return err
		}
	}

	return nil
}

// RenderLineNumbers renders the line numbers to the writer. The lineCount is the number of lines in the source.
func (r *Renderer) RenderLineNumbers(w io.Writer, lineCount int) error {
	if _, err := fmt.Fprintf(w, "<div class=\"%slns\">", r.ClassNamePrefix); err != nil {
		return err
	}

	for i := range lineCount {
		if _, err := fmt.Fprintf(w, "<a class=\"%sln\" href=\"#L%d\">%d</a>\n", r.ClassNamePrefix, i+1, i+1); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(w, "</div>")
	return err
}

func (r *Renderer) themeAttributeCallback(captureNames []string) AttributeCallback {
	return func(h highlight.Highlight, languageName string) string {
		if h == highlight.DefaultHighlight {
			return ""
		}

		return fmt.Sprintf(`class="%s%s"`, r.ClassNamePrefix, captureNames[h])
	}

}

func (r *Renderer) RenderSymbols(w io.Writer, allTags iter.Seq2[tags.Tag, error], source []byte, syntaxTypeNames []string) ([]tags.Tag, error) {
	var symbols []tags.Tag

	if _, err := fmt.Fprintf(w, "<ul class=\"%ssymbols\">\n", r.ClassNamePrefix); err != nil {
		return nil, err
	}

	for tag, err := range allTags {
		if err != nil {
			return nil, err
		}

		symbols = append(symbols, tag)

		if !tag.IsDefinition {
			continue
		}

		fmt.Fprintf(w, "<li>")

		if tag.Docs != "" {
			fmt.Fprintf(w, "<details><summary>")
		}

		fmt.Fprintf(w, "<a href=\"#%s%s\"><span>%s</span> <span>%s</span></a>", r.IDPrefix, tag.Name(source), syntaxTypeNames[tag.SyntaxTypeID], tag.FullName(source))

		if tag.Docs != "" {
			fmt.Fprintf(w, "</summary>%s</details>", tag.Docs)
		}

		if _, err = fmt.Fprintf(w, "<li>\n"); err != nil {
			return nil, err
		}
	}

	if _, err := fmt.Fprintf(w, "</ul>\n"); err != nil {
		return nil, err
	}

	return symbols, nil
}

// RenderDocument renders a full HTML document with the code and theme embedded.
func (r *Renderer) RenderDocument(w io.Writer, events iter.Seq2[highlight.Event, error], allTags iter.Seq2[tags.Tag, error], allFolds iter.Seq2[folds.Fold, error], title string, source []byte, captureNames []string, syntaxTypeNames []string, theme map[string]string) error {
	if _, err := fmt.Fprintf(w, "<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<title>%s</title>\n<style>\n", html.EscapeString(title)); err != nil {
		return err
	}

	if err := r.RenderCSS(w, theme); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "</style>\n</head>\n<body>\n<div class=\"%shl\">", r.ClassNamePrefix); err != nil {
		return err
	}

	if r.LineNumbers {
		if err := r.RenderLineNumbers(w, bytes.Count(source, []byte("\n"))+1); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "<pre><code class=\"%scode\">", r.ClassNamePrefix); err != nil {
		return err
	}

	renderAllTags := allTags
	var symbolsAllTags iter.Seq2[tags.Tag, error]
	if r.Symbols {
		renderAllTags, symbolsAllTags = teeiter.New(allTags)
	}

	if err := r.Render(w, events, renderAllTags, allFolds, source, r.themeAttributeCallback(captureNames)); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "</code></pre>"); err != nil {
		return err
	}

	if r.Symbols {
		if _, err := r.RenderSymbols(w, symbolsAllTags, source, syntaxTypeNames); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "</div>\n</body>\n</html>\n"); err != nil {
		return err
	}

	return nil
}
