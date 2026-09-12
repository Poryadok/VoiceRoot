package grpcsvc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/pkg/principal"
	"voice/backend/subscription/internal/store"

	commonv1 "voice.app/voice/common/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
)

func TestR23SubscriptionLifecycleHandlersAreImplemented(t *testing.T) {
	svc := NewSubscriptionGRPC(nil)
	t.Run("ApplySpaceLifecycleFence", func(t *testing.T) {
		fence := r23FenceRequest(uuid.New(), uuid.New(), 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, r23Manifest("manifest", 0))
		_, err := svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, context.Background(), subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, fence), fence)
		require.NotEqual(t, codes.Unimplemented, status.Code(err), "ApplySpaceLifecycleFence still resolves to generated UnimplementedSubscriptionServiceServer")
	})
	t.Run("PurgeSpace", func(t *testing.T) {
		purge := r23PurgeRequest(uuid.New(), uuid.New(), 2, r23Manifest("manifest", 0))
		_, err := svc.PurgeSpace(r23SpacePrincipal(t, context.Background(), subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, purge), purge)
		require.NotEqual(t, codes.Unimplemented, status.Code(err), "PurgeSpace still resolves to generated UnimplementedSubscriptionServiceServer")
	})
}

func TestR23SubscriptionLifecycleRequiresCanonicalSpacePrincipalBeforeStore(t *testing.T) {
	svc := NewSubscriptionGRPC(nil)
	request := r23FenceRequest(uuid.New(), uuid.New(), 7, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, r23Manifest("root", 4))
	rpc := subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	cases := []struct {
		name string
		ctx  context.Context
	}{
		{"missing", context.Background()},
		{"raw service metadata", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-service-id", "space"))},
		{"wrong kind", principal.WithVerified(context.Background(), r23Principal("user", "space", "service:space", "subscription", rpc, hash))},
		{"wrong issuer", principal.WithVerified(context.Background(), r23Principal("service", "gateway", "service:gateway", "subscription", rpc, hash))},
		{"wrong subject", principal.WithVerified(context.Background(), r23Principal("service", "space", "service:gateway", "subscription", rpc, hash))},
		{"wrong audience", principal.WithVerified(context.Background(), r23Principal("service", "space", "service:space", "role", rpc, hash))},
		{"wrong RPC", principal.WithVerified(context.Background(), r23Principal("service", "space", "service:space", "subscription", subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, hash))},
		{"wrong request hash", principal.WithVerified(context.Background(), r23Principal("service", "space", "service:space", "subscription", rpc, "sha256:"+strings.Repeat("0", 64)))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, callErr := svc.ApplySpaceLifecycleFence(tc.ctx, request)
			require.Equal(t, codes.PermissionDenied, status.Code(callErr), "untrusted input must fail before the nil store can be reached")
		})
	}
}

func TestR23SubscriptionPurgeRequiresCanonicalSpacePrincipalBeforeStore(t *testing.T) {
	svc := NewSubscriptionGRPC(nil)
	request := r23PurgeRequest(uuid.New(), uuid.New(), 7, r23Manifest("root", 4))
	rpc := subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	cases := []struct {
		name string
		ctx  context.Context
	}{
		{"missing", context.Background()},
		{"raw service metadata", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-service-id", "space"))},
		{"wrong kind", principal.WithVerified(context.Background(), r23Principal("user", "space", "service:space", "subscription", rpc, hash))},
		{"wrong issuer", principal.WithVerified(context.Background(), r23Principal("service", "gateway", "service:gateway", "subscription", rpc, hash))},
		{"wrong subject", principal.WithVerified(context.Background(), r23Principal("service", "space", "service:gateway", "subscription", rpc, hash))},
		{"wrong audience", principal.WithVerified(context.Background(), r23Principal("service", "space", "service:space", "role", rpc, hash))},
		{"wrong RPC", principal.WithVerified(context.Background(), r23Principal("service", "space", "service:space", "subscription", subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, hash))},
		{"wrong request hash", principal.WithVerified(context.Background(), r23Principal("service", "space", "service:space", "subscription", rpc, "sha256:"+strings.Repeat("0", 64)))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, callErr := svc.PurgeSpace(tc.ctx, request)
			require.Equal(t, codes.PermissionDenied, status.Code(callErr), "untrusted purge must fail before the nil store can be reached")
		})
	}
}

