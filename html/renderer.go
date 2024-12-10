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
	"call": {"function", "method", "variable"},
	"type": {"class", "interface", "struct"},
}

func escapeClassName(className string) string {
	return strings.ReplaceAll(className, ".", "-")
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

func (r *Renderer) addText(w io.Writer, line uint, source []byte, hs []highlight.Highlight, languages []string, allFolds []folds.Fold, callback AttributeCallback) error {
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

			var skipNewLine bool
			//if slices.ContainsFunc(allFolds, func(f folds.Fold) bool {
			//	return f.LineRange.StartPoint.Row == line
			//}) {
			//	fmt.Fprintf(w, "</summary>")
			//	skipNewLine = true
			//}

			if slices.ContainsFunc(allFolds, func(f folds.Fold) bool {
				return f.LineRange.EndPoint.Row == line
			}) {
				fmt.Fprintf(w, "</details>")
				skipNewLine = true
			}

			if !skipNewLine {
				if _, err := w.Write([]byte(string(c))); err != nil {
					return err
				}
			}

			line++

			if slices.ContainsFunc(allFolds, func(f folds.Fold) bool {
				return f.LineRange.StartPoint.Row == line-1
			}) {
				//fmt.Fprintf(w, "<details open><summary>")
				fmt.Fprintf(w, "<details open><summary></summary>")
			}

			if _, err := fmt.Fprintf(w, "<a href=\"#L%d\" class=\"%sln\">%d</a><span id=\"L%d\" class=\"%sl\">", line+1, r.Options.ClassNamePrefix, line+1, line+1, r.Options.ClassNamePrefix); err != nil {
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

		return fmt.Sprintf(`class="%s%s"`, r.Options.ClassNamePrefix, escapeClassName(captureNames[h]))
	}
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
		if _, err := fmt.Fprintf(w, ".%s%s {%s}\n", r.Options.ClassNamePrefix, escapeClassName(name), style); err != nil {
			return err
		}
	}

	return nil
}

// Render renders the code to the writer with spans for each highlight capture.
// The [AttributeCallback] is used to generate the classes or inline styles for each span.
func (r *Renderer) Render(w io.Writer, events iter.Seq2[highlight.Event, error], allTags []ResolvedTag, allFolds []folds.Fold, source []byte, syntaxTypeNames []string, callback AttributeCallback) error {
	nextTag, closeTag := iter.Pull(slices.Values(allTags))
	defer closeTag()

	currentTag, currentTagOk := nextTag()

	var (
		highlights []highlight.Highlight
		languages  []string
		line       uint
	)

	if _, err := fmt.Fprintf(w, "<a href=\"#L%d\" class=\"%sln\">%d</a><span id=\"L%d\" class=\"%sl\">", line+1, r.Options.ClassNamePrefix, line+1, line+1, r.Options.ClassNamePrefix); err != nil {
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
				if i > currentTag.Tag.NameRange.Start {
					for {
						currentTag, currentTagOk = nextTag()
						if !currentTagOk {
							break
						}
						if e.StartByte <= currentTag.Tag.NameRange.Start {
							break
						}
					}
				}

				if i == currentTag.Tag.NameRange.Start {
					i = currentTag.Tag.NameRange.End - 1

					name := html.EscapeString(currentTag.Tag.Name(source))
					if currentTag.Tag.IsDefinition {
						renderName := name
						if r.Options.DebugTags {
							renderName = "#" + name
						}
						name = fmt.Sprintf("<a id=\"%s%s\" class=\"%sdef\" href=\"#%s%s\" title=\"%s\">%s</a>", r.Options.IDPrefix, currentTag.ID, r.Options.ClassNamePrefix, r.Options.IDPrefix, currentTag.ID, currentTag.ID, renderName)
						text = append(text, []byte(name)...)
						continue
					} else {
						renderName := name
						if r.Options.DebugTags {
							renderName = "@" + name
							if currentTag.Def == nil {
								renderName = "!" + renderName
							}
						}
						name = fmt.Sprintf("<a class=\"%sref\" href=\"#%s%s\" title=\"%s\">%s</a>", r.Options.ClassNamePrefix, r.Options.IDPrefix, currentTag.ID, currentTag.ID, renderName)
					}
					text = append(text, []byte(name)...)
					continue
				}
				text = append(text, []byte(html.EscapeString(string(source[i])))...)
			}

			if err = r.addText(w, line, text, highlights, languages, allFolds, callback); err != nil {
				return fmt.Errorf("error while writing source: %w", err)
			}

			line += uint(bytes.Count(source[e.StartByte:e.EndByte], []byte("\n")))
		}
	}

	return nil
}

// RenderSymbols renders the symbols list to the writer.
func (r *Renderer) RenderSymbols(w io.Writer, resolvedTags []ResolvedTag, source []byte, syntaxTypeNames []string, theme Theme) error {
	if _, err := fmt.Fprintf(w, "<ul class=\"%ssymbols\">\n", r.Options.ClassNamePrefix); err != nil {
		return err
	}

	for _, tag := range resolvedTags {
		symbolName := syntaxTypeNames[tag.Tag.SyntaxTypeID]

		if !tag.Tag.IsDefinition {
			continue
		}

		globalSymbol, ok := r.Options.CodeStyleSymbol[symbolName]
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

		if tag.Tag.Docs != "" {
			if _, err := fmt.Fprintf(w, "<details name=\"symbol\"><summary>"); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(w, "<span class=\"%s%s %ssymbol-kind\">%s</span> ", r.Options.ClassNamePrefix, globalSymbol, r.Options.ClassNamePrefix, r.Options.SymbolKindNames[symbolName]); err != nil {
			return err
		}

		if _, err := fmt.Fprintf(w, "<a href=\"#%s%s\">%s</a>", r.Options.IDPrefix, tag.ID, tag.Tag.FullName(source)); err != nil {
			return err
		}

		if tag.Tag.Docs != "" {
			if _, err := fmt.Fprintf(w, "</summary><p>%s</p></details>", tag.Tag.Docs); err != nil {
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
	resolvedTags, err := r.ResolveRefs(tagsIter, source, syntaxTypeNames)
	if err != nil {
		return err
	}

	var allFolds []folds.Fold
	for fold, err := range foldsIter {
		if err != nil {
			return err
		}
		allFolds = append(allFolds, fold)
	}

	if _, err = fmt.Fprintf(w, "<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<title>%s</title>\n<style>\n", html.EscapeString(title)); err != nil {
		return err
	}

	if err = r.RenderCSS(w, theme); err != nil {
		return err
	}

	if _, err = fmt.Fprintf(w, "</style>\n</head>\n<body>\n<pre class=\"%shl\"><code class=\"%scode\">", r.Options.ClassNamePrefix, r.Options.ClassNamePrefix); err != nil {
		return err
	}

	if err = r.Render(w, events, resolvedTags, allFolds, source, syntaxTypeNames, r.themeAttributeCallback(captureNames)); err != nil {
		return err
	}

	if _, err = fmt.Fprintf(w, "</code>"); err != nil {
		return err
	}

	if r.Options.ShowSymbols {
		if err = r.RenderSymbols(w, resolvedTags, source, syntaxTypeNames, theme); err != nil {
			return err
		}
	}

	if _, err = fmt.Fprintf(w, "</pre>\n</body>\n</html>\n"); err != nil {
		return err
	}

	return nil
}
