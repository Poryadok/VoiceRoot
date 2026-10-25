//go:build spacebanintegration

package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
	"voice/backend/space/internal/lifecycleprincipal"
)

// Component fixtures deliberately control User facts and Role observations.
// They are not B15 protected-provider/production qualification.
type nativeBanCaptureFixture func(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error)

func (f nativeBanCaptureFixture) CaptureSpaceMemberAccountBindings(ctx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	return f(ctx, r)
}
func nativeBanProfile(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("10000000-0000-0000-0000-%012x", n))
}
func nativeBanAccount(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("20000000-0000-0000-0000-%012x", n))
}
func nativeBanSpace(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("30000000-0000-0000-0000-%012x", n))
}

func nativeBanOwnerFixture(r *userv1.CaptureSpaceMemberAccountBindingsRequest) *userv1.CaptureSpaceMemberAccountBindingsResponse {
	out := &userv1.CaptureSpaceMemberAccountBindingsResponse{Protocol: 1, OperationId: r.OperationId, SpaceId: r.SpaceId, Purpose: r.Purpose, CohortDigest: append([]byte(nil), r.CohortDigest...), RequestDigest: spaceaccountcapture.RequestDigest(r), OwnerSourceRevision: 901, UserSourceGeneration: "50000000-0000-0000-0000-000000000001", IdentityCut: &userv1.SpaceAccountIdentityCut{Owner: "user", Stream: "identity-association", HistoryGeneration: "50000000-0000-0000-0000-000000000002", ThroughRevision: 3, KeysetRevision: 1, Digest: bytes.Repeat([]byte{1}, 32)}}
	for _, entry := range r.Cohort.Entries {
		account := nativeBanAccount(2)
		if entry.ProfileId == nativeBanProfile(1).String() {
			account = nativeBanAccount(1)
		}
		if entry.ProfileId == nativeBanProfile(6).String() {
			account = nativeBanAccount(3)
		}
		association := uuid.NewSHA1(uuid.NameSpaceOID, []byte("component-user-association:"+entry.ProfileId))
		receipt := uuid.NewSHA1(uuid.NameSpaceOID, []byte("component-user-receipt:"+entry.ProfileId))
		digest := sha256.Sum256([]byte("component-user-receipt:" + entry.ProfileId))
		out.Results = append(out.Results, &userv1.SpaceAccountCaptureResult{Entry: proto.Clone(entry).(*userv1.SpaceAccountCaptureEntry), Outcome: 1, AccountId: account.String(), AssociationId: association.String(), AssociationRevision: 1, ProfileRevision: 7, Origin: 1, AssociationOrigin: 2, OwnerReceiptId: receipt.String(), OwnerReceiptDigest: digest[:], CaptureCut: timestamppb.Now(), Frozen: account == nativeBanAccount(2), Deleted: account == nativeBanAccount(2)})
	}
	return out
}

func nativeBanIntentField(t *testing.T, r proto.Message, operation uuid.UUID) {
	t.Helper()
	field := r.ProtoReflect().Descriptor().Fields().ByName("operation_id")
	require.NotNil(t, field, "actual reviewed Producer13 SDK must be in the qualified input; no invented descriptor")
	r.ProtoReflect().Set(field, protoreflect.ValueOfString(operation.String()))
}

// This is the real receiver/signature path with controlled Auth dependency,
// rather than a public constructor for VerifiedAdmissionCall.
func nativeBanCall(t *testing.T, r proto.Message, actor uuid.UUID) (context.Context, *lifecycleprincipal.VerifiedAdmissionCall) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	method := spacev1.SpaceService_BanMember_FullMethodName
	if _, ok := r.(*spacev1.UnbanMemberRequest); ok {
		method = spacev1.SpaceService_UnbanMember_FullMethodName
	}
	account := nativeBanAccount(1)
	if actor == nativeBanProfile(6) {
		account = nativeBanAccount(3)
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	transport := uuid.NewString()
	field := r.ProtoReflect().Descriptor().Fields().ByName("operation_id")
	require.NotNil(t, field)
	require.NotEqual(t, r.ProtoReflect().Get(field).String(), transport, "logical intent must differ from fresh signed transport ID")
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: transport, RequestHash: hash, AccountID: account.String(), ProfileID: actor.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", transport))
	v := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	var call *lifecycleprincipal.VerifiedAdmissionCall
	var verified context.Context
	_, err = v.Unary()(ctx, r, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		verified = ctx
		var err error
		if method == spacev1.SpaceService_BanMember_FullMethodName {
			call, err = lifecycleprincipal.RequireModerationBan(ctx, r)
		} else {
			call, err = lifecycleprincipal.RequireModerationUnban(ctx, r)
		}
		return nil, err
	})
	require.NoError(t, err)
	return verified, call
}

