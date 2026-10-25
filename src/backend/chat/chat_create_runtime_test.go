package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

type creationAuditClient struct {
	spacev1.SpaceServiceClient
	append func(context.Context, *spacev1.AppendAuditEventRequest) (*spacev1.AppendAuditEventResponse, error)
}

func (c creationAuditClient) AppendAuditEvent(ctx context.Context, req *spacev1.AppendAuditEventRequest, _ ...grpc.CallOption) (*spacev1.AppendAuditEventResponse, error) {
	return c.append(ctx, req)
}

func TestChatCreationFactoryDeliversAuditToDedicatedOwnerListener(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	compositeCalls, auditCalls := 0, 0
	composite := creationAuditClient{append: func(context.Context, *spacev1.AppendAuditEventRequest) (*spacev1.AppendAuditEventResponse, error) {
		compositeCalls++
		return nil, status.Error(codes.PermissionDenied, "composite listener permits only commit/outcome")
	}}
	audit := creationAuditClient{append: func(ctx context.Context, req *spacev1.AppendAuditEventRequest) (*spacev1.AppendAuditEventResponse, error) {
		auditCalls++
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		_, err = principal.VerifyService(ctx, md.Get("authorization")[0][7:], principal.VerifyConfig{ExpectedIssuer: "chat", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_AppendAuditEvent_FullMethodName, ExpectedRequestID: req.AuditEventId, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		require.NoError(t, err)
		return &spacev1.AppendAuditEventResponse{}, nil
	}}
	req := &spacev1.AppendAuditEventRequest{AuditEventId: uuid.NewString(), SpaceId: uuid.NewString(), ActorProfileId: uuid.NewString(), Action: "chat_created", TargetType: "chat", TargetId: uuid.NewString(), DetailsJson: "{}", OccurredAt: timestamppb.Now()}
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	require.NoError(t, err)
	coordinator := newChatCreationCoordinator(nil, composite, audit, issuer, nil, nil)
	err = coordinator.Effects.Deliver(context.Background(), &store.SpaceCompositeEffect{Kind: "AUDIT", EventID: uuid.MustParse(req.AuditEventId), Payload: wire})
	require.NoError(t, err)
	require.Zero(t, compositeCalls)
	require.Equal(t, 1, auditCalls)
}

func TestChatCreationAuditConfigNeverFallsBackToComposite(t *testing.T) {
	for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		t.Setenv("CHAT_CREATE_SPACE_"+suffix, "configured-composite")
		t.Setenv("CHAT_CREATE_AUDIT_"+suffix, "")
	}
	conn, err := dialChatCreationOwnerFromEnv("CHAT_CREATE_AUDIT_")
	require.Nil(t, conn)
	require.Error(t, err)
	ca, _, _, cert, key := authFloorTestPKI(t)
	for suffix, value := range map[string]string{"GRPC_ADDR": "127.0.0.1:1", "TLS_CA_FILE": ca, "TLS_SERVER_NAME": "auth.fixture", "CLIENT_CERT_FILE": cert, "CLIENT_KEY_FILE": key} {
		t.Setenv("CHAT_CREATE_AUDIT_"+suffix, value)
	}
	conn, err = dialChatCreationOwnerFromEnv("CHAT_CREATE_AUDIT_")
	require.NoError(t, err, "complete audit config constructs its own mTLS client without composite fallback")
	require.NotNil(t, conn)
	require.NoError(t, conn.Close())
}
