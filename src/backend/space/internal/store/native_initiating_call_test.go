package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
)

// The verifier actually signs/verifies the initiating call; the controllable
// monotonic clock changes only after a real publication SQL wait is observed.
// Controlled epoch/replay callbacks prove this private capability/deadline seam,
// not live Auth/Redis/Role authorization or a complete public Join journey.
func signedNativeInitiatingCall(t *testing.T, in MemberAdmissionInput) (*memberAdmissionInitiatingCall, *atomic.Int64) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	op := uuid.NewString()
	req := &spacev1.JoinSpaceRequest{SpaceId: in.SpaceID.String(), OperationId: &op}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_JoinSpace_FullMethodName, RequestID: op, RequestHash: hash, AccountID: in.AccountID.String(), ProfileID: in.ProfileID.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	clock := new(atomic.Int64)
	started := time.Now()
	v := lifecycleprincipal.Verifier{Clock: func() time.Time { return started.Add(time.Duration(clock.Load())) }, Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", op))
	var call *lifecycleprincipal.VerifiedAdmissionCall
	_, err = v.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_JoinSpace_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		var e error
		call, e = lifecycleprincipal.RequireAdmissionJoinSpace(ctx, req)
		return nil, e
	})
	require.NoError(t, err)
	require.NotNil(t, call)
	return &memberAdmissionInitiatingCall{call: call, request: req}, clock
}

