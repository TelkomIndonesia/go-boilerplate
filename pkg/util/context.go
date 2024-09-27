package util

import (
	"context"
)

type contextkey struct{}

func ContextWithMatcher(pctx context.Context) (ctx context.Context, matcher func(ctx context.Context) bool) {
	ctx = context.WithValue(pctx, contextkey{}, pctx)
	matcher = func(ctx context.Context) bool {
		if ctx == nil {
			return false
		}

		p, ok := ctx.Value(contextkey{}).(context.Context)
		if !ok {
			return false
		}
		return p == pctx
	}
	return
}
