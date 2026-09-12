package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type recordingChatClient struct {
	chatv1.ChatServiceClient
	ctx context.Context
	req *chatv1.GetSpacePurgeManifestPageRequest
}

func (c *recordingChatClient) GetSpacePurgeManifestPage(ctx context.Context, req *chatv1.GetSpacePurgeManifestPageRequest, _ ...grpc.CallOption) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
	c.ctx = ctx
	c.req = req
	return &chatv1.GetSpacePurgeManifestPageResponse{}, nil
}

func TestSignedManifestClientUsesFreshExactSearchPrincipal(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "search", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	recording := &recordingChatClient{}
	client := &signedChatManifestClient{ChatServiceClient: recording, issuer: issuer}
	req := &chatv1.GetSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: "00000000-0000-0000-0000-000000000001", DeletionOperationId: "00000000-0000-0000-0000-000000000002", Generation: 1, ManifestId: "root"}
	_, err = client.GetSpacePurgeManifestPage(context.Background(), req)
	require.NoError(t, err)
	md, ok := metadata.FromOutgoingContext(recording.ctx)
	require.True(t, ok)
	require.Len(t, md.Get("authorization"), 1)
	require.Len(t, md.Get("x-request-id"), 1)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	_, err = principal.VerifyService(context.Background(), md.Get("authorization")[0][len("Bearer "):], principal.VerifyConfig{ExpectedIssuer: "search", ExpectedAudience: "chat", ExpectedRPC: chatv1.ChatService_GetSpacePurgeManifestPage_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
	require.NoError(t, err)
}
