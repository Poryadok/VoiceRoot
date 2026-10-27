package reminderhint

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// Publish settles only a previously committed hint intent. The caller must
// already have atomically committed the unique in-app insertion and this intent,
// and rechecked current recipient/policy/Subscription authority before dispatch.
// Its PubAck is never an insertion or client-receipt result.
func Publish(ctx context.Context, js nats.JetStreamContext, hint Hint, now time.Time) (*nats.PubAck, error) {
	if js == nil {
		return nil, fmt.Errorf("reminder hint transport unavailable")
	}
	if err := hint.Validate(now); err != nil {
		return nil, err
	}
	info, err := js.StreamInfo(Stream, nats.Context(ctx))
	if err != nil {
		return nil, err
	}
	if info.Config.Name != Stream || len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != Subject || info.Config.Retention != nats.LimitsPolicy || info.Config.Storage != nats.FileStorage || info.Config.MaxAge != 7*24*time.Hour {
		return nil, fmt.Errorf("reminder hint stream configuration mismatch")
	}
	raw, err := json.Marshal(hint)
	if err != nil {
		return nil, err
	}
	ack, err := js.Publish(Subject, raw, nats.MsgId(hint.NotificationID), nats.ExpectStream(Stream), nats.Context(ctx))
	if err != nil {
		return nil, err
	}
	if ack == nil || ack.Stream != Stream || ack.Sequence == 0 {
		return nil, fmt.Errorf("reminder hint acknowledgement mismatch")
	}
	return ack, nil
}
