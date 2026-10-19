package reportprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func reportreadRequest() *spacev1.ResolveSpaceReportReadContextRequest {
	return &spacev1.ResolveSpaceReportReadContextRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), SpaceId: uuid.NewString(), ViewerProfileId: uuid.NewString(), Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
}
func reportreadSigner(t *testing.T, name string, key *rsa.PrivateKey, now time.Time) *principal.Issuer {
	t.Helper()
	v, e := principal.NewIssuer(principal.IssuerConfig{Issuer: name, KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, e)
	return v
}
func signedContext(t *testing.T, signer *principal.Issuer, req *spacev1.ResolveSpaceReportReadContextRequest, change func(*principal.ServiceInput)) context.Context {
	t.Helper()
	hash, e := principal.RequestHash(req)
	require.NoError(t, e)
	in := principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_ResolveSpaceReportReadContext_FullMethodName, RequestID: req.OperationId, RequestHash: hash}
	if change != nil {
		change(&in)
	}
	token, e := signer.IssueService(in)
	require.NoError(t, e)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", in.RequestID))
}
func TestReportReadVerifierExactMessagingBodyAndDependencies(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	now := time.Now().UTC()
	signer := reportreadSigner(t, "messaging", key, now)
	base := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	req := reportreadRequest()
	info := &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ResolveSpaceReportReadContext_FullMethodName}
	_, e = base.Unary()(signedContext(t, signer, req, nil), req, info, func(ctx context.Context, value any) (any, error) {
		require.True(t, Producer(ctx, req))
		changed := proto.Clone(req).(*spacev1.ResolveSpaceReportReadContextRequest)
		changed.ViewerProfileId = uuid.NewString()
		require.False(t, Producer(ctx, changed))
		return &spacev1.ResolveSpaceReportReadContextResponse{}, nil
	})
	require.NoError(t, e)
	for _, tc := range []struct {
		name   string
		change func(context.Context, *spacev1.ResolveSpaceReportReadContextRequest, *Verifier) context.Context
		code   codes.Code
	}{
		{"raw actor", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			md, _ := metadata.FromIncomingContext(c)
			md = md.Copy()
			md.Set("x-profile-id", r.ViewerProfileId)
			return metadata.NewIncomingContext(c, md)
		}, codes.Unauthenticated},
		{"duplicate bearer", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			md, _ := metadata.FromIncomingContext(c)
			md = md.Copy()
			md.Append("authorization", md.Get("authorization")[0])
			return metadata.NewIncomingContext(c, md)
		}, codes.Unauthenticated},
		{"foreign viewer", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			r.ViewerProfileId = uuid.NewString()
			return c
		}, codes.Unauthenticated},
		{"foreign Space", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			r.SpaceId = uuid.NewString()
			return c
		}, codes.Unauthenticated},
		{"foreign Chat", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			r.Chat.Id = uuid.NewString()
			return c
		}, codes.Unauthenticated},
		{"foreign operation", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			r.OperationId = uuid.NewString()
			return c
		}, codes.Unauthenticated},
		{"outer unknown", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			r.ProtoReflect().SetUnknown([]byte{0xa0, 6, 1})
			return c
		}, codes.InvalidArgument},
		{"nested unknown", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			r.Chat.ProtoReflect().SetUnknown([]byte{0xa0, 6, 1})
			return c
		}, codes.InvalidArgument},
		{"missing type", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			r.Chat.Type = nil
			return c
		}, codes.InvalidArgument},
		{"standalone DM", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			r.Chat.Type = chatv1.ChatType_CHAT_TYPE_DM.Enum()
			return c
		}, codes.InvalidArgument},
		{"protocol", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			r.ProtocolVersion = 2
			return c
		}, codes.InvalidArgument},
		{"JWKS unavailable", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			v.Keys = func(context.Context, string, string) (*rsa.PublicKey, error) { return nil, errors.New("down") }
			return c
		}, codes.Unauthenticated},
		{"replay unavailable", func(c context.Context, r *spacev1.ResolveSpaceReportReadContextRequest, v *Verifier) context.Context {
			v.Replay = func(context.Context, string, string, time.Time) error { return errors.New("down") }
			return c
		}, codes.Unauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := reportreadRequest()
			v := base
			c := tc.change(signedContext(t, signer, r, nil), r, &v)
			_, e := v.Unary()(c, r, info, func(context.Context, any) (any, error) { t.Fatal("invalid request reached owner"); return nil, nil })
			require.Equal(t, tc.code, status.Code(e))
		})
	}
	for _, issuer := range []string{"role", "gateway", "chat"} {
		r := reportreadRequest()
		_, e := base.Unary()(signedContext(t, reportreadSigner(t, issuer, key, now), r, nil), r, info, func(context.Context, any) (any, error) { t.Fatal("wrong issuer reached owner"); return nil, nil })
		require.Equal(t, codes.Unauthenticated, status.Code(e))
	}
	for _, change := range []func(*principal.ServiceInput){func(in *principal.ServiceInput) { in.Audience = "role" }, func(in *principal.ServiceInput) { in.RPC = spacev1.SpaceService_GetSpace_FullMethodName }} {
		r := reportreadRequest()
		_, e := base.Unary()(signedContext(t, signer, r, change), r, info, func(context.Context, any) (any, error) { t.Fatal("wrong binding reached owner"); return nil, nil })
		require.Equal(t, codes.Unauthenticated, status.Code(e))
	}
	_, e = OrdinaryUnary()(context.Background(), req, info, func(context.Context, any) (any, error) {
		t.Fatal("ordinary listener admitted report read")
		return nil, nil
	})
	require.Equal(t, codes.PermissionDenied, status.Code(e))
}
