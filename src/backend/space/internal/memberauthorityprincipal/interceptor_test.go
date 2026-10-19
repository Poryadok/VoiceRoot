package memberauthorityprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func memberauthorityRequest() *spacev1.GetMemberAdmissionAuthorityRequest {
	return &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), ProfileId: uuid.NewString()}
}

func memberauthoritySigner(t *testing.T, name string, key *rsa.PrivateKey, now time.Time) *principal.Issuer {
	t.Helper()
	i, err := principal.NewIssuer(principal.IssuerConfig{Issuer: name, KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	return i
}

func signedContext(t *testing.T, signer *principal.Issuer, req *spacev1.GetMemberAdmissionAuthorityRequest, change func(*principal.ServiceInput)) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	in := principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash}
	if change != nil {
		change(&in)
	}
	token, err := signer.IssueService(in)
	require.NoError(t, err)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", in.RequestID))
}

func TestMemberAuthorityVerifierAuthenticatesExactRoleAndChatBeforeHandler(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	for _, producer := range []string{"role"} {
		req := memberauthorityRequest()
		ctx := signedContext(t, memberauthoritySigner(t, producer, key, now), req, nil)
		called := false
		_, err = v.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName}, func(ctx context.Context, value any) (any, error) {
			called = true
			got, ok := Producer(ctx, req)
			require.True(t, ok)
			require.Equal(t, producer, got)
			changed := proto.Clone(req).(*spacev1.GetMemberAdmissionAuthorityRequest)
			changed.ProfileId = uuid.NewString()
			_, ok = Producer(ctx, changed)
			require.False(t, ok, "private proof binds the exact body consumed by handler")
			return &spacev1.GetMemberAdmissionAuthorityResponse{}, nil
		})
		require.NoError(t, err)
		require.True(t, called)
	}
	fake := principal.WithVerified(context.Background(), principal.Principal{Issuer: "role", Subject: "service:role", Kind: "service"})
	_, ok := Producer(fake, memberauthorityRequest())
	require.False(t, ok, "generic prepopulated principal must not bypass authentication")
}

func TestMemberAuthorityVerifierDeniesMalformedBindingUnknownFieldsAndReplay(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	signer := memberauthoritySigner(t, "role", key, now)
	base := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	for _, tc := range []struct {
		name   string
		modify func(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest, *Verifier) context.Context
		code   codes.Code
	}{
		{"no metadata", func(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest, *Verifier) context.Context {
			return context.Background()
		}, codes.Unauthenticated},
		{"raw actor", func(c context.Context, _ *spacev1.GetMemberAdmissionAuthorityRequest, _ *Verifier) context.Context {
			m, _ := metadata.FromIncomingContext(c)
			m = m.Copy()
			m.Set("x-profile-id", uuid.NewString())
			return metadata.NewIncomingContext(c, m)
		}, codes.Unauthenticated},
		{"duplicate authorization", func(c context.Context, _ *spacev1.GetMemberAdmissionAuthorityRequest, _ *Verifier) context.Context {
			m, _ := metadata.FromIncomingContext(c)
			m = m.Copy()
			m.Append("authorization", m.Get("authorization")[0])
			return metadata.NewIncomingContext(c, m)
		}, codes.Unauthenticated},
		{"duplicate request id", func(c context.Context, _ *spacev1.GetMemberAdmissionAuthorityRequest, _ *Verifier) context.Context {
			m, _ := metadata.FromIncomingContext(c)
			m = m.Copy()
			m.Append("x-request-id", uuid.NewString())
			return metadata.NewIncomingContext(c, m)
		}, codes.Unauthenticated},
		{"changed actor", func(c context.Context, r *spacev1.GetMemberAdmissionAuthorityRequest, _ *Verifier) context.Context {
			r.ProfileId = uuid.NewString()
			return c
		}, codes.Unauthenticated},
		{"unknown outer field", func(c context.Context, r *spacev1.GetMemberAdmissionAuthorityRequest, _ *Verifier) context.Context {
			r.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 1})
			return c
		}, codes.InvalidArgument},
		{"invalid protocol", func(c context.Context, r *spacev1.GetMemberAdmissionAuthorityRequest, _ *Verifier) context.Context {
			r.ProtocolVersion = 0
			return c
		}, codes.InvalidArgument},
		{"missing keys", func(c context.Context, _ *spacev1.GetMemberAdmissionAuthorityRequest, v *Verifier) context.Context {
			v.Keys = nil
			return c
		}, codes.Unauthenticated},
		{"missing replay guard", func(c context.Context, _ *spacev1.GetMemberAdmissionAuthorityRequest, v *Verifier) context.Context {
			v.Replay = nil
			return c
		}, codes.Unauthenticated},
		{"dependency failure", func(c context.Context, _ *spacev1.GetMemberAdmissionAuthorityRequest, v *Verifier) context.Context {
			v.Keys = func(context.Context, string, string) (*rsa.PublicKey, error) { return nil, errors.New("unavailable") }
			return c
		}, codes.Unauthenticated},
		{"replay", func(c context.Context, _ *spacev1.GetMemberAdmissionAuthorityRequest, v *Verifier) context.Context {
			v.Replay = func(context.Context, string, string, time.Time) error { return errors.New("already used") }
			return c
		}, codes.Unauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := memberauthorityRequest()
			v := base
			ctx := tc.modify(signedContext(t, signer, req, nil), req, &v)
			_, err := v.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName}, func(context.Context, any) (any, error) { t.Fatal("denied request reached handler"); return nil, nil })
			require.Equal(t, tc.code, status.Code(err))
		})
	}
	for _, change := range []func(*principal.ServiceInput){func(i *principal.ServiceInput) { i.Audience = "role" }, func(i *principal.ServiceInput) { i.RPC = spacev1.SpaceService_GetAuditLog_FullMethodName }} {
		req := memberauthorityRequest()
		ctx := signedContext(t, signer, req, change)
		_, err := base.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName}, func(context.Context, any) (any, error) { t.Fatal("wrong binding reached handler"); return nil, nil })
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	for _, issuer := range []*principal.Issuer{memberauthoritySigner(t, "gateway", key, now), memberauthoritySigner(t, "chat", key, now), memberauthoritySigner(t, "role", key, now.Add(-time.Minute))} {
		req := memberauthorityRequest()
		ctx := signedContext(t, issuer, req, nil)
		_, err := base.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName}, func(context.Context, any) (any, error) {
			t.Fatal("foreign/expired credential reached handler")
			return nil, nil
		})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
}

func TestOrdinaryListenerRejectsMemberAuthorityImport(t *testing.T) {
	_, err := OrdinaryUnary()(context.Background(), memberauthorityRequest(), &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName}, func(context.Context, any) (any, error) {
		t.Fatal("ordinary listener reached memberauthority handler")
		return nil, nil
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
