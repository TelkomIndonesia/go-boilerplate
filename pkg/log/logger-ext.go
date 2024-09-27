package log

import "context"

type loggerExt struct {
	LoggerBase

	logFuncs []LogFunc
	ctx      context.Context
}

func WithLoggerExt(l LoggerBase) Logger {
	return loggerExt{
		LoggerBase: l,
	}
}

func (l loggerExt) WithLog(fns ...LogFunc) Logger {
	l.logFuncs = append(l.logFuncs, fns...)
	return l
}

func (l loggerExt) WithTrace(ctx context.Context) Logger {
	l.ctx = ctx
	return l
}

func (l loggerExt) Debug(message string, fn ...LogFunc) {
	l.LoggerBase.Debug(message, l.wrap(fn...))
}

func (l loggerExt) Info(message string, fn ...LogFunc) {
	l.LoggerBase.Info(message, l.wrap(fn...))
}

func (l loggerExt) Warn(message string, fn ...LogFunc) {
	l.LoggerBase.Warn(message, l.wrap(fn...))
}

func (l loggerExt) Error(message string, fn ...LogFunc) {
	l.LoggerBase.Error(message, l.wrap(fn...))
}

func (l loggerExt) Fatal(message string, fn ...LogFunc) {
	l.LoggerBase.Fatal(message, l.wrap(fn...))
}

func (l loggerExt) wrap(fn ...LogFunc) LogFunc {
	return WithTrace(l.ctx, append(l.logFuncs, fn...)...)
}
