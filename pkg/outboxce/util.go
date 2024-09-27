package outboxce

import (
	"context"

	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/tink-crypto/tink-go/v2/tink"
)

func RelayLoopWithRetry(ctx context.Context, m Manager, s RelayFunc, l log.Logger) {
	if m == nil {
		m = ManagerNOP()
	}
	if l == nil {
		l = log.Global()
	}
	for {
		if err := m.RelayLoop(ctx, s); err != nil {
			l.Warn("got outbox observer error", log.WithTrace(ctx, log.Error("error", err)))
		}

		select {
		case <-ctx.Done():
			return

		case <-time.After(time.Minute):
		}
	}
}

func TenantAEAD(dk *tinkx.DerivableKeyset[tinkx.PrimitiveAEAD]) func(event.Event) (tink.AEAD, error) {
	return func(e event.Event) (tink.AEAD, error) {
		aead, err := dk.GetPrimitive([]byte(e.Subject()))
		return aead, err
	}
}
