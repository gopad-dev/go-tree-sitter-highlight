package folds

import (
	"bytes"
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

			// if the fold is a single line, skip it
			if startRow == capture.Node.EndPosition().Row {
				continue
			}

			return &Fold{
				Range:     capture.Node.Range(),
				LineRange: newLineRange(f.Source, capture.Node.Range()),
			}, nil
		}
	}
}

func newLineRange(source []byte, r tree_sitter.Range) tree_sitter.Range {
	lineStartByte := r.StartByte - r.StartPoint.Column

	textAfterLineStart := source[lineStartByte:]
	lineEndByte := lineStartByte
	var lineEndColum uint
	var lines uint
	for {
		lineEnd := bytes.IndexByte(textAfterLineStart, '\n')
		if lineEnd == -1 {
			break
		}

		if lines == r.EndPoint.Row-r.StartPoint.Row {
			lineEndColum = uint(lineEnd)
			break
		}
		textAfterLineStart = textAfterLineStart[lineEnd+1:]
		lineEndByte += uint(lineEnd) + 1
		lines++
	}

	return tree_sitter.Range{
		StartByte: lineStartByte,
		StartPoint: tree_sitter.Point{
			Row:    r.StartPoint.Row,
			Column: 0,
		},
		EndByte: lineStartByte + lineEndByte,
		EndPoint: tree_sitter.Point{
			Row:    r.EndPoint.Row,
			Column: lineEndColum,
		},
	}
}
