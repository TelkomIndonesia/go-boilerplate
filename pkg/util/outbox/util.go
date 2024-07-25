package outbox

import (
	"context"

	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

func WatchOutboxesLoop(ctx context.Context, m Manager, l log.Logger) {
	if m == nil {
		m = ManagerNOP()
	}
	if l == nil {
		l = log.Global()
	}
	for {
		if err := m.WatchOuboxes(ctx); err != nil {
			l.Warn("got outbox watcher error", log.Error("error", err), log.TraceContext("trace-id", ctx))
		}

		select {
		case <-ctx.Done():
			return

		case <-time.After(time.Minute):
		}
	}
}
