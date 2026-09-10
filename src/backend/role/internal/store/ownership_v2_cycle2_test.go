package store

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func v2ActionInput(in OwnershipTransferV2Input, action string) OwnershipTransferV2Input {
	h := sha256.Sum256([]byte(action))
	in.RequestHash = h[:]
	return in
}

func v2Terminal(s *RoleStore, action string) func(context.Context, OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error) {
	if action == "finalized" {
		return s.FinalizeOwnershipTransfer
	}
	return s.AbortOwnershipTransfer
}

func v2ReadActionHashes(t *testing.T, s *RoleStore, id uuid.UUID) (string, []byte, []byte, []byte) {
	t.Helper()
	var state string
	var prepare, finalize, abort []byte
	require.NoError(t, s.Pool.QueryRow(context.Background(), `SELECT state,prepare_request_hash,finalize_request_hash,abort_request_hash FROM ownership_transfer_v2 WHERE operation_id=$1`, id).Scan(&state, &prepare, &finalize, &abort))
	return state, prepare, finalize, abort
}

func TestOwnershipTransferV2_Cycle2PreparedTerminalMatrixAndActionHashes(t *testing.T) {
	for _, terminal := range []string{"finalized", "aborted"} {
		t.Run(terminal, func(t *testing.T) {
			s, prepare := v2Fixture(t)
			prepare.IntentBytes = append(prepare.IntentBytes, 0xa0, 0x06, 0x01)
			_, err := s.PrepareOwnershipTransfer(context.Background(), prepare)
			require.NoError(t, err)
			in := v2ActionInput(prepare, terminal)
			first, err := v2Terminal(s, terminal)(context.Background(), in)
			require.NoError(t, err, "each action accepts its own first wrapper hash")
			require.Equal(t, terminal, first.State)
			require.Equal(t, prepare.IntentBytes, first.IntentBytes)
			owner := prepare.OldOwnerProfileID
			if terminal == "finalized" {
				owner = prepare.NewOwnerProfileID
			}
			require.Equal(t, owner, first.CurrentOwnerProfileID)
			v2RequireOwner(t, s, prepare, owner)
			fresh := &RoleStore{Pool: s.Pool}
			replay, err := v2Terminal(fresh, terminal)(context.Background(), in)
			require.NoError(t, err)
			require.Equal(t, first, replay)
			for _, changedField := range []string{"wrapper", "intent", "tuple", "version"} {
				changed := in
				switch changedField {
				case "wrapper":
					changed = v2ActionInput(in, "different accepted action body")
				case "intent":
					changed.IntentBytes = append([]byte(nil), in.IntentBytes...)
					changed.IntentBytes[len(changed.IntentBytes)-1] = 2
				case "tuple":
					changed.NewOwnerProfileID = uuid.New()
					changed.IntentBytes = v2IntentBytes(t, changed)
				case "version":
					changed.ProtocolVersion = 3
					changed.IntentBytes = v2IntentBytes(t, changed)
				}
				_, err := v2Terminal(s, terminal)(context.Background(), changed)
				require.ErrorIs(t, err, ErrOwnershipTransferConflict, changedField)
			}
			_, err = s.PrepareOwnershipTransfer(context.Background(), prepare)
			require.ErrorIs(t, err, ErrOwnershipTransferState)
			opposite := "aborted"
			if terminal == "aborted" {
				opposite = "finalized"
			}
			_, err = v2Terminal(s, opposite)(context.Background(), v2ActionInput(prepare, opposite))
			require.ErrorIs(t, err, ErrOwnershipTransferState)
			state, prepHash, finalHash, abortHash := v2ReadActionHashes(t, s, prepare.OperationID)
			require.Equal(t, terminal, state)
			require.Equal(t, prepare.RequestHash, prepHash)
			if terminal == "finalized" {
				require.Equal(t, in.RequestHash, finalHash)
				require.Nil(t, abortHash)
			} else {
				require.Nil(t, finalHash)
				require.Equal(t, in.RequestHash, abortHash)
			}
			v2RequireOwner(t, s, prepare, owner)
		})
	}
}