func TestNativeInitiatingCallExpiryRollsBackAfterSQL(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	legacy := &SpaceStore{Pool: db.Pool}
	space, err := legacy.CreateSpace(ctx, uuid.New(), "earliest ingress deadline", "", "public")
	require.NoError(t, err)
	db.Run(true, "up")
	require.NoError(t, nativeReadinessTx(ctx, db.Pool, space.ID, func(tx pgx.Tx) error {
		return (&SpaceStore{Pool: db.Pool, tx: tx}).InitializeScopedNativeMembership(ctx, space.ID)
	}))
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	st := &SpaceStore{Pool: serving}
	for index, phase := range []string{"published outcome", "known guard denial"} {
		t.Run(phase, func(t *testing.T) {
			in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
			i, e := st.PrepareMemberAdmission(ctx, in)
			require.NoError(t, e)
			role := uuid.New()
			receipt, e := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
			require.NoError(t, e)
			initiating, clock := signedNativeInitiatingCall(t, in)
			holder, e := db.Pool.Begin(ctx)
			require.NoError(t, e)
			defer holder.Rollback(ctx)
			key := int64(934551200 + index)
			_, e = holder.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, key)
			require.NoError(t, e)
			entered := make(chan struct{})
			wait := func(ctx context.Context, scoped *SpaceStore) error {
				clock.Store(int64(4900 * time.Millisecond))
				if e := initiating.check(ctx); e != nil {
					return e
				}
				close(entered)
				_, e := scoped.tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, key)
				return e
			}
			var guard MemberAdmissionFinalGuard
			var hook MemberAdmissionPublicationHook
			if index == 0 {
				hook = func(ctx context.Context, s *SpaceStore, _ *MemberAdmissionIntent, _ *MembershipRow) error {
					return wait(ctx, s)
				}
			} else {
				guard = func(ctx context.Context, s *SpaceStore, _ *MemberAdmissionIntent) error {
					if e := wait(ctx, s); e != nil {
						return e
					}
					return ErrGuestsNotAllowed
				}
			}
			result := make(chan error, 1)
			go func() {
				_, e := st.publishMemberAdmissionWithInitiatingCall(ctx, in.OperationID, role, receipt[:], guard, nil, hook, initiating)
				result <- e
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("publisher never reached its authenticated local guard")
			}
			require.Eventually(t, func() bool {
				var blocked bool
				return db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND classid=0 AND objid::bigint=$1 AND NOT granted)`, key).Scan(&blocked) == nil && blocked
			}, 3*time.Second, 5*time.Millisecond, "real publication SQL must be waiting after ActorFor passed")
			clock.Store(int64(5050 * time.Millisecond))
			require.NoError(t, holder.Commit(ctx))
			select {
			case e = <-result:
			case <-time.After(5 * time.Second):
				t.Fatal("publisher did not finish after lock release")
			}
			saved, readErr := st.LookupMemberAdmission(ctx, in.OperationID)
			require.NoError(t, readErr)
			t.Logf("publication after expired ingress: error=%v state=%s", e, saved.State)
			require.ErrorIs(t, e, ErrJoinAttemptUnavailable)
			require.Equal(t, "PREPARING", saved.State, "expired initiating call cannot publish or permanently retire P")
			var clean bool
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.space_members WHERE space_id=$1 AND profile_id=$2)
 AND NOT EXISTS(SELECT 1 FROM public.space_member_admission_outbox WHERE operation_id=$3)
 AND NOT EXISTS(SELECT 1 FROM public.space_member_role_cleanup WHERE space_id=$1 AND profile_id=$2)
 AND EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='NONE')`, space.ID, in.ProfileID, in.OperationID).Scan(&clean))
			require.True(t, clean, "rollback covers member, event, retirement cleanup and effective history")
		})
	}
	t.Run("fresh call and inert expired replay", func(t *testing.T) {
		in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
		i, e := st.PrepareMemberAdmission(ctx, in)
		require.NoError(t, e)
		role := uuid.New()
		receipt, e := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
		require.NoError(t, e)
		initiating, clock := signedNativeInitiatingCall(t, in)
		original := proto.Clone(initiating.request).(*spacev1.JoinSpaceRequest)
		frozen := newMemberAdmissionInitiatingCall(initiating.call, original)
		original.SpaceId = uuid.NewString()
		require.NoError(t, frozen.check(ctx), "final call owns its exact original request snapshot")
		require.Error(t, newMemberAdmissionInitiatingCall(initiating.call, original).check(ctx), "changed body is not the authenticated call")
		calls := 0
		hook := func(context.Context, *SpaceStore, *MemberAdmissionIntent, *MembershipRow) error { calls++; return nil }
		member, e := st.publishMemberAdmissionWithInitiatingCall(ctx, in.OperationID, role, receipt[:], nil, nil, hook, frozen)
		require.NoError(t, e)
		require.NotNil(t, member)
		require.Equal(t, 1, calls)
		clock.Store(int64(5050 * time.Millisecond))
		replayed, e := st.publishMemberAdmissionWithInitiatingCall(ctx, in.OperationID, role, receipt[:], nil, nil, hook, frozen)
		require.NoError(t, e, "saved publication replay never refreshes or rechecks new initiating authority")
		require.Equal(t, member.JoinedAt, replayed.JoinedAt)
		require.Equal(t, 1, calls)
		replayed, e = st.CompleteInteractiveMemberAdmissionWithPublicationCall(ctx, in.OperationID, nil, nil, nil, nil, hook, nil, nil)
		require.NoError(t, e, "public coordinator's completed replay stays before Role/owner/call checks")
		require.Equal(t, member.JoinedAt, replayed.JoinedAt)
		require.Equal(t, 1, calls)
	})
	t.Run("fresh call preserves known denial", func(t *testing.T) {
		in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
		i, e := st.PrepareMemberAdmission(ctx, in)
		require.NoError(t, e)
		role := uuid.New()
		receipt, e := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
		require.NoError(t, e)
		initiating, _ := signedNativeInitiatingCall(t, in)
		_, e = st.publishMemberAdmissionWithInitiatingCall(ctx, in.OperationID, role, receipt[:], func(context.Context, *SpaceStore, *MemberAdmissionIntent) error { return ErrGuestsNotAllowed }, nil, nil, initiating)
		require.ErrorIs(t, e, ErrGuestsNotAllowed)
		saved, e := st.LookupMemberAdmission(ctx, in.OperationID)
		require.NoError(t, e)
		require.Equal(t, "RETIRED", saved.State)
	})
}
