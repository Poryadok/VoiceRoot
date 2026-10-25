package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/principal"
)

type messageAuthorityReaderFunc func(context.Context, *commonv1.BotMessageSendBinding) (bool, error)

func (f messageAuthorityReaderFunc) CheckCurrentMessageSendAuthority(ctx context.Context, b *commonv1.BotMessageSendBinding) (bool, error) {
	return f(ctx, b)
}

func TestBotMessageSendAuthorityEchoDenialAndUnknown(t *testing.T) {
	for _, test := range []struct {
		name    string
		allow   bool
		failure error
		code    codes.Code
	}{{"allowed", true, nil, codes.OK}, {"revoked", false, nil, codes.OK}, {"unknown", false, errors.New("owner down"), codes.Unavailable}} {
		t.Run(test.name, func(t *testing.T) {
			req := sendAuthorityRequest()
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			svc := NewBotGRPC(nil, nil)
			svc.MessageAuthorityReader = messageAuthorityReaderFunc(func(_ context.Context, b *commonv1.BotMessageSendBinding) (bool, error) {
				require.True(t, proto.Equal(req.Binding, b))
				return test.allow, test.failure
			})
			p := principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "bot", RPC: botv1.BotService_CheckBotMessageSendAuthority_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash}
			p.ExpiresAt = time.Now().Add(time.Minute)
			ctx := principal.WithVerified(context.Background(), p)
			out, err := svc.CheckBotMessageSendAuthority(ctx, req)
			require.Equal(t, test.code, status.Code(err))
			if err == nil {
				require.Equal(t, test.allow, out.Allowed)
				require.True(t, proto.Equal(req.Binding, out.Binding))
				require.NotSame(t, req.Binding, out.Binding)
			}
			req.Binding.ChatId = uuid.NewString()
			_, err = svc.CheckBotMessageSendAuthority(ctx, req)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}

func sendAuthorityRequest() *botv1.CheckBotMessageSendAuthorityRequest {
	return &botv1.CheckBotMessageSendAuthorityRequest{Binding: &commonv1.BotMessageSendBinding{ProtocolVersion: 1, BotId: uuid.NewString(), ActorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), InstallationId: uuid.NewString(), InstallOperationId: uuid.NewString(), InstallationGeneration: 1, ChatId: uuid.NewString()}}
}

func TestBotMessageSendAuthorityRejectsUnverifiedAndForeignCaller(t *testing.T) {
	req := sendAuthorityRequest()
	svc := NewBotGRPC(nil, nil)
	_, err := svc.CheckBotMessageSendAuthority(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "bot", RPC: botv1.BotService_CheckBotMessageSendAuthority_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
	_, err = svc.CheckBotMessageSendAuthority(ctx, req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestBotMessageSendAuthorityDoesNotPublishAfterOriginalDeadline(t *testing.T) {
	req := sendAuthorityRequest()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "bot", RPC: botv1.BotService_CheckBotMessageSendAuthority_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, ExpiresAt: time.Now().Add(time.Minute)}
	svc := NewBotGRPC(nil, nil)
	svc.MessageAuthorityReader = messageAuthorityReaderFunc(func(ctx context.Context, _ *commonv1.BotMessageSendBinding) (bool, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 5*time.Second)
		<-ctx.Done()
		return true, nil // A late successful database result cannot escape its budget.
	})
	ctx, cancel := context.WithTimeout(principal.WithVerified(t.Context(), p), 20*time.Millisecond)
	defer cancel()
	out, err := svc.CheckBotMessageSendAuthority(ctx, req)
	require.Nil(t, out)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
