package grpcsvc_test

import (
	"context"
	"crypto/rsa"
	"errors"
	"strings"
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

type globalTerminalControlledRole struct {
	rolev1.RoleServiceClient
	call func(context.Context, *rolev1.RegisterBotGlobalTerminalRequest) (*rolev1.RegisterBotGlobalTerminalResponse, error)
}

func (c globalTerminalControlledRole) RegisterBotGlobalTerminal(ctx context.Context, q *rolev1.RegisterBotGlobalTerminalRequest, _ ...grpc.CallOption) (*rolev1.RegisterBotGlobalTerminalResponse, error) {
	return c.call(ctx, q)
}

func TestGlobalRootRequiresOwnRoleAndSpaceTerminalReceipts(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL distinct terminal participant recovery")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_role_terminal_worker", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	owner, actor := uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Two terminal participants", "", `[]`, actor)
	require.NoError(t, err)
	issuer, key := installationTestSigningKey(t)
	q := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	root, err := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	seen := map[string]bool{}
	verify := func(callCtx context.Context, q proto.Message, audience, method string) {
		md, ok := metadata.FromOutgoingContext(callCtx)
		require.True(t, ok)
		require.Equal(t, []string{root.OperationID.String()}, md.Get("x-request-id"))
		require.Empty(t, md.Get("x-voice-profile-id"))
		hash, e := principal.RequestHash(q)
		require.NoError(t, e)
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		config := principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: audience, ExpectedRPC: method, ExpectedRequestID: root.OperationID.String(), ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return key, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			if seen[jti] {
				return errors.New("replayed credential")
			}
			seen[jti] = true
			return nil
		}}
		proof, e := principal.VerifyService(callCtx, token, config)
		require.NoError(t, e)
		require.Equal(t, "service:bot", proof.Subject)
		require.Empty(t, proof.AccountID)
		require.Empty(t, proof.ProfileID)
		require.Zero(t, proof.SessionEpoch)
		config.ExpectedAudience = "wrong-participant"
		_, e = principal.VerifyService(callCtx, token, config)
		require.Error(t, e)
	}
	roleReceipt := &rolev1.RegisterBotGlobalTerminalResponse{Receipt: &rolev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, Participant: "role", BotId: bot.ID.String(), DecisionId: root.OperationID.String(), ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED}}
	roleCalls, spaceCalls := 0, 0
	var roleBody []byte
	// Both receivers are controlled RSA boundaries, not Role26/Space48 runtime.
	roleClient := globalTerminalControlledRole{call: func(callCtx context.Context, request *rolev1.RegisterBotGlobalTerminalRequest) (*rolev1.RegisterBotGlobalTerminalResponse, error) {
		roleCalls++
		verify(callCtx, request, "role", rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName)
		require.Equal(t, bot.ID.String(), request.BotId)
		require.Equal(t, root.OperationID.String(), request.DecisionId)
		raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
		require.NoError(t, e)
		if roleBody == nil {
			roleBody = append([]byte(nil), raw...)
		}
		require.Equal(t, roleBody, raw)
		if roleCalls == 1 {
			return nil, status.Error(codes.DeadlineExceeded, "controlled committed Role terminal ACK lost")
		}
		response := proto.Clone(roleReceipt).(*rolev1.RegisterBotGlobalTerminalResponse)
		if roleCalls == 2 {
			response.Receipt.Participant = "space"
		}
		return response, nil
	}}
	spaceClient := globalTerminalControlledSpace{call: func(callCtx context.Context, request *spacev1.RegisterBotGlobalTerminalRequest) (*spacev1.RegisterBotGlobalTerminalResponse, error) {
		spaceCalls++
		verify(callCtx, request, "space", spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName)
		return &spacev1.RegisterBotGlobalTerminalResponse{Receipt: &spacev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, BotId: request.BotId, DecisionId: request.DecisionId, ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED}}, nil
	}}
	svc := &grpcsvc.BotGRPC{Store: st, PrincipalIssuer: issuer, CleanupRole: roleClient, CleanupSpace: spaceClient}
	for attempt := 1; attempt <= 3; attempt++ {
		_, err = pool.Exec(ctx, `UPDATE bot_deletion_operations SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, root.OperationID)
		require.NoError(t, err)
		worked, e := svc.ProcessNextBotDeletion(ctx)
		require.True(t, worked)
		require.Equal(t, attempt, roleCalls, "independent Role registration must actually be attempted")
		var state string
		require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM bot_deletion_operations WHERE operation_id=$1`, root.OperationID).Scan(&state))
		if attempt < 3 {
			require.Error(t, e)
			require.Equal(t, "RETRY_WAIT", state)
		} else {
			require.NoError(t, e)
			require.Equal(t, "COMPLETE", state)
		}
	}
	require.Equal(t, 1, spaceCalls)
	require.Len(t, seen, 4)
	var saved []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT receipt_bytes FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND participant='role'`, root.OperationID).Scan(&saved))
	expected, err := (proto.MarshalOptions{Deterministic: true}).Marshal(roleReceipt)
	require.NoError(t, err)
	require.Equal(t, expected, saved)
	worked, err := svc.ProcessNextBotDeletion(ctx)
	require.NoError(t, err)
	require.False(t, worked)
	require.Equal(t, 3, roleCalls)
	require.Equal(t, 1, spaceCalls)
	// An opaque historical/internal root without verified admission cannot be
	// promoted into fresh registration authority just because UUIDs exist.
	legacy, _, err := st.CreateBot(ctx, uuid.New(), "Unqualified retained root", "", `[]`, uuid.New())
	require.NoError(t, err)
	legacyRoot, err := st.BeginBotDeletion(ctx, legacy.ID, legacy.OwnerAccountID)
	require.NoError(t, err)
	worked, err = svc.ProcessNextBotDeletion(ctx)
	require.True(t, worked)
	require.Error(t, err)
	require.Equal(t, 3, roleCalls)
	require.Equal(t, 1, spaceCalls)
	var unsealed int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND state='PENDING' AND request_bytes IS NULL AND receipt_bytes IS NULL`, legacyRoot.OperationID).Scan(&unsealed))
	require.Equal(t, 2, unsealed)
}