func TestR23SubscriptionFreezeRestoreAndChangedBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL lifecycle contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r23StartGRPCSubscriptionPostgres(t, ctx)
	st := &store.SubscriptionStore{Pool: pool}
	svc := NewSubscriptionGRPC(st)
	spaceID, purchaserID, deletionID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, r23SeedSpacePro(ctx, pool, spaceID, purchaserID, "evt-freeze"))
	manifest := r23Manifest(deletionID.String(), 3)

	frozenRequest := r23FenceRequest(spaceID, deletionID, 7, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	frozen, err := svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, frozenRequest), frozenRequest)
	require.NoError(t, err)
	r23AssertFenceReceipt(t, frozen.GetReceipt(), frozenRequest.GetFence(), commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)

	replayRequest := proto.Clone(frozenRequest).(*subscriptionv1.ApplySpaceLifecycleFenceRequest)
	replayed, err := NewSubscriptionGRPC(st).ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, replayRequest), replayRequest)
	require.NoError(t, err)
	require.Equal(t, r23ProtoBytes(t, frozen), r23ProtoBytes(t, replayed), "response-loss replay after handler restart must be byte-identical")

	hasPro, err := st.HasActiveSpaceProForSpace(ctx, spaceID)
	require.NoError(t, err)
	require.False(t, hasPro, "FROZEN is checked in entitlement reads")
	_, err = st.ActivateSpacePro(ctx, spaceID, purchaserID, "evt-frozen-mutation", []byte(`{"status":"active"}`))
	require.Error(t, err, "FROZEN is checked in the same transaction as entitlement mutation")

	changed := proto.Clone(frozenRequest).(*subscriptionv1.ApplySpaceLifecycleFenceRequest)
	changed.Fence.Manifest.ManifestSha256 = bytes.Repeat([]byte{9}, 32)
	_, err = svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, changed), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "same operation/generation with changed bytes conflicts")
	changedState := proto.Clone(frozenRequest).(*subscriptionv1.ApplySpaceLifecycleFenceRequest)
	changedState.Fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	_, err = svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, changedState), changedState)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "same operation/generation cannot change desired state")

	liveRequest := r23FenceRequest(spaceID, deletionID, 8, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, manifest)
	live, err := svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, liveRequest), liveRequest)
	require.NoError(t, err)
	r23AssertFenceReceipt(t, live.GetReceipt(), liveRequest.GetFence(), commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE)
	hasPro, err = st.HasActiveSpaceProForSpace(ctx, spaceID)
	require.NoError(t, err)
	require.True(t, hasPro, "only the next LIVE generation restores entitlement use")
	stale := r23FenceRequest(spaceID, uuid.New(), 7, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, r23Manifest("stale", 0))
	_, err = svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, stale), stale)
	require.NoError(t, err, "lower generation is a stale no-op")
	hasPro, err = st.HasActiveSpaceProForSpace(ctx, spaceID)
	require.NoError(t, err)
	require.True(t, hasPro, "stale FROZEN request cannot move the next LIVE generation")

	gap := r23FenceRequest(spaceID, deletionID, 10, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	_, err = svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, gap), gap)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "generation gaps reconcile through Space")
}

func TestR23SubscriptionFrozenReadAndMutationLinearizeBehindFenceTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL lifecycle linearization race requires testcontainers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := r23StartGRPCSubscriptionPostgres(t, ctx)
	st := &store.SubscriptionStore{Pool: pool}
	svc := NewSubscriptionGRPC(st)
	spaceID, purchaserID, deletionID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, r23SeedSpacePro(ctx, pool, spaceID, purchaserID, "evt-linearize-seed"))
	manifest := r23Manifest(deletionID.String(), 1)
	r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest))
	r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, manifest))

	barrier, err := pool.Begin(ctx)
	require.NoError(t, err)
	var locked uuid.UUID
	require.NoError(t, barrier.QueryRow(ctx, `SELECT space_id FROM subscription_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&locked))
	require.Equal(t, spaceID, locked)

	freezeRequest := r23FenceRequest(spaceID, deletionID, 3, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	freezeCtx := r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, freezeRequest)
	freezeDone := make(chan error, 1)
	go func() {
		_, freezeErr := NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool}).ApplySpaceLifecycleFence(freezeCtx, freezeRequest)
		freezeDone <- freezeErr
	}()
	r23WaitForBlockedSubscriptionTransactions(t, ctx, pool, 1)

	type readResult struct {
		active bool
		err    error
	}
	readDone := make(chan readResult, 1)
	mutationDone := make(chan error, 1)
	go func() {
		active, readErr := st.HasActiveSpaceProForSpace(ctx, spaceID)
		readDone <- readResult{active: active, err: readErr}
	}()
	go func() {
		_, mutationErr := st.ActivateSpacePro(ctx, spaceID, purchaserID, "evt-linearize-race", []byte(`{"status":"active"}`))
		mutationDone <- mutationErr
	}()
	r23WaitForBlockedSubscriptionTransactions(t, ctx, pool, 3)
	require.NoError(t, barrier.Commit(ctx))
	require.NoError(t, <-freezeDone)
	read := <-readDone
	require.NoError(t, read.err)
	require.False(t, read.active, "entitlement read queued after FROZEN must observe FROZEN")
	require.Error(t, <-mutationDone, "mutation queued after FROZEN must reject in its own transaction")
	require.Equal(t, int64(0), r23RowCount(t, ctx, pool, "billing_events", "provider_event_id", "evt-linearize-race"), "rejected mutation cannot leave a billing side effect")
}

func TestR23SubscriptionPurgeAtomicCleanupPermanentDedupAndRestartReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL purge contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r23StartGRPCSubscriptionPostgres(t, ctx)
	st := &store.SubscriptionStore{Pool: pool}
	svc := NewSubscriptionGRPC(st)
	r23SetServiceDependency(t, svc, "provider renewal cancellation", &r23RenewalCanceller{})
	spaceID, purchaserID, deletionID := uuid.New(), uuid.New(), uuid.New()
	eventID := []byte("evt-purge-preserved")
	require.NoError(t, r23SeedSpacePro(ctx, pool, spaceID, purchaserID, string(eventID)))
	dedupDigest := r23ProviderWebhookDigest([]byte("fixture-provider-key"), "paddle", eventID)
	_, err := pool.Exec(ctx, `
INSERT INTO subscription_provider_event_fences(provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until)
VALUES('paddle',$1,now(),'space_pro_activated','fixture-v1','infinity')`, dedupDigest)
	require.NoError(t, err)
	manifest := r23Manifest(deletionID.String(), 1)
	r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 11, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest))
	r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 12, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, manifest))
	lateRestore := r23FenceRequest(spaceID, deletionID, 13, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, manifest)
	_, err = svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, lateRestore), lateRestore)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "PURGE_DECIDED is irreversible")

	purgeRequest := r23PurgeRequest(spaceID, deletionID, 12, manifest)
	accepted, err := svc.PurgeSpace(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, purgeRequest), purgeRequest)
	require.NoError(t, err)
	r23AssertPurgeReceipt(t, accepted.GetReceipt(), purgeRequest.GetPurge())
	require.Equal(t, int64(0), r23RowCount(t, ctx, pool, "space_subscriptions", "space_id", spaceID))
	var rawBillingRows int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT count(*) FROM billing_events
WHERE provider_event_id=$1 OR details @> '{"billing":"detail"}'::jsonb`, string(eventID)).Scan(&rawBillingRows))
	require.Zero(t, rawBillingRows, "actual purge must erase the raw provider_event_id and billing details instead of detaching them")
	require.Equal(t, int64(1), r23RowCount(t, ctx, pool, "subscription_provider_event_fences", "provider_event_hmac", dedupDigest), "billing cleanup must preserve provider idempotency authority")
	var boundedFullEvidence bool
	require.NoError(t, pool.QueryRow(ctx, `
SELECT request_bytes IS NOT NULL AND receipt_bytes IS NOT NULL
   AND retain_until = completed_at + interval '30 days'
FROM subscription_space_lifecycle_operations
WHERE space_id=$1 AND operation_kind='PURGE'`, spaceID).Scan(&boundedFullEvidence))
	require.True(t, boundedFullEvidence, "full purge request/receipt bytes retain exactly 30 days from participant completion")
	var compactPurged bool
	require.NoError(t, pool.QueryRow(ctx, `
SELECT state='PURGED' AND generation=$2 AND deletion_operation_id=$3
   AND manifest_sha256=$4 AND request_sha256=$5
FROM subscription_space_lifecycle_fences WHERE space_id=$1`, spaceID, purgeRequest.GetPurge().GetGeneration(), deletionID,
		purgeRequest.GetPurge().GetManifest().GetManifestSha256(), r23DomainHash(t, purgeRequest.GetPurge())).Scan(&compactPurged))
	require.True(t, compactPurged, "successful cleanup must first commit the compact permanent PURGED authority")
	_, err = pool.Exec(ctx, `UPDATE subscription_space_lifecycle_fences SET state='LIVE' WHERE space_id=$1`, spaceID)
	r23RequireGRPCSQLState(t, err, "55000")
	_, err = pool.Exec(ctx, `DELETE FROM subscription_space_lifecycle_fences WHERE space_id=$1`, spaceID)
	r23RequireGRPCSQLState(t, err, "55000")

	restarted := NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool})
	replayRequest := proto.Clone(purgeRequest).(*subscriptionv1.PurgeSpaceRequest)
	replayed, err := restarted.PurgeSpace(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, replayRequest), replayRequest)
	require.NoError(t, err)
	require.Equal(t, r23ProtoBytes(t, accepted), r23ProtoBytes(t, replayed), "exact purge replay survives response loss and process restart")

	changed := proto.Clone(purgeRequest).(*subscriptionv1.PurgeSpaceRequest)
	changed.Purge.Manifest.ItemCount++
	_, err = restarted.PurgeSpace(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, changed), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	_, err = st.ActivateSpacePro(ctx, spaceID, purchaserID, "evt-recreate", []byte(`{}`))
	require.Error(t, err, "permanent PURGED fence blocks identifier reuse before mutation")
}

