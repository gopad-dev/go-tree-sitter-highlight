package tags

import (
	"context"
	"iter"

	"github.com/tree-sitter/go-tree-sitter"

	"go.gopad.dev/go-tree-sitter-highlight/internal/peekiter"
)

func newIgnoredTag(nameRange byteRange) Tag {
	return Tag{
		Range: byteRange{
			Start: ^uint(0),
			End:   ^uint(0),
		},
		NameRange: nameRange,
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
		Docs:         "",
		IsDefinition: false,
		SyntaxTypeID: 0,
	}
}

type Tag struct {
	Range            byteRange
	NameRange        byteRange
	LineRange        byteRange
	Span             pointRange
	UTF16ColumnRange byteRange
	Docs             string
	IsDefinition     bool
	SyntaxTypeID     uint
}

func (t Tag) IsIgnored() bool {
	return t.Range.Start == ^uint(0)
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
	Range     byteRange
	LocalDefs []localDef
}

func New() *Context {
	return &Context{
		Parser: tree_sitter.NewParser(),
		cursor: tree_sitter.NewQueryCursor(),
	}
}

type Context struct {
	Parser *tree_sitter.Parser
	cursor *tree_sitter.QueryCursor
}

func (c *Context) GenerateTags(ctx context.Context, cfg Configuration, source []byte) (iter.Seq2[Tag, error], bool, error) {
	err := c.Parser.SetLanguage(cfg.Language)
	if err != nil {
		return nil, false, err
	}

	c.Parser.Reset()
	tree := c.Parser.ParseCtx(ctx, source, nil)

	matches := peekiter.NewQueryMatches(c.cursor.Matches(cfg.Query, tree.RootNode(), source))

	i := &iterator{
		Ctx:     ctx,
		Source:  source,
		Tree:    tree,
		Matches: matches,
		Cfg:     cfg,
		Scopes: []localScope{
			{
				Inherits: false,
				Range: byteRange{
					Start: 0,
					End:   uint(len(source)),
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
