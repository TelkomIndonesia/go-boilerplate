package util

import "github.com/telkomindonesia/go-boilerplate/pkg/util/log"

func Require[T any](f func() (T, error), l log.Logger) T {
	if l == nil {
		l = log.Global()
	}

	t, err := f()
	if err != nil {
		l.Fatal("requirement-unsatisfied", log.Any("error", err))
	}
	return t
}
