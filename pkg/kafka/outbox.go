package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/outbox"
)

func (k *Kafka) OutboxRelayer() outbox.RelayFunc {
	return func(ctx context.Context, o []outbox.Outbox[outbox.Serialized]) (err error) {
		msgs := make([]Message, 0, len(o))
		for _, o := range o {
			var content map[string]interface{}
			if err = o.Content.Unmarshal(&content); err != nil {
				return fmt.Errorf("fail to unmarshal content: %w", err)
			}

			// TODO: change this to more proper message such as one defined using protobuf
			msg := map[string]interface{}{
				"id":           o.ID,
				"tenant_id":    o.TenantID,
				"event_name":   o.EventName,
				"content_type": o.ContentType,
				"content":      content,
				"created_at":   o.CreatedAt,
			}
			var kmsg Message
			if kmsg.Value, err = json.Marshal(msg); err != nil {
				return fmt.Errorf("fail to marshal outbox: %w", err)
			}
			msgs = append(msgs, kmsg)
		}
		return k.Write(ctx, k.topic, msgs...)
	}
}
