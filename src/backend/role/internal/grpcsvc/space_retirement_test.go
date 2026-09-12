package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
)

type r23RetirementFixture struct {
	svc                 *RoleGRPC
	store               *store.RoleStore
	spaceID, ownerID    uuid.UUID
	deletionOperationID uuid.UUID
	purgeDecidedAt      time.Time
	manifestID          uuid.UUID
	manifestSHA         []byte
}

func r23RoleRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func r23ApplyMigration(t *testing.T, ctx context.Context, st *store.RoleStore, name string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r23RoleRepoRoot(t), "src", "backend", "migrations", "role_db", name))
	require.NoError(t, err, "R23 RED requires the numbered migration")
	_, err = st.Pool.Exec(ctx, string(b))
	require.NoError(t, err)
}

func newR23RetirementFixture(t *testing.T) r23RetirementFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	pool := store.StartRoleDBForStoreTest(t, ctx)
	t.Cleanup(pool.Close)
	store.ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &store.RoleStore{Pool: pool}
	r23ApplyMigration(t, ctx, st, "000011_voice_policy_epoch.up.sql")
	r23ApplyMigration(t, ctx, st, "000012_space_retirement.up.sql")
	f := r23RetirementFixture{
		svc: &RoleGRPC{Store: st}, store: st,
		spaceID: uuid.New(), ownerID: uuid.New(), deletionOperationID: uuid.New(), manifestID: uuid.New(),
		purgeDecidedAt: time.Date(2026, 9, 12, 7, 8, 9, 123456000, time.UTC),
	}
	f.manifestSHA = bytes.Repeat([]byte{0x5a}, sha256.Size)
	require.NoError(t, st.BootstrapSpaceRoles(ctx, f.spaceID, f.ownerID))
	memberID, err := st.RoleIDByName(ctx, f.spaceID, "Member")
	require.NoError(t, err)
	custom, err := st.CreateCustomRole(ctx, f.spaceID, "retirement fixture", 17, 7, &f.ownerID)
	require.NoError(t, err)
	require.NoError(t, st.AssignMemberRole(ctx, f.spaceID, uuid.New(), custom.ID, f.ownerID))
	require.NoError(t, st.SetChatOverride(ctx, uuid.New(), custom.ID, 1, 2))
	require.NoError(t, st.SetVoiceRoomOverride(ctx, uuid.New(), memberID, 4, 8))
	return f
}

func (f r23RetirementFixture) request() *rolev1.RetireSpaceRequest {
	return &rolev1.RetireSpaceRequest{
		ProtocolVersion:     1,
		SpaceId:             f.spaceID.String(),
		DeletionOperationId: f.deletionOperationID.String(),
		Generation:          7,
		PurgeDecidedAt:      timestamppb.New(f.purgeDecidedAt),
		Manifest:            &commonv1.ManifestBinding{ManifestId: f.manifestID.String(), ManifestSha256: append([]byte(nil), f.manifestSHA...), ItemCount: 23},
	}
}

func r23VerifiedContext(t *testing.T, rpc string, req proto.Message) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role",
		RPC: rpc, RequestID: uuid.NewString(), RequestHash: hash,
	})
}

