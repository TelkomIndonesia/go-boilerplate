package log

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type loggerExt struct {
	l LoggerBase

	attrs     []Attr
	withTrace bool
}

func (l loggerExt) Enabled(ctx context.Context, lvl Level) bool {
	return l.Enabled(ctx, lvl)
}

func WithLoggerExt(l LoggerBase) Logger {
	return loggerExt{
		l: l,
	}
}

func (l loggerExt) WithAttrs(fns ...Attr) Logger {
	nattrs := make([]Attr, 0, len(l.attrs)+len(fns))
	nattrs = append(nattrs, l.attrs...)
	l.attrs = append(nattrs, fns...)
	return l
}

func (l loggerExt) WithTrace() Logger {
	l.withTrace = true
	return l
}

func (l loggerExt) Debug(ctx context.Context, message string, attrs ...Attr) {
	l.invoke(l.l.Debug, ctx, message, attrs...)
}

func (l loggerExt) Info(ctx context.Context, message string, attrs ...Attr) {
	l.invoke(l.l.Info, ctx, message, attrs...)
}

func (l loggerExt) Warn(ctx context.Context, message string, attrs ...Attr) {
	l.invoke(l.l.Warn, ctx, message, attrs...)
}

func (l loggerExt) Error(ctx context.Context, message string, attrs ...Attr) {
	l.invoke(l.l.Error, ctx, message, attrs...)
}

func (l loggerExt) Fatal(ctx context.Context, message string, attrs ...Attr) {
	l.invoke(l.l.Fatal, ctx, message, attrs...)
}

func (l loggerExt) invoke(fn func(context.Context, string, ...Attr), ctx context.Context, message string, attrs ...Attr) {
	if len(l.attrs) > 0 {
		attrs = append(l.attrs, attrs...)
	}

	l.addToSpan(ctx, attrs...)
	fn(ctx, message, attrs...)
}

func (l loggerExt) addToSpan(ctx context.Context, attrs ...Attr) {
	if ctx == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().HasTraceID() {
		return
	}

	tattrs := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		if !l.withTrace && !attr.withTrace {
			continue
		}

		if attr.err != nil {
			span.RecordError(attr.err, trace.WithStackTrace(true))
			span.SetStatus(codes.Error, attr.err.Error())
			continue
		}

		added := true
		switch attr.attr.Value.Kind() {
		case slog.KindString:
			tattrs = append(tattrs, attribute.String(attr.attr.Key, attr.attr.Value.String()))
		case slog.KindDuration:
			tattrs = append(tattrs, attribute.String(attr.attr.Key, attr.attr.Value.Duration().String()))
		case slog.KindTime:
			tattrs = append(tattrs, attribute.String(attr.attr.Key, attr.attr.Value.Time().Format(time.RFC3339)))
		case slog.KindBool:
			tattrs = append(tattrs, attribute.Bool(attr.attr.Key, attr.attr.Value.Bool()))
		case slog.KindInt64:
			tattrs = append(tattrs, attribute.Int64(attr.attr.Key, attr.attr.Value.Int64()))
		case slog.KindFloat64:
			tattrs = append(tattrs, attribute.Float64(attr.attr.Key, attr.attr.Value.Float64()))
		default:
			added = false
		}
		if added {
			continue
		}

		switch v := attr.attr.Value.Any().(type) {
		case bool:
			tattrs = append(tattrs, attribute.Bool(attr.attr.Key, v))
		case []bool:
			tattrs = append(tattrs, attribute.BoolSlice(attr.attr.Key, v))
		case int:
			tattrs = append(tattrs, attribute.Int(attr.attr.Key, v))
		case []int:
			tattrs = append(tattrs, attribute.IntSlice(attr.attr.Key, v))
		case int64:
			tattrs = append(tattrs, attribute.Int64(attr.attr.Key, v))
		case []int64:
			tattrs = append(tattrs, attribute.Int64Slice(attr.attr.Key, v))
		case float64:
			tattrs = append(tattrs, attribute.Float64(attr.attr.Key, v))
		case []float64:
			tattrs = append(tattrs, attribute.Float64Slice(attr.attr.Key, v))
		case string:
			tattrs = append(tattrs, attribute.String(attr.attr.Key, v))
		case []string:
			tattrs = append(tattrs, attribute.StringSlice(attr.attr.Key, v))
		case fmt.Stringer:
			tattrs = append(tattrs, attribute.Stringer(attr.attr.Key, v))
		default:
			b, err := json.Marshal(v)
			if err == nil {
				tattrs = append(tattrs, attribute.String(attr.attr.Key, string(b)))
			}
		}
	}
	span.SetAttributes(tattrs...)
}
