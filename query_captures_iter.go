package highlight

import (
	"slices"

	"github.com/tree-sitter/go-tree-sitter"
)

type peekedCapture struct {
	match tree_sitter.QueryMatch
	index uint
	ok    bool
}

func newQueryCapturesIter(iter tree_sitter.QueryCaptures) *queryCapturesIter {
	return &queryCapturesIter{captures: iter}
}

type queryCapturesIter struct {
	captures tree_sitter.QueryCaptures
	peeked   *peekedCapture
}

func (q *queryCapturesIter) next() (tree_sitter.QueryMatch, uint, bool) {
	match, index := q.captures.Next()
	if match == nil {
		return tree_sitter.QueryMatch{}, index, false
	}

	match.Captures = slices.Clone(match.Captures)
	return *match, index, true
}

func (q *queryCapturesIter) Next() (tree_sitter.QueryMatch, uint, bool) {
	if q.peeked != nil {
		peeked := q.peeked
		q.peeked = nil
		return peeked.match, peeked.index, peeked.ok
	}
	return q.next()
}

func (q *queryCapturesIter) Peek() (tree_sitter.QueryMatch, uint, bool) {
	if q.peeked == nil {
		match, index, ok := q.next()
		q.peeked = &peekedCapture{
			match: match,
			index: index,
			ok:    ok,
		}
	}

	return q.peeked.match, q.peeked.index, q.peeked.ok
}