func TestR23PaddleAndCloudPaymentsWebhookPathsFenceBeforeSideEffectsAndDenyPostPurgeReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL provider webhook contract requires testcontainers")
	}
	for _, provider := range []string{"paddle", "cloudpayments"} {
		t.Run(provider, func(t *testing.T) {
			ctx := context.Background()
			pool := r23StartGRPCSubscriptionPostgres(t, ctx)
			st := &store.SubscriptionStore{Pool: pool}
			svc := NewSubscriptionGRPC(st)
			spaceID, purchaserID, deletionID := uuid.New(), uuid.New(), uuid.New()
			key := []byte("r23-webhook-domain-key")
			keys := &r23ObservedProviderKeys{pool: pool, version: "kms-r23-active", key: key}
			r23SetServiceDependency(t, svc, "provider event HMAC key source", keys)
			r23SetServiceDependency(t, svc, "provider renewal cancellation", &r23RenewalCanceller{})

			var eventID, rawBody, signature string
			var cloudVerifier *r23CloudPaymentsVerifier
			if provider == "paddle" {
				rawBody, eventID = spaceProActivatedWebhookBody(t, spaceID, purchaserID)
				signature = signedWebhook(t, rawBody)
			} else {
				eventID = "cp_evt_" + uuid.NewString()
				rawBody = `{"transactionId":"` + eventID + `","status":"Completed"}`
				signature = "fixture-cloudpayments-signature"
				cloudVerifier = &r23CloudPaymentsVerifier{
					rawBody: rawBody, signature: signature, eventID: eventID,
					spaceID: spaceID.String(), purchaserID: purchaserID.String(),
				}
				r23SetServiceDependency(t, svc, "CloudPayments verification and decoding", cloudVerifier)
			}
			digest := r23ProviderWebhookDigest(key, provider, []byte(eventID))
			r23InstallProviderFenceBeforeBusinessWriteGuard(t, ctx, pool, provider, digest)
			if provider == "paddle" {
				_, err := svc.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{RawBody: rawBody, Signature: signature})
				require.NoError(t, err)
			} else {
				_, err := svc.HandleCloudPaymentsWebhook(ctx, &subscriptionv1.HandleCloudPaymentsWebhookRequest{RawBody: rawBody, Signature: signature})
				require.NoError(t, err)
			}
			require.Equal(t, []string{provider}, keys.providers)
			require.False(t, keys.sideEffectBeforeFirstKey, "canonical HMAC key must be obtained before any billing, entitlement, or dedup write")
			var firstSeen time.Time
			require.NoError(t, pool.QueryRow(ctx, `SELECT first_seen_at FROM subscription_provider_event_fences WHERE provider=$1 AND provider_event_hmac=$2`, provider, digest).Scan(&firstSeen))

			manifest := r23Manifest(deletionID.String(), 0)
			r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest))
			r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, manifest))
			purge := r23PurgeRequest(spaceID, deletionID, 2, manifest)
			_, err := svc.PurgeSpace(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, purge), purge)
			require.NoError(t, err)

			freshKeys := &r23ObservedProviderKeys{pool: pool, version: "kms-r23-active", key: key}
			fresh := NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool})
			r23SetServiceDependency(t, fresh, "provider event HMAC key source", freshKeys)
			if cloudVerifier != nil {
				freshVerifier := *cloudVerifier
				r23SetServiceDependency(t, fresh, "CloudPayments verification and decoding", &freshVerifier)
			}
			if provider == "paddle" {
				_, err = fresh.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{RawBody: rawBody, Signature: signature})
			} else {
				_, err = fresh.HandleCloudPaymentsWebhook(ctx, &subscriptionv1.HandleCloudPaymentsWebhookRequest{RawBody: rawBody, Signature: signature})
			}
			require.NoError(t, err, "permanent replay is an accepted no-op after purge")
			require.Equal(t, []string{provider}, freshKeys.providers, "fresh process must recompute the permanent lookup and cannot use process-local replay state")
			require.Equal(t, int64(0), r23RowCount(t, ctx, pool, "space_subscriptions", "space_id", spaceID), "post-purge replay cannot recreate entitlement")
			var replayFirstSeen time.Time
			require.NoError(t, pool.QueryRow(ctx, `SELECT first_seen_at FROM subscription_provider_event_fences WHERE provider=$1 AND provider_event_hmac=$2`, provider, digest).Scan(&replayFirstSeen))
			require.Equal(t, firstSeen, replayFirstSeen, "post-purge replay cannot rewrite permanent evidence")
		})
	}
}

