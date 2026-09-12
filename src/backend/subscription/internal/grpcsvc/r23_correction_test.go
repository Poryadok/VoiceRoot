package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/subscription/internal/store"

	commonv1 "voice.app/voice/common/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
)

func TestR23CorrectionCurrentOnlyProviderKeySourceFailsClosed(t *testing.T) {
	svc := NewSubscriptionGRPC(nil)
	svc.ProviderEventKeys = currentOnlyProviderKeys{}
	for _, provider := range []string{"paddle", "cloudpayments"} {
		t.Run(provider, func(t *testing.T) {
			_, err := svc.providerEventHMACKeys(context.Background(), provider)
			require.ErrorContains(t, err, "retained provider event HMAC keys unavailable")
		})
	}
}

func TestR23CorrectionProviderKeysPutCurrentBeforeRetainedVersions(t *testing.T) {
	current := ProviderEventHMACKey{Version: "kms-current", Key: []byte("current-provider-key")}
	old := ProviderEventHMACKey{Version: "kms-old", Key: []byte("old-provider-key")}
	svc := NewSubscriptionGRPC(nil)
	svc.ProviderEventKeys = &rotatedProviderKeys{current: current, retained: []ProviderEventHMACKey{old, current}}

	keys, err := svc.providerEventHMACKeys(context.Background(), "paddle")
	require.NoError(t, err)
	require.Equal(t, []string{current.Version, old.Version}, []string{keys[0].Version, keys[1].Version})
}

