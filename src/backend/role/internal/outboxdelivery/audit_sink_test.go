package outboxdelivery

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type auditRPCFixture struct {
	call func(context.Context, *spacev1.AppendAuditEventRequest) (*spacev1.AppendAuditEventResponse, error)
}

func (f auditRPCFixture) AppendAuditEvent(ctx context.Context, request *spacev1.AppendAuditEventRequest, _ ...grpc.CallOption) (*spacev1.AppendAuditEventResponse, error) {
	return f.call(ctx, request)
}

func auditRequestFixture() *spacev1.AppendAuditEventRequest {
	return &spacev1.AppendAuditEventRequest{AuditEventId: uuid.NewString(), SpaceId: uuid.NewString(), ActorProfileId: uuid.NewString(), Action: "role_created", TargetType: "role", TargetId: uuid.NewString(), DetailsJson: "{}", OccurredAt: &timestamppb.Timestamp{Seconds: 1, Nanos: 999}}
}

func TestSignedAuditSink_FreshExactServiceCredentialAndNoInheritedAuthority(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "role", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	request := auditRequestFixture()
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	require.NoError(t, err)
	event := uuid.MustParse(request.AuditEventId)
	var tokens, jtis []string
	rpc := auditRPCFixture{call: func(ctx context.Context, got *spacev1.AppendAuditEventRequest) (*spacev1.AppendAuditEventResponse, error) {
		require.True(t, proto.Equal(request, got))
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md, 2, "inherited raw identity or duplicate credential cannot be forwarded")
		require.Equal(t, []string{event.String()}, md.Get("x-request-id"))
		require.Len(t, md.Get("authorization"), 1)
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		hash, hashErr := principal.RequestHash(got)
		require.NoError(t, hashErr)
		verified, verifyErr := principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "role", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_AppendAuditEvent_FullMethodName, ExpectedRequestID: event.String(), ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error { jtis = append(jtis, jti); return nil }})
		require.NoError(t, verifyErr)
		require.Empty(t, verified.ProfileID, "service identity never impersonates the event actor")
		tokens = append(tokens, token)
		response := &spacev1.AppendAuditEventResponse{}
		response.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
		return response, nil
	}}
	sink := &SignedAuditSink{Client: rpc, Issuer: issuer}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer stale", "x-request-id", "stale", "x-voice-profile-id", uuid.NewString()))
	require.NoError(t, sink.Append(ctx, event, payload))
	require.NoError(t, sink.Append(ctx, event, payload))
	require.Len(t, tokens, 2)
	require.NotEqual(t, tokens[0], tokens[1])
	require.NotEqual(t, jtis[0], jtis[1])
}

func TestSignedAuditSink_InvalidFrozenRequestNeverCallsRemote(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "role", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	called := false
	rpc := auditRPCFixture{call: func(context.Context, *spacev1.AppendAuditEventRequest) (*spacev1.AppendAuditEventResponse, error) {
		called = true
		return nil, errors.New("unexpected")
	}}
	for name, change := range map[string]func(*spacev1.AppendAuditEventRequest){
		"foreign action":        func(r *spacev1.AppendAuditEventRequest) { r.Action, r.TargetType = "chat_created", "chat" },
		"noncanonical details":  func(r *spacev1.AppendAuditEventRequest) { r.DetailsJson = "{}\n" },
		"unknown request field": func(r *spacev1.AppendAuditEventRequest) { r.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01}) },
		"unknown timestamp field": func(r *spacev1.AppendAuditEventRequest) {
			r.OccurredAt.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := auditRequestFixture()
			change(r)
			payload, err := proto.Marshal(r)
			require.NoError(t, err)
			require.ErrorIs(t, (&SignedAuditSink{Client: rpc, Issuer: issuer}).Append(context.Background(), uuid.MustParse(r.AuditEventId), payload), ErrAuditPayloadInvalid)
			require.False(t, called)
		})
	}
}
