package grpcsvc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
	"voice/backend/role/permissions"

	rolev1 "voice.app/voice/role/v1"
)

func trustedOwnershipTransferContext(ctx context.Context, rpc, spaceID, oldOwnerID, newOwnerID, operationID string) context.Context {
	var req proto.Message = &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID}
	if rpc == compensateOwnershipTransferRPC {
		req = &rolev1.CompensateOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID}
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		panic(err)
	}
	return principal.WithVerified(ctx, principal.Principal{
		Kind:        "service",
		Issuer:      "space",
		Subject:     "service:space",
		Audience:    "role",
		RPC:         rpc,
		RequestID:   "test-request",
		RequestHash: hash,
	})
}

func ownerRoleNames(t *testing.T, s *store.RoleStore, spaceID, profileID uuid.UUID) []string {
	t.Helper()
	roles, err := s.GetMemberRoles(context.Background(), spaceID, profileID)
	require.NoError(t, err)
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

func TestApplyOwnershipTransfer_RequiresVerifiedSpacePrincipal(t *testing.T) {
	spaceID, oldOwnerID, newOwnerID := uuid.New(), uuid.New(), uuid.New()
	_, err := ownershipTransferInput(context.Background(), applyOwnershipTransferRPC, &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwnerID.String(), NewOwnerProfileId: newOwnerID.String(), OperationId: uuid.NewString()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestOwnershipTransferInput_RequiresExactVerifiedBinding(t *testing.T) {
	spaceID, oldOwnerID, newOwnerID, operationID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	ctx := trustedOwnershipTransferContext(context.Background(), applyOwnershipTransferRPC, spaceID, oldOwnerID, newOwnerID, operationID)
	_, err := ownershipTransferInput(ctx, applyOwnershipTransferRPC, &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID})
	require.NoError(t, err)

	wrongRPC := trustedOwnershipTransferContext(context.Background(), compensateOwnershipTransferRPC, spaceID, oldOwnerID, newOwnerID, operationID)
	_, err = ownershipTransferInput(wrongRPC, applyOwnershipTransferRPC, &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	wrongBinding := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: applyOwnershipTransferRPC, RequestID: "request", RequestHash: "sha256:not-the-request"})
	_, err = ownershipTransferInput(wrongBinding, applyOwnershipTransferRPC, &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID, OldOwnerProfileId: oldOwnerID, NewOwnerProfileId: newOwnerID, OperationId: operationID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestOwnershipTransferLifecycle_AppliesReplaysConflictsAndCompensates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	spaceID, oldOwnerID, newOwnerID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(context.Background(), spaceID, oldOwnerID))
	svc := &RoleGRPC{Store: s}
	opID := uuid.NewString()
	apply := &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwnerID.String(), NewOwnerProfileId: newOwnerID.String(), OperationId: opID}
	applyCtx := trustedOwnershipTransferContext(context.Background(), "/voice.role.v1.RoleService/ApplyOwnershipTransfer", apply.SpaceId, apply.OldOwnerProfileId, apply.NewOwnerProfileId, apply.OperationId)

	resp, err := svc.ApplyOwnershipTransfer(applyCtx, apply)
	require.NoError(t, err)
	require.Equal(t, newOwnerID.String(), resp.GetCurrentOwnerProfileId())
	require.NotContains(t, ownerRoleNames(t, s, spaceID, oldOwnerID), permissions.RoleOwner)
	require.Contains(t, ownerRoleNames(t, s, spaceID, newOwnerID), permissions.RoleOwner)

	resp, err = svc.ApplyOwnershipTransfer(applyCtx, apply)
	require.NoError(t, err)
	require.Equal(t, newOwnerID.String(), resp.GetCurrentOwnerProfileId())

	changed := &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwnerID.String(), NewOwnerProfileId: uuid.NewString(), OperationId: opID}
	changedCtx := trustedOwnershipTransferContext(context.Background(), "/voice.role.v1.RoleService/ApplyOwnershipTransfer", changed.SpaceId, changed.OldOwnerProfileId, changed.NewOwnerProfileId, changed.OperationId)
	_, err = svc.ApplyOwnershipTransfer(changedCtx, changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))

	compensate := &rolev1.CompensateOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwnerID.String(), NewOwnerProfileId: newOwnerID.String(), OperationId: opID}
	compensateCtx := trustedOwnershipTransferContext(context.Background(), "/voice.role.v1.RoleService/CompensateOwnershipTransfer", compensate.SpaceId, compensate.OldOwnerProfileId, compensate.NewOwnerProfileId, compensate.OperationId)
	compensateResp, err := svc.CompensateOwnershipTransfer(compensateCtx, compensate)
	require.NoError(t, err)
	require.Equal(t, oldOwnerID.String(), compensateResp.GetCurrentOwnerProfileId())
	require.Contains(t, ownerRoleNames(t, s, spaceID, oldOwnerID), permissions.RoleOwner)
	require.NotContains(t, ownerRoleNames(t, s, spaceID, newOwnerID), permissions.RoleOwner)

	compensateResp, err = svc.CompensateOwnershipTransfer(compensateCtx, compensate)
	require.NoError(t, err)
	require.Equal(t, oldOwnerID.String(), compensateResp.GetCurrentOwnerProfileId())
	// A stale successful Apply receipt must not resurrect a compensated operation.
	resp, err = svc.ApplyOwnershipTransfer(applyCtx, apply)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Nil(t, resp)
	requireSoleTerminalOwner(t, s, spaceID, oldOwnerID)
}

