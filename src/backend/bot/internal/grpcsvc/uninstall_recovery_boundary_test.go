package grpcsvc_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botv1 "voice.app/voice/bot/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type unavailableLegacyUninstallRole struct {
	rolev1.RoleServiceClient
	calls *int
}

func (r unavailableLegacyUninstallRole) DeleteRolesCreatedByProfile(context.Context, *rolev1.DeleteRolesCreatedByProfileRequest, ...grpc.CallOption) (*rolev1.DeleteRolesCreatedByProfileResponse, error) {
	*r.calls++
	return nil, status.Error(codes.Unavailable, "Role cleanup unavailable")
}
func (r unavailableLegacyUninstallRole) GetMemberRoles(context.Context, *rolev1.GetMemberRolesRequest, ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	*r.calls++
	return nil, status.Error(codes.Unavailable, "Role cleanup unavailable")
}

func TestUninstallRetainsCanceledAccessAndPendingCleanupWithoutLegacyFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL uninstall cleanup boundary")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_uninstall_recovery", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	owner, installer, space := uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Pending uninstall", "", `[]`, uuid.New())
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	svc := grpcsvc.NewBotGRPC(st, nil)
	svc.PrincipalIssuer = installationTestIssuer(t)
	svc.InstallationRole = installationPermissionClient{allowed: true}
	removed, rawCalls := 0, 0
	svc.InstallationSpace = intentObservingSpace{remove: func(context.Context, *spacev1.RemoveBotMemberRequest) error { removed++; return nil }}
	svc.Role = unavailableLegacyUninstallRole{calls: &rawCalls}
	request := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-user-id", owner.String(), "x-voice-profile-id", installer.String()))
	// A request-bound signed component actor passes the real initiation guard;
	// current manageBots and native cancellation are controlled protocol fixtures.
	_, err = svc.UninstallBotFromSpace(verifiedMutationActor(t, ctx, request, botv1.BotService_UninstallBotFromSpace_FullMethodName), request)
	// The public Empty response is not specified as an all-participant receipt.
	// This oracle checks authority and retained work, not a newly invented code.
	if err != nil {
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
	require.Equal(t, 1, removed, "live Space access must already be canceled")
	require.Zero(t, rawCalls, "ordinary Role helpers are not private cleanup authority")
	var state string
	var installations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM bot_installation_intents WHERE installation_id=$1`, intent.InstallationID).Scan(&state))
	require.Equal(t, "CANCELING", state)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_installations WHERE id=$1`, intent.InstallationID).Scan(&installations))
	require.Equal(t, 1, installations, "retain the obligation until typed participant receipts")
	current, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "live", current.Status, "Space uninstall must not globally disable the application")
	var globalRoots, uninstallRoots, pending int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&globalRoots))
	require.Zero(t, globalRoots)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_operations WHERE bot_id=$1 AND state='RETRY_WAIT'`, bot.ID).Scan(&uninstallRoots))
	require.Equal(t, 1, uninstallRoots)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_effects WHERE kind='ROLE_REMOVE' AND state='PENDING'`).Scan(&pending))
	require.Equal(t, 1, pending)
	_, err = st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.ErrorIs(t, err, store.ErrInstallationConflict, "reinstall cannot bypass pending cleanup")
}

type controlledUninstallRole struct {
	rolev1.RoleServiceClient
	t                *testing.T
	request          []byte
	response         *rolev1.ApplyBotInstallationRemovalResponse
	calls            int
	loseACK, corrupt bool
}

