package tags

import (
	"bytes"
	"context"
	"iter"

	"github.com/tree-sitter/go-tree-sitter"

	"go.gopad.dev/go-tree-sitter-highlight/internal/peekiter"
)

func newIgnoredTag(nameRange byteRange, scopeRange *byteRange) Tag {
	return Tag{
		Range: byteRange{
			Start: ^uint(0),
			End:   ^uint(0),
		},
		NameRange:  nameRange,
		ScopeRange: scopeRange,
		LineRange: byteRange{
			Start: 0,
			End:   0,
		},
		Span: pointRange{
			Start: tree_sitter.Point{
				Row:    0,
				Column: 0,
			},
			End: tree_sitter.Point{
				Row:    0,
				Column: 0,
			},
		},
		UTF16ColumnRange: byteRange{},
		Docs:             "",
		IsDefinition:     false,
		SyntaxTypeID:     0,
	}
}

type Tag struct {
	Range            byteRange
	NameRange        byteRange
	ScopeRange       *byteRange
	LineRange        byteRange
	LineRow          uint
	Span             pointRange
	UTF16ColumnRange byteRange
	Docs             string
	IsDefinition     bool
	SyntaxTypeID     uint
}

func (t Tag) IsIgnored() bool {
	return t.Range.Start == ^uint(0)
}

func (t Tag) Name(source []byte) string {
	return string(source[t.NameRange.Start:t.NameRange.End])
}

func (t Tag) ScopeName(source []byte) string {
	if t.ScopeRange == nil {
		return ""
	}
	return string(source[t.ScopeRange.Start:t.ScopeRange.End])
}

func (t Tag) FullName(source []byte) string {
	name := t.Name(source)
	if t.ScopeRange != nil {
		name = t.ScopeName(source) + "." + name
	}
	return name
}

func (t Tag) Content(source []byte) string {
	return string(source[t.Range.Start:t.Range.End])
}

func (t Tag) Line(source []byte) string {
	return string(source[t.LineRange.Start:t.LineRange.End])
}

type localDef struct {
	Name []byte
}

func newByteRange(start uint, end uint) byteRange {
	return byteRange{
		Start: start,
		End:   end,
	}
}

type byteRange struct {
	Start uint
	End   uint
}

type pointRange struct {
	Start tree_sitter.Point
	End   tree_sitter.Point
}

type localScope struct {
	Inherits  bool
	Range     tree_sitter.Range
	LocalDefs []localDef
}

func New() *Tagger {
	return &Tagger{
		Parser: tree_sitter.NewParser(),
		cursor: tree_sitter.NewQueryCursor(),
	}
}

type Tagger struct {
	Parser *tree_sitter.Parser
	cursor *tree_sitter.QueryCursor
}

func (c *Tagger) Tags(ctx context.Context, cfg Configuration, source []byte) (iter.Seq2[Tag, error], bool, error) {
	err := c.Parser.SetLanguage(cfg.Language)
	if err != nil {
		return nil, false, err
	}

	c.Parser.Reset()
	tree := c.Parser.ParseCtx(ctx, source, nil)

	matches := peekiter.NewQueryMatches(c.cursor.Matches(cfg.Query, tree.RootNode(), source))

	var endColumn uint
	if lastNewline := bytes.LastIndexByte(source, '\n'); lastNewline != -1 {
		endColumn = uint(len(source[lastNewline:]))
	} else {
		endColumn = uint(len(source))
	}

	i := &iterator{
		Ctx:     ctx,
		Source:  source,
		Tree:    tree,
		Matches: matches,
		Cfg:     cfg,
		Scopes: []localScope{
			{
				Inherits: false,
				Range: tree_sitter.Range{
					StartByte: 0,
					StartPoint: tree_sitter.Point{
						Row:    0,
						Column: 0,
					},
					EndByte: uint(len(source)),
					EndPoint: tree_sitter.Point{
						Row:    uint(bytes.Count(source, []byte("\n"))),
						Column: endColumn,
					},
				},
				LocalDefs: nil,
			},
		},
		TagQueue:     nil,
		PrevLineInfo: nil,
	}

	return func(yield func(Tag, error) bool) {
		for {
			tag, err := i.next()
			if err != nil {
				yield(Tag{}, err)
				return
			}
			if tag == nil {
				return
			}
			if !yield(*tag, nil) {
				return
			}
		}
	}, tree.RootNode().HasError(), nil
}
