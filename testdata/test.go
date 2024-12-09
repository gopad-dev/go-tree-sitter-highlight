package folds

import (
	"context"
	"iter"

	"github.com/tree-sitter/go-tree-sitter"

	"go.gopad.dev/go-tree-sitter-highlight/internal/peekiter"
)

// NewConfiguration returns a new Configuration.
func NewConfiguration(language *tree_sitter.Language, foldsQuery []byte) (*Configuration, error) {
	query, err := tree_sitter.NewQuery(language, string(foldsQuery))
	if err != nil {
		return nil, err
	}

	var foldCaptureIndex uint
	for i, captureName := range query.CaptureNames() {
		ui := uint(i)
		switch captureName {
		case "fold":
			foldCaptureIndex = ui
		}
	}

	return &Configuration{
		Language:         language,
		Query:            query,
		foldCaptureIndex: foldCaptureIndex,
	}, nil
}

type Configuration struct {
	Language         *tree_sitter.Language
	Query            *tree_sitter.Query
	foldCaptureIndex uint
}

type Fold struct {
	Range tree_sitter.Range
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

			if !yield(*fold, nil) {
				return
			}
		}
	}, nil
}

type iterator struct {
	Ctx         context.Context
	Source      []byte
	Tree        *tree_sitter.Tree
	Captures    *peekiter.QueryCaptures
	Cfg         Configuration
	LastFoldRow uint
}

func (f *iterator) next() (*Fold, error) {
	for {
		// check for cancellation
		select {
		case <-f.Ctx.Done():
			return nil, f.Ctx.Err()
		default:
		}

		match, captureIndex, ok := f.Captures.Next()
		if !ok {
			return nil, nil
		}

		capture := match.Captures[captureIndex]

		if uint(capture.Index) == f.Cfg.foldCaptureIndex {
			startRow := capture.Node.StartPosition().Row

			// if the fold is at the same row as the last fold, skip it
			if startRow == f.LastFoldRow {
				continue
			}

			f.LastFoldRow = startRow

			return &Fold{
				Range: capture.Node.Range(),
			}, nil
		}
	}
}
