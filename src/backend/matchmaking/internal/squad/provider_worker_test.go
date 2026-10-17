package squad

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

func TestProviderTeardownClassificationKeepsUncertainFailuresRetryable(t *testing.T) {
	for _, code := range []codes.Code{codes.NotFound, codes.FailedPrecondition, codes.InvalidArgument} {
		if !isTerminalProviderTeardownError(status.Error(code, "fixed fixture error")) {
			t.Errorf("known terminal provider code %s was not classified as contract mismatch", code)
		}
	}
	for _, code := range []codes.Code{codes.Unavailable, codes.Unauthenticated, codes.PermissionDenied, codes.Unimplemented, codes.DeadlineExceeded, codes.Internal} {
		if isTerminalProviderTeardownError(status.Error(code, "fixed fixture error")) {
			t.Errorf("uncertain provider code %s was incorrectly made terminal", code)
		}
	}
	if !isTerminalProviderTeardownError(errProviderContract) {
		t.Fatal("locally detected immutable binding mismatch was not terminal")
	}
}

func TestMatchSquadTeardownRetryDelayUsesBoundedExponentialBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		jitter  float64
		want    time.Duration
	}{
		{attempt: 1, jitter: 0, want: time.Second},
		{attempt: 1, jitter: 1, want: 1250 * time.Millisecond},
		{attempt: 2, jitter: 1, want: 2500 * time.Millisecond},
		{attempt: 99, jitter: 1, want: 5 * time.Minute},
	}
	for _, tt := range tests {
		if got := matchSquadTeardownRetryDelay(tt.attempt, tt.jitter); got != tt.want {
			t.Errorf("matchSquadTeardownRetryDelay(%d, %v) = %s, want %s", tt.attempt, tt.jitter, got, tt.want)
		}
	}
	if got := matchSquadTeardownRetryDelay(2, -0.1); got != 2*time.Second {
		t.Fatalf("negative jitter was not clamped: %s", got)
	}
	if got := matchSquadTeardownRetryDelay(2, 1.1); got != 2500*time.Millisecond {
		t.Fatalf("jitter above one was not clamped: %s", got)
	}
}

func TestMatchSquadTeardownAttemptsReceiveFreshTenSecondContext(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	first, cancelFirst := matchSquadTeardownAttemptContext(parent)
	firstDeadline, ok := first.Deadline()
	if !ok || time.Until(firstDeadline) > 10*time.Second || time.Until(firstDeadline) < 9*time.Second {
		t.Fatalf("first attempt deadline = %v, ok=%v", firstDeadline, ok)
	}
	cancelFirst()
	if first.Err() == nil {
		t.Fatal("first attempt context was not canceled")
	}

	second, cancelSecond := matchSquadTeardownAttemptContext(parent)
	defer cancelSecond()
	secondDeadline, ok := second.Deadline()
	if !ok || !secondDeadline.After(firstDeadline) {
		t.Fatalf("second attempt did not receive a fresh deadline: first=%v second=%v", firstDeadline, secondDeadline)
	}
	if second.Err() != nil {
		t.Fatalf("canceling one attempt canceled a later attempt: %v", second.Err())
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
