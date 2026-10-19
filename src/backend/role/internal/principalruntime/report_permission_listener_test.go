package principalruntime

import (
	"context"
	"crypto/tls"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestReportPermissionProtectedTLSAndBinding(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.ReportPermissionEnabled = true
	f.config.JWKSURLs["messaging"] = f.config.JWKSURLs["space"]
	f.config.JWKSURLs["moderation"] = f.config.JWKSURLs["space"]
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	method := rolev1.RoleService_CheckPermission_FullMethodName
	request := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
	for _, tc := range []struct{ issuer, permission string }{{"messaging", "TEXT_CHAT_VIEW"}, {"messaging", "TEXT_CHAT_READ_HISTORY"}, {"moderation", "MODERATION_MANAGE_REPORTS"}} {
		r := proto.Clone(request).(*rolev1.CheckPermissionRequest)
		r.PermissionName = tc.permission
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		id := uuid.NewString()
		token := issueRuntimeToken(t, f.key, tc.issuer, "current", "role", method, id, hash)
		ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 2*time.Second)
		_, err = client.CheckPermission(ctx, r)
		require.NoError(t, err)
		p := <-recorder.principals
		require.Equal(t, "service:"+tc.issuer, p.Subject)
		_, err = client.CheckPermission(ctx, r)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		cancel()
	}
	require.EqualValues(t, 3, recorder.calls.Load())
	for _, name := range []string{"wrong_issuer", "wrong_audience", "wrong_rpc", "wrong_hash", "duplicate_authorization", "duplicate_request_id", "raw_identity", "nested_unknown", "cross_purpose", "unrelated_method"} {
		t.Run(name, func(t *testing.T) {
			r := proto.Clone(request).(*rolev1.CheckPermissionRequest)
			issuer, audience, rpc := "messaging", "role", method
			if name == "wrong_issuer" {
				issuer = "moderation"
			}
			if name == "wrong_audience" {
				audience = "space"
			}
			if name == "wrong_rpc" {
				rpc = rolev1.RoleService_ListRoles_FullMethodName
			}
			if name == "nested_unknown" {
				r.Chat.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 1})
			}
			if name == "cross_purpose" {
				r.PermissionName = "SPACE_MANAGE_SETTINGS"
			}
			hash, err := principal.RequestHash(r)
			require.NoError(t, err)
			if name == "wrong_hash" {
				hash = runtimeHash
			}
			id := uuid.NewString()
			token := issueRuntimeToken(t, f.key, issuer, "current", audience, rpc, id, hash)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)
			if name == "duplicate_authorization" {
				md.Append("authorization", "Bearer "+token)
			}
			if name == "duplicate_request_id" {
				md.Append("x-request-id", id)
			}
			if name == "raw_identity" {
				md.Set("x-voice-profile-id", r.ProfileId)
			}
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), 2*time.Second)
			defer cancel()
			if name == "unrelated_method" {
				_, err = client.ListRoles(ctx, &rolev1.ListRolesRequest{SpaceId: r.SpaceId})
			} else {
				_, err = client.CheckPermission(ctx, r)
			}
			require.Error(t, err)
			require.EqualValues(t, 3, recorder.calls.Load(), "rejected before handler")
		})
	}
	for _, tc := range []struct {
		name      string
		transport credentials.TransportCredentials
	}{{"missing_client_cert", credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "127.0.0.1"})}, {"wrong_hostname", runtimeTLSCredentialsWithServerName(f, roots, "wrong.invalid")}} {
		t.Run(tc.name, func(t *testing.T) {
			c := runtimeListenerClient(t, address, tc.transport)
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			_, err := c.CheckPermission(ctx, request)
			require.Error(t, err)
			require.EqualValues(t, 3, recorder.calls.Load())
		})
	}
}
