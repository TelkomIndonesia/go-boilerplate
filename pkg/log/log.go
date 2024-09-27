package log

import "github.com/telkomindonesia/go-boilerplate/pkg/log/internal"

type Log interface {
	internal.Log
}

type LogFunc func(Log)

type Loggable interface {
	AsLog() any
}