func (r *controlledUninstallRole) ApplyBotInstallationRemoval(ctx context.Context, q *rolev1.ApplyBotInstallationRemovalRequest, _ ...grpc.CallOption) (*rolev1.ApplyBotInstallationRemovalResponse, error) {
	r.calls++
	require.Equal(r.t, "space_uninstall", q.Purpose, "ordinary uninstall cannot acquire global terminal authority")
	outgoing, _ := metadata.FromOutgoingContext(ctx)
	require.Len(r.t, outgoing.Get("authorization"), 1)
	require.Empty(r.t, outgoing.Get("x-voice-profile-id"))
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(r.t, err)
	if r.request != nil {
		require.True(r.t, bytes.Equal(r.request, body), "recovery must retain exact child bytes")
	} else {
		r.request = body
		evidence, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q.GetNative())
		require.NoError(r.t, err)
		bodySHA, evidenceSHA := sha256.Sum256(body), sha256.Sum256(evidence)
		r.response = &rolev1.ApplyBotInstallationRemovalResponse{Receipt: &rolev1.BotInstallationRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, DecisionId: q.DecisionId, Purpose: q.Purpose, BotId: q.BotId, ActorProfileId: q.ActorProfileId, SpaceId: q.SpaceId, EvidenceKind: "native", EvidenceSha256: evidenceSHA[:], FrozenScopeSha256: append([]byte(nil), q.FrozenScopeSha256...), RequestSha256: bodySHA[:], Participant: "role", Outcome: "COMPLETED", CompletedAt: timestamppb.Now()}}
	}
	if r.loseACK {
		r.loseACK = false
		return nil, status.Error(codes.Unavailable, "controlled participant committed but reply was lost")
	}
	result := proto.Clone(r.response).(*rolev1.ApplyBotInstallationRemovalResponse)
	if r.corrupt {
		result.Receipt.Purpose = "global_delete"
	}
	return result, nil
}