func TestR23PaddleAndCloudPaymentsWebhookFenceAndBusinessEffectsRollbackTogether(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL webhook transaction rollback contract requires testcontainers")
	}
	for _, provider := range []string{"paddle", "cloudpayments"} {
		t.Run(provider, func(t *testing.T) {
			ctx := context.Background()
			pool := r23StartGRPCSubscriptionPostgres(t, ctx)
			spaceID, purchaserID := uuid.New(), uuid.New()
			key := []byte("r23-webhook-rollback-key")
			var eventID, rawBody, signature string
			var cloudVerifier *r23CloudPaymentsVerifier
			if provider == "paddle" {
				rawBody, eventID = spaceProActivatedWebhookBody(t, spaceID, purchaserID)
				signature = signedWebhook(t, rawBody)
			} else {
				eventID = "cp_rollback_" + uuid.NewString()
				rawBody = `{"transactionId":"` + eventID + `","status":"Completed"}`
				signature = "fixture-cloudpayments-signature"
				cloudVerifier = &r23CloudPaymentsVerifier{
					rawBody: rawBody, signature: signature, eventID: eventID,
					spaceID: spaceID.String(), purchaserID: purchaserID.String(),
				}
			}
			configure := func(svc *SubscriptionGRPC, keys *r23ObservedProviderKeys) {
				r23SetServiceDependency(t, svc, "provider event HMAC key source", keys)
				if cloudVerifier != nil {
					verifierCopy := *cloudVerifier
					r23SetServiceDependency(t, svc, "CloudPayments verification and decoding", &verifierCopy)
				}
			}
			call := func(svc *SubscriptionGRPC) error {
				if provider == "paddle" {
					_, err := svc.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{RawBody: rawBody, Signature: signature})
					return err
				}
				_, err := svc.HandleCloudPaymentsWebhook(ctx, &subscriptionv1.HandleCloudPaymentsWebhookRequest{RawBody: rawBody, Signature: signature})
				return err
			}

			_, err := pool.Exec(ctx, `
CREATE FUNCTION r23_fail_entitlement_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION USING ERRCODE='40001', MESSAGE='fixture entitlement mutation failure'; END $$;
CREATE TRIGGER r23_fail_entitlement_mutation BEFORE INSERT ON space_subscriptions
FOR EACH ROW EXECUTE FUNCTION r23_fail_entitlement_mutation()`)
			require.NoError(t, err)
			keys := &r23ObservedProviderKeys{pool: pool, version: "kms-r23-active", key: key}
			svc := NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool})
			configure(svc, keys)
			err = call(svc)
			require.Equal(t, codes.Internal, status.Code(err))
			digest := r23ProviderWebhookDigest(key, provider, []byte(eventID))
			require.Equal(t, int64(0), r23RowCount(t, ctx, pool, "subscription_provider_event_fences", "provider_event_hmac", digest), "failed entitlement mutation rolls back its HMAC fence")
			require.Equal(t, int64(0), r23RowCount(t, ctx, pool, "billing_events", "provider_event_id", eventID), "failed entitlement mutation rolls back billing detail")
			require.Equal(t, int64(0), r23RowCount(t, ctx, pool, "space_subscriptions", "space_id", spaceID))

			_, err = pool.Exec(ctx, `DROP TRIGGER r23_fail_entitlement_mutation ON space_subscriptions; DROP FUNCTION r23_fail_entitlement_mutation()`)
			require.NoError(t, err)
			freshKeys := &r23ObservedProviderKeys{pool: pool, version: "kms-r23-active", key: key}
			fresh := NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool})
			configure(fresh, freshKeys)
			require.NoError(t, call(fresh), "retry through a fresh service must be a first delivery because the failed transaction left no fence")
			require.Equal(t, int64(1), r23RowCount(t, ctx, pool, "subscription_provider_event_fences", "provider_event_hmac", digest))
			require.Equal(t, int64(1), r23RowCount(t, ctx, pool, "billing_events", "provider_event_id", eventID))
			require.Equal(t, int64(1), r23RowCount(t, ctx, pool, "space_subscriptions", "space_id", spaceID))
		})
	}
}

