package grpcsvc_test

import (
	"bytes"
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
	spacev1 "voice.app/voice/space/v1"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type globalTerminalControlledSpace struct {
	spacev1.SpaceServiceClient
	call func(context.Context, *spacev1.RegisterBotGlobalTerminalRequest) (*spacev1.RegisterBotGlobalTerminalResponse, error)
}

func (c globalTerminalControlledSpace) RegisterBotGlobalTerminal(ctx context.Context, q *spacev1.RegisterBotGlobalTerminalRequest, _ ...grpc.CallOption) (*spacev1.RegisterBotGlobalTerminalResponse, error) {
	return c.call(ctx, q)
}

func TestGlobalTerminalWorkerPersistsExactMinimalRequestAndQualifiedReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL global terminal journal")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_terminal_worker", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	owner, actor := uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Minimal worker", "", `[]`, actor)
	require.NoError(t, err)
	issuer, key := installationTestSigningKey(t)
	q := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	root, err := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	var first []byte
	seen := map[string]bool{}
	calls := 0
	receiver := &spacev1.RegisterBotGlobalTerminalResponse{Receipt: &spacev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, BotId: bot.ID.String(), DecisionId: root.OperationID.String(), ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED}}
	// This is a controlled cryptographic receiver, not mTLS/Space48 transport.
	client := globalTerminalControlledSpace{call: func(callCtx context.Context, request *spacev1.RegisterBotGlobalTerminalRequest) (*spacev1.RegisterBotGlobalTerminalResponse, error) {
		calls++
		md, ok := metadata.FromOutgoingContext(callCtx)
		require.True(t, ok)
		require.Empty(t, md.Get("x-voice-profile-id"))
		require.Equal(t, []string{root.OperationID.String()}, md.Get("x-request-id"))
		requestHash, e := principal.RequestHash(request)
		require.NoError(t, e)
		proof, e := principal.VerifyService(callCtx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName, ExpectedRequestID: root.OperationID.String(), ExpectedRequestHash: requestHash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return key, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			if seen[jti] {
				return errors.New("replayed credential")
			}
			seen[jti] = true
			return nil
		}})
		require.NoError(t, e)
		require.Equal(t, "service:bot", proof.Subject)
		require.Empty(t, proof.AccountID)
		require.Empty(t, proof.ProfileID)
		require.Zero(t, proof.SessionEpoch)
		requestBytes, e := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
		require.NoError(t, e)
		if first == nil {
			first = bytes.Clone(requestBytes)
		}
		require.Equal(t, first, requestBytes)
		switch calls {
		case 1:
			return nil, status.Error(codes.DeadlineExceeded, "controlled committed terminal ACK lost")
		case 2:
			wrong := proto.Clone(receiver).(*spacev1.RegisterBotGlobalTerminalResponse)
			wrong.Receipt.DecisionId = uuid.NewString()
			return wrong, nil
		default:
			return proto.Clone(receiver).(*spacev1.RegisterBotGlobalTerminalResponse), nil
		}
	}}
	svc := &grpcsvc.BotGRPC{Store: st, PrincipalIssuer: issuer, CleanupSpace: client}
	for n := 0; n < 4; n++ {
		_, err = pool.Exec(ctx, `UPDATE bot_deletion_operations SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, root.OperationID)
		require.NoError(t, err)
		worked, e := svc.ProcessNextBotDeletion(ctx)
		require.True(t, worked)
		require.Equal(t, codes.Unavailable, status.Code(e), "missing Role terminal dependency remains retryable and independent")
		var state string
		var savedRequest, savedReceipt []byte
		require.NoError(t, pool.QueryRow(ctx, `SELECT state,request_bytes,receipt_bytes FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND participant='space'`, root.OperationID).Scan(&state, &savedRequest, &savedReceipt))
		require.Equal(t, first, savedRequest)
		if n < 2 {
			require.Equal(t, "PENDING", state)
			require.Nil(t, savedReceipt)
		} else {
			require.Equal(t, "COMPLETE", state)
			expected, e := (proto.MarshalOptions{Deterministic: true}).Marshal(receiver)
			require.NoError(t, e)
			require.Equal(t, expected, savedReceipt)
		}
	}
	require.Equal(t, 3, calls, "saved qualified terminal is revalidated without another remote call")
	require.Len(t, seen, 3)
}
