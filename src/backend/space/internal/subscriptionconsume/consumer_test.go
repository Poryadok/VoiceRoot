package subscriptionconsume

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	eventsv1 "voice.app/voice/events/v1"
)

type captureEntitlements struct {
	upserts  []upsertCall
	finalize []uuid.UUID
}

type upsertCall struct {
	SpaceID     uuid.UUID
	PurchaserID uuid.UUID
	Status      string
}

func (c *captureEntitlements) UpsertSpaceSubscription(_ context.Context, spaceID, purchaserAccountID uuid.UUID, status string) error {
	c.upserts = append(c.upserts, upsertCall{SpaceID: spaceID, PurchaserID: purchaserAccountID, Status: status})
	return nil
}

func (c *captureEntitlements) FinalizeSpacePro(_ context.Context, spaceID uuid.UUID) error {
	c.finalize = append(c.finalize, spaceID)
	return nil
}

func TestApplySpaceProStarted_UpsertsActive(t *testing.T) {
	ents := &captureEntitlements{}
	spaceID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	purchaser := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	ApplySubscriptionEvent(ents, &eventsv1.SubscriptionStreamEvent{
		Payload: &eventsv1.SubscriptionStreamEvent_SpaceProStarted{
			SpaceProStarted: &eventsv1.SpaceProStarted{
				SpaceId:            spaceID.String(),
				PurchaserAccountId: purchaser.String(),
			},
		},
	})
	require.Len(t, ents.upserts, 1)
	require.Equal(t, spaceID, ents.upserts[0].SpaceID)
	require.Equal(t, purchaser, ents.upserts[0].PurchaserID)
	require.Equal(t, "active", ents.upserts[0].Status)
}

func TestApplySpaceProExpired_Finalizes(t *testing.T) {
	ents := &captureEntitlements{}
	spaceID := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	ApplySubscriptionEvent(ents, &eventsv1.SubscriptionStreamEvent{
		Payload: &eventsv1.SubscriptionStreamEvent_SpaceProExpired{
			SpaceProExpired: &eventsv1.SpaceProExpired{SpaceId: spaceID.String()},
		},
	})
	require.Equal(t, []uuid.UUID{spaceID}, ents.finalize)
}