func TestR23SubscriptionConcurrentPurgeAndCleanupFailureConverge(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL purge race contract requires testcontainers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := r23StartGRPCSubscriptionPostgres(t, ctx)
	st := &store.SubscriptionStore{Pool: pool}
	svc := NewSubscriptionGRPC(st)
	canceller := &r23RenewalCanceller{}
	r23SetServiceDependency(t, svc, "provider renewal cancellation", canceller)
	spaceID, purchaserID, deletionID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, r23SeedSpacePro(ctx, pool, spaceID, purchaserID, "evt-race"))
	manifest := r23Manifest(deletionID.String(), 0)
	r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 20, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest))
	r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 21, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, manifest))
	purge := r23PurgeRequest(spaceID, deletionID, 21, manifest)

	_, err := pool.Exec(ctx, `
CREATE FUNCTION r23_reject_space_subscription_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fixture cleanup failure'; END $$;
CREATE TRIGGER r23_reject_space_subscription_delete BEFORE DELETE ON space_subscriptions
FOR EACH ROW EXECUTE FUNCTION r23_reject_space_subscription_delete()`)
	require.NoError(t, err)
	_, err = svc.PurgeSpace(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, purge), purge)
	require.Equal(t, codes.Internal, status.Code(err))
	require.Equal(t, int64(1), r23RowCount(t, ctx, pool, "space_subscriptions", "space_id", spaceID), "cleanup failure rolls back entitlement deletion")
	require.Equal(t, int64(0), r23TerminalPurgeOperationCount(t, ctx, pool, spaceID), "cleanup failure cannot manufacture a completion receipt")
	_, err = pool.Exec(ctx, `DROP TRIGGER r23_reject_space_subscription_delete ON space_subscriptions; DROP FUNCTION r23_reject_space_subscription_delete()`)
	require.NoError(t, err)

	type result struct {
		response *subscriptionv1.PurgeSpaceResponse
		err      error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	requests := []*subscriptionv1.PurgeSpaceRequest{
		proto.Clone(purge).(*subscriptionv1.PurgeSpaceRequest),
		proto.Clone(purge).(*subscriptionv1.PurgeSpaceRequest),
	}
	contexts := []context.Context{
		r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, requests[0]),
		r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, requests[1]),
	}
	services := []*SubscriptionGRPC{
		NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool}),
		NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool}),
	}
	for _, restarted := range services {
		r23SetServiceDependency(t, restarted, "provider renewal cancellation", canceller)
	}
	for i := 0; i < 2; i++ {
		index := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, callErr := services[index].PurgeSpace(contexts[index], requests[index])
			results <- result{response, callErr}
		}()
	}
	wg.Wait()
	close(results)
	var receipts [][]byte
	for got := range results {
		require.NoError(t, got.err)
		receipts = append(receipts, r23ProtoBytes(t, got.response))
	}
	require.Len(t, receipts, 2)
	require.Equal(t, receipts[0], receipts[1])
	require.Equal(t, int64(1), r23TerminalPurgeOperationCount(t, ctx, pool, spaceID))
	require.Equal(t, int64(0), r23RowCount(t, ctx, pool, "space_subscriptions", "space_id", spaceID))
}

