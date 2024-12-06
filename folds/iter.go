package folds

import (
	"context"

	"github.com/tree-sitter/go-tree-sitter"

	"go.gopad.dev/go-tree-sitter-highlight/internal/peekiter"
)

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
