package store

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/permissions"
)

func v2Fixture(t *testing.T) (*RoleStore, OwnershipTransferV2Input) {
	t.Helper()
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	s := &RoleStore{Pool: pool}
	in := OwnershipTransferV2Input{ProtocolVersion: 2, SpaceID: uuid.New(), OldOwnerProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), OperationID: uuid.New()}
	in.IntentBytes = v2IntentBytes(t, in)
	hash := sha256.Sum256([]byte("accepted prepare wrapper"))
	in.RequestHash = hash[:]
	require.NoError(t, s.BootstrapSpaceRoles(ctx, in.SpaceID, in.OldOwnerProfileID))
	return s, in
}

func v2IntentBytes(t *testing.T, in OwnershipTransferV2Input) []byte {
	t.Helper()
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&rolev1.OwnershipTransferIntent{ProtocolVersion: in.ProtocolVersion, SpaceId: in.SpaceID.String(), OldOwnerProfileId: in.OldOwnerProfileID.String(), NewOwnerProfileId: in.NewOwnerProfileID.String(), OperationId: in.OperationID.String()})
	require.NoError(t, err)
	return wire
}

func v2RequireOwner(t *testing.T, s *RoleStore, in OwnershipTransferV2Input, owner uuid.UUID) {
	t.Helper()
	// Ordinary Role reads will freeze while prepared; inspect persisted authority directly.
	rows, err := s.Pool.Query(context.Background(), `SELECT mr.profile_id::text FROM member_roles mr JOIN roles r ON r.id=mr.role_id WHERE mr.space_id=$1 AND r.name=$2`, in.SpaceID, permissions.RoleOwner)
	require.NoError(t, err)
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{owner.String()}, ids)
}

func v2RequireNoLedger(t *testing.T, s *RoleStore) {
	t.Helper()
	var count int
	require.NoError(t, s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ownership_transfer_v2`).Scan(&count))
	require.Zero(t, count)
}

func TestOwnershipTransferV2_PreparePersistsWithoutMovingOwnerAndReplays(t *testing.T) {
	s, in := v2Fixture(t)
	// A valid unknown field belongs to the immutable intent and must survive receipts.
	in.IntentBytes = append(in.IntentBytes, 0xa0, 0x06, 0x01)
	first, err := s.PrepareOwnershipTransfer(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, "prepared", first.State)
	require.Equal(t, uuid.Nil, first.CurrentOwnerProfileID)
	require.Equal(t, in.IntentBytes, first.IntentBytes)
	v2RequireOwner(t, s, in, in.OldOwnerProfileID)
	var state string
	var intent, intentHash, prepareHash, finalizeHash, abortHash []byte
	require.NoError(t, s.Pool.QueryRow(context.Background(), `SELECT state,intent_bytes,intent_hash,prepare_request_hash,finalize_request_hash,abort_request_hash FROM ownership_transfer_v2 WHERE operation_id=$1`, in.OperationID).Scan(&state, &intent, &intentHash, &prepareHash, &finalizeHash, &abortHash))
	require.Equal(t, "prepared", state)
	require.Equal(t, in.IntentBytes, intent)
	wantHash := sha256.Sum256(in.IntentBytes)
	require.Equal(t, wantHash[:], intentHash)
	require.Equal(t, in.RequestHash, prepareHash)
	require.Nil(t, finalizeHash)
	require.Nil(t, abortHash)
	// A fresh store object must use the durable receipt, not process memory.
	replay, err := (&RoleStore{Pool: s.Pool}).PrepareOwnershipTransfer(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	var count int
	require.NoError(t, s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ownership_transfer_v2 WHERE operation_id=$1`, in.OperationID).Scan(&count))
	require.Equal(t, 1, count)
	v2RequireOwner(t, s, in, in.OldOwnerProfileID)
}