func TestR23SubscriptionPurgePersistsProviderRenewalCancellationForDurableRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL provider cancellation retry contract requires testcontainers")
	}
	for _, provider := range []string{"paddle", "cloudpayments"} {
		t.Run(provider, func(t *testing.T) {
			ctx := context.Background()
			pool := r23StartGRPCSubscriptionPostgres(t, ctx)
			spaceID, purchaserID, deletionID := uuid.New(), uuid.New(), uuid.New()
			key := []byte("r23-cancellation-replay-key")
			var eventID, rawBody, signature string
			var cloudVerifier *r23CloudPaymentsVerifier
			if provider == "paddle" {
				rawBody, eventID = spaceProActivatedWebhookBody(t, spaceID, purchaserID)
				signature = signedWebhook(t, rawBody)
			} else {
				eventID = "cp_cancel_" + uuid.NewString()
				rawBody = `{"transactionId":"` + eventID + `","status":"Completed"}`
				signature = "fixture-cloudpayments-signature"
				cloudVerifier = &r23CloudPaymentsVerifier{
					rawBody: rawBody, signature: signature, eventID: eventID,
					spaceID: spaceID.String(), purchaserID: purchaserID.String(),
				}
			}
			callWebhook := func(svc *SubscriptionGRPC) error {
				if provider == "paddle" {
					_, err := svc.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{RawBody: rawBody, Signature: signature})
					return err
				}
				_, err := svc.HandleCloudPaymentsWebhook(ctx, &subscriptionv1.HandleCloudPaymentsWebhookRequest{RawBody: rawBody, Signature: signature})
				return err
			}
			configureWebhook := func(svc *SubscriptionGRPC) *r23ObservedProviderKeys {
				keys := &r23ObservedProviderKeys{pool: pool, version: "kms-r23-active", key: key}
				r23SetServiceDependency(t, svc, "provider event HMAC key source", keys)
				if cloudVerifier != nil {
					verifierCopy := *cloudVerifier
					r23SetServiceDependency(t, svc, "CloudPayments verification and decoding", &verifierCopy)
				}
				return keys
			}

			canceller := &r23RenewalCanceller{failuresRemaining: 1}
			svc := NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool})
			configureWebhook(svc)
			r23SetServiceDependency(t, svc, "provider renewal cancellation", canceller)
			require.NoError(t, callWebhook(svc))
			originalBilling := r23CaptureBillingEvidence(t, ctx, pool, provider, eventID)
			require.NotEqual(t, uuid.Nil, originalBilling.id)
			require.Equal(t, provider, originalBilling.provider)
			require.Equal(t, eventID, originalBilling.providerEventID)
			require.NotEmpty(t, originalBilling.details)

			manifest := r23Manifest(deletionID.String(), 0)
			r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest))
			r23ApplyFence(t, ctx, svc, r23FenceRequest(spaceID, deletionID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, manifest))
			purge := r23PurgeRequest(spaceID, deletionID, 2, manifest)

			_, err := svc.PurgeSpace(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, purge), purge)
			require.Equal(t, codes.Unavailable, status.Code(err), "transient provider cancellation must remain retryable")
			require.Equal(t, int64(1), r23RowCount(t, ctx, pool, "space_subscriptions", "space_id", spaceID), "local billing detail remains until provider cancellation succeeds")
			r23AssertBillingEvidenceUnchanged(t, ctx, pool, originalBilling)
			var retryState, retryKey string
			var attempts int
			var retryAtPresent, errorPresent bool
			require.NoError(t, pool.QueryRow(ctx, `
SELECT provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key,
       provider_cancel_next_attempt_at IS NOT NULL,provider_cancel_last_error IS NOT NULL
FROM subscription_space_lifecycle_operations
WHERE space_id=$1 AND deletion_operation_id=$2 AND operation_kind='PURGE'`, spaceID, deletionID).
				Scan(&retryState, &attempts, &retryKey, &retryAtPresent, &errorPresent))
			require.Equal(t, "RETRYABLE", retryState)
			require.Equal(t, 1, attempts)
			require.NotEmpty(t, retryKey)
			require.True(t, retryAtPresent)
			require.True(t, errorPresent)

			restarted := NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool})
			r23SetServiceDependency(t, restarted, "provider renewal cancellation", canceller)
			accepted, err := restarted.PurgeSpace(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, purge), purge)
			require.NoError(t, err)
			r23AssertPurgeReceipt(t, accepted.GetReceipt(), purge.GetPurge())
			require.Len(t, canceller.calls, 2)
			require.Equal(t, canceller.calls[0], canceller.calls[1], "provider, subscription and idempotency binding survive process restart")
			require.Equal(t, provider, canceller.calls[1].provider)
			require.Equal(t, retryKey, canceller.calls[1].idempotencyKey)
			require.Zero(t, r23RowCount(t, ctx, pool, "billing_events", "id", originalBilling.id), "successful cancellation retry must delete the captured billing row ID")
			require.Zero(t, r23AnyRawBillingEvidenceCount(t, ctx, pool, eventID), "successful cancellation retry must erase raw provider_event_id and payload")
			var completedState string
			var completedAttempts int
			var pendingRetryDetail bool
			require.NoError(t, pool.QueryRow(ctx, `
SELECT provider_cancel_state,provider_cancel_attempts,
       provider_cancel_next_attempt_at IS NOT NULL OR provider_cancel_last_error IS NOT NULL
FROM subscription_space_lifecycle_operations
WHERE space_id=$1 AND deletion_operation_id=$2 AND operation_kind='PURGE'`, spaceID, deletionID).
				Scan(&completedState, &completedAttempts, &pendingRetryDetail))
			require.Equal(t, "COMPLETED", completedState)
			require.Equal(t, 2, completedAttempts)
			require.False(t, pendingRetryDetail, "successful provider cancellation clears transient retry detail")

			fresh := NewSubscriptionGRPC(&store.SubscriptionStore{Pool: pool})
			freshKeys := configureWebhook(fresh)
			require.NoError(t, callWebhook(fresh), "post-PURGED provider replay through a fresh service/store is an accepted database-backed no-op")
			require.Equal(t, []string{provider}, freshKeys.providers)
			require.Zero(t, r23RowCount(t, ctx, pool, "billing_events", "id", originalBilling.id), "post-PURGED replay must keep the captured billing row ID absent")
			require.Zero(t, r23AnyRawBillingEvidenceCount(t, ctx, pool, eventID), "post-PURGED replay must keep raw provider_event_id and payload absent")
			require.Equal(t, int64(0), r23RowCount(t, ctx, pool, "space_subscriptions", "space_id", spaceID))
			require.Len(t, canceller.calls, 2, "post-PURGED provider replay cannot contact renewal cancellation again")
		})
	}
}

func TestR23SubscriptionServiceHasNoProviderFenceReadSurface(t *testing.T) {
	for _, method := range subscriptionv1.SubscriptionService_ServiceDesc.Methods {
		name := strings.ToLower(method.MethodName)
		require.NotContains(t, name, "providerfence")
		require.NotContains(t, name, "dedup")
	}
}

func r23StartGRPCSubscriptionPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return startSubscriptionPostgres(t, ctx)
}

func r23FenceRequest(spaceID, deletionID uuid.UUID, generation uint64, state commonv1.LifecycleFenceState, manifest *commonv1.ManifestBinding) *subscriptionv1.ApplySpaceLifecycleFenceRequest {
	return &subscriptionv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(),
		Generation: generation, DesiredState: state, Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding),
	}}
}

func r23PurgeRequest(spaceID, deletionID uuid.UUID, generation uint64, manifest *commonv1.ManifestBinding) *subscriptionv1.PurgeSpaceRequest {
	return &subscriptionv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: generation,
		PurgeDecidedAt: timestamppb.New(time.Date(2026, 9, 12, 12, 0, 0, 123000000, time.UTC)),
		ParticipantId:  commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
		Manifest:       proto.Clone(manifest).(*commonv1.ManifestBinding),
	}}
}

func r23Manifest(id string, count uint64) *commonv1.ManifestBinding {
	sum := sha256.Sum256([]byte("r23-manifest:" + id))
	return &commonv1.ManifestBinding{ManifestId: id, ManifestSha256: sum[:], ItemCount: count}
}