func nativeBanExecute(t *testing.T, st *SpaceStore, space, account, actor, operation uuid.UUID, hint *uuid.UUID, authorize func(context.Context) error) error {
	t.Helper()
	return nativeBanExecuteReason(t, st, space, account, actor, operation, hint, nil, authorize)
}

func nativeBanExecuteReason(t *testing.T, st *SpaceStore, space, account, actor, operation uuid.UUID, hint *uuid.UUID, reason *string, authorize func(context.Context) error) error {
	t.Helper()
	r := &spacev1.BanMemberRequest{SpaceId: space.String(), AccountId: account.String(), Reason: reason}
	if hint != nil {
		value := hint.String()
		r.ProfileId = &value
	}
	nativeBanIntentField(t, r, operation)
	ctx, call := nativeBanCall(t, r, actor)
	return st.BanAccount(ctx, AccountBanInput{SpaceID: space, AccountID: account, ActorProfileID: actor, ProfileHint: hint, Reason: reason, InitiatingCall: call, Request: r, Authorize: authorize})
}

func nativeBanAssertNoEffect(t *testing.T, pool *pgxpool.Pool, space uuid.UUID) {
	t.Helper()
	var bans, audit int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM space_bans WHERE space_id=$1`, space).Scan(&bans))
	require.Zero(t, bans)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space).Scan(&audit))
	require.Zero(t, audit)
}

func nativeUnbanExecute(t *testing.T, st *SpaceStore, space, account, operation uuid.UUID, authorize func(context.Context) error) error {
	t.Helper()
	r := &spacev1.UnbanMemberRequest{SpaceId: space.String(), AccountId: account.String()}
	nativeBanIntentField(t, r, operation)
	ctx, call := nativeBanCall(t, r, nativeBanProfile(1))
	return st.UnbanAccount(ctx, call, r, authorize)
}

func TestAccountBanNative50(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "fresh real45-transition50 seeded fixture required")
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	admin := &SpaceStore{Pool: owner}
	for i := 1; i <= 24; i++ {
		space := nativeBanSpace(i)
		require.NoError(t, admin.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error { return scoped.InitializeScopedNativeMembership(ctx, space) }), "actual installer enrollment must precede serving grants")
	}
	serving := projectionServingLogin(t, ctx, owner)
	st := &SpaceStore{Pool: serving, AccountCaptures: nativeBanCaptureFixture(func(ctx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		return nativeBanOwnerFixture(r), nil
	})}
	allow := func(context.Context) error { return nil }

	t.Run("complete positive legacy pending and community cohort", func(t *testing.T) {
		space := nativeBanSpace(1)
		for _, n := range []int{7, 8} {
			intent, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space, ProfileID: nativeBanProfile(n), AccountID: nativeBanAccount(2)})
			require.NoError(t, err)
			role := uuid.New()
			receipt, err := memberadmission.EnsureReceiptFingerprint(intent.EnsureBinding(), role.String())
			require.NoError(t, err)
			_, err = st.PublishMemberAdmission(ctx, intent.Input.OperationID, role, receipt[:], nil)
			require.NoError(t, err, "actual noninteractive prepare/publish; controlled Role receipt is component-only")
		}
		pending, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space, ProfileID: nativeBanProfile(4), AccountID: nativeBanAccount(2)})
		require.NoError(t, err)
		var observed atomic.Int32
		st.AccountCaptures = nativeBanCaptureFixture(func(callCtx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
			observed.Add(1)
			require.Len(t, r.Cohort.Entries, 9, "all ordinary, preparing and community sources, including dual source")
			conn, err := serving.Acquire(callCtx)
			if err != nil {
				return nil, err
			}
			defer conn.Release()
			tx, err := conn.Begin(callCtx)
			if err != nil {
				return nil, err
			}
			defer tx.Rollback(callCtx)
			var free bool
			err = tx.QueryRow(callCtx, `SELECT pg_try_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(space)).Scan(&free)
			if err != nil {
				return nil, err
			}
			require.True(t, free, "owner RPC is outside every Space fence")
			var generation int
			var legacy bool
			require.NoError(t, tx.QueryRow(callCtx, `SELECT generation,legacy FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space, nativeBanProfile(2)).Scan(&generation, &legacy))
			require.Zero(t, generation)
			require.True(t, legacy, "qualification has not rewritten LEGACY0")
			return nativeBanOwnerFixture(r), nil
		})
		operation := uuid.New()
		reason := "repeated spam"
		require.NoError(t, nativeBanExecuteReason(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, &reason, allow))
		require.EqualValues(t, 1, observed.Load())
		var count int
		require.NoError(t, serving.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space).Scan(&count))
		require.Equal(t, 2, count, "distinct union leaves owner and unrelated B")
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=ANY($2::uuid[])`, space, []uuid.UUID{nativeBanProfile(2), nativeBanProfile(3), nativeBanProfile(7), nativeBanProfile(8)}).Scan(&count))
		require.Zero(t, count)
		var state string
		require.NoError(t, serving.QueryRow(ctx, `SELECT state FROM space_member_admission_intents WHERE operation_id=$1`, pending.Input.OperationID).Scan(&state))
		require.Equal(t, "RETIRED", state)
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_community_member_heads WHERE space_id=$1 AND profile_id=ANY($2::uuid[]) AND state='RETIRED' AND retired_through=generation`, space, []uuid.UUID{nativeBanProfile(2), nativeBanProfile(5)}).Scan(&count))
		require.Equal(t, 2, count)
		require.NoError(t, serving.QueryRow(ctx, `SELECT cardinality(matched_profiles) FROM space_account_ban_operations WHERE actor_profile_id=$1 AND client_operation_id=$2`, nativeBanProfile(1), operation).Scan(&count))
		require.Equal(t, 6, count)
		st.AccountCaptures = nil
		require.NoError(t, nativeBanExecuteReason(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, &reason, func(context.Context) error {
			t.Error("completed replay must not perform current Role check")
			return status.Error(codes.PermissionDenied, "lost Role")
		}), "lost-response retry uses saved outcome even with User unavailable")
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space).Scan(&count))
		require.Equal(t, 1, count)
		var details string
		require.NoError(t, serving.QueryRow(ctx, `SELECT details::text FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space).Scan(&details))
		require.JSONEq(t, `{"reason":"repeated spam"}`, details, "only allowlisted canonical Ban details are public")
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM audit_outbox o JOIN audit_log a ON a.id=o.audit_event_id WHERE a.space_id=$1 AND a.action='member_banned'`, space).Scan(&count))
		require.Equal(t, 1, count, "saved outcome replay does not append a second audit event or outbox row")
	})

	t.Run("wrong hint and owner alias zero effect", func(t *testing.T) {
		st.AccountCaptures = nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
			return nativeBanOwnerFixture(r), nil
		})
		hint := nativeBanProfile(6)
		err := nativeBanExecute(t, st, nativeBanSpace(2), nativeBanAccount(2), nativeBanProfile(1), uuid.New(), &hint, allow)
		require.ErrorIs(t, err, ErrMemberAdmissionConflict)
		nativeBanAssertNoEffect(t, serving, nativeBanSpace(2))
		hint = nativeBanProfile(2)
		st.AccountCaptures = nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
			out := nativeBanOwnerFixture(r)
			for _, result := range out.Results {
				if result.Entry.ProfileId == nativeBanProfile(1).String() {
					result.AccountId = nativeBanAccount(2).String()
				}
			}
			return out, nil
		})
		err = nativeBanExecute(t, st, nativeBanSpace(3), nativeBanAccount(2), nativeBanProfile(1), uuid.New(), &hint, allow)
		require.ErrorIs(t, err, ErrCannotBanOwner)
		nativeBanAssertNoEffect(t, serving, nativeBanSpace(3))
	})

	t.Run("incomplete is private reconciliation without partial ban", func(t *testing.T) {
		space := nativeBanSpace(4)
		st.AccountCaptures = nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
			out := nativeBanOwnerFixture(r)
			out.Results[0] = &userv1.SpaceAccountCaptureResult{Entry: out.Results[0].Entry, Outcome: 3}
			return out, nil
		})
		err := nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), uuid.New(), nil, allow)
		require.ErrorIs(t, err, ErrAccountBanUnavailable)
		nativeBanAssertNoEffect(t, serving, space)
		var count int
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_reconciliation WHERE space_id=$1 AND reason='INCOMPLETE'`, space).Scan(&count))
		require.Equal(t, 1, count)
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1`, space).Scan(&count))
		require.Equal(t, 4, count)
	})

	t.Run("unban cannot revive unknown legacy partial ban", func(t *testing.T) {
		space := nativeBanSpace(5)
		var count int
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1 AND account_id=$2`, space, nativeBanAccount(2)).Scan(&count))
		require.Equal(t, 1, count, "explicit pre50 partial-ban component seed required")
		operation := uuid.New()
		err := nativeUnbanExecute(t, st, space, nativeBanAccount(2), operation, allow)
		require.ErrorIs(t, err, ErrAccountBanUnavailable, "unknown old account sources cannot become eligible by deleting their ban")
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1 AND account_id=$2`, space, nativeBanAccount(2)).Scan(&count))
		require.Equal(t, 1, count)
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_unbanned'`, space).Scan(&count))
		require.Zero(t, count)
		st.AccountCaptures = nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
			return nativeBanOwnerFixture(r), nil
		})
		require.NoError(t, nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), uuid.New(), nil, allow), "full reconciliation retires every old target source")
		require.NoError(t, nativeUnbanExecute(t, st, space, nativeBanAccount(2), operation, allow), "same failed Unban intent can retry after exact reconciliation")
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1`, space).Scan(&count))
		require.Equal(t, 2, count)
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_community_member_heads WHERE space_id=$1 AND state='ACTIVE'`, space).Scan(&count))
		require.Zero(t, count, "Unban creates no new community episode")
		st.AccountCaptures = nil
		require.NoError(t, nativeUnbanExecute(t, st, space, nativeBanAccount(2), operation, func(context.Context) error {
			t.Error("saved Unban outcome must precede a fresh Role check")
			return status.Error(codes.PermissionDenied, "lost Role")
		}))
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_unbanned'`, space).Scan(&count))
		require.Equal(t, 1, count)
	})

	t.Run("prepare during capture forces full immutable cohort replan", func(t *testing.T) {
		space := nativeBanSpace(6)
		var calls atomic.Int32
		var pending *MemberAdmissionIntent
		st.AccountCaptures = nativeBanCaptureFixture(func(callCtx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
			if calls.Add(1) == 1 {
				var err error
				pending, err = st.PrepareMemberAdmission(callCtx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space, ProfileID: nativeBanProfile(9), AccountID: nativeBanAccount(2)})
				if err != nil {
					return nil, err
				}
			}
			return nativeBanOwnerFixture(r), nil
		})
		operation := uuid.New()
		require.NoError(t, nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, allow))
		require.EqualValues(t, 2, calls.Load(), "stale response may not qualify the changed source cohort")
		require.NotNil(t, pending)
		var state string
		require.NoError(t, serving.QueryRow(ctx, `SELECT state FROM space_member_admission_intents WHERE operation_id=$1`, pending.Input.OperationID).Scan(&state))
		require.Equal(t, "RETIRED", state)
		var children, distinctManifests, selectedSeq int
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*),count(DISTINCT cohort_digest) FROM space_account_ban_captures WHERE actor_profile_id=$1 AND client_operation_id=$2`, nativeBanProfile(1), operation).Scan(&children, &distinctManifests))
		require.Equal(t, 2, children)
		require.Equal(t, 2, distinctManifests)
		require.NoError(t, serving.QueryRow(ctx, `SELECT selected_attempt_seq FROM space_account_ban_operations WHERE actor_profile_id=$1 AND client_operation_id=$2 AND state='COMMITTED'`, nativeBanProfile(1), operation).Scan(&selectedSeq))
		require.Equal(t, 2, selectedSeq)
		role := uuid.New()
		receipt, err := memberadmission.EnsureReceiptFingerprint(pending.EnsureBinding(), role.String())
		require.NoError(t, err)
		_, err = st.PublishMemberAdmission(ctx, pending.Input.OperationID, role, receipt[:], nil)
		require.Error(t, err, "late READY cannot publish the retired preparing source")
		var published int
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space, nativeBanProfile(9)).Scan(&published))
		require.Zero(t, published)
	})
}
