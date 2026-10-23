package installprincipal

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
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

func TestAdministratorCapabilityRequiresRealBoundVerifier(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	request := &botv1.InstallBotInSpaceRequest{BotId: uuid.NewString(), SpaceId: uuid.NewString()}
	hash, _ := principal.RequestHash(request)
	account, profile, requestID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: botv1.BotService_InstallBotInSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: account, ProfileID: profile, SessionEpoch: 2, ClientExpiresAt: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	v := &Verifier{Clock: func() time.Time { return now }, Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(_ context.Context, _, jti string, _ time.Time) error {
		if seen[jti] {
			return errors.New("replay")
		}
		seen[jti] = true
		return nil
	}, Epoch: func(_ context.Context, a string, e int64) error {
		if a != account || e != 2 {
			return errors.New("wrong epoch")
		}
		return nil
	}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	var captured context.Context
	next := func(c context.Context, _ any) (any, error) { captured = c; return nil, nil }
	info := &grpc.UnaryServerInfo{FullMethod: botv1.BotService_InstallBotInSpace_FullMethodName}
	if _, err = v.Unary()(ctx, request, info, next); err != nil {
		t.Fatal(err)
	}
	a, err := Require(captured, info.FullMethod, request)
	if err != nil || a.ProfileID().String() != profile || a.AccountID().String() != account {
		t.Fatal("no valid private capability", err)
	}
	changed := proto.Clone(request).(*botv1.InstallBotInSpaceRequest)
	changed.SpaceId = uuid.NewString()
	if _, err = Require(captured, info.FullMethod, changed); err == nil {
		t.Fatal("changed body accepted")
	}
	if _, err = v.Unary()(ctx, request, info, next); err == nil {
		t.Fatal("JWT replay accepted")
	}
	if _, err = Require(principal.WithVerified(context.Background(), principal.Principal{}), info.FullMethod, request); err == nil {
		t.Fatal("generic context minted capability")
	}
}