func r23Principal(kind, issuer, subject, audience, rpc, requestHash string) principal.Principal {
	return principal.Principal{Kind: kind, Issuer: issuer, Subject: subject, Audience: audience, RPC: rpc, RequestID: uuid.NewString(), RequestHash: requestHash}
}

func r23SpacePrincipal(t *testing.T, ctx context.Context, rpc string, request proto.Message) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	return principal.WithVerified(ctx, r23Principal("service", "space", "service:space", "subscription", rpc, hash))
}

func r23AssertFenceReceipt(t *testing.T, receipt *commonv1.SpaceLifecycleFenceReceipt, request *commonv1.SpaceLifecycleFenceRequest, state commonv1.LifecycleFenceState) {
	t.Helper()
	require.NotNil(t, receipt)
	require.Equal(t, uint32(1), receipt.GetProtocolVersion())
	require.NotEqual(t, uuid.Nil, uuid.MustParse(receipt.GetReceiptId()))
	require.Equal(t, request.GetSpaceId(), receipt.GetSpaceId())
	require.Equal(t, request.GetDeletionOperationId(), receipt.GetDeletionOperationId())
	require.Equal(t, request.GetGeneration(), receipt.GetGeneration())
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION, receipt.GetParticipantId())
	require.Equal(t, state, receipt.GetAppliedState())
	require.Equal(t, r23DomainHash(t, request), receipt.GetRequestSha256())
	require.Equal(t, request.GetManifest().GetManifestSha256(), receipt.GetManifestSha256())
	require.True(t, receipt.GetAppliedAt().IsValid())
}

func r23AssertPurgeReceipt(t *testing.T, receipt *commonv1.SpacePurgeReceipt, request *commonv1.SpacePurgeRequest) {
	t.Helper()
	require.NotNil(t, receipt)
	require.Equal(t, uint32(1), receipt.GetProtocolVersion())
	require.NotEqual(t, uuid.Nil, uuid.MustParse(receipt.GetReceiptId()))
	require.Equal(t, request.GetSpaceId(), receipt.GetSpaceId())
	require.Equal(t, request.GetDeletionOperationId(), receipt.GetDeletionOperationId())
	require.Equal(t, request.GetGeneration(), receipt.GetGeneration())
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION, receipt.GetParticipantId())
	require.Equal(t, commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, receipt.GetState())
	require.Equal(t, r23DomainHash(t, request), receipt.GetRequestSha256())
	require.True(t, receipt.GetCompletedAt().IsValid())
}

func r23DomainHash(t *testing.T, message proto.Message) []byte {
	t.Helper()
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	require.NoError(t, err)
	h := sha256.New()
	_, _ = h.Write([]byte(message.ProtoReflect().Descriptor().FullName()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(wire)
	return h.Sum(nil)
}

func r23ProtoBytes(t *testing.T, message proto.Message) []byte {
	t.Helper()
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	require.NoError(t, err)
	return wire
}

func r23ApplyFence(t *testing.T, ctx context.Context, svc *SubscriptionGRPC, request *subscriptionv1.ApplySpaceLifecycleFenceRequest) {
	t.Helper()
	response, err := svc.ApplySpaceLifecycleFence(r23SpacePrincipal(t, ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, request), request)
	require.NoError(t, err)
	require.NotNil(t, response.GetReceipt())
}

func r23SeedSpacePro(ctx context.Context, pool *pgxpool.Pool, spaceID, purchaserID uuid.UUID, eventID string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	subscriptionID := uuid.New()
	_, err = tx.Exec(ctx, `
INSERT INTO space_subscriptions(id,space_id,purchaser_account_id,plan,billing_period,status,provider,provider_subscription_id,current_period_start,current_period_end)
VALUES($1,$2,$3,'space_pro','monthly','active','paddle',$4,now(),now()+interval '30 days')`, subscriptionID, spaceID, purchaserID, "sub_"+eventID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO billing_events(space_subscription_id,type,provider,provider_event_id,details)
VALUES($1,'subscription_created','paddle',$2,$3::jsonb)`, subscriptionID, eventID, `{"billing":"detail"}`)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func r23RowCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, column string, value any) int64 {
	t.Helper()
	query := "SELECT count(*) FROM " + table
	args := []any(nil)
	if value != nil {
		query += " WHERE " + column + "=$1"
		args = append(args, value)
	} else if column != "" {
		query += " WHERE " + column + " IS NOT NULL"
	}
	var count int64
	require.NoError(t, pool.QueryRow(ctx, query, args...).Scan(&count))
	return count
}

type r23BillingEvidence struct {
	id              uuid.UUID
	provider        string
	providerEventID string
	details         []byte
}

func r23CaptureBillingEvidence(t *testing.T, ctx context.Context, pool *pgxpool.Pool, provider, eventID string) r23BillingEvidence {
	t.Helper()
	var evidence r23BillingEvidence
	var details string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT id,provider,provider_event_id,details::text
FROM billing_events
WHERE provider=$1 AND provider_event_id=$2`, provider, eventID).
		Scan(&evidence.id, &evidence.provider, &evidence.providerEventID, &details))
	evidence.details = []byte(details)
	return evidence
}

func r23AssertBillingEvidenceUnchanged(t *testing.T, ctx context.Context, pool *pgxpool.Pool, expected r23BillingEvidence) {
	t.Helper()
	var actual r23BillingEvidence
	var details string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT id,provider,provider_event_id,details::text
FROM billing_events
WHERE id=$1`, expected.id).
		Scan(&actual.id, &actual.provider, &actual.providerEventID, &details))
	actual.details = []byte(details)
	require.Equal(t, expected, actual, "transient cancellation failure must preserve the same billing row, exact provider_event_id and byte-identical stored details")
}

func r23AnyRawBillingEvidenceCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, eventID string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT count(*) FROM billing_events
WHERE provider_event_id=$1 OR details::text LIKE '%' || $1 || '%'`, eventID).Scan(&count))
	return count
}

