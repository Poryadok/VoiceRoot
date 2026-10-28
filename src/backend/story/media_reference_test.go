package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
)

type referenceReplyClient struct {
	filev1.FileServiceClient
	corrupt  bool
	requests []*filev1.ClaimStoryMediaRequest
	contexts []context.Context
}

func referenceRequestHash(req proto.Message) []byte {
	wire, _ := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	sum := sha256.Sum256(append(append([]byte(req.ProtoReflect().Descriptor().FullName()), 0), wire...))
	return sum[:]
}
func (c *referenceReplyClient) ClaimStoryMedia(ctx context.Context, req *filev1.ClaimStoryMediaRequest, _ ...grpc.CallOption) (*filev1.ClaimStoryMediaResponse, error) {
	c.requests = append(c.requests, proto.Clone(req).(*filev1.ClaimStoryMediaRequest))
	c.contexts = append(c.contexts, ctx)
	hash := referenceRequestHash(req)
	if c.corrupt {
		hash[0] ^= 1
	}
	return &filev1.ClaimStoryMediaResponse{OperationId: req.OperationId, ReceiptId: uuid.NewString(), RequestSha256: hash}, nil
}

func TestStoryReferenceClientBindsReceiptAndKeepsOperationAcrossFreshCredentials(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "story", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	replies := &referenceReplyClient{}
	client := &signedStoryMediaClient{FileServiceClient: replies, issuer: issuer}
	story, file, author := uuid.New(), uuid.New(), uuid.New()
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer attacker", "x-voice-profile-id", "attacker"))
	for i := 0; i < 2; i++ {
		require.NoError(t, client.ClaimStoryMedia(ctx, story, file, author, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO))
	}
	require.True(t, proto.Equal(replies.requests[0], replies.requests[1]))
	var previous string
	for _, sent := range replies.contexts {
		md, _ := metadata.FromOutgoingContext(sent)
		require.Len(t, md, 2)
		require.Empty(t, md.Get("x-voice-profile-id"))
		require.NotEqual(t, previous, md.Get("authorization")[0])
		previous = md.Get("authorization")[0]
	}
	replies.corrupt = true
	require.Error(t, client.ClaimStoryMedia(ctx, story, file, author, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO), "an unrelated receipt cannot publish a Story")
}