func TestOwnershipTransferV2_Cycle2RejectedOwnerStateDoesNotPinTerminalHash(t *testing.T) {
	for _, action := range []string{"finalized", "aborted"} {
		for _, corruption := range []string{"missing membership", "wrong owner", "multiple owners", "missing owner role"} {
			t.Run(action+"/"+corruption, func(t *testing.T) {
				s, in := v2Fixture(t)
				ctx := context.Background()
				_, err := s.PrepareOwnershipTransfer(ctx, in)
				require.NoError(t, err)
				var roleID uuid.UUID
				require.NoError(t, s.Pool.QueryRow(ctx, `SELECT id FROM roles WHERE space_id=$1 AND name='Owner'`, in.SpaceID).Scan(&roleID))
				switch corruption {
				case "missing membership":
					_, err = s.Pool.Exec(ctx, `DELETE FROM member_roles WHERE role_id=$1`, roleID)
				case "wrong owner":
					_, err = s.Pool.Exec(ctx, `UPDATE member_roles SET profile_id=$2 WHERE role_id=$1`, roleID, uuid.New())
				case "multiple owners":
					_, err = s.Pool.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$4)`, in.SpaceID, uuid.New(), roleID, in.OldOwnerProfileID)
				case "missing owner role":
					_, err = s.Pool.Exec(ctx, `UPDATE roles SET is_system=false WHERE id=$1`, roleID)
				}
				require.NoError(t, err)
				_, err = v2Terminal(s, action)(ctx, v2ActionInput(in, "rejected terminal body"))
				require.ErrorIs(t, err, ErrOwnershipTransferState)
				state, prepHash, finalHash, abortHash := v2ReadActionHashes(t, s, in.OperationID)
				require.Equal(t, "prepared", state)
				require.Equal(t, in.RequestHash, prepHash)
				require.Nil(t, finalHash)
				require.Nil(t, abortHash)
				_, err = s.Pool.Exec(ctx, `DELETE FROM member_roles WHERE role_id=$1`, roleID)
				require.NoError(t, err)
				_, err = s.Pool.Exec(ctx, `UPDATE roles SET is_system=true WHERE id=$1`, roleID)
				require.NoError(t, err)
				_, err = s.Pool.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$2)`, in.SpaceID, in.OldOwnerProfileID, roleID)
				require.NoError(t, err)
				accepted := v2ActionInput(in, "first accepted terminal body")
				_, err = v2Terminal(s, action)(ctx, accepted)
				require.NoError(t, err, "failed transition must not pin rejected hash")
			})
		}
	}
}

func TestOwnershipTransferV2_Cycle2HistoricalReplayFrozenByOtherOperationThenImmutable(t *testing.T) {
	for _, terminal := range []string{"finalized", "aborted"} {
		t.Run(terminal, func(t *testing.T) {
			s, first := v2Fixture(t)
			ctx := context.Background()
			_, err := s.PrepareOwnershipTransfer(ctx, first)
			require.NoError(t, err)
			history := v2ActionInput(first, terminal)
			receipt, err := v2Terminal(s, terminal)(ctx, history)
			require.NoError(t, err)
			second := first
			second.OperationID = uuid.New()
			second.OldOwnerProfileID = receipt.CurrentOwnerProfileID
			second.NewOwnerProfileID = uuid.New()
			second.IntentBytes = v2IntentBytes(t, second)
			_, err = s.PrepareOwnershipTransfer(ctx, second)
			require.NoError(t, err)
			_, err = v2Terminal(s, terminal)(ctx, history)
			require.ErrorIs(t, err, ErrSpaceFrozen)
			unrelated := second
			unrelated.OperationID = uuid.New()
			unrelated.IntentBytes = v2IntentBytes(t, unrelated)
			for _, fn := range []func(context.Context, OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error){s.PrepareOwnershipTransfer, s.FinalizeOwnershipTransfer, s.AbortOwnershipTransfer} {
				_, err = fn(ctx, unrelated)
				require.ErrorIs(t, err, ErrSpaceFrozen)
			}
			_, err = s.FinalizeOwnershipTransfer(ctx, v2ActionInput(second, "second final"))
			require.NoError(t, err)
			replay, err := v2Terminal(&RoleStore{Pool: s.Pool}, terminal)(ctx, history)
			require.NoError(t, err)
			require.Equal(t, receipt, replay)
			v2RequireOwner(t, s, second, second.NewOwnerProfileID)
		})
	}
}