func r23TerminalPurgeOperationCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, spaceID uuid.UUID) int64 {
	t.Helper()
	var count int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_space_lifecycle_operations WHERE space_id=$1 AND operation_kind='PURGE' AND terminal_state='COMPLETED'`, spaceID).Scan(&count))
	return count
}

type r23ObservedProviderKeys struct {
	mu                       sync.Mutex
	pool                     *pgxpool.Pool
	version                  string
	key                      []byte
	providers                []string
	sideEffectBeforeFirstKey bool
}

// CurrentProviderEventHMACKey is the test-only key-provider seam expected by
// both real webhook handlers. The key bytes never enter provider payloads.
func (k *r23ObservedProviderKeys) CurrentProviderEventHMACKey(ctx context.Context, provider string) (string, []byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.providers) == 0 {
		var rows int64
		if err := k.pool.QueryRow(ctx, `
SELECT (SELECT count(*) FROM billing_events)
     + (SELECT count(*) FROM space_subscriptions)
     + (SELECT count(*) FROM subscription_provider_event_fences)`).Scan(&rows); err != nil {
			return "", nil, err
		}
		k.sideEffectBeforeFirstKey = rows != 0
	}
	k.providers = append(k.providers, provider)
	return k.version, append([]byte(nil), k.key...), nil
}

type r23CloudPaymentsVerifier struct {
	rawBody     string
	signature   string
	eventID     string
	spaceID     string
	purchaserID string
}

// VerifyAndDecodeCloudPaymentsWebhook keeps provider-specific verification at
// the boundary while returning only canonical fields used by Subscription.
func (v *r23CloudPaymentsVerifier) VerifyAndDecodeCloudPaymentsWebhook(_ context.Context, rawBody, signature string) (eventID, eventType, spaceID, purchaserAccountID string, details []byte, err error) {
	if rawBody != v.rawBody || signature != v.signature {
		return "", "", "", "", nil, errors.New("cloudpayments fixture verification failed")
	}
	return v.eventID, "subscription.activated", v.spaceID, v.purchaserID, []byte(rawBody), nil
}

type r23RenewalCancellationCall struct {
	provider               string
	providerSubscriptionID string
	idempotencyKey         string
}

type r23RenewalCanceller struct {
	mu                sync.Mutex
	failuresRemaining int
	calls             []r23RenewalCancellationCall
}

// CancelSpaceRenewal is an injected, credential-free seam. Tests never call a
// live provider and require a stable idempotency binding across durable retry.
func (c *r23RenewalCanceller) CancelSpaceRenewal(_ context.Context, provider, providerSubscriptionID, idempotencyKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, r23RenewalCancellationCall{provider: provider, providerSubscriptionID: providerSubscriptionID, idempotencyKey: idempotencyKey})
	if c.failuresRemaining > 0 {
		c.failuresRemaining--
		return errors.New("transient provider cancellation failure")
	}
	return nil
}

func r23SetServiceDependency(t *testing.T, svc *SubscriptionGRPC, contract string, dependency any) {
	t.Helper()
	service := reflect.ValueOf(svc).Elem()
	value := reflect.ValueOf(dependency)
	matches := 0
	for i := 0; i < service.NumField(); i++ {
		field := service.Field(i)
		if field.CanSet() && value.Type().AssignableTo(field.Type()) {
			field.Set(value)
			matches++
		}
	}
	require.Equal(t, 1, matches, "production SubscriptionGRPC injectable %s seam is absent or ambiguous for %T", contract, dependency)
}

func r23ProviderWebhookDigest(key []byte, provider string, eventID []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("voice-subscription-provider-dedup-v1"))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(strings.ToLower(provider)))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(eventID)
	return mac.Sum(nil)
}

func r23InstallProviderFenceBeforeBusinessWriteGuard(t *testing.T, ctx context.Context, pool *pgxpool.Pool, provider string, digest []byte) {
	t.Helper()
	_, err := pool.Exec(ctx, `
CREATE TABLE r23_expected_provider_fence(provider text NOT NULL,digest bytea NOT NULL)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO r23_expected_provider_fence(provider,digest) VALUES($1,$2)`, provider, digest)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
CREATE FUNCTION r23_require_provider_fence_before_business_write() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM r23_expected_provider_fence expected
    JOIN subscription_provider_event_fences fence
      ON fence.provider=expected.provider AND fence.provider_event_hmac=expected.digest
  ) THEN
    RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='business write overtook permanent provider fence';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER r23_billing_requires_provider_fence BEFORE INSERT ON billing_events
FOR EACH ROW EXECUTE FUNCTION r23_require_provider_fence_before_business_write();
CREATE TRIGGER r23_entitlement_requires_provider_fence BEFORE INSERT ON space_subscriptions
FOR EACH ROW EXECUTE FUNCTION r23_require_provider_fence_before_business_write();`)
	require.NoError(t, err)
}

func r23WaitForBlockedSubscriptionTransactions(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int) {
	t.Helper()
	// The barrier, fence transition, queued read, and queued mutation occupy all
	// four connections in pgxpool's default configuration. Querying
	// pg_stat_activity through that pool would then wait for a connection instead
	// of observing those three PostgreSQL lock waiters.
	monitor, err := pgx.Connect(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer monitor.Close(context.Background())

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked int
		err = monitor.QueryRow(ctx, `
SELECT count(*) FROM pg_stat_activity
WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'`).Scan(&blocked)
		require.NoError(t, err)
		if blocked >= want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("only %d of %d Subscription transactions reached the lifecycle lock", blocked, want)
		case <-ticker.C:
		}
	}
}

func r23RequireGRPCSQLState(t *testing.T, err error, state string) {
	t.Helper()
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected PostgreSQL error, got %T: %v", err, err)
	require.Equal(t, state, pgErr.Code)
}
