package folds

import (
	"context"
)

// Folds is a function that folds over a list of items.
func Folds(ctx context.Context) {
	i := iterator{
		Ctx: ctx,
	}
}

// iterator is a type that iterates over a list of items.
type iterator struct {
	Ctx context.Context
}

func main() {
	ctx := context.Background()
	Folds(ctx)
}
