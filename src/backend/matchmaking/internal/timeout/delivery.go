package timeout

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
	"voice/backend/matchmaking/internal/store"
)

type CapturedTimeoutPublisher interface {
	PublishCapturedTimeout(context.Context, uuid.UUID, []byte) error
}

// RelayNativeTimeouts dispatches only committed terminal facts, independently
// of fresh source/admission availability. A lost PubAck/SQL ACK reuses identity.
func RelayNativeTimeouts(ctx context.Context, st *store.MatchStore, publisher CapturedTimeoutPublisher) error {
	if ctx == nil || st == nil || publisher == nil {
		return store.ErrSourcePartyUnavailable
	}
	facts, err := st.ClaimNativeTimeouts(ctx, 100, 30*time.Second)
	if err != nil {
		return err
	}
	var result error
	for _, fact := range facts {
		publishErr := publisher.PublishCapturedTimeout(ctx, fact.EventID, fact.Payload)
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		finishErr := st.FinishNativeTimeout(cleanup, fact, publishErr == nil)
		cancel()
		result = errors.Join(result, publishErr, finishErr)
	}
	return result
}