func r23PersistenceSnapshot(t *testing.T, f r23RetirementFixture) string {
	t.Helper()
	var snapshot string
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT jsonb_build_object(
 'lifecycle',(SELECT jsonb_agg(to_jsonb(x) ORDER BY space_id) FROM role_space_lifecycle x),
 'receipts',(SELECT jsonb_agg(to_jsonb(x) ORDER BY space_id) FROM role_space_retirement_receipts x),
 'roles',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM roles x),
 'members',(SELECT jsonb_agg(to_jsonb(x) ORDER BY space_id,profile_id,role_id) FROM member_roles x),
 'chat',(SELECT jsonb_agg(to_jsonb(x) ORDER BY chat_id,role_id) FROM chat_overrides x),
 'voice',(SELECT jsonb_agg(to_jsonb(x) ORDER BY voice_room_id,role_id) FROM voice_room_overrides x),
 'epochs',(SELECT jsonb_agg(to_jsonb(x) ORDER BY space_id) FROM role_voice_policy_epochs x),
 'outbox',(SELECT jsonb_agg(to_jsonb(x) ORDER BY space_id,policy_epoch) FROM role_voice_policy_outbox x)
)::text`).Scan(&snapshot))
	return snapshot
}

func (f r23RetirementFixture) retire(t *testing.T, req *rolev1.RetireSpaceRequest) (*rolev1.RetireSpaceResponse, error) {
	t.Helper()
	return f.svc.RetireSpace(r23VerifiedContext(t, rolev1.RoleService_RetireSpace_FullMethodName, req), req)
}

func TestRetireSpace_HandlerIsImplemented(t *testing.T) {
	f := r23RetirementFixture{spaceID: uuid.New(), deletionOperationID: uuid.New(), manifestID: uuid.New(), purgeDecidedAt: time.Now().UTC(), manifestSHA: make([]byte, 32)}
	_, err := (&RoleGRPC{}).RetireSpace(r23VerifiedContext(t, rolev1.RoleService_RetireSpace_FullMethodName, f.request()), f.request())
	require.NotEqual(t, codes.Unimplemented, status.Code(err), "generated embedding is not a Role retirement implementation")
}

func TestRetireSpace_ExactReceiptAtomicCleanupAndFinalPolicyInvalidation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	ctx := context.Background()
	req := f.request()
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	require.NoError(t, err)
	requestDigest := sha256.Sum256(append(append([]byte("voice.role.v1.RetireSpaceRequest"), 0), requestBytes...))
	var epochBefore int64
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT policy_epoch FROM role_voice_policy_epochs WHERE space_id=$1`, f.spaceID).Scan(&epochBefore))

	response, err := f.retire(t, req)
	require.NoError(t, err)
	require.NotNil(t, response.GetReceipt())
	receipt := response.GetReceipt()
	require.Equal(t, uint32(1), receipt.GetProtocolVersion())
	require.Equal(t, f.spaceID.String(), receipt.GetSpaceId())
	require.Equal(t, f.deletionOperationID.String(), receipt.GetDeletionOperationId())
	require.Equal(t, uint64(7), receipt.GetGeneration())
	require.Equal(t, rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, receipt.GetState())
	require.Equal(t, requestDigest[:], receipt.GetRequestSha256())
	require.Equal(t, f.manifestSHA, receipt.GetManifestSha256())
	require.NoError(t, receipt.GetRetiredAt().CheckValid())
	require.False(t, receipt.GetRetiredAt().AsTime().Before(f.purgeDecidedAt))
	receiptID, err := uuid.Parse(receipt.GetReceiptId())
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, receiptID)

	var roles, members, chatOverrides, voiceOverrides int
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM roles WHERE space_id=$1),
	 (SELECT count(*) FROM member_roles WHERE space_id=$1),
	 (SELECT count(*) FROM chat_overrides c JOIN roles r ON r.id=c.role_id WHERE r.space_id=$1),
	 (SELECT count(*) FROM voice_room_overrides v JOIN roles r ON r.id=v.role_id WHERE r.space_id=$1)`, f.spaceID).Scan(&roles, &members, &chatOverrides, &voiceOverrides))
	require.Zero(t, roles)
	require.Zero(t, members)
	require.Zero(t, chatOverrides)
	require.Zero(t, voiceOverrides)

	var retiredAt time.Time
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT retired_at FROM role_space_lifecycle WHERE space_id=$1`, f.spaceID).Scan(&retiredAt))
	require.Equal(t, receipt.GetRetiredAt().AsTime(), retiredAt.UTC())
	var epochAfter, newSnapshots int64
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT policy_epoch FROM role_voice_policy_epochs WHERE space_id=$1`, f.spaceID).Scan(&epochAfter))
	require.Equal(t, epochBefore+1, epochAfter, "retirement publishes one final Space-wide invalidation")
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT count(*) FROM role_voice_policy_outbox WHERE space_id=$1 AND policy_epoch>$2`, f.spaceID, epochBefore).Scan(&newSnapshots))
	require.EqualValues(t, 1, newSnapshots)
}

