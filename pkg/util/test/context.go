package test

import (
	"context"
)

func ContextWithMatcher(pctx context.Context) (ctx context.Context, matcher func(ctx context.Context) bool) {
	parent := struct{}{}
	ctx = context.WithValue(pctx, parent, pctx)
	matcher = func(ctx context.Context) bool {
		if ctx == nil {
			return false
		}
		p, ok := ctx.Value(parent).(context.Context)
		if !ok {
			return false
		}
		return p == pctx
	}
	return
}
