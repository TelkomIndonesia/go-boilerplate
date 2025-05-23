package logotel

import (
	"github.com/telkomindonesia/go-boilerplate/pkg/log/internal"
	"go.opentelemetry.io/contrib/bridges/otelslog"
)

func init() {
	internal.DefaultHandler = internal.NewBiHandler(internal.DefaultHandler, otelslog.NewHandler("otel"))
}