func terminalOwnershipContext(t *testing.T, rpc string, req proto.Message) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: rpc, RequestID: "terminal-operation", RequestHash: hash})
}

func TestOwnershipTransferLifecycle_CompensationBeforeDelayedApplyIsTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st, cleanup := startRoleStoreTest(t)
	defer cleanup()
	spaceID, oldOwner, newOwner := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(context.Background(), spaceID, oldOwner))
	svc := &RoleGRPC{Store: st}
	apply := &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwner.String(), NewOwnerProfileId: newOwner.String(), OperationId: uuid.NewString()}
	compensate := &rolev1.CompensateOwnershipTransferRequest{SpaceId: apply.SpaceId, OldOwnerProfileId: apply.OldOwnerProfileId, NewOwnerProfileId: apply.NewOwnerProfileId, OperationId: apply.OperationId}
	applyCtx := terminalOwnershipContext(t, applyOwnershipTransferRPC, apply)
	compensateCtx := terminalOwnershipContext(t, compensateOwnershipTransferRPC, compensate)
	ready, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	done := make(chan struct{})
	defer func() {
		unblock()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("delayed Apply goroutine did not finish")
		}
	}()
	lateResult := make(chan error, 1)
	go func() {
		defer close(done)
		close(ready)
		<-release // Already-issued RPC arrives at the handler/store only after compensation.
		_, err := svc.ApplyOwnershipTransfer(applyCtx, apply)
		lateResult <- err
	}()
	<-ready
	response, err := svc.CompensateOwnershipTransfer(compensateCtx, compensate)
	require.NoError(t, err, "compensation must create the durable abort receipt before a delayed Apply arrives")
	require.Equal(t, oldOwner.String(), response.GetCurrentOwnerProfileId())
	// Prove the outcome is persisted, rather than relying only on a handler response.
	var receiptOwner string
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT current_owner_profile_id FROM ownership_transfer_role_receipts WHERE operation_id=$1 AND action='compensate'`, apply.OperationId).Scan(&receiptOwner))
	require.Equal(t, oldOwner.String(), receiptOwner)
	unblock()
	select {
	case err := <-lateResult:
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	case <-time.After(3 * time.Second):
		t.Fatal("delayed Apply did not finish")
	}
	response, err = svc.CompensateOwnershipTransfer(compensateCtx, compensate)
	require.NoError(t, err)
	require.Equal(t, oldOwner.String(), response.GetCurrentOwnerProfileId())
	requireSoleTerminalOwner(t, st, spaceID, oldOwner)
	for _, change := range []string{"tuple", "hash"} {
		t.Run(change, func(t *testing.T) {
			changed := proto.Clone(compensate).(*rolev1.CompensateOwnershipTransferRequest)
			if change == "tuple" {
				changed.NewOwnerProfileId = uuid.NewString()
			} else {
				changed.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			}
			_, err := svc.CompensateOwnershipTransfer(terminalOwnershipContext(t, compensateOwnershipTransferRPC, changed), changed)
			require.Equal(t, codes.AlreadyExists, status.Code(err), "same operation cannot change its terminal tuple or canonical request binding")
		})
	}
	requireSoleTerminalOwner(t, st, spaceID, oldOwner)
}

func requireSoleTerminalOwner(t *testing.T, st *store.RoleStore, spaceID, owner uuid.UUID) {
	t.Helper()
	rows, err := st.Pool.Query(context.Background(), `SELECT mr.profile_id::text FROM member_roles mr JOIN roles r ON r.id=mr.role_id WHERE mr.space_id=$1 AND r.name=$2`, spaceID, permissions.RoleOwner)
	require.NoError(t, err)
	defer rows.Close()
	var owners []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		owners = append(owners, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{owner.String()}, owners)
}

func TestOwnershipTransferLifecycle_CompensationRequiresSoleExpectedOldOwner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	for _, scenario := range []string{"wrong owner", "multiple owners"} {
		t.Run(scenario, func(t *testing.T) {
			st, cleanup := startRoleStoreTest(t)
			defer cleanup()
			spaceID, oldOwner, newOwner, otherOwner := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			actualOwner := oldOwner
			if scenario == "wrong owner" {
				actualOwner = otherOwner
			}
			require.NoError(t, st.BootstrapSpaceRoles(context.Background(), spaceID, actualOwner))
			if scenario == "multiple owners" {
				_, err := st.Pool.Exec(context.Background(), `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) SELECT $1,$2,id,$3 FROM roles WHERE space_id=$1 AND name=$4`, spaceID, otherOwner, oldOwner, permissions.RoleOwner)
				require.NoError(t, err)
			}
			req := &rolev1.CompensateOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: oldOwner.String(), NewOwnerProfileId: newOwner.String(), OperationId: uuid.NewString()}
			_, err := (&RoleGRPC{Store: st}).CompensateOwnershipTransfer(terminalOwnershipContext(t, compensateOwnershipTransferRPC, req), req)
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			var receipts int
			require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ownership_transfer_role_receipts WHERE operation_id=$1`, req.OperationId).Scan(&receipts))
			require.Zero(t, receipts, "failed state validation cannot fabricate an abort receipt")
			require.Contains(t, ownerRoleNames(t, st, spaceID, actualOwner), permissions.RoleOwner)
			if scenario == "multiple owners" {
				require.Contains(t, ownerRoleNames(t, st, spaceID, otherOwner), permissions.RoleOwner)
			}
			require.NotContains(t, ownerRoleNames(t, st, spaceID, newOwner), permissions.RoleOwner)
		})
	}
}