func TestRetireSpace_RefusesPreparedButAcceptsTerminalOwnershipLedgers(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, state := range []string{"prepared", "finalized", "aborted", "legacy_v1"} {
		t.Run(state, func(t *testing.T) {
			f := newR23RetirementFixture(t)
			ctx := context.Background()
			opID, nextOwner := uuid.New(), uuid.New()
			var delayed func() error
			if state == "legacy_v1" {
				request := &rolev1.ApplyOwnershipTransferRequest{SpaceId: f.spaceID.String(), OldOwnerProfileId: f.ownerID.String(), NewOwnerProfileId: nextOwner.String(), OperationId: opID.String()}
				requestHash, err := principal.RequestHash(request)
				require.NoError(t, err)
				_, err = f.store.Pool.Exec(ctx, `INSERT INTO ownership_transfer_role_receipts
				 (operation_id,action,space_id,old_owner_profile_id,new_owner_profile_id,request_hash,current_owner_profile_id)
				 VALUES($1,'apply',$2,$3,$4,$5,$4)`, opID, f.spaceID, f.ownerID, nextOwner, requestHash)
				require.NoError(t, err)
				delayed = func() error {
					_, callErr := f.svc.ApplyOwnershipTransfer(r23VerifiedContext(t, rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName, request), request)
					return callErr
				}
			} else {
				intent := &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: f.spaceID.String(), OldOwnerProfileId: f.ownerID.String(), NewOwnerProfileId: nextOwner.String(), OperationId: opID.String()}
				intentBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(intent)
				require.NoError(t, err)
				intentHash := sha256.Sum256(intentBytes)
				prepareRequest := &rolev1.PrepareOwnershipTransferRequest{Intent: intent}
				prepareHash := any(r23PrincipalHashBytes(t, prepareRequest))
				finalizeHash, abortHash := any(nil), any(nil)
				if state == "prepared" || state == "finalized" {
					prepareHash = r23PrincipalHashBytes(t, prepareRequest)
				}
				if state == "finalized" {
					request := &rolev1.FinalizeOwnershipTransferRequest{Intent: intent}
					finalizeHash = r23PrincipalHashBytes(t, request)
					delayed = func() error {
						_, callErr := f.svc.FinalizeOwnershipTransfer(r23VerifiedContext(t, rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName, request), request)
						return callErr
					}
				}
				if state == "aborted" {
					request := &rolev1.AbortOwnershipTransferRequest{Intent: intent}
					prepareHash = nil
					abortHash = r23PrincipalHashBytes(t, request)
					delayed = func() error {
						_, callErr := f.svc.AbortOwnershipTransfer(r23VerifiedContext(t, rolev1.RoleService_AbortOwnershipTransfer_FullMethodName, request), request)
						return callErr
					}
				}
				_, err = f.store.Pool.Exec(ctx, `INSERT INTO ownership_transfer_v2
				 (operation_id,space_id,protocol_version,old_owner_profile_id,new_owner_profile_id,intent_bytes,intent_hash,state,prepare_request_hash,finalize_request_hash,abort_request_hash)
				 VALUES($1,$2,2,$3,$4,$5,$6,$7,$8,$9,$10)`, opID, f.spaceID, f.ownerID, nextOwner, intentBytes, intentHash[:], state, prepareHash, finalizeHash, abortHash)
				require.NoError(t, err)
			}
			response, err := f.retire(t, f.request())
			if state == "prepared" {
				require.Equal(t, codes.FailedPrecondition, status.Code(err))
				require.Nil(t, response)
				var roles, retired int
				require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM roles WHERE space_id=$1),(SELECT count(*) FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL)`, f.spaceID).Scan(&roles, &retired))
				require.Positive(t, roles)
				require.Zero(t, retired)
				return
			}
			require.NoError(t, err)
			require.Equal(t, rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, response.GetReceipt().GetState())
			require.Equal(t, codes.FailedPrecondition, status.Code(delayed()), "retirement fence must win before terminal ownership receipt replay")
		})
	}
}

func r23PrincipalHashBytes(t *testing.T, req proto.Message) []byte {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	b, err := hex.DecodeString(strings.TrimPrefix(hash, "sha256:"))
	require.NoError(t, err)
	return b
}

func TestRetireSpace_ExactReplaySurvivesResponseLossAndRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	req := f.request()
	first, err := f.retire(t, req) // Treat the successful return as lost by the caller.
	require.NoError(t, err)
	committed := r23PersistenceSnapshot(t, f)
	f.svc = &RoleGRPC{Store: &store.RoleStore{Pool: f.store.Pool}}
	replayed, err := f.retire(t, proto.Clone(req).(*rolev1.RetireSpaceRequest))
	require.NoError(t, err)
	firstBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(first)
	require.NoError(t, err)
	replayBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(replayed)
	require.NoError(t, err)
	require.Equal(t, firstBytes, replayBytes, "restart replay must return byte-identical stored evidence")
	require.Equal(t, committed, r23PersistenceSnapshot(t, f), "replay cannot rewrite receipt, fence, epoch, or outbox")
}

func TestRetireSpace_CompactReplayRequiresSemanticEvidenceBeyondHash(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	retiredAt := time.Now().UTC().Add(-31 * 24 * time.Hour).Truncate(time.Microsecond)
	f.purgeDecidedAt = retiredAt.Add(-time.Minute)
	req := f.request()
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	require.NoError(t, err)
	requestHash := r23DomainHashBytes("voice.role.v1.RetireSpaceRequest", requestBytes)
	receipt := retirementResponse(store.SpaceRetirementReceipt{
		ProtocolVersion: 1, ReceiptID: uuid.New(), SpaceID: f.spaceID, DeletionOperationID: f.deletionOperationID,
		Generation: req.GetGeneration(), RequestSHA256: requestHash, ManifestSHA256: f.manifestSHA, RetiredAt: retiredAt,
	})
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	require.NoError(t, err)
	tx, err := f.store.Pool.Begin(context.Background())
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(context.Background(), `SELECT set_config('voice.role_retirement_space_id',$1,true)`, f.spaceID.String())
	require.NoError(t, err)
	_, err = tx.Exec(context.Background(), `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,$2)`, f.spaceID, retiredAt)
	require.NoError(t, err)
	_, err = tx.Exec(context.Background(), `DELETE FROM member_roles WHERE space_id=$1`, f.spaceID)
	require.NoError(t, err)
	_, err = tx.Exec(context.Background(), `DELETE FROM roles WHERE space_id=$1`, f.spaceID)
	require.NoError(t, err)
	_, err = tx.Exec(context.Background(), `INSERT INTO role_space_retirement_receipts
 (space_id,deletion_operation_id,protocol_version,generation,purge_decided_at,manifest_id,manifest_item_count,receipt_id,
  request_sha256,manifest_sha256,request_bytes,receipt_bytes,retired_at,full_bytes_retain_until)
 VALUES($1,$2,1,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)`,
		f.spaceID, f.deletionOperationID, req.GetGeneration(), f.purgeDecidedAt, f.manifestID, req.GetManifest().GetItemCount(),
		uuid.MustParse(receipt.GetReceipt().GetReceiptId()), requestHash, f.manifestSHA, requestBytes, receiptBytes, retiredAt)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(context.Background()))
	_, err = f.store.Pool.Exec(context.Background(), `UPDATE role_space_retirement_receipts SET request_bytes=NULL,receipt_bytes=NULL WHERE space_id=$1`, f.spaceID)
	require.NoError(t, err)

	exact, err := f.retire(t, req)
	require.NoError(t, err)
	exactBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(exact)
	require.NoError(t, err)
	require.Equal(t, receiptBytes, exactBytes, "compact semantic evidence must reconstruct the exact known-field receipt")
	committed := r23PersistenceSnapshot(t, f)
	for _, change := range []struct {
		name   string
		mutate func(*rolev1.RetireSpaceRequest)
	}{
		{"operation", func(r *rolev1.RetireSpaceRequest) { r.DeletionOperationId = uuid.NewString() }},
		{"generation", func(r *rolev1.RetireSpaceRequest) { r.Generation++ }},
		{"purge_decided_at", func(r *rolev1.RetireSpaceRequest) {
			r.PurgeDecidedAt = timestamppb.New(f.purgeDecidedAt.Add(time.Second))
		}},
		{"manifest_id", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestId = uuid.NewString() }},
		{"manifest_hash", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestSha256[0] ^= 0xff }},
		{"item_count", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ItemCount++ }},
	} {
		t.Run(change.name, func(t *testing.T) {
			changed := proto.Clone(req).(*rolev1.RetireSpaceRequest)
			change.mutate(changed)
			response, err := f.retire(t, changed)
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			require.Nil(t, response)
			require.Equal(t, committed, r23PersistenceSnapshot(t, f))
		})
	}

	base := store.SpaceRetirementInput{
		ProtocolVersion: 1, SpaceID: f.spaceID, DeletionOperationID: f.deletionOperationID,
		Generation: req.GetGeneration(), PurgeDecidedAt: f.purgeDecidedAt, ManifestID: f.manifestID,
		ManifestItemCount: req.GetManifest().GetItemCount(), RequestSHA256: requestHash,
		ManifestSHA256: f.manifestSHA, RequestBytes: requestBytes,
	}
	for _, mutate := range []func(*store.SpaceRetirementInput){
		func(in *store.SpaceRetirementInput) { in.PurgeDecidedAt = in.PurgeDecidedAt.Add(time.Second) },
		func(in *store.SpaceRetirementInput) { in.ManifestID = uuid.New() },
		func(in *store.SpaceRetirementInput) { in.ManifestItemCount++ },
	} {
		forged := base
		mutate(&forged)
		_, err := f.store.RetireSpace(context.Background(), forged, func(store.SpaceRetirementReceipt) ([]byte, error) { return receiptBytes, nil })
		require.ErrorIs(t, err, store.ErrSpaceRetirementConflict, "compact replay must compare semantics even when the caller reuses the accepted hash and bytes")
	}
	require.Equal(t, committed, r23PersistenceSnapshot(t, f))
}

func r23DomainHashBytes(name string, wire []byte) []byte {
	digest := sha256.Sum256(append(append([]byte(name), 0), wire...))
	return digest[:]
}

func TestRetireSpace_ChangedBindingAndReceiptLookupFailureFailClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	req := f.request()
	_, err := f.retire(t, req)
	require.NoError(t, err)
	committed := r23PersistenceSnapshot(t, f)
	for _, change := range []struct {
		name   string
		mutate func(*rolev1.RetireSpaceRequest)
	}{
		{"operation", func(r *rolev1.RetireSpaceRequest) { r.DeletionOperationId = uuid.NewString() }},
		{"generation", func(r *rolev1.RetireSpaceRequest) { r.Generation++ }},
		{"purge_decided_at", func(r *rolev1.RetireSpaceRequest) {
			r.PurgeDecidedAt = timestamppb.New(f.purgeDecidedAt.Add(time.Second))
		}},
		{"manifest_id", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestId = uuid.NewString() }},
		{"manifest_hash", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestSha256[0] ^= 0xff }},
		{"item_count", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ItemCount++ }},
	} {
		t.Run(change.name, func(t *testing.T) {
			changed := proto.Clone(req).(*rolev1.RetireSpaceRequest)
			change.mutate(changed)
			response, err := f.retire(t, changed)
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			require.Nil(t, response)
			require.Equal(t, committed, r23PersistenceSnapshot(t, f), "conflict cannot rewrite committed retirement evidence")
		})
	}
	_, err = f.store.Pool.Exec(context.Background(), `ALTER TABLE role_space_retirement_receipts RENAME TO unavailable_role_space_retirement_receipts`)
	require.NoError(t, err)
	response, err := f.retire(t, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Nil(t, response)
	_, err = f.store.Pool.Exec(context.Background(), `ALTER TABLE unavailable_role_space_retirement_receipts RENAME TO role_space_retirement_receipts`)
	require.NoError(t, err)
	require.Equal(t, committed, r23PersistenceSnapshot(t, f), "lookup failure cannot rewrite committed retirement evidence")
}

func TestRetireSpace_FailpointRollsBackFenceReceiptCleanupAndInvalidation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	before := r23PersistenceSnapshot(t, f)
	_, err := f.store.Pool.Exec(context.Background(), `CREATE FUNCTION r23_reject_retirement_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'injected retirement receipt failure' USING ERRCODE='23514'; END $$;
 CREATE TRIGGER r23_reject_retirement_receipt BEFORE INSERT ON role_space_retirement_receipts
 FOR EACH ROW EXECUTE FUNCTION r23_reject_retirement_receipt();`)
	require.NoError(t, err)
	response, err := f.retire(t, f.request())
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Nil(t, response)
	require.Equal(t, before, r23PersistenceSnapshot(t, f), "one failed statement must roll back fence, receipt, cleanup, epoch, and outbox")
	_, err = f.store.Pool.Exec(context.Background(), `DROP TRIGGER r23_reject_retirement_receipt ON role_space_retirement_receipts; DROP FUNCTION r23_reject_retirement_receipt()`)
	require.NoError(t, err)
	response, err = f.retire(t, f.request())
	require.NoError(t, err)
	require.Equal(t, rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, response.GetReceipt().GetState())
}

func TestRetireSpace_RejectsUnknownAndInvalidRequestsBeforeMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	before := r23PersistenceSnapshot(t, f)
	for _, tc := range []struct {
		name   string
		mutate func(*rolev1.RetireSpaceRequest)
	}{
		{"protocol", func(r *rolev1.RetireSpaceRequest) { r.ProtocolVersion = 2 }},
		{"space", func(r *rolev1.RetireSpaceRequest) { r.SpaceId = "not-a-uuid" }},
		{"space_noncanonical", func(r *rolev1.RetireSpaceRequest) { r.SpaceId = strings.ToUpper(r.SpaceId) }},
		{"space_nil", func(r *rolev1.RetireSpaceRequest) { r.SpaceId = uuid.Nil.String() }},
		{"operation", func(r *rolev1.RetireSpaceRequest) { r.DeletionOperationId = "not-a-uuid" }},
		{"operation_noncanonical", func(r *rolev1.RetireSpaceRequest) { r.DeletionOperationId = strings.ToUpper(r.DeletionOperationId) }},
		{"operation_nil", func(r *rolev1.RetireSpaceRequest) { r.DeletionOperationId = uuid.Nil.String() }},
		{"generation", func(r *rolev1.RetireSpaceRequest) { r.Generation = 0 }},
		{"timestamp", func(r *rolev1.RetireSpaceRequest) { r.PurgeDecidedAt = nil }},
		{"timestamp_invalid", func(r *rolev1.RetireSpaceRequest) { r.PurgeDecidedAt = &timestamppb.Timestamp{Seconds: 253402300800} }},
		{"timestamp_submicrosecond", func(r *rolev1.RetireSpaceRequest) { r.PurgeDecidedAt.Nanos++ }},
		{"manifest", func(r *rolev1.RetireSpaceRequest) { r.Manifest = nil }},
		{"manifest_id", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestId = "not-a-uuid" }},
		{"manifest_id_noncanonical", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestId = strings.ToUpper(r.Manifest.ManifestId) }},
		{"manifest_id_nil", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestId = uuid.Nil.String() }},
		{"manifest_hash", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestSha256 = []byte{1} }},
		{"unknown", func(r *rolev1.RetireSpaceRequest) { r.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01}) }},
		{"nested_unknown", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := f.request()
			tc.mutate(req)
			response, err := f.retire(t, req)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Nil(t, response)
			var retired int
			require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT count(*) FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL`, f.spaceID).Scan(&retired))
			require.Zero(t, retired)
			require.Equal(t, before, r23PersistenceSnapshot(t, f))
		})
	}
}