func TestOwnershipTransferV2_AbsentFinalizeCannotMoveOwnerOrCreateReceipt(t *testing.T) {
	s, in := v2Fixture(t)
	_, err := s.FinalizeOwnershipTransfer(context.Background(), in)
	require.ErrorIs(t, err, ErrOwnershipTransferMissing)
	v2RequireNoLedger(t, s)
	v2RequireOwner(t, s, in, in.OldOwnerProfileID)
}

func TestOwnershipTransferV2_AbortBeforePrepareIsDurableBarrier(t *testing.T) {
	s, in := v2Fixture(t)
	first, err := s.AbortOwnershipTransfer(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, "aborted", first.State)
	require.Equal(t, in.OldOwnerProfileID, first.CurrentOwnerProfileID)
	require.Equal(t, in.IntentBytes, first.IntentBytes)
	var state string
	var prepareHash, finalizeHash, abortHash []byte
	require.NoError(t, s.Pool.QueryRow(context.Background(), `SELECT state,prepare_request_hash,finalize_request_hash,abort_request_hash FROM ownership_transfer_v2 WHERE operation_id=$1`, in.OperationID).Scan(&state, &prepareHash, &finalizeHash, &abortHash))
	require.Equal(t, "aborted", state)
	require.Nil(t, prepareHash)
	require.Nil(t, finalizeHash)
	require.Equal(t, in.RequestHash, abortHash)
	replay, err := (&RoleStore{Pool: s.Pool}).AbortOwnershipTransfer(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	_, err = s.PrepareOwnershipTransfer(context.Background(), in)
	require.ErrorIs(t, err, ErrOwnershipTransferState)
	_, err = s.FinalizeOwnershipTransfer(context.Background(), in)
	require.ErrorIs(t, err, ErrOwnershipTransferState)
	v2RequireOwner(t, s, in, in.OldOwnerProfileID)
}

func TestOwnershipTransferV2_RejectsIncoherentCanonicalInputWithoutPersistence(t *testing.T) {
	s, base := v2Fixture(t)
	cases := map[string]func(*OwnershipTransferV2Input){
		"normalized space":      func(in *OwnershipTransferV2Input) { in.SpaceID = uuid.New() },
		"normalized old owner":  func(in *OwnershipTransferV2Input) { in.OldOwnerProfileID = uuid.New() },
		"normalized new owner":  func(in *OwnershipTransferV2Input) { in.NewOwnerProfileID = uuid.New() },
		"normalized operation":  func(in *OwnershipTransferV2Input) { in.OperationID = uuid.New() },
		"normalized version":    func(in *OwnershipTransferV2Input) { in.ProtocolVersion = 3 },
		"unsupported version":   func(in *OwnershipTransferV2Input) { in.ProtocolVersion = 3; in.IntentBytes = v2IntentBytes(t, *in) },
		"empty wire":            func(in *OwnershipTransferV2Input) { in.IntentBytes = nil },
		"malformed wire":        func(in *OwnershipTransferV2Input) { in.IntentBytes = []byte{0xff} },
		"duplicate known field": func(in *OwnershipTransferV2Input) { in.IntentBytes = append(in.IntentBytes, 0x08, 0x02) },
		"invalid wire uuid": func(in *OwnershipTransferV2Input) {
			in.IntentBytes = protowire.AppendString(protowire.AppendTag(in.IntentBytes, 2, protowire.BytesType), "not-a-uuid")
		},
		"short request hash": func(in *OwnershipTransferV2Input) { in.RequestHash = make([]byte, 31) },
		"long request hash":  func(in *OwnershipTransferV2Input) { in.RequestHash = make([]byte, 33) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := base
			in.IntentBytes = append([]byte(nil), base.IntentBytes...)
			in.RequestHash = append([]byte(nil), base.RequestHash...)
			change(&in)
			for _, action := range []func(context.Context, OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error){s.PrepareOwnershipTransfer, s.FinalizeOwnershipTransfer, s.AbortOwnershipTransfer} {
				_, err := action(context.Background(), in)
				require.ErrorIs(t, err, ErrOwnershipTransferV2InvalidInput)
				v2RequireNoLedger(t, s)
				v2RequireOwner(t, s, base, base.OldOwnerProfileID)
			}
		})
	}
}

