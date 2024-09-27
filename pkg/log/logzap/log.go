package logzap

import (
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapLog struct {
	fields []zap.Field
}

func newZapLog(fn ...log.LogFunc) *zapLog {
	lc := &zapLog{fields: make([]zapcore.Field, 0, len(fn))}
	for _, fn := range fn {
		fn(lc)
	}
	return lc
}

func (lc *zapLog) Any(key string, value any) {
	lc.fields = append(lc.fields, zap.Any(key, value))

}
func (lc *zapLog) Bool(key string, value bool) {
	lc.fields = append(lc.fields, zap.Bool(key, value))

}
func (lc *zapLog) ByteString(key string, value []byte) {
	lc.fields = append(lc.fields, zap.ByteString(key, value))

}
func (lc *zapLog) String(key string, value string) {
	lc.fields = append(lc.fields, zap.String(key, value))

}
func (lc *zapLog) Float64(key string, value float64) {
	lc.fields = append(lc.fields, zap.Float64(key, value))

}
func (lc *zapLog) Int64(key string, value int64) {
	lc.fields = append(lc.fields, zap.Int64(key, value))

}
func (lc *zapLog) Uint64(key string, value uint64) {
	lc.fields = append(lc.fields, zap.Uint64(key, value))

}
func (lc *zapLog) Time(key string, value time.Time) {
	lc.fields = append(lc.fields, zap.Time(key, value))
}
func (lc *zapLog) Error(key string, value error) {
	lc.fields = append(lc.fields, zap.NamedError(key, value))
}
