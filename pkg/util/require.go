package util

import "github.com/telkomindonesia/go-boilerplate/pkg/util/logger"

func Require[T any](f func() (T, error), l logger.Logger) T {
	if l == nil {
		l = logger.Global()
	}

	t, err := f()
	if err != nil {
		l.Fatal("requirement-unsatisfied", logger.Any("error", err))
	}
	return t
}
