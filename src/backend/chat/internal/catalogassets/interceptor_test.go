package catalogassets

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestCatalogPrincipalExactBindingAndReplay(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	account, profile := uuid.NewString(), uuid.NewString()
	gateway, _ := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "one", PrivateKey: key})
	messaging, _ := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "one", PrivateKey: key})
	fileIssuer, _ := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: "one", PrivateKey: key})
	seen := map[string]bool{}
	var mu sync.Mutex
	auth := Authenticator{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, SessionEpoch: func(_ context.Context, id string, epoch int64) error {
		if id != account || epoch != 7 {
			return errors.New("stale session")
		}
		return nil
	}, Replay: func(_ context.Context, issuer, jti string, _ time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		if seen[issuer+jti] {
			return errors.New("replay")
		}
		seen[issuer+jti] = true
		return nil
	}}
	resolve := &chatv1.ResolveGifAssetRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), Provider: "fixture", ProviderId: "one", DestinationChat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_DM.Enum()}}
	validate := &chatv1.ValidateGifAssetRequest{ProtocolVersion: 1, ActorAccountId: account, ActorProfileId: profile, Provider: "fixture", ProviderId: "one", FileId: uuid.NewString(), DestinationChat: resolve.DestinationChat}
	authorize := &chatv1.AuthorizeCatalogDestinationRequest{ProtocolVersion: 1, ActorAccountId: account, ActorProfileId: profile, DestinationChat: resolve.DestinationChat}
	issue := func(method string, req proto.Message, kind string) (context.Context, string) {
		t.Helper()
		hash, err := principal.RequestHash(req)
		if err != nil {
			t.Fatal(err)
		}
		rid := uuid.NewString()
		var token string
		if kind == "delegated" {
			token, err = gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "chat", RPC: method, RequestID: rid, RequestHash: hash, AccountID: account, ProfileID: profile, SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
		} else {
			issuer := messaging
			if kind == "file" {
				issuer = fileIssuer
			}
			token, err = issuer.IssueService(principal.ServiceInput{Audience: "chat", RPC: method, RequestID: rid, RequestHash: hash})
		}
		if err != nil {
			t.Fatal(err)
		}
		return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", rid)), token
	}
	call := func(ctx context.Context, method string, req proto.Message) (bool, error) {
		called := false
		_, err := auth.UnaryInterceptor()(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		return called, err
	}
	for _, tc := range []struct {
		method string
		req    proto.Message
		kind   string
	}{{chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName, resolve, "delegated"}, {chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName, validate, "service"}, {chatv1.CatalogAssetService_AuthorizeCatalogDestination_FullMethodName, authorize, "file"}} {
		ctx, _ := issue(tc.method, tc.req, tc.kind)
		if called, err := call(ctx, tc.method, tc.req); err != nil || !called {
			t.Fatalf("valid %s called=%v err=%v", tc.method, called, err)
		}
		if called, err := call(ctx, tc.method, tc.req); called || status.Code(err) != codes.Unauthenticated {
			t.Fatalf("replay called=%v err=%v", called, err)
		}
	}
	ctx, _ := issue(chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName, resolve, "delegated")
	changed := proto.Clone(resolve).(*chatv1.ResolveGifAssetRequest)
	changed.ProviderId = "other"
	if called, err := call(ctx, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName, changed); called || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("changed body called=%v err=%v", called, err)
	}
	for _, kind := range []string{"wrong-method", "raw-identity", "duplicate-authorization", "unknown-field", "nested-unknown", "missing"} {
		req := proto.Clone(resolve).(*chatv1.ResolveGifAssetRequest)
		ctx, token := issue(chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName, req, "delegated")
		method := chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName
		want := codes.Unauthenticated
		md, _ := metadata.FromIncomingContext(ctx)
		switch kind {
		case "wrong-method":
			method = chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName
		case "raw-identity":
			md.Set("x-voice-profile-id", profile)
		case "duplicate-authorization":
			md.Append("authorization", "Bearer "+token)
		case "unknown-field":
			req.ProtoReflect().SetUnknown([]byte{0xf8, 0x07, 0x01})
			want = codes.InvalidArgument
		case "nested-unknown":
			req.DestinationChat.ProtoReflect().SetUnknown([]byte{0xf8, 0x07, 0x01})
			want = codes.InvalidArgument
		case "missing":
			md = metadata.MD{}
		}
		ctx = metadata.NewIncomingContext(context.Background(), md)
		if called, err := call(ctx, method, req); called || status.Code(err) != want {
			t.Fatalf("%s called=%v err=%v want=%v", kind, called, err, want)
		}
	}
}
