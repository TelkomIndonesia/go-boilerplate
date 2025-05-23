package log

import "log/slog"

var globalLogger = NewLogger()

func Global() Logger {
	return globalLogger
}

func SetGlobal(l Logger) {
	globalLogger = l
	if l, ok := l.(*loggerExt); ok {
		if l, ok := l.l.(*logger); ok {
			slog.SetDefault(l.l)
		}
	}
}
