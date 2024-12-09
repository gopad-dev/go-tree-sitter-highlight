package html

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"io"
	"iter"
	"slices"
	"strings"
	"text/template"
	"unicode/utf8"

	"go.gopad.dev/go-tree-sitter-highlight/folds"
	"go.gopad.dev/go-tree-sitter-highlight/highlight"
	"go.gopad.dev/go-tree-sitter-highlight/tags"
)

//go:embed templates/css.tmpl
var css string

var similarSyntaxTypeNames = map[string][]string{
	"call": {"function", "method"},
	"type": {"class", "interface"},
}

// AttributeCallback is a callback function that returns the html element attributes for a highlight span.
// This can be anything from classes, ids, or inline styles.
type AttributeCallback func(h highlight.Highlight, languageName string) string

// NewRenderer returns a new Renderer.
func NewRenderer(options *Options) *Renderer {
	var opts Options
	if options == nil {
		opts = defaultOptions()
	} else {
		opts = *options
	}

	if opts.TagIDCallback == nil {
		opts.TagIDCallback = defaultTagIDCallback
	}

	cssTmpl := template.Must(template.New("css").Parse(css))

	return &Renderer{
		cssTmpl: cssTmpl,
		Options: opts,
	}
}

// Renderer is a renderer that outputs HTML.
type Renderer struct {
	cssTmpl *template.Template
	Options Options
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

			if _, err := fmt.Fprintf(w, "<span id=\"L%d\" class=\"%sl\">", line, r.Options.ClassNamePrefix); err != nil {
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

func (r *Renderer) themeAttributeCallback(captureNames []string) AttributeCallback {
	return func(h highlight.Highlight, languageName string) string {
		if h == highlight.DefaultHighlight {
			return ""
		}

		return fmt.Sprintf(`class="%s%s"`, r.Options.ClassNamePrefix, captureNames[h])
	}
}

func findDefForRef(ref tags.Tag, allTags []tags.Tag, source []byte, syntaxTypeNames []string) *tags.Tag {
	for _, tag := range allTags {
		if !tag.IsDefinition {
			continue
		}

		if tag.Name(source) == ref.Name(source) {
			if ref.SyntaxTypeID == 0 || tag.SyntaxTypeID == ref.SyntaxTypeID {
				return &tag
			}

			syntaxName := syntaxTypeNames[ref.SyntaxTypeID]
			names, ok := similarSyntaxTypeNames[syntaxName]
			if ok && slices.Contains(names, syntaxTypeNames[tag.SyntaxTypeID]) {
				return &tag
			}
		}
	}

	return nil
}

// RenderCSS renders the css classes for a theme to the writer.
func (r *Renderer) RenderCSS(w io.Writer, theme Theme) error {
	var cssData = struct {
		ClassNamePrefix string
		IDPrefix        string
		Theme           Theme
	}{
		ClassNamePrefix: r.Options.ClassNamePrefix,
		IDPrefix:        r.Options.IDPrefix,
		Theme:           theme,
	}
	if err := r.cssTmpl.Execute(w, cssData); err != nil {
		return err
	}

	for name, style := range theme.CodeStyles {
		if _, err := fmt.Fprintf(w, ".%s%s {%s}\n", r.Options.ClassNamePrefix, name, style); err != nil {
			return err
		}
	}

	return nil
}

// RenderLineNumbers renders the line numbers to the writer. The lineCount is the number of lines in the source.
func (r *Renderer) RenderLineNumbers(w io.Writer, lineCount int) error {
	if _, err := fmt.Fprintf(w, "<div class=\"%slns\">", r.Options.ClassNamePrefix); err != nil {
		return err
	}

	for i := range lineCount {
		if _, err := fmt.Fprintf(w, "<a class=\"%sln\" href=\"#L%d\">%d</a>\n", r.Options.ClassNamePrefix, i+1, i+1); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(w, "</div>")
	return err
}

// Render renders the code to the writer with spans for each highlight capture.
// The [AttributeCallback] is used to generate the classes or inline styles for each span.
func (r *Renderer) Render(w io.Writer, events iter.Seq2[highlight.Event, error], allTags []tags.Tag, allFolds iter.Seq2[folds.Fold, error], source []byte, syntaxTypeNames []string, callback AttributeCallback) error {
	nextTag, closeTag := iter.Pull(slices.Values(allTags))
	defer closeTag()

	currentTag, currentTagOk := nextTag()

	var (
		highlights []highlight.Highlight
		languages  []string
		line       int
	)

	line++
	if _, err := fmt.Fprintf(w, "<span id=\"L%d\" class=\"%sl\">", line, r.Options.ClassNamePrefix); err != nil {
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
						currentTag, currentTagOk = nextTag()
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

					i = currentTag.NameRange.End - 1

					if currentTag.IsDefinition {
						renderName := name
						if r.Options.DebugTags {
							renderName = "#" + name
						}
						tagID := r.Options.TagIDCallback(currentTag, source, syntaxTypeNames)
						name = fmt.Sprintf("<a id=\"%s%s\" class=\"%sdef\" href=\"#%s%s\">%s</a>", r.Options.IDPrefix, tagID, r.Options.ClassNamePrefix, r.Options.IDPrefix, tagID, renderName)
						text = append(text, []byte(name)...)
						continue
					} else {
						renderName := name

						def := findDefForRef(currentTag, allTags, source, syntaxTypeNames)
						if r.Options.DebugTags {
							renderName = "@" + name
							if def == nil {
								renderName = "!" + renderName
								def = &currentTag
							}
						}
						if def != nil {
							tagID := r.Options.TagIDCallback(*def, source, syntaxTypeNames)
							name = fmt.Sprintf("<a class=\"%sref\" href=\"#%s%s\">%s</a>", r.Options.ClassNamePrefix, r.Options.IDPrefix, tagID, renderName)
						}
					}
					text = append(text, []byte(name)...)
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

// RenderSymbols renders the symbols list to the writer.
func (r *Renderer) RenderSymbols(w io.Writer, allTags []tags.Tag, source []byte, syntaxTypeNames []string, theme Theme) error {
	if _, err := fmt.Fprintf(w, "<ul class=\"%ssymbols\">\n", r.Options.ClassNamePrefix); err != nil {
		return err
	}

	for _, tag := range allTags {
		symbolName := syntaxTypeNames[tag.SyntaxTypeID]

		if !tag.IsDefinition {
			continue
		}

		globalSymbol, ok := r.Options.GlobalSymbols[symbolName]
		if !ok {
			continue
		}

		for {
			_, ok = theme.CodeStyles[globalSymbol]
			if ok {
				break
			}

			i := strings.Index(globalSymbol, ".")
			if i == -1 {
				break
			}

			globalSymbol = globalSymbol[:i]
		}

		if _, err := fmt.Fprintf(w, "<li>"); err != nil {
			return err
		}

		if tag.Docs != "" {
			if _, err := fmt.Fprintf(w, "<details name=\"symbol\"><summary>"); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(w, "<span class=\"%s%s %ssymbol-kind\">%s</span> ", r.Options.ClassNamePrefix, globalSymbol, r.Options.ClassNamePrefix, symbolName); err != nil {
			return err
		}

		if _, err := fmt.Fprintf(w, "<a href=\"#%s%s\">%s</a>", r.Options.IDPrefix, r.Options.TagIDCallback(tag, source, syntaxTypeNames), tag.FullName(source)); err != nil {
			return err
		}

		if tag.Docs != "" {
			if _, err := fmt.Fprintf(w, "</summary><p>%s</p></details>", tag.Docs); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(w, "</li>\n"); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "</ul>\n"); err != nil {
		return err
	}

	return nil
}

// RenderDocument renders a full HTML document with the code and theme embedded.
func (r *Renderer) RenderDocument(w io.Writer, events iter.Seq2[highlight.Event, error], tagsIter iter.Seq2[tags.Tag, error], foldsIter iter.Seq2[folds.Fold, error], title string, source []byte, captureNames []string, syntaxTypeNames []string, theme Theme) error {
	var allTags []tags.Tag
	for tag, err := range tagsIter {
		if err != nil {
			return err
		}
		allTags = append(allTags, tag)
	}

	if _, err := fmt.Fprintf(w, "<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<title>%s</title>\n<style>\n", html.EscapeString(title)); err != nil {
		return err
	}

	if err := r.RenderCSS(w, theme); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "</style>\n</head>\n<body>\n<pre class=\"%shl\">", r.Options.ClassNamePrefix); err != nil {
		return err
	}

	if r.Options.ShowLineNumbers {
		if err := r.RenderLineNumbers(w, bytes.Count(source, []byte("\n"))+1); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "<code class=\"%scode\">", r.Options.ClassNamePrefix); err != nil {
		return err
	}

	if err := r.Render(w, events, allTags, foldsIter, source, syntaxTypeNames, r.themeAttributeCallback(captureNames)); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "</code>"); err != nil {
		return err
	}

	if r.Options.ShowSymbols {
		if err := r.RenderSymbols(w, allTags, source, syntaxTypeNames, theme); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "</pre>\n</body>\n</html>\n"); err != nil {
		return err
	}

	return nil
}
