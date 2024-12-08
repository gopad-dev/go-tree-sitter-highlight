package teeiter

import (
	"iter"
)

type entry[K any, V any] struct {
	k K
	v V
}

func New[K any, V any](i iter.Seq2[K, V]) (iter.Seq2[K, V], iter.Seq2[K, V]) {
	next, stop := iter.Pull2(i)

	var seq []entry[K, V]
	iter1 := func(yield func(K, V) bool) {
		defer stop()
		for {
			k, v, ok := next()
			if !ok {
				return
			}
			seq = append(seq, entry[K, V]{
				k: k,
				v: v,
			})

			if !yield(k, v) {
				return
			}
		}
	}

	iter2 := func(yield func(K, V) bool) {
		for _, s := range seq {
			if !yield(s.k, s.v) {
				return
			}
		}
	}

	return iter1, iter2
}
