package outbox

import (
	"context"

	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

func ObserveWithRetry(ctx context.Context, m Manager, s RelayFunc, l log.Logger) {
	if m == nil {
		m = ManagerNOP()
	}
	if l == nil {
		l = log.Global()
	}
	for {
		if err := m.Observe(ctx, s); err != nil {
			l.Warn("got outbox observer error", log.Error("error", err), log.TraceContext("trace-id", ctx))
		}

		select {
		case <-ctx.Done():
			return

		case <-time.After(time.Minute):
		}
	}
}
