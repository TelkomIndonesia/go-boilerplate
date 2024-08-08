package outboxce

import (
	"context"
	"errors"

	"github.com/cloudevents/sdk-go/v2/event"
)

type RelayFunc func(ctx context.Context, ce []event.Event) error

var _ []error = []error{&RelayErrors{}, &RelayError{}}

type RelayErrors []*RelayError

func (p *RelayErrors) Error() string {
	errs := make([]error, 0, len(*p))
	for _, e := range *p {
		errs = append(errs, e)
	}
	return errors.Join(errs...).Error()
}

type RelayError struct {
	Err   error
	Event event.Event
}

func (p *RelayError) Error() string {
	return p.Err.Error()
}

func (p *RelayError) As(i interface{}) bool {
	switch v := i.(type) {
	case **RelayError:
		*v = p
		return true
	case **RelayErrors:
		*v = &RelayErrors{p}
		return true
	}
	return false
}
