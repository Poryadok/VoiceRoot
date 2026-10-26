package catalogassets

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	catalogactorv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/pkg/principal"
)

type actorServerFixture struct {
	authv1.UnimplementedAuthServiceServer
	catalogactorv1.UnimplementedCatalogActorAuthorityServiceServer
	key  *rsa.PublicKey
	mode string
}

func (s *actorServerFixture) verify(ctx context.Context, req proto.Message, audience, method string) error {
	md, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return err
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return err
	}
	_, err = principal.VerifyService(ctx, md.BearerToken, principal.VerifyConfig{ExpectedIssuer: "chat", ExpectedAudience: audience, ExpectedRPC: method, ExpectedRequestID: md.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return s.key, nil }})
	return err
}
func (s *actorServerFixture) GetChatSessionEpochFloor(ctx context.Context, r *authv1.GetChatSessionEpochFloorRequest) (*authv1.GetChatSessionEpochFloorResponse, error) {
	if err := s.verify(ctx, r, "auth", authv1.AuthService_GetChatSessionEpochFloor_FullMethodName); err != nil {
		return nil, err
	}
	if s.mode == "account-unavailable" {
		return nil, status.Error(codes.NotFound, "account unavailable")
	}
	return &authv1.GetChatSessionEpochFloorResponse{SessionEpochFloor: 7}, nil
}
func (s *actorServerFixture) GetCatalogActorStates(ctx context.Context, r *catalogactorv1.GetCatalogActorStatesRequest) (*catalogactorv1.GetCatalogActorStatesResponse, error) {
	if err := s.verify(ctx, r, "user", catalogactorv1.CatalogActorAuthorityService_GetCatalogActorStates_FullMethodName); err != nil {
		return nil, err
	}
	response := &catalogactorv1.GetCatalogActorStatesResponse{ProtocolVersion: 1}
	for _, actor := range r.Actors {
		state := catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_NOT_DELETED
		if s.mode == "unknown" {
			state = catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_UNKNOWN
		}
		if s.mode == "deleted" {
			state = catalogactorv1.CatalogActorState_CATALOG_ACTOR_STATE_SOFT_DELETED
		}
		if s.mode == "unspecified" {
			state = 0
		}
		response.States = append(response.States, &catalogactorv1.CatalogActorStateEntry{Actor: actor, State: state})
	}
	if s.mode == "owner" {
		response.States[0].Actor.AccountId = uuid.NewString()
	}
	if s.mode == "version" {
		response.ProtocolVersion = 2
	}
	if s.mode == "missing" {
		response.States = nil
	}
	if s.mode == "duplicate" {
		response.States = append(response.States, response.States[0])
	}
	if s.mode == "fields" {
		response.ProtoReflect().SetUnknown([]byte{0xf8, 0x07, 0x01})
	}
	return response, nil
}

func TestCatalogActorObserverRequiresFreshExactOwnerAndDeletionSnapshot(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, _ := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "one", PrivateKey: key})
	fixture := &actorServerFixture{key: &key.PublicKey}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	authv1.RegisterAuthServiceServer(server, fixture)
	catalogactorv1.RegisterCatalogActorAuthorityServiceServer(server, fixture)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///catalog-actor", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	observer := &GRPCActorObserver{Auth: authv1.NewAuthServiceClient(conn), User: catalogactorv1.NewCatalogActorAuthorityServiceClient(conn), Issuer: issuer}
	actor := Actor{AccountID: uuid.New(), ProfileID: uuid.New()}
	observation, err := observer.Observe(context.Background(), actor)
	if err != nil || observation.Profile != ProfileLive || observation.SessionEpochFloor != 7 {
		t.Fatalf("live=%+v err=%v", observation, err)
	}
	for _, mode := range []string{"unknown", "deleted", "owner", "version", "missing", "duplicate", "fields", "unspecified", "account-unavailable"} {
		fixture.mode = mode
		observation, err := observer.Observe(context.Background(), actor)
		switch mode {
		case "unknown":
			if err != nil || observation.Profile != ProfileUnknown {
				t.Fatalf("unknown=%+v err=%v", observation, err)
			}
		case "deleted":
			if err != nil || observation.Profile != ProfileSoftDeleted {
				t.Fatalf("deleted=%+v err=%v", observation, err)
			}
		default:
			if err == nil {
				t.Fatalf("accepted malformed/unavailable %s=%+v", mode, observation)
			}
		}
	}
}
