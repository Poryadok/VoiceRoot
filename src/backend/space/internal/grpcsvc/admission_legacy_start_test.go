package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func TestLegacyJoinStartRealIngressAndCanonicalGuest(t *testing.T) {
	ctx := context.Background()
	st, space := admissionReadStore(t)
	for _, name := range []string{"000040_join_decision_basis_v2.up.sql", "000043_join_public_operation_context.up.sql", "000045_join_legacy_selection_context.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = st.Pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	var elapsed atomic.Int64
	clock := func() time.Time { return now.Add(time.Duration(elapsed.Load())) }
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	currentType := "regular"
	subjects := &admissionauthority.Client{Issuer: issuer, Clock: clock, Auth: admissionSubjectFixture(func(_ context.Context, request *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START, request.Phase)
		var saved []byte
		var selected *uuid.UUID
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT legacy_selection_sha256,selected_attempt_id FROM space_join_operations WHERE actor_account_id=$1 AND actor_profile_id=$2`, request.AccountId, request.ProfileId).Scan(&saved, &selected))
		require.Equal(t, saved, request.AdmissionBindingSha256)
		require.Nil(t, selected, "A is absent before current owner classification")
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
		require.NoError(t, err)
		sha := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: sha[:], AccountId: request.AccountId, ProfileId: request.ProfileId, AccountType: currentType, AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	invoke := func(request proto.Message, rpc string) (*store.JoinAttempt, error) {
		account, profile := uuid.New(), uuid.New()
		before := proto.Clone(request)
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: rpc, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
		require.NoError(t, err)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
		result, err := verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: rpc}, func(callctx context.Context, _ any) (any, error) {
			p, err := st.PrepareLegacyJoinStart(callctx, request)
			if err != nil {
				return nil, err
			}
			reservation, e := p.Reservation()
			require.NoError(t, e)
			require.Equal(t, "RESERVED", reservation.State)
			require.Nil(t, reservation.AttemptID)
			require.Equal(t, account, reservation.Input.ActorAccountID)
			require.Equal(t, profile, reservation.Input.ActorProfileID)
			require.Equal(t, rpc, reservation.Input.Method)
			reservation.Input.RequestSHA256[0] ^= 1
			unchanged, e := p.Reservation()
			require.NoError(t, e)
			require.NotEqual(t, reservation.Input.RequestSHA256, unchanged.Input.RequestSHA256)
			expected, err := p.OwnerRequest()
			if err != nil {
				return nil, err
			}
			proof, err := subjects.Observe(callctx, expected)
			if err != nil {
				return nil, err
			}
			decision, err := p.Seal(proof)
			if err != nil {
				return nil, err
			}
			return st.CommitLegacyJoinStart(callctx, decision)
		})
		if err != nil {
			return nil, err
		}
		require.True(t, proto.Equal(before, request), "private server UUID is never inserted into the signed no-key request")
		a := result.(*store.JoinAttempt)
		var origin int
		var operation uuid.UUID
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT operation_origin,operation_id FROM space_join_operations WHERE actor_profile_id=$1 AND selected_attempt_id=$2`, profile, a.ID).Scan(&origin, &operation))
		switch r := request.(type) {
		case *spacev1.JoinSpaceRequest:
			if r.OperationId == nil {
				require.Equal(t, 2, origin)
			} else {
				require.Equal(t, 1, origin)
				require.Equal(t, r.GetOperationId(), operation.String())
			}
		case *spacev1.JoinByInviteRequest:
			require.Nil(t, r.OperationId)
			require.Equal(t, 2, origin)
		}
		return a, nil
	}
	_, err = st.PrepareLegacyJoinStart(ctx, &spacev1.JoinSpaceRequest{SpaceId: space.String()})
	require.Error(t, err, "ordinary context cannot allocate an admission")
	a, err := invoke(&spacev1.JoinSpaceRequest{SpaceId: space.String()}, spacev1.SpaceService_JoinSpace_FullMethodName)
	require.NoError(t, err)
	require.Equal(t, "COLLECTING", a.State, "START is not a successful legacy admission")
	op := uuid.NewString()
	_, err = invoke(&spacev1.JoinSpaceRequest{SpaceId: space.String(), OperationId: &op}, spacev1.SpaceService_JoinSpace_FullMethodName)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET allow_guests=true WHERE id=$1`, space)
	require.NoError(t, err)
	invite, err := st.CreateInvite(ctx, store.CreateInviteInput{SpaceID: space, CreatorProfileID: uuid.New()})
	require.NoError(t, err)
	currentType = "guest"
	a, err = invoke(&spacev1.JoinByInviteRequest{Code: invite.Code}, spacev1.SpaceService_JoinByInvite_FullMethodName)
	require.NoError(t, err)
	var guest bool
	var exactInvite uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT guest,invite_id FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&guest, &exactInvite))
	require.True(t, guest, "current Auth type overrides regular ingress hint")
	require.Equal(t, invite.ID, exactInvite)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT use_count FROM invites WHERE id=$1`, invite.ID).Scan(&count))
	require.Zero(t, count)
	currentType = "regular"
	// The A row and selected pin are already written when this trigger waits.
	// Strict ingress/owner expiry must roll both back after the SQL wait.
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION hold_legacy_start_pin() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.selected_attempt_id IS NULL AND NEW.selected_attempt_id IS NOT NULL THEN PERFORM pg_advisory_xact_lock(90006945); END IF; RETURN NEW; END $$; CREATE TRIGGER hold_legacy_start_pin BEFORE UPDATE ON space_join_operations FOR EACH ROW EXECUTE FUNCTION hold_legacy_start_pin()`)
	require.NoError(t, err)
	blocker, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer blocker.Release()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_lock(90006945)`)
	require.NoError(t, err)
	expiringOp := uuid.NewString()
	done := make(chan error, 1)
	go func() {
		_, err := invoke(&spacev1.JoinSpaceRequest{SpaceId: space.String(), OperationId: &expiringOp}, spacev1.SpaceService_JoinSpace_FullMethodName)
		done <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=90006945 AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	elapsed.Store(int64(5 * time.Second))
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_unlock(90006945)`)
	require.NoError(t, err)
	require.Error(t, <-done)
	var selected *uuid.UUID
	var account uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT actor_account_id,selected_attempt_id FROM space_join_operations WHERE operation_id=$1`, expiringOp).Scan(&account, &selected))
	require.Nil(t, selected)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE account_id=$1`, account).Scan(&count))
	require.Zero(t, count, "expired final pin must not leave an allocated A")
}