func TestR23CorrectionPurgedFenceRejectsFrozenThenLiveAndKeepsEntitlementsDenied(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL terminal lifecycle fence requires testcontainers")
	}
	ctx := context.Background()
	pool := r23StartGRPCSubscriptionPostgres(t, ctx)
	st := &store.SubscriptionStore{Pool: pool}
	svc := NewSubscriptionGRPC(st)
	svc.ProviderRenewals = &r23RenewalCanceller{}
	spaceID, purchaserID, deletionID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, r23SeedSpacePro(ctx, pool, spaceID, purchaserID, "evt-terminal-fence"))
	manifest := r23Manifest(deletionID.String(), 1)

	r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest))
	r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, manifest))
	prematureFrozen := r23FenceRequest(spaceID, deletionID, 3, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	_, err := svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, prematureFrozen), prematureFrozen)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "PURGE_DECIDED cannot transition to FROZEN")
	purge := r23PurgeRequest(spaceID, deletionID, 2, manifest)
	_, err = svc.PurgeSpace(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, purge), purge)
	require.NoError(t, err)

	frozen := r23FenceRequest(spaceID, deletionID, 3, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	_, frozenErr := svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, frozen), frozen)
	live := r23FenceRequest(spaceID, deletionID, 4, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, manifest)
	_, liveErr := svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, live), live)
	require.Equal(t, codes.FailedPrecondition, status.Code(frozenErr), "PURGED cannot transition to a newer FROZEN generation")
	require.Equal(t, codes.FailedPrecondition, status.Code(liveErr), "PURGED cannot transition to any later generation")

	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM subscription_space_lifecycle_fences WHERE space_id=$1`, spaceID).Scan(&state))
	require.Equal(t, "PURGED", state, "rejected transitions cannot replace permanent authority")
	_, err = st.ActivateSpacePro(ctx, spaceID, purchaserID, "evt-terminal-recreate", []byte(`{"status":"active"}`))
	require.ErrorIs(t, err, store.ErrLifecycleState, "governed activation remains denied after transition attempts")
	active, err := st.HasActiveSpaceProForSpace(ctx, spaceID)
	require.NoError(t, err)
	require.False(t, active, "governed entitlement remains denied after transition attempts")
}

func TestR23CorrectionProviderReplaySurvivesHMACRotationAfterPurge(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL provider rotation replay requires testcontainers")
	}
	for _, provider := range []string{"paddle", "cloudpayments"} {
		t.Run(provider, func(t *testing.T) {
			ctx := context.Background()
			pool := r23StartGRPCSubscriptionPostgres(t, ctx)
			spaceID, purchaserID, deletionID := uuid.New(), uuid.New(), uuid.New()
			oldKey := ProviderEventHMACKey{Version: "kms-old", Key: []byte("old-provider-key")}
			newKey := ProviderEventHMACKey{Version: "kms-current", Key: []byte("current-provider-key")}

			var eventID, rawBody, signature string
			var verifier *r23CloudPaymentsVerifier
			if provider == "paddle" {
				rawBody, eventID = spaceProActivatedWebhookBody(t, spaceID, purchaserID)
				signature = signedWebhook(t, rawBody)
			} else {
				eventID = "cp_rotation_" + uuid.NewString()
				rawBody, signature = `{"transactionId":"`+eventID+`","status":"Completed"}`, "fixture-cloudpayments-signature"
				verifier = &r23CloudPaymentsVerifier{rawBody: rawBody, signature: signature, eventID: eventID, spaceID: spaceID.String(), purchaserID: purchaserID.String()}
			}
			call := func(svc *SubscriptionGRPC) error {
				if provider == "paddle" {
					_, err := svc.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{RawBody: rawBody, Signature: signature})
					return err
				}
				_, err := svc.HandleCloudPaymentsWebhook(ctx, &subscriptionv1.HandleCloudPaymentsWebhookRequest{RawBody: rawBody, Signature: signature})
				return err
			}
			configure := func(keys *rotatedProviderKeys) *SubscriptionGRPC {
				svc := NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool})
				svc.ProviderEventKeys = keys
				if verifier != nil {
					copy := *verifier
					svc.CloudPayments = &copy
				}
				return svc
			}

			svc := configure(&rotatedProviderKeys{current: oldKey, retained: []ProviderEventHMACKey{oldKey}})
			require.NoError(t, call(svc))
			manifest := r23Manifest(deletionID.String(), 0)
			r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest))
			r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, manifest))
			canceller := &r23RenewalCanceller{}
			svc.ProviderRenewals = canceller
			purge := r23PurgeRequest(spaceID, deletionID, 2, manifest)
			_, err := svc.PurgeSpace(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, purge), purge)
			require.NoError(t, err)

			require.NoError(t, call(configure(&rotatedProviderKeys{current: newKey, retained: []ProviderEventHMACKey{newKey, oldKey}})), "an old permanent digest must replay after P90D key rotation")
			require.Equal(t, int64(0), r23RowCount(t, ctx, pool, "space_subscriptions", "space_id", spaceID))
			require.Equal(t, int64(1), r23RowCount(t, ctx, pool, "subscription_provider_event_fences", "provider", provider))
			var version string
			require.NoError(t, pool.QueryRow(ctx, `SELECT key_version FROM subscription_provider_event_fences WHERE provider=$1`, provider).Scan(&version))
			require.Equal(t, oldKey.Version, version, "replay cannot rewrite the first permanent key version")

			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(context.Background()) }()
			_, err = (&store.SubscriptionStore{Pool: pool}).RecordProviderEventWithKeysTx(ctx, tx, provider, []byte(eventID), "conflicting_outcome", []store.ProviderEventHMACKey{
				{Version: newKey.Version, Key: newKey.Key},
				{Version: oldKey.Version, Key: oldKey.Key},
			})
			require.ErrorIs(t, err, store.ErrLifecycleBinding, "rotated post-purge event binding conflict must fail closed")
		})
	}
}

type currentOnlyProviderKeys struct{}

func (currentOnlyProviderKeys) CurrentProviderEventHMACKey(context.Context, string) (string, []byte, error) {
	return "current-only", []byte("current-only-provider-key"), nil
}

func (testProviderEventKeys) ProviderEventHMACVerificationKeys(context.Context, string) ([]ProviderEventHMACKey, error) {
	return []ProviderEventHMACKey{{Version: "test-v1", Key: []byte("subscription-test-provider-event-key")}}, nil
}

func (k *r23ObservedProviderKeys) ProviderEventHMACVerificationKeys(context.Context, string) ([]ProviderEventHMACKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return []ProviderEventHMACKey{{Version: k.version, Key: append([]byte(nil), k.key...)}}, nil
}

type rotatedProviderKeys struct {
	current  ProviderEventHMACKey
	retained []ProviderEventHMACKey
}

func (k *rotatedProviderKeys) CurrentProviderEventHMACKey(context.Context, string) (string, []byte, error) {
	return k.current.Version, append([]byte(nil), k.current.Key...), nil
}

func (k *rotatedProviderKeys) ProviderEventHMACVerificationKeys(context.Context, string) ([]ProviderEventHMACKey, error) {
	out := make([]ProviderEventHMACKey, len(k.retained))
	copy(out, k.retained)
	return out, nil
}