func TestOwnershipTransferV2_Cycle2V1OperationCannotBeReinterpreted(t *testing.T) {
	for _, legacyAction := range []string{"apply", "compensate"} {
		t.Run(legacyAction, func(t *testing.T) {
			s, in := v2Fixture(t)
			ctx := context.Background()
			_, err := s.Pool.Exec(ctx, `INSERT INTO ownership_transfer_role_receipts(operation_id,action,space_id,old_owner_profile_id,new_owner_profile_id,request_hash,current_owner_profile_id) VALUES($1,$2,$3,$4,$5,'legacy-request',$4)`, in.OperationID, legacyAction, in.SpaceID, in.OldOwnerProfileID, in.NewOwnerProfileID)
			require.NoError(t, err)
			for _, fn := range []func(context.Context, OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error){s.PrepareOwnershipTransfer, s.FinalizeOwnershipTransfer, s.AbortOwnershipTransfer} {
				_, err := fn(ctx, in)
				require.ErrorIs(t, err, ErrOwnershipTransferConflict)
				v2RequireNoLedger(t, s)
				v2RequireOwner(t, s, in, in.OldOwnerProfileID)
			}
		})
	}
}

func TestOwnershipTransferV2_Cycle2RetiredFenceAndReceiptSurviveOrdinaryCleanup(t *testing.T) {
	s, in := v2Fixture(t)
	ctx := context.Background()
	_, err := s.AbortOwnershipTransfer(ctx, in)
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,now())`, in.SpaceID)
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `DELETE FROM member_roles WHERE space_id=$1`, in.SpaceID)
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `DELETE FROM roles WHERE space_id=$1`, in.SpaceID)
	require.NoError(t, err)
	state, _, _, abort := v2ReadActionHashes(t, s, in.OperationID)
	require.Equal(t, "aborted", state)
	require.Equal(t, in.RequestHash, abort)
	for _, operation := range []uuid.UUID{in.OperationID, uuid.New()} {
		request := in
		request.OperationID = operation
		request.IntentBytes = v2IntentBytes(t, request)
		for _, fn := range []func(context.Context, OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error){s.PrepareOwnershipTransfer, s.FinalizeOwnershipTransfer, s.AbortOwnershipTransfer} {
			_, err := fn(ctx, request)
			require.ErrorIs(t, err, ErrSpaceRetired)
		}
	}
	var retired bool
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT retired_at IS NOT NULL FROM role_space_lifecycle WHERE space_id=$1`, in.SpaceID).Scan(&retired))
	require.True(t, retired)
}

func TestOwnershipTransferV2_Cycle2SchemaRejectsInvalidActionHistory(t *testing.T) {
	s, in := v2Fixture(t)
	ctx := context.Background()
	_, err := s.PrepareOwnershipTransfer(ctx, in)
	require.NoError(t, err)
	for _, assignment := range []string{
		"prepare_request_hash=NULL", "finalize_request_hash=prepare_request_hash", "abort_request_hash=prepare_request_hash",
		"state='finalized'", "state='aborted'", "protocol_version=3", "intent_bytes=''::bytea", "intent_hash='x'::bytea", "prepare_request_hash='x'::bytea", "new_owner_profile_id=old_owner_profile_id",
	} {
		t.Run(assignment, func(t *testing.T) {
			_, err := s.Pool.Exec(ctx, `UPDATE ownership_transfer_v2 SET `+assignment+` WHERE operation_id=$1`, in.OperationID)
			var pgerr *pgconn.PgError
			require.ErrorAs(t, err, &pgerr)
			require.Equal(t, "23514", pgerr.Code)
		})
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO ownership_transfer_v2 SELECT $2,space_id,protocol_version,old_owner_profile_id,new_owner_profile_id,intent_bytes,intent_hash,state,prepare_request_hash,finalize_request_hash,abort_request_hash,created_at,updated_at FROM ownership_transfer_v2 WHERE operation_id=$1`, in.OperationID, uuid.New())
	var pgerr *pgconn.PgError
	require.ErrorAs(t, err, &pgerr)
	require.Equal(t, "23505", pgerr.Code)
	state, prepare, finalize, abort := v2ReadActionHashes(t, s, in.OperationID)
	require.Equal(t, "prepared", state)
	require.Equal(t, in.RequestHash, prepare)
	require.Nil(t, finalize)
	require.Nil(t, abort)
}
