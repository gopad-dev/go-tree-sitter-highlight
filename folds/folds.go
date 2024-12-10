package folds

import (
	"context"
	"iter"
	"log"

	"github.com/tree-sitter/go-tree-sitter"

	"go.gopad.dev/go-tree-sitter-highlight/internal/peekiter"
)

type Fold struct {
	Range     tree_sitter.Range
	LineRange tree_sitter.Range
}

func New() *Folder {
	return &Folder{
		Parser: tree_sitter.NewParser(),
		cursor: tree_sitter.NewQueryCursor(),
	}
}

type Folder struct {
	Parser *tree_sitter.Parser
	cursor *tree_sitter.QueryCursor
}

func (c *Folder) Folds(ctx context.Context, cfg Configuration, source []byte) (iter.Seq2[Fold, error], error) {
	err := c.Parser.SetLanguage(cfg.Language)
	if err != nil {
		return nil, err
	}

	c.Parser.Reset()
	tree := c.Parser.ParseCtx(ctx, source, nil)

	captures := peekiter.NewQueryCaptures(c.cursor.Captures(cfg.Query, tree.RootNode(), source))

	i := iterator{
		Ctx:      ctx,
		Source:   source,
		Tree:     tree,
		Captures: captures,
		Cfg:      cfg,
	}

	return func(yield func(Fold, error) bool) {
		for {
			fold, err := i.next()
			if err != nil {
				yield(Fold{}, err)
				return
			}

			if fold == nil {
				return
			}

			log.Printf("found fold: [%d,%d-%d,%d] [%d,%d-%d,%d]\n", fold.Range.StartPoint.Row, fold.Range.StartPoint.Column, fold.Range.EndPoint.Row, fold.Range.EndPoint.Column, fold.LineRange.StartPoint.Row, fold.LineRange.StartPoint.Column, fold.LineRange.EndPoint.Row, fold.LineRange.EndPoint.Column)
			if !yield(*fold, nil) {
				return
			}
		}
	}, nil
}
