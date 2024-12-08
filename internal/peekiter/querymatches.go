package peekiter

import (
	"slices"

	"github.com/tree-sitter/go-tree-sitter"
)

type peekedQueryMatch struct {
	match tree_sitter.QueryMatch
	ok    bool
}

func NewQueryMatches(iter tree_sitter.QueryMatches) *QueryMatches {
	return &QueryMatches{captures: iter}
}

type QueryMatches struct {
	captures tree_sitter.QueryMatches
	peeked   *peekedQueryMatch
}

func (q *QueryMatches) next() (tree_sitter.QueryMatch, bool) {
	match := q.captures.Next()
	if match == nil {
		return tree_sitter.QueryMatch{}, false
	}

	match.Captures = slices.Clone(match.Captures)
	return *match, true
}

func (q *QueryMatches) Next() (tree_sitter.QueryMatch, bool) {
	if q.peeked != nil {
		peeked := q.peeked
		q.peeked = nil
		return peeked.match, peeked.ok
	}
	return q.next()
}

func (q *QueryMatches) Peek() (tree_sitter.QueryMatch, bool) {
	if q.peeked == nil {
		match, ok := q.next()
		q.peeked = &peekedQueryMatch{
			match: match,
			ok:    ok,
		}
	}

	return q.peeked.match, q.peeked.ok
}
