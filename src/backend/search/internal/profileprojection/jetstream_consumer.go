package profileprojection

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
)

const (
	userProjectionStream  = "user_profile_projection"
	userProjectionSubject = "user.search_profile_projection"
	userProjectionDurable = "search-user-profile-projection-v1"
)

// CheckpointApplier applies a delivered User authority record and its replay
// checkpoint in the StoreAdapter's single transaction before Ack.
type CheckpointApplier interface {
	ApplyAndCheckpoint(context.Context, *userv1.SearchProfileProjectionEvent, uint64) (ApplyResult, error)
}

func RunJetStreamConsumer(ctx context.Context, natsURL string, adapter CheckpointApplier, ready chan<- error) (err error) {
	defer func() {
		if ready != nil {
			ready <- err
			close(ready)
		}
	}()
	if natsURL == "" || adapter == nil {
		return fmt.Errorf("projection consumer requires NATS URL and store")
	}
	nc, err := nats.Connect(natsURL, nats.Name("voice-search-user-projection"), nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1))
	if err != nil {
		return err
	}
	defer func() {
		if drainErr := nc.Drain(); err == nil && drainErr != nil {
			err = drainErr
		}
	}()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	_, err = js.Subscribe(userProjectionSubject, func(msg *nats.Msg) {
		event := &userv1.SearchProfileProjectionEvent{}
		if err := proto.Unmarshal(msg.Data, event); err != nil {
			_ = msg.Term()
			return
		}
		if _, err := adapter.ApplyAndCheckpoint(ctx, event, event.GetJournalOffset()); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
		// The projection durable is provisioned before Search starts. Bind-only
		// prevents an application credential from creating or mutating consumers.
	}, nats.Bind(userProjectionStream, userProjectionDurable), nats.ManualAck())
	if err != nil {
		return err
	}
	if ready != nil {
		ready <- nil
		ready = nil
	}
	<-ctx.Done()
	return nil
}
