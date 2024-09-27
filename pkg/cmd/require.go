package cmd

import "github.com/telkomindonesia/go-boilerplate/pkg/log"

func require[T any](f func() (T, error), l log.Logger) T {
	if l == nil {
		l = log.Global()
	}

	t, err := f()
	if err != nil {
		l.Fatal("requirement-unsatisfied", log.Error("error", err))
	}
	return t
}
