package outbox

import (
	"context"
	"database/sql"

	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

func txRollbackDeferer(tx *sql.Tx, err *error) func() {
	return func() {
		if *err != nil {
			tx.Rollback()
		}
	}
}

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