func TestUninstallTypedCleanupLostACKRecoveryAndReinstall(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL uninstall journal recovery")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_uninstall_typed", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	owner, manager, space, otherSpace := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	managerAccount := uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Uninstall recovery", "", `[]`, uuid.New())
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, manager, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	other, err := st.PrepareInstallation(ctx, bot.ID, otherSpace, manager, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, other, nil))
	role := &controlledUninstallRole{t: t, loseACK: true}
	svc := grpcsvc.NewBotGRPC(st, nil)
	svc.PrincipalIssuer = installationTestIssuer(t)
	svc.InstallationRole = installationPermissionClient{allowed: true}
	svc.CleanupRole = role
	removed := 0
	svc.InstallationSpace = intentObservingSpace{remove: func(context.Context, *spacev1.RemoveBotMemberRequest) error { removed++; return nil }}
	request := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-user-id", managerAccount.String(), "x-voice-profile-id", manager.String()))
	_, err = svc.UninstallBotFromSpace(verifiedMutationActor(t, ctx, request, botv1.BotService_UninstallBotFromSpace_FullMethodName), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, 1, removed)
	require.Equal(t, 1, role.calls)
	op, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, managerAccount, op.OwnerAccountID, "historical column stores the verified initiating account; Space managers need not own the Bot")
	require.Equal(t, "RETRY_WAIT", op.State)
	originalDecision := append([]byte(nil), op.DecisionBytes...)
	var targetCount, globalCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_installations WHERE operation_id=$1`, op.OperationID).Scan(&targetCount))
	require.Equal(t, 1, targetCount)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&globalCount))
	require.Zero(t, globalCount)
	// Recreate both service and store on the same real Pool. This is not an OS
	// process restart or real Role receiver proof.
	recovered := grpcsvc.NewBotGRPC(&store.BotStore{Pool: pool}, nil)
	recovered.PrincipalIssuer = svc.PrincipalIssuer
	recovered.InstallationSpace = svc.InstallationSpace
	recovered.CleanupRole = role
	role.corrupt = true
	_, err = pool.Exec(ctx, `UPDATE bot_uninstall_operations SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, op.OperationID)
	require.NoError(t, err)
	worked, err := recovered.ProcessNextBotDeletion(ctx)
	require.True(t, worked)
	require.Error(t, err)
	op, err = st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, "RETRY_WAIT", op.State)
	require.Equal(t, 1, removed, "saved native ACK is exact and independently reusable")
	role.corrupt = false
	_, err = pool.Exec(ctx, `UPDATE bot_uninstall_operations SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, op.OperationID)
	require.NoError(t, err)
	worked, err = recovered.ProcessNextBotDeletion(ctx)
	require.True(t, worked)
	require.NoError(t, err)
	op, err = st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, "COMPLETE", op.State)
	require.Equal(t, originalDecision, op.DecisionBytes)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM bot_installation_intents WHERE installation_id=$1`, intent.InstallationID).Scan(&state))
	require.Equal(t, "CANCELED", state)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_installations WHERE id=$1`, other.InstallationID).Scan(&count))
	require.Equal(t, 1, count)
	current, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "live", current.Status)
	before := role.calls
	_, err = svc.UninstallBotFromSpace(verifiedMutationActor(t, ctx, request, botv1.BotService_UninstallBotFromSpace_FullMethodName), request)
	require.NoError(t, err)
	require.Equal(t, before, role.calls)
	replacement, err := st.PrepareInstallation(ctx, bot.ID, space, manager, nil)
	require.NoError(t, err)
	require.NotEqual(t, intent.InstallationID, replacement.InstallationID)
	require.Greater(t, replacement.Generation, intent.Generation)
	require.NoError(t, st.CompleteInstallation(ctx, replacement, nil))
	require.NoError(t, pool.QueryRow(ctx, `SELECT cancel_operation_id FROM bot_installation_intents WHERE installation_id=$1`, intent.InstallationID).Scan(&intent.CancelOperationID))
	require.NoError(t, st.CompleteInstallationCancellation(ctx, intent), "historical exact cancellation cannot erase the replacement")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_installations WHERE id=$1`, replacement.InstallationID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestUninstallDecisionRollbackConcurrentAdmissionAndGlobalCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("actual PostgreSQL uninstall decision isolation")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_uninstall_atomic", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	owner, manager, space := uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Atomic uninstall", "", `[]`, uuid.New())
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, manager, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	svc := grpcsvc.NewBotGRPC(st, nil)
	svc.PrincipalIssuer = installationTestIssuer(t)
	svc.InstallationRole = installationPermissionClient{allowed: true}
	removed := 0
	svc.InstallationSpace = intentObservingSpace{remove: func(context.Context, *spacev1.RemoveBotMemberRequest) error { removed++; return nil }}
	q := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	userCtx := metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-user-id", owner.String(), "x-voice-profile-id", manager.String()))
	_, err = pool.Exec(ctx, `ALTER TABLE bot_uninstall_installations ADD CONSTRAINT injected_uninstall_snapshot_failure CHECK(false)`)
	require.NoError(t, err)
	_, err = svc.UninstallBotFromSpace(verifiedMutationActor(t, userCtx, q, botv1.BotService_UninstallBotFromSpace_FullMethodName), q)
	require.Error(t, err)
	require.Zero(t, removed)
	var state string
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM bot_installation_intents WHERE installation_id=$1`, intent.InstallationID).Scan(&state))
	require.Equal(t, "INSTALLED", state)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_operations`).Scan(&n))
	require.Zero(t, n)
	_, err = pool.Exec(ctx, `ALTER TABLE bot_uninstall_installations DROP CONSTRAINT injected_uninstall_snapshot_failure`)
	require.NoError(t, err)
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	// Concurrent Store admission uses controlled already-verified decision facts.
	// Transport credential/replay behavior is covered by separate receiver tests.
	results := make(chan error, 8)
	for k := 0; k < 8; k++ {
		go func() {
			_, e := st.BeginInstallationCancellationWithAdmission(ctx, bot.ID, space, manager, store.DeletionAdmission{AccountID: owner, ProfileID: manager, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_UninstallBotFromSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
			results <- e
		}()
	}
	for k := 0; k < 8; k++ {
		require.NoError(t, <-results)
	}
	root, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, "PENDING", root.State)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_operations`).Scan(&n))
	require.Equal(t, 1, n)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_installations`).Scan(&n))
	require.Equal(t, 1, n)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_effects`).Scan(&n))
	require.Equal(t, 2, n)
	source, err := st.LifecycleEffects(ctx, root)
	require.NoError(t, err)
	require.Len(t, source, 2)
	originalScope := append([]byte(nil), source[0].Target.ScopeBytes...)
	global, err := st.BeginBotDeletion(ctx, bot.ID, owner)
	require.NoError(t, err)
	require.Equal(t, "PENDING", global.State)
	targets, err := st.BotDeletionEffects(ctx, global.OperationID)
	require.NoError(t, err)
	require.Len(t, targets, 2)
	require.Equal(t, intent.InstallationID, targets[0].Target.InstallationID, "global revoke still captures unfinished uninstall source")
	source, err = st.LifecycleEffects(ctx, root)
	require.NoError(t, err)
	require.Equal(t, originalScope, source[0].Target.ScopeBytes, "global source removal cannot erase frozen uninstall input")
}