func TestRetireSpace_RequiresTrustedCanonicalSpacePrincipalBeforeMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	req := f.request()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	valid := principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: rolev1.RoleService_RetireSpace_FullMethodName, RequestID: "r23-trusted", RequestHash: hash}
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{"missing", context.Background()},
		{"kind", principal.WithVerified(context.Background(), func() principal.Principal { p := valid; p.Kind = "user"; return p }())},
		{"issuer", principal.WithVerified(context.Background(), func() principal.Principal { p := valid; p.Issuer = "gateway"; return p }())},
		{"subject", principal.WithVerified(context.Background(), func() principal.Principal { p := valid; p.Subject = "service:chat"; return p }())},
		{"audience", principal.WithVerified(context.Background(), func() principal.Principal { p := valid; p.Audience = "space"; return p }())},
		{"rpc", principal.WithVerified(context.Background(), func() principal.Principal {
			p := valid
			p.RPC = rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName
			return p
		}())},
		{"request_id", principal.WithVerified(context.Background(), func() principal.Principal { p := valid; p.RequestID = ""; return p }())},
		{"request_hash", principal.WithVerified(context.Background(), func() principal.Principal { p := valid; p.RequestHash = "sha256:" + strings.Repeat("0", 64); return p }())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := r23PersistenceSnapshot(t, f)
			response, err := f.svc.RetireSpace(tc.ctx, req)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.Nil(t, response)
			require.Equal(t, before, r23PersistenceSnapshot(t, f))
		})
	}
}

