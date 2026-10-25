package grpcsvc

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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/store"
)

// This spy proves only authenticated adapter dispatch. The separate store
// fixture checks actual typed publication and cancellation state.
type protectedBotStoreSpy struct {
	calls  int
	err    error
	add    *spacev1.AddBotMemberRequest
	remove *spacev1.RemoveBotMemberRequest
}

func (s *protectedBotStoreSpy) AddBotMemberWithInstallation(_ context.Context, proof *botmembershipprincipal.VerifiedInstallation, req *spacev1.AddBotMemberRequest) error {
	if _, err := proof.Check(req); err != nil {
		return err
	}
	s.calls++
	s.add = proto.Clone(req).(*spacev1.AddBotMemberRequest)
	return s.err
}
func (s *protectedBotStoreSpy) RemoveBotMemberWithInstallation(_ context.Context, proof *botmembershipprincipal.VerifiedInstallation, req *spacev1.RemoveBotMemberRequest) error {
	if _, err := proof.Check(req); err != nil {
		return err
	}
	s.calls++
	s.remove = proto.Clone(req).(*spacev1.RemoveBotMemberRequest)
	return s.err
}

func protectedBotAdd() *spacev1.AddBotMemberRequest {
	intent := &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: uuid.NewString(), ActorProfileId: uuid.NewString(), Generation: 1}
	return &spacev1.AddBotMemberRequest{SpaceId: intent.SpaceId, ProfileId: intent.ActorProfileId, InstallationIntent: intent}
}

func protectedBotRemove(add *spacev1.AddBotMemberRequest) *spacev1.RemoveBotMemberRequest {
	intent := proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent)
	intent.OperationId = uuid.NewString()
	return &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: add.ProfileId, InstallationIntent: intent, InstallOperationId: add.InstallationIntent.OperationId}
}

func protectedBotContext(t *testing.T, key *rsa.PrivateKey, issuer string, now time.Time, request proto.Message, method string, change func(*principal.ServiceInput)) context.Context {
	t.Helper()
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	var operation string
	switch req := request.(type) {
	case *spacev1.AddBotMemberRequest:
		operation = req.InstallationIntent.OperationId
	case *spacev1.RemoveBotMemberRequest:
		operation = req.InstallationIntent.OperationId
	}
	in := principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: hash}
	if change != nil {
		change(&in)
	}
	token, err := signer.IssueService(in)
	require.NoError(t, err)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", in.RequestID))
}

func protectedBotInvoke(ctx context.Context, verifier botmembershipprincipal.Verifier, adapter *BotMembershipProtected, req proto.Message, method string) (any, error) {
	return verifier.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, value any) (any, error) {
		switch value := value.(type) {
		case *spacev1.AddBotMemberRequest:
			return adapter.AddBotMember(ctx, value)
		case *spacev1.RemoveBotMemberRequest:
			return adapter.RemoveBotMember(ctx, value)
		default:
			return nil, errors.New("unexpected request")
		}
	})
}

func TestProtectedBotMembershipAuthenticatesExactAddAndCancel(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	seen := map[string]bool{}
	verifier := botmembershipprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: func() time.Time { return now }, Replay: func(_ context.Context, issuer, jti string, _ time.Time) error {
		if seen[issuer+":"+jti] {
			return errors.New("replayed")
		}
		seen[issuer+":"+jti] = true
		return nil
	}}
	persistence := &protectedBotStoreSpy{}
	adapter, err := NewBotMembershipProtected(persistence)
	require.NoError(t, err)
	add := protectedBotAdd()
	ctx := protectedBotContext(t, key, "bot", now, add, spacev1.SpaceService_AddBotMember_FullMethodName, nil)
	_, err = protectedBotInvoke(ctx, verifier, adapter, add, spacev1.SpaceService_AddBotMember_FullMethodName)
	require.NoError(t, err)
	require.True(t, proto.Equal(add, persistence.add))
	_, err = protectedBotInvoke(ctx, verifier, adapter, add, spacev1.SpaceService_AddBotMember_FullMethodName)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "old transport token is replayed, not a business retry")
	ctx = protectedBotContext(t, key, "bot", now, add, spacev1.SpaceService_AddBotMember_FullMethodName, nil)
	_, err = protectedBotInvoke(ctx, verifier, adapter, add, spacev1.SpaceService_AddBotMember_FullMethodName)
	require.NoError(t, err, "fresh transport credential retains the same exact operation and body")
	remove := protectedBotRemove(add)
	ctx = protectedBotContext(t, key, "bot", now, remove, spacev1.SpaceService_RemoveBotMember_FullMethodName, nil)
	_, err = protectedBotInvoke(ctx, verifier, adapter, remove, spacev1.SpaceService_RemoveBotMember_FullMethodName)
	require.NoError(t, err)
	require.True(t, proto.Equal(remove, persistence.remove))
	require.Equal(t, add.InstallationIntent.OperationId, persistence.remove.InstallOperationId)
	require.Equal(t, 3, persistence.calls)
}

