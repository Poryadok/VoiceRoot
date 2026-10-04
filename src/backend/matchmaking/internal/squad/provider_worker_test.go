package squad

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestUnmarshalFrozenRequestRequiresCanonicalBytesAndDigest(t *testing.T) {
	message := &chatv1.TeardownMatchSquadChatRequest{
		ProtocolVersion: 1, TeardownOperationId: uuid.NewString(), MatchId: uuid.NewString(),
		ChatId: uuid.NewString(), CreationReceiptId: uuid.NewString(), ParticipantManifestSha256: make([]byte, 32),
		CreationRequestSha256: make([]byte, 32),
	}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if err := unmarshalFrozenRequest(raw, digest[:], new(chatv1.TeardownMatchSquadChatRequest)); err != nil {
		t.Fatalf("valid frozen request rejected: %v", err)
	}
	if err := unmarshalFrozenRequest(raw, make([]byte, 32), new(chatv1.TeardownMatchSquadChatRequest)); err == nil {
		t.Fatal("request with a mismatched immutable digest accepted")
	}
	unknown := append(append([]byte(nil), raw...), 0x98, 0x06, 0x01) // unknown field 99, varint 1
	unknownDigest := sha256.Sum256(unknown)
	if err := unmarshalFrozenRequest(unknown, unknownDigest[:], new(chatv1.TeardownMatchSquadChatRequest)); err == nil {
		t.Fatal("request with unknown protobuf field accepted")
	}
}

func TestProviderCallContextBindsExactRequestAndDropsCallerMetadata(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "matchmaking", KeyID: "test", PrivateKey: key, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	operationID := uuid.New()
	request := &chatv1.CompactMatchSquadChatRequest{ProtocolVersion: 1, OperationId: operationID.String(), TeardownAggregateId: uuid.NewString(), MatchId: uuid.NewString(), ChatId: uuid.NewString()}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	worker := &MatchSquadProviderWorker{Issuer: issuer}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer caller-token", "x-profile-id", "untrusted-profile"))
	callCtx, err := worker.callContext(ctx, "chat", chatCompactRPC, operationID, request)
	if err != nil {
		t.Fatal(err)
	}
	md, ok := metadata.FromOutgoingContext(callCtx)
	if !ok {
		t.Fatal("protected provider metadata missing")
	}
	if got := md.Get("x-profile-id"); len(got) != 0 {
		t.Fatalf("caller identity metadata forwarded: %v", got)
	}
	if got := md.Get("x-request-id"); len(got) != 1 || got[0] != operationID.String() {
		t.Fatalf("request ID = %v", got)
	}
	if got := md.Get("authorization"); len(got) != 1 || got[0] == "Bearer caller-token" {
		t.Fatal("caller bearer was not replaced by service credential")
	}
	verified, err := principal.VerifyService(context.Background(), md.Get("authorization")[0][len("Bearer "):], principal.VerifyConfig{
		ExpectedIssuer: "matchmaking", ExpectedAudience: "chat", ExpectedRPC: chatCompactRPC,
		ExpectedRequestID: operationID.String(), ExpectedRequestHash: hash, Clock: func() time.Time { return now },
		KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			if issuer != "matchmaking" || kid != "test" {
				return nil, nil
			}
			return &key.PublicKey, nil
		},
	})
	if err != nil {
		t.Fatalf("issued provider credential rejected: %v", err)
	}
	if verified.Subject != "service:matchmaking" || verified.RequestHash != hash {
		t.Fatalf("verified binding = %+v", verified)
	}
}