func TestRetireSpace_DelayedOwnershipOrdinaryAndBootstrapCannotRecreateAuthority(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	events := &ordinaryScopeEvents{pool: f.store.Pool}
	f.svc.Events = events
	_, err := f.retire(t, f.request())
	require.NoError(t, err)
	committed := r23PersistenceSnapshot(t, f)
	oldOperation, nextOwner := uuid.New(), uuid.New()
	v1 := &rolev1.ApplyOwnershipTransferRequest{SpaceId: f.spaceID.String(), OldOwnerProfileId: f.ownerID.String(), NewOwnerProfileId: nextOwner.String(), OperationId: oldOperation.String()}
	v2Intent := &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: f.spaceID.String(), OldOwnerProfileId: f.ownerID.String(), NewOwnerProfileId: nextOwner.String(), OperationId: oldOperation.String()}
	v2 := &rolev1.PrepareOwnershipTransferRequest{Intent: v2Intent}
	bootstrap := &rolev1.BootstrapSpaceRolesRequest{SpaceId: f.spaceID.String(), OwnerProfileId: uuid.NewString()}
	v1ctx := r23VerifiedContext(t, rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName, v1)
	v2ctx := r23VerifiedContext(t, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, v2)

	ownershipCalls := []func() error{
		func() error {
			_, e := f.svc.ApplyOwnershipTransfer(v1ctx, v1)
			return e
		},
		func() error {
			_, e := f.svc.PrepareOwnershipTransfer(v2ctx, v2)
			return e
		},
	}
	ordinaryCalls := []func() error{
		func() error { _, e := f.svc.BootstrapSpaceRoles(context.Background(), bootstrap); return e },
		func() error {
			_, e := f.svc.CreateRole(context.Background(), &rolev1.CreateRoleRequest{SpaceId: f.spaceID.String(), Name: "identifier reuse", Position: 1})
			return e
		},
	}
	var wg sync.WaitGroup
	type result struct {
		ownership bool
		err       error
	}
	results := make(chan result, (len(ownershipCalls)+len(ordinaryCalls))*4)
	for i := 0; i < 4; i++ {
		for _, call := range ownershipCalls {
			wg.Add(1)
			go func(call func() error) { defer wg.Done(); results <- result{ownership: true, err: call()} }(call)
		}
		for _, call := range ordinaryCalls {
			wg.Add(1)
			go func(call func() error) { defer wg.Done(); results <- result{err: call()} }(call)
		}
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result.ownership {
			require.Equal(t, codes.FailedPrecondition, status.Code(result.err))
		} else {
			require.Equal(t, codes.Unavailable, status.Code(result.err))
		}
	}
	require.Equal(t, committed, r23PersistenceSnapshot(t, f), "delayed calls cannot mutate receipt, fence, authority, epoch, or outbox")
	count, visibilityErrors := events.snapshot()
	require.Zero(t, count)
	require.Empty(t, visibilityErrors)
}