func TestProtectedBotMembershipRejectsUnqualifiedCallsBeforeStore(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	for _, name := range []string{"missing", "generic principal", "foreign issuer", "wrong audience", "wrong hash", "wrong operation", "expired", "changed tuple", "unknown field", "raw identity", "duplicate bearer", "missing replay", "missing keys", "cross method"} {
		t.Run(name, func(t *testing.T) {
			req := protectedBotAdd()
			method := spacev1.SpaceService_AddBotMember_FullMethodName
			verifier := botmembershipprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: func() time.Time { return now }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
			issuer, issued := "bot", now
			if name == "foreign issuer" {
				issuer = "gateway"
			}
			if name == "expired" {
				issued = now.Add(-time.Minute)
			}
			ctx := protectedBotContext(t, key, issuer, issued, req, method, func(in *principal.ServiceInput) {
				switch name {
				case "wrong audience":
					in.Audience = "role"
				case "wrong hash":
					in.RequestHash = "sha256:" + strings.Repeat("0", 64)
				case "wrong operation":
					in.RequestID = uuid.NewString()
				}
			})
			switch name {
			case "missing":
				ctx = context.Background()
			case "generic principal":
				ctx = principal.WithVerified(context.Background(), principal.Principal{Issuer: "bot", Subject: "service:bot", Kind: "service", Audience: "space", RPC: method})
			case "changed tuple":
				req.InstallationIntent.InstallationId = uuid.NewString()
			case "unknown field":
				req.InstallationIntent.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 1})
			case "raw identity", "duplicate bearer":
				md, _ := metadata.FromIncomingContext(ctx)
				md = md.Copy()
				if name == "raw identity" {
					md.Set("x-profile-id", uuid.NewString())
				} else {
					md.Append("authorization", md.Get("authorization")[0])
				}
				ctx = metadata.NewIncomingContext(ctx, md)
			case "missing replay":
				verifier.Replay = nil
			case "missing keys":
				verifier.Keys = nil
			case "cross method":
				method = spacev1.SpaceService_RemoveBotMember_FullMethodName
			}
			persistence := &protectedBotStoreSpy{}
			adapter, err := NewBotMembershipProtected(persistence)
			require.NoError(t, err)
			_, err = protectedBotInvoke(ctx, verifier, adapter, req, method)
			require.Error(t, err)
			require.Zero(t, persistence.calls)
		})
	}
}

func TestProtectedBotMembershipHasNoOrdinaryOrOtherMethodFallback(t *testing.T) {
	adapter, err := NewBotMembershipProtected(&protectedBotStoreSpy{})
	require.NoError(t, err)
	_, err = adapter.AddBotMember(context.Background(), protectedBotAdd())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = adapter.AddBotMember(principal.WithVerified(context.Background(), principal.Principal{Issuer: "bot", Subject: "service:bot", Kind: "service", Audience: "space"}), protectedBotAdd())
	require.Equal(t, codes.Unauthenticated, status.Code(err), "ambient principal alone is not the private installation capability")
	for _, method := range []string{spacev1.SpaceService_AddBotMember_FullMethodName, spacev1.SpaceService_RemoveBotMember_FullMethodName} {
		_, err = botmembershipprincipal.OrdinaryUnary()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
			t.Fatal("ordinary listener reached protected method")
			return nil, nil
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
	_, err = adapter.GetSpace(context.Background(), &spacev1.GetSpaceRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err))
	_, err = NewBotMembershipProtected(nil)
	require.Error(t, err)
	var absent *store.SpaceStore
	_, err = NewBotMembershipProtected(absent)
	require.Error(t, err)
}

func TestProtectedBotMembershipPreservesStoreErrorMapping(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	for _, failure := range []error{store.ErrOwnershipFrozen, store.ErrLifecycleFrozen, store.ErrOwnershipScopeUnavailable, store.ErrMemberAuthorityUnavailable, store.ErrMemberAdmissionConflict} {
		req := protectedBotAdd()
		persistence := &protectedBotStoreSpy{err: failure}
		adapter, err := NewBotMembershipProtected(persistence)
		require.NoError(t, err)
		v := botmembershipprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
		_, err = protectedBotInvoke(protectedBotContext(t, key, "bot", now, req, spacev1.SpaceService_AddBotMember_FullMethodName, nil), v, adapter, req, spacev1.SpaceService_AddBotMember_FullMethodName)
		require.Equal(t, status.Code(mapSpaceStoreError(failure)), status.Code(err))
	}
}
