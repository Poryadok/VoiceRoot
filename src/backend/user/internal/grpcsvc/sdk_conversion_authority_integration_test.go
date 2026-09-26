package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"voice/backend/pkg/principal"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func sdkVerifiedAuth(t *testing.T, ctx context.Context, rpc string, req proto.Message) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(ctx, principal.Principal{
		Kind: "service", Issuer: "auth", Subject: "service:auth",
		Audience: "user", RPC: rpc, RequestHash: hash,
	})
}

func sdkTombstoneTestHash(t *testing.T, req *userv1.RecordSdkAuthorTombstoneRequest) string {
	t.Helper()
	// Struct field order is the canonical ASCII key order, with no request_hash.
	canonical := struct {
		ExpectedProfileRevision uint64 `json:"expected_profile_revision"`
		FreezeReceiptID         string `json:"freeze_receipt_id"`
		FrozenAuthorityEpoch    uint64 `json:"frozen_authority_epoch"`
		FrozenBindingID         string `json:"frozen_binding_id"`
		OperationID             string `json:"operation_id"`
		SourceAccountID         string `json:"source_account_id"`
		SourceActorID           string `json:"source_actor_id"`
		TargetAccountID         string `json:"target_account_id"`
		TargetProfileID         string `json:"target_profile_id"`
		Version                 uint32 `json:"version"`
	}{
		ExpectedProfileRevision: req.GetExpectedProfileRevision(),
		FreezeReceiptID:         req.GetFreezeReceiptId(),
		FrozenAuthorityEpoch:    req.GetFrozenAuthorityEpoch(),
		FrozenBindingID:         req.GetFrozenBindingId(),
		OperationID:             req.GetOperationId(),
		SourceAccountID:         req.GetSourceAccountId(),
		SourceActorID:           req.GetSourceActorId(),
		TargetAccountID:         req.GetTargetAccountId(),
		TargetProfileID:         req.GetTargetProfileId(),
		Version:                 req.GetVersion(),
	}
	encoded, err := json.Marshal(canonical)
	require.NoError(t, err)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// The conversion preview and code exchange must use a read-only, exact-pair
// lookup. In particular, looking up a nonexistent account must not provision it.
func TestGetSdkProfileEligibility_ExactOwnerLifecycleAndRevision(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	domain := &SdkConversionGRPC{Profiles: store.NewProfileStore(pool)}
	owner, other, profile := uuid.New(), uuid.New(), uuid.New()
	call := func(req *userv1.GetSdkProfileEligibilityRequest) (*userv1.GetSdkProfileEligibilityResponse, error) {
		t.Helper()
		return domain.GetSdkProfileEligibility(sdkVerifiedAuth(t, ctx,
			userv1.UserService_GetSdkProfileEligibility_FullMethodName, req), req)
	}

	var before int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM profiles").Scan(&before))
	_, err := call(&userv1.GetSdkProfileEligibilityRequest{
		AccountId: owner.String(), ProfileId: profile.String(),
	})
	require.Equal(t, codes.NotFound, status.Code(err))
	var after int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM profiles").Scan(&after))
	require.Equal(t, before, after, "eligibility lookup must not provision a profile")

	_, err = pool.Exec(ctx, `INSERT INTO profiles
		(id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'sdkeligible', '0001', 'Eligible', true)`, profile, owner)
	require.NoError(t, err)
	_, err = call(&userv1.GetSdkProfileEligibilityRequest{
		AccountId: other.String(), ProfileId: profile.String(),
	})
	require.Equal(t, codes.NotFound, status.Code(err), "foreign profile ownership must not be disclosed")

	lookup := func() *userv1.GetSdkProfileEligibilityResponse {
		t.Helper()
		got, lookupErr := call(&userv1.GetSdkProfileEligibilityRequest{
			AccountId: owner.String(), ProfileId: profile.String(),
		})
		require.NoError(t, lookupErr)
		require.Equal(t, owner.String(), got.GetAccountId())
		require.Equal(t, profile.String(), got.GetProfileId())
		require.Positive(t, got.GetProfileRevision())
		return got
	}
	initial := lookup()
	require.False(t, initial.GetDeleted())
	require.False(t, initial.GetFrozen())

	cli, _ := startUserGRPCForPhase13(t, store.NewProfileStore(pool), nil)
	_, err = cli.UpdateProfile(withAccountTier(ctx, owner, "free"), &userv1.UpdateProfileRequest{
		ProfileId: profile.String(), DisplayName: proto.String("Eligible II"),
	})
	require.NoError(t, err)
	updated := lookup()
	require.Greater(t, updated.GetProfileRevision(), initial.GetProfileRevision())

	_, err = pool.Exec(ctx, "UPDATE profiles SET frozen_at = now() WHERE id = $1", profile)
	require.NoError(t, err)
	frozen := lookup()
	require.True(t, frozen.GetFrozen())
	require.False(t, frozen.GetDeleted())
	require.Greater(t, frozen.GetProfileRevision(), updated.GetProfileRevision())

	_, err = pool.Exec(ctx, "UPDATE profiles SET deleted_at = now() WHERE id = $1", profile)
	require.NoError(t, err)
	deleted := lookup()
	require.True(t, deleted.GetDeleted())
	require.True(t, deleted.GetFrozen())
	require.Greater(t, deleted.GetProfileRevision(), frozen.GetProfileRevision())
}

func TestGetSdkProfileEligibility_RequiresExactAuthWorkloadCaller(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	cli, _ := startUserGRPCForPhase13(t, store.NewProfileStore(pool), nil)
	domain := &SdkConversionGRPC{Profiles: store.NewProfileStore(pool)}
	req := &userv1.GetSdkProfileEligibilityRequest{AccountId: uuid.NewString(), ProfileId: uuid.NewString()}
	// The ordinary listener never exposes either conversion RPC, even with a
	// legacy internal caller marker.
	marker := metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, "auth")
	_, err := cli.GetSdkProfileEligibility(marker, req)
	require.Equal(t, codes.Unimplemented, status.Code(err))
	_, err = cli.RecordSdkAuthorTombstone(marker, &userv1.RecordSdkAuthorTombstoneRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err))

	_, err = domain.GetSdkProfileEligibility(marker, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	valid := sdkVerifiedAuth(t, ctx, userv1.UserService_GetSdkProfileEligibility_FullMethodName, req)
	wrongIssuer, ok := principal.FromContext(valid)
	require.True(t, ok)
	wrongIssuer.Issuer = "social"
	_, err = domain.GetSdkProfileEligibility(principal.WithVerified(ctx, wrongIssuer), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	wrongRPC := wrongIssuer
	wrongRPC.Issuer = "auth"
	wrongRPC.RPC = userv1.UserService_RecordSdkAuthorTombstone_FullMethodName
	_, err = domain.GetSdkProfileEligibility(principal.WithVerified(ctx, wrongRPC), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	changedBody := proto.Clone(req).(*userv1.GetSdkProfileEligibilityRequest)
	changedBody.ProfileId = uuid.NewString()
	_, err = domain.GetSdkProfileEligibility(valid, changedBody)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	badID := &userv1.GetSdkProfileEligibilityRequest{
		AccountId: "bad", ProfileId: uuid.NewString(),
	}
	_, err = domain.GetSdkProfileEligibility(sdkVerifiedAuth(t, ctx,
		userv1.UserService_GetSdkProfileEligibility_FullMethodName, badID), badID)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// SourceActorId is an Auth SDK actor alias. User deliberately has no source
// profile to look up or rewrite when it preserves historical attribution.
func TestRecordSdkAuthorTombstone_ImmutableReceiptAndFencedIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	domain := &SdkConversionGRPC{Profiles: store.NewProfileStore(pool)}
	record := func(req *userv1.RecordSdkAuthorTombstoneRequest) (*userv1.RecordSdkAuthorTombstoneResponse, error) {
		t.Helper()
		return domain.RecordSdkAuthorTombstone(sdkVerifiedAuth(t, ctx,
			userv1.UserService_RecordSdkAuthorTombstone_FullMethodName, req), req)
	}
	targetAccount, targetProfile := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles
		(id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'sdktarget', '0001', 'Target', true)`, targetProfile, targetAccount)
	require.NoError(t, err)
	eligibilityReq := &userv1.GetSdkProfileEligibilityRequest{
		AccountId: targetAccount.String(), ProfileId: targetProfile.String(),
	}
	eligibility, err := domain.GetSdkProfileEligibility(sdkVerifiedAuth(t, ctx,
		userv1.UserService_GetSdkProfileEligibility_FullMethodName, eligibilityReq), eligibilityReq)
	require.NoError(t, err)

	req := &userv1.RecordSdkAuthorTombstoneRequest{
		Version: 1, OperationId: uuid.NewString(),
		SourceAccountId: uuid.NewString(), SourceActorId: uuid.NewString(),
		TargetAccountId: targetAccount.String(), TargetProfileId: targetProfile.String(),
		ExpectedProfileRevision: eligibility.GetProfileRevision(),
		FrozenBindingId:         uuid.NewString(), FrozenAuthorityEpoch: 3,
		FreezeReceiptId: uuid.NewString(),
	}
	req.RequestHash = sdkTombstoneTestHash(t, req)
	_, err = domain.RecordSdkAuthorTombstone(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	wrong := sdkVerifiedAuth(t, ctx, userv1.UserService_RecordSdkAuthorTombstone_FullMethodName, req)
	wrongPrincipal, ok := principal.FromContext(wrong)
	require.True(t, ok)
	wrongPrincipal.Issuer = "social"
	_, err = domain.RecordSdkAuthorTombstone(principal.WithVerified(ctx, wrongPrincipal), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	wrongPrincipal.Issuer = "auth"
	wrongPrincipal.RPC = userv1.UserService_GetSdkProfileEligibility_FullMethodName
	_, err = domain.RecordSdkAuthorTombstone(principal.WithVerified(ctx, wrongPrincipal), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	wrongBody := proto.Clone(req).(*userv1.RecordSdkAuthorTombstoneRequest)
	wrongBody.SourceActorId = uuid.NewString()
	wrongBody.RequestHash = sdkTombstoneTestHash(t, wrongBody)
	_, err = domain.RecordSdkAuthorTombstone(wrong, wrongBody)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	badHash := proto.Clone(req).(*userv1.RecordSdkAuthorTombstoneRequest)
	badHash.RequestHash = strings.Repeat("b", 64)
	_, err = record(badHash)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "request_hash must match canonical JSON")

	first, err := record(req)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.GetVersion())
	require.NotEmpty(t, first.GetReceiptId())
	require.Equal(t, req.GetOperationId(), first.GetOperationId())
	require.Equal(t, req.GetSourceAccountId(), first.GetSourceAccountId())
	require.Equal(t, req.GetSourceActorId(), first.GetSourceActorId())
	require.Equal(t, req.GetTargetAccountId(), first.GetTargetAccountId())
	require.Equal(t, req.GetTargetProfileId(), first.GetTargetProfileId())
	require.Equal(t, req.GetFrozenBindingId(), first.GetFrozenBindingId())
	require.Equal(t, req.GetFrozenAuthorityEpoch(), first.GetFrozenAuthorityEpoch())
	require.Equal(t, req.GetFreezeReceiptId(), first.GetFreezeReceiptId())
	require.Equal(t, req.GetRequestHash(), first.GetRequestHash())
	require.Equal(t, req.GetExpectedProfileRevision(), first.GetProfileRevision())
	require.Positive(t, first.GetTombstoneRevision())
	require.NotNil(t, first.GetCommittedAt())
	_, err = pool.Exec(ctx, `UPDATE sdk_author_tombstones SET request_hash = $1 WHERE receipt_id = $2`, strings.Repeat("0", 64), first.GetReceiptId())
	require.Error(t, err, "committed author receipts must reject mutation")
	_, err = pool.Exec(ctx, `DELETE FROM sdk_author_tombstones WHERE receipt_id = $1`, first.GetReceiptId())
	require.Error(t, err, "committed author receipts must reject deletion")
	_, err = pool.Exec(ctx, `TRUNCATE TABLE sdk_author_tombstones`)
	require.Error(t, err, "committed author receipts must reject truncation")

	// A committed receipt is stable after a later profile change. Replay does
	// not demand the original expected revision still be current.
	cli, _ := startUserGRPCForPhase13(t, store.NewProfileStore(pool), nil)
	_, err = cli.UpdateProfile(withAccountTier(ctx, targetAccount, "free"), &userv1.UpdateProfileRequest{
		ProfileId: targetProfile.String(), DisplayName: proto.String("Target II"),
	})
	require.NoError(t, err)
	replay, err := record(proto.Clone(req).(*userv1.RecordSdkAuthorTombstoneRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(first, replay), "same operation and hash must return the identical receipt")

	changed := func(edit func(*userv1.RecordSdkAuthorTombstoneRequest)) {
		t.Helper()
		conflict := proto.Clone(req).(*userv1.RecordSdkAuthorTombstoneRequest)
		edit(conflict)
		conflict.RequestHash = sdkTombstoneTestHash(t, conflict)
		_, callErr := record(conflict)
		require.Error(t, callErr)
	}
	changed(func(r *userv1.RecordSdkAuthorTombstoneRequest) { r.SourceActorId = uuid.NewString() })
	changed(func(r *userv1.RecordSdkAuthorTombstoneRequest) { r.TargetAccountId = uuid.NewString() })
	changed(func(r *userv1.RecordSdkAuthorTombstoneRequest) { r.TargetProfileId = uuid.NewString() })
	changed(func(r *userv1.RecordSdkAuthorTombstoneRequest) { r.FrozenAuthorityEpoch++ })
	changed(func(r *userv1.RecordSdkAuthorTombstoneRequest) { r.ExpectedProfileRevision++ })

	stale := proto.Clone(req).(*userv1.RecordSdkAuthorTombstoneRequest)
	stale.OperationId = uuid.NewString()
	stale.RequestHash = sdkTombstoneTestHash(t, stale)
	_, err = record(stale)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	invalidActor := proto.Clone(req).(*userv1.RecordSdkAuthorTombstoneRequest)
	invalidActor.OperationId = uuid.NewString()
	invalidActor.SourceActorId = "not-an-actor-uuid"
	invalidActor.RequestHash = sdkTombstoneTestHash(t, invalidActor)
	_, err = record(invalidActor)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