func TestOwnershipTransferV2_PreparedImmutableBindingConflicts(t *testing.T) {
	s, base := v2Fixture(t)
	base.IntentBytes = append(base.IntentBytes, 0xa0, 0x06, 0x01)
	first, err := s.PrepareOwnershipTransfer(context.Background(), base)
	require.NoError(t, err)
	for _, name := range []string{"new owner", "old owner", "space", "version", "unknown intent", "wrapper hash"} {
		t.Run(name, func(t *testing.T) {
			in := base
			in.IntentBytes = append([]byte(nil), base.IntentBytes...)
			in.RequestHash = append([]byte(nil), base.RequestHash...)
			switch name {
			case "new owner":
				in.NewOwnerProfileID = uuid.New()
			case "old owner":
				in.OldOwnerProfileID = uuid.New()
			case "space":
				in.SpaceID = uuid.New()
			case "version":
				in.ProtocolVersion = 3
			case "unknown intent":
				in.IntentBytes[len(in.IntentBytes)-1] = 2
			case "wrapper hash":
				in.RequestHash[0] ^= 1
			}
			if name != "unknown intent" && name != "wrapper hash" {
				in.IntentBytes = append(v2IntentBytes(t, in), 0xa0, 0x06, 0x01)
			}
			_, err := s.PrepareOwnershipTransfer(context.Background(), in)
			require.ErrorIs(t, err, ErrOwnershipTransferConflict)
			replay, err := s.PrepareOwnershipTransfer(context.Background(), base)
			require.NoError(t, err)
			require.Equal(t, first, replay, "rejected changes cannot replace accepted intent or action hash")
			v2RequireOwner(t, s, base, base.OldOwnerProfileID)
		})
	}
}

func TestOwnershipTransferV2_AbsentPrepareAndAbortRequireSoleExpectedOwner(t *testing.T) {
	for _, actionName := range []string{"prepare", "abort"} {
		for _, scenario := range []string{"wrong owner", "multiple owners"} {
			t.Run(actionName+"/"+scenario, func(t *testing.T) {
				s, in := v2Fixture(t)
				other := uuid.New()
				if scenario == "wrong owner" {
					// Keep the fixture's sole owner unchanged; submit a coherent but stale old owner.
					in.OldOwnerProfileID = other
					in.IntentBytes = v2IntentBytes(t, in)
				} else {
					_, err := s.Pool.Exec(context.Background(), `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) SELECT $1,$2,id,$3 FROM roles WHERE space_id=$1 AND name=$4`, in.SpaceID, other, in.OldOwnerProfileID, permissions.RoleOwner)
					require.NoError(t, err)
				}
				var before, after []string
				readOwners := func() []string {
					rows, err := s.Pool.Query(context.Background(), `SELECT mr.profile_id::text FROM member_roles mr JOIN roles r ON r.id=mr.role_id WHERE mr.space_id=$1 AND r.name=$2 ORDER BY mr.profile_id`, in.SpaceID, permissions.RoleOwner)
					require.NoError(t, err)
					defer rows.Close()
					var owners []string
					for rows.Next() {
						var id string
						require.NoError(t, rows.Scan(&id))
						owners = append(owners, id)
					}
					require.NoError(t, rows.Err())
					return owners
				}
				before = readOwners()
				action := s.PrepareOwnershipTransfer
				if actionName == "abort" {
					action = s.AbortOwnershipTransfer
				}
				_, err := action(context.Background(), in)
				require.ErrorIs(t, err, ErrOwnershipTransferState)
				v2RequireNoLedger(t, s)
				after = readOwners()
				require.Equal(t, before, after, "failed owner validation cannot modify authority")
			})
		}
	}
}
