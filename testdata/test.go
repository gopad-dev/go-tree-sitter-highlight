package folds

import (
	"context"
)

func Folds(ctx context.Context) {
	i := iterator{
		Ctx: ctx,
	}
}

type iterator struct {
	Ctx context.Context
}
