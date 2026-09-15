package grpcsvc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type threadListGuardSpy struct {
	faultGuard
	calls         int
	chat, profile uuid.UUID
}

func (g *threadListGuardSpy) EnsureMember(_ context.Context, chat, profile uuid.UUID) error {
	g.calls++
	g.chat, g.profile = chat, profile
	return g.memberErr
}

func TestListThreadsUnavailableWithoutVerifiedProjection(t *testing.T) {
	for _, typ := range []chatv1.ChatType{chatv1.ChatType_CHAT_TYPE_DM, chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL} {
		for _, cursor := range []string{"", "tampered", "opaque-old-cursor"} {
			t.Run(fmt.Sprintf("%s/%s", typ, cursor), func(t *testing.T) {
				chat, profile := uuid.New(), uuid.New()
				guard := &threadListGuardSpy{}
				// No database is needed for the readiness refusal. Code review
				// separately verifies that the handler has no legacy store call.
				svc := &MessagingGRPC{Messages: &store.MessagesStore{}, ChatGuard: guard}
				resp, err := svc.ListThreads(profileCtx(uuid.New(), profile), &messagingv1.ListThreadsRequest{
					Chat: &chatv1.ChatRef{Id: chat.String(), Type: &typ},
					Page: &commonv1.CursorPageRequest{PageSize: 10, Cursor: cursor},
				})
				require.Nil(t, resp)
				require.Equal(t, codes.Unavailable, status.Code(err))
				require.Equal(t, "thread list unavailable", status.Convert(err).Message())
				require.Equal(t, 1, guard.calls)
				require.Equal(t, chat, guard.chat)
				require.Equal(t, profile, guard.profile)
			})
		}
	}
}

func TestListThreadsMembershipBeforeCursorAndOpaqueFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want codes.Code
	}{
		{"sentinel denial", store.ErrNotChatMember, codes.PermissionDenied},
		{"wrapped denial", fmt.Errorf("private lookup: %w", store.ErrNotChatMember), codes.PermissionDenied},
		{"grpc denial", status.Error(codes.PermissionDenied, "private policy detail"), codes.PermissionDenied},
		{"dependency", errors.New("private postgres address"), codes.Unavailable},
		{"dependency grpc", status.Error(codes.Unavailable, "private grpc address"), codes.Unavailable},
		{"canceled", context.Canceled, codes.Canceled},
		{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded},
		{"grpc canceled", status.Error(codes.Canceled, "private cancel detail"), codes.Canceled},
		{"grpc deadline", status.Error(codes.DeadlineExceeded, "private timeout detail"), codes.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			guard := &threadListGuardSpy{faultGuard: faultGuard{memberErr: tc.err}}
			svc := &MessagingGRPC{Messages: &store.MessagesStore{}, ChatGuard: guard, Logger: slog.New(slog.NewJSONHandler(&logs, nil))}
			resp, err := svc.ListThreads(profileCtx(uuid.New(), uuid.New()), &messagingv1.ListThreadsRequest{
				Chat: chatDMRef(uuid.New()), Page: &commonv1.CursorPageRequest{Cursor: "not-a-valid-cursor"},
			})
			require.Nil(t, resp)
			require.Equal(t, tc.want, status.Code(err))
			require.NotContains(t, status.Convert(err).Message(), "private")
			require.Equal(t, 1, guard.calls)
			if tc.want == codes.Unavailable {
				require.Equal(t, "thread list unavailable", status.Convert(err).Message())
				require.Contains(t, logs.String(), "private", "dependency diagnostics belong in server logs")
			}
		})
	}
}

func TestListThreadsValidationAndRequiredConfiguration(t *testing.T) {
	ctx := profileCtx(uuid.New(), uuid.New())
	req := &messagingv1.ListThreadsRequest{Chat: chatDMRef(uuid.New())}
	var nilGuard *typedNilChatGuard
	for _, svc := range []*MessagingGRPC{nil, {}, {Messages: &store.MessagesStore{}}, {Messages: &store.MessagesStore{}, ChatGuard: nilGuard}} {
		resp, err := svc.ListThreads(ctx, req)
		require.Nil(t, resp)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	}
	guard := &threadListGuardSpy{}
	svc := &MessagingGRPC{Messages: &store.MessagesStore{}, ChatGuard: guard}
	_, err := svc.ListThreads(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = svc.ListThreads(ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = svc.ListThreads(ctx, &messagingv1.ListThreadsRequest{Chat: &chatv1.ChatRef{Id: "bad"}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	invalidType := chatv1.ChatType(999)
	_, err = svc.ListThreads(ctx, &messagingv1.ListThreadsRequest{Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: &invalidType}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Zero(t, guard.calls)
}

func TestListThreadsCanceledContextCannotBecomeReadinessError(t *testing.T) {
	ctx, cancel := context.WithCancel(profileCtx(uuid.New(), uuid.New()))
	cancel()
	svc := &MessagingGRPC{Messages: &store.MessagesStore{}, ChatGuard: &threadListGuardSpy{}}
	resp, err := svc.ListThreads(ctx, &messagingv1.ListThreadsRequest{Chat: chatDMRef(uuid.New())})
	require.Nil(t, resp)
	require.Equal(t, codes.Canceled, status.Code(err))
}
