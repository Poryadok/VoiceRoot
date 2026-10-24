package spacemembership

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type membershipTransport struct {
	spacev1.SpaceServiceClient
	add func(context.Context, *spacev1.AddBotMemberRequest) error
}

func (f membershipTransport) AddBotMember(ctx context.Context, r *spacev1.AddBotMemberRequest, _ ...grpc.CallOption) (*spacev1.AddBotMemberResponse, error) {
	return &spacev1.AddBotMemberResponse{}, f.add(ctx, r)
}

func TestMembershipRetryKeepsOperationAndReplacesCredentials(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	space, actor, operation := uuid.NewString(), uuid.NewString(), uuid.NewString()
	req := &spacev1.AddBotMemberRequest{SpaceId: space, ProfileId: actor, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: operation, BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: space, ActorProfileId: actor, Generation: 1}}
	raw, _ := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	var jwtIDs []string
	calls := 0
	client := &Client{issuer: issuer, space: membershipTransport{add: func(ctx context.Context, got *spacev1.AddBotMemberRequest) error {
		calls++
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok || len(md) != 2 || len(md.Get("authorization")) != 1 || len(md.Get("x-request-id")) != 1 {
			t.Fatal("raw/incoming metadata forwarded")
		}
		transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), md))
		if err != nil {
			t.Fatal(err)
		}
		hash, _ := principal.RequestHash(got)
		p, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_AddBotMember_FullMethodName, ExpectedRequestID: operation, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		if err != nil {
			t.Fatal(err)
		}
		jwtIDs = append(jwtIDs, p.JWTID)
		actual, _ := proto.MarshalOptions{Deterministic: true}.Marshal(got)
		if string(actual) != string(raw) {
			t.Fatal("retry changed business operation")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("unbounded protected call")
		}
		if calls == 1 {
			return context.DeadlineExceeded
		}
		return nil
	}}}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer HUMAN_SECRET", "x-voice-profile-id", uuid.NewString()))
	if err = client.Add(ctx, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("unknown outcome was hidden")
	}
	if err = client.Add(ctx, req); err != nil {
		t.Fatal(err)
	}
	if len(jwtIDs) != 2 || jwtIDs[0] == jwtIDs[1] {
		t.Fatal("transport retry reused JWT/JTI")
	}
	bad := proto.Clone(req).(*spacev1.AddBotMemberRequest)
	bad.InstallationIntent.Generation = 0
	if err = client.Add(ctx, bad); err == nil || calls != 2 {
		t.Fatal("invalid operation reached transport")
	}
}