const r23OperationLockNamespace int64 = 0x524f5032

func TestRetireSpace_OperationLockPrecedesSpaceAndWinsAllAuthorityRaces(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	events := &ordinaryScopeEvents{pool: f.store.Pool}
	f.svc.Events = events
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	blocker, err := f.store.Pool.Begin(ctx)
	require.NoError(t, err)
	var racerPools []*pgxpool.Pool
	defer func() {
		// Release the advisory lock before closing any blocked racer pool. This
		// also keeps every early return from stranding a pool close behind it.
		_ = blocker.Rollback(context.Background())
		for _, racerPool := range racerPools {
			if racerPool != nil {
				racerPool.Close()
			}
		}
	}()
	var blockerPID int32
	require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID))
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, f.spaceID.String())
	require.NoError(t, err)

	retirementDone := make(chan error, 1)
	req := f.request()
	retirementCtx := r23VerifiedContext(t, rolev1.RoleService_RetireSpace_FullMethodName, req)
	go func() {
		_, retireErr := f.svc.RetireSpace(retirementCtx, req)
		retirementDone <- retireErr
	}()
	var retirementPID int32
	require.Eventually(t, func() bool {
		err := f.store.Pool.QueryRow(ctx, `SELECT w.pid FROM pg_locks w JOIN pg_locks b
 ON b.locktype=w.locktype AND b.database IS NOT DISTINCT FROM w.database
 AND b.classid=w.classid AND b.objid=w.objid AND b.objsubid=w.objsubid
 WHERE b.pid=$1 AND b.locktype='advisory' AND b.granted AND NOT w.granted LIMIT 1`, blockerPID).Scan(&retirementPID)
		return err == nil
	}, 5*time.Second, 20*time.Millisecond, "retirement must queue on the exact Space advisory lock")
	var operationLockHeld bool
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks
 WHERE pid=$1 AND locktype='advisory' AND granted AND objsubid=2 AND classid::bigint=$2)`, retirementPID, r23OperationLockNamespace).Scan(&operationLockHeld))
	require.True(t, operationLockHeld, "retirement must hold deletion-operation lock before waiting for the Space lock")

	oldOwner, newOwner := f.ownerID, uuid.New()
	makeIntent := func() *rolev1.OwnershipTransferIntent {
		return &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: f.spaceID.String(), OldOwnerProfileId: oldOwner.String(), NewOwnerProfileId: newOwner.String(), OperationId: uuid.NewString()}
	}
	prepare := &rolev1.PrepareOwnershipTransferRequest{Intent: makeIntent()}
	finalize := &rolev1.FinalizeOwnershipTransferRequest{Intent: makeIntent()}
	abort := &rolev1.AbortOwnershipTransferRequest{Intent: makeIntent()}
	prepareCtx := r23VerifiedContext(t, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, prepare)
	finalizeCtx := r23VerifiedContext(t, rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName, finalize)
	abortCtx := r23VerifiedContext(t, rolev1.RoleService_AbortOwnershipTransfer_FullMethodName, abort)
	type raceResult struct {
		ownership bool
		err       error
	}
	results := make(chan raceResult, 5)
	calls := []struct {
		ownership bool
		call      func(*RoleGRPC) error
	}{
		{true, func(svc *RoleGRPC) error { _, e := svc.PrepareOwnershipTransfer(prepareCtx, prepare); return e }},
		{true, func(svc *RoleGRPC) error { _, e := svc.FinalizeOwnershipTransfer(finalizeCtx, finalize); return e }},
		{true, func(svc *RoleGRPC) error { _, e := svc.AbortOwnershipTransfer(abortCtx, abort); return e }},
		{false, func(svc *RoleGRPC) error {
			_, e := svc.CreateRole(context.Background(), &rolev1.CreateRoleRequest{SpaceId: f.spaceID.String(), Name: "must not survive retirement", Position: 1})
			return e
		}},
		{false, func(svc *RoleGRPC) error {
			_, e := svc.BootstrapSpaceRoles(context.Background(), &rolev1.BootstrapSpaceRolesRequest{SpaceId: f.spaceID.String(), OwnerProfileId: uuid.NewString()})
			return e
		}},
	}
	// The cleanup defer above intentionally releases blocker before these pools.
	racerPools = make([]*pgxpool.Pool, len(calls))
	racerPIDs := make([]int32, len(calls))
	for i := range calls {
		config := f.store.Pool.Config().Copy()
		config.MaxConns = 1
		config.MinConns = 0
		racerPools[i], err = pgxpool.NewWithConfig(ctx, config)
		require.NoError(t, err)
		require.NoError(t, racerPools[i].QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&racerPIDs[i]))
	}
	for i, tc := range calls {
		go func(i int, tc struct {
			ownership bool
			call      func(*RoleGRPC) error
		}) {
			svc := &RoleGRPC{Store: &store.RoleStore{Pool: racerPools[i]}, Events: events}
			results <- raceResult{ownership: tc.ownership, err: tc.call(svc)}
		}(i, tc)
	}
	// Each racer owns one connection, so this is a readiness barrier for these
	// exact backend PIDs rather than an advisory count sampled from a shared pool.
	require.Eventually(t, func() bool {
		var waiters int
		err := f.store.Pool.QueryRow(ctx, `SELECT count(DISTINCT w.pid) FROM pg_locks w JOIN pg_locks b
 ON b.locktype=w.locktype AND b.database IS NOT DISTINCT FROM w.database
 AND b.classid=w.classid AND b.objid=w.objid AND b.objsubid=w.objsubid
	WHERE b.pid=$1 AND b.locktype='advisory' AND b.granted AND NOT w.granted AND w.pid=ANY($2)`, blockerPID, racerPIDs).Scan(&waiters)
		return err == nil && waiters == len(calls)
	}, 5*time.Second, 20*time.Millisecond, "every authority race must wait on the blocker-held Space advisory lock")
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-retirementDone)
	for range calls {
		result := <-results
		if result.ownership {
			require.Equal(t, codes.FailedPrecondition, status.Code(result.err))
		} else {
			require.Equal(t, codes.Unavailable, status.Code(result.err))
		}
	}
	var roles, racedOperations int
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM roles WHERE space_id=$1),
 (SELECT count(*) FROM ownership_transfer_v2 WHERE operation_id=ANY($2))`, f.spaceID,
		[]uuid.UUID{uuid.MustParse(prepare.Intent.OperationId), uuid.MustParse(finalize.Intent.OperationId), uuid.MustParse(abort.Intent.OperationId)}).Scan(&roles, &racedOperations))
	require.Zero(t, roles)
	require.Zero(t, racedOperations)
	count, visibilityErrors := events.snapshot()
	require.Zero(t, count)
	require.Empty(t, visibilityErrors)
}

func TestRetireSpace_RequestHashRuleIsFrozen(t *testing.T) {
	f := r23RetirementFixture{spaceID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), deletionOperationID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), manifestID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), purgeDecidedAt: time.Unix(1_700_000_000, 123_000_000).UTC(), manifestSHA: bytes.Repeat([]byte{0x44}, 32)}
	b, err := (proto.MarshalOptions{Deterministic: true}).Marshal(f.request())
	require.NoError(t, err)
	digest := sha256.Sum256(append(append([]byte("voice.role.v1.RetireSpaceRequest"), 0), b...))
	require.Equal(t, "68bd741dd25bad486363a370f1bd97bf3df472aaab3f220d5f9dbc286ddf67fd", hex.EncodeToString(digest[:]))
}
