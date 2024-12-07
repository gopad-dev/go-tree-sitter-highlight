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
	"go.gopad.dev/go-tree-sitter-highlight/tags"
)

var (
	escapeAmpersand   = []byte("&amp;")
	escapeSingle      = []byte("&#39;")
	escapeLessThan    = []byte("&lt;")
	escapeGreaterThan = []byte("&gt;")
	escapeDouble      = []byte("&#34;")
)

// AttributeCallback is a callback function that returns the html element attributes for a highlight span.
// This can be anything from classes, ids, or inline styles.
type AttributeCallback func(h highlight.Highlight, languageName string) string

type Theme struct {
	TabWidth                    int
	BackgroundColor             string
	Color                       string
	LineNumbersBackgroundColor  string
	LineNumberColor             string
	SelectedLineBackgroundColor string
}

// NewRenderer returns a new Renderer.
func NewRenderer() *Renderer {
	return &Renderer{
		ClassNamePrefix: "hl-",
		LineNumbers:     true,
		Theme: Theme{
			BackgroundColor:             "#212122",
			Color:                       "#f8f8f2",
			LineNumbersBackgroundColor:  "#2b2b2b",
			LineNumberColor:             "#8b8b8b",
			SelectedLineBackgroundColor: "#43494a",
		},
	}
}

// Renderer is a renderer that outputs HTML.
type Renderer struct {
	ClassNamePrefix string
	LineNumbers     bool
	Theme           Theme
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

		var b []byte
		switch c {
		case '&':
			b = escapeAmpersand
		case '\'':
			b = escapeSingle
		case '<':
			b = escapeLessThan
		case '>':
			b = escapeGreaterThan
		case '"':
			b = escapeDouble
		default:
			b = []byte(string(c))
		}

		if _, err := w.Write(b); err != nil {
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
func (r *Renderer) Render(w io.Writer, events iter.Seq2[highlight.Event, error], source []byte, callback AttributeCallback) error {
	var (
		highlights []highlight.Highlight
		languages  []string
		line       int
		folds      []Fold
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
		// case EventFoldStart:
		// 	if _, err = fmt.Fprintf(w, "<details open><summary>"); err != nil {
		// 		return fmt.Errorf("error while starting fold: %w", err)
		// 	}
		// 	folds = append(folds, Fold{Range: e.Range})
		// case EventFoldEnd:
		// 	if _, err = fmt.Fprintf(w, "</details>"); err != nil {
		// 		return fmt.Errorf("error while ending fold: %w", err)
		// 	}
		// 	folds = folds[:len(folds)-1]
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
			text := source[e.StartByte:e.EndByte]
			if len(folds) > 0 {
				fold := folds[len(folds)-1]
				foldText := source[fold.Range.StartByte:fold.Range.EndByte]

				index := bytes.Index(foldText, []byte("\n"))
				if index > -1 {
					firstLineByte := fold.Range.StartByte + uint(index)
					if firstLineByte >= e.StartByte && firstLineByte < e.EndByte {
						if err = r.addText(w, line, text[:index], highlights, languages, callback); err != nil {
							return fmt.Errorf("error while writing source: %w", err)
						}
						if _, err = w.Write([]byte("</summary>")); err != nil {
							return fmt.Errorf("error while writing source: %w", err)
						}
						text = text[index:]
						line += 1
					}
				}
			}

			if err = r.addText(w, line, text, highlights, languages, callback); err != nil {
				return fmt.Errorf("error while writing source: %w", err)
			}

			line += bytes.Count(text, []byte("\n"))
		}
	}

	return nil
}

// RenderCSS renders the css classes for a theme to the writer.
func (r *Renderer) RenderCSS(w io.Writer, theme map[string]string) error {
	if r.LineNumbers {
		if _, err := fmt.Fprintf(w, ".%shl{background-color:%s;color:%s;tab-width:%d;}\n"+
			".%slns {float: left;padding-left:0.5rem;padding-right: 0.5rem;background-color:%s;color:%s;}\n"+
			".%scode {display:inline-block;}\n"+
			".%scode summary {list-style-type: none;}\n"+
			".%scode summary::before {content:\"🞂\";position:absolute;}\n"+
			".%scode details[open] summary::before{content:\"🞃\";}\n"+
			".%sln {display:inline-block;text-align:right;text-decoration:none;user-select:none;color:unset;}\n"+
			".%sln:hover {text-decoration:underline;}\n"+
			".%sln:focus {outline: none;}\n"+
			".%sl {display:inline-block;width:100%%;padding-left:0.5rem;padding-right:0.5rem;}\n"+
			".%sl:before {content:\"\\200b\";user-select:none;}\n"+
			".%sl:target {background-color:%s;}\n",
			r.ClassNamePrefix, r.Theme.BackgroundColor, r.Theme.Color, r.Theme.TabWidth,
			r.ClassNamePrefix, r.Theme.LineNumbersBackgroundColor, r.Theme.LineNumberColor,
			r.ClassNamePrefix, r.ClassNamePrefix,
			r.ClassNamePrefix,
			r.ClassNamePrefix,
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

// RenderDocument renders a full HTML document with the code and theme embedded.
func (r *Renderer) RenderDocument(w io.Writer, events iter.Seq2[highlight.Event, error], allTags iter.Seq2[tags.Tag, error], allFolds iter.Seq2[folds.Fold, error], title string, source []byte, captureNames []string, theme map[string]string) error {
	if _, err := fmt.Fprintf(w, "<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<title>%s</title>\n<style>\n", html.EscapeString(title)); err != nil {
		return err
	}

	if err := r.RenderCSS(w, theme); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "</style>\n</head>\n<body>\n<pre class=\"%shl\">", r.ClassNamePrefix); err != nil {
		return err
	}

	if r.LineNumbers {
		if err := r.RenderLineNumbers(w, bytes.Count(source, []byte("\n"))+1); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "<code class=\"%scode\">", r.ClassNamePrefix); err != nil {
		return err
	}

	if err := r.Render(w, events, source, r.themeAttributeCallback(captureNames)); err != nil {
		return err
	}

	_, err := fmt.Fprintf(w, "</code></pre>\n</body>\n</html>\n")
	return err
}
