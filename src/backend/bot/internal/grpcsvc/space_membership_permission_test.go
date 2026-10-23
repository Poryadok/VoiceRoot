package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/installprincipal"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

type permissionJournal struct {
	MembershipJournal
	calls int
}

func (j *permissionJournal) PrepareInstall(context.Context, uuid.UUID, installprincipal.Administrator, *botv1.InstallBotInSpaceRequest) (store.MembershipRecord, error) {
	j.calls++
	return store.MembershipRecord{}, store.ErrMembershipConflict
}
func (j *permissionJournal) BeginCancel(context.Context, uuid.UUID, installprincipal.Administrator, uuid.UUID, uuid.UUID) (store.MembershipRecord, error) {
	j.calls++
	return store.MembershipRecord{}, store.ErrMembershipConflict
}

type permissionTransport struct {
	MembershipTransport
	allow          bool
	err            error
	calls          int
	profile, space uuid.UUID
}

func (p *permissionTransport) CheckManageBots(_ context.Context, space, profile uuid.UUID) (bool, error) {
	p.calls++
	p.profile = profile
	p.space = space
	return p.allow, p.err
}

type forbiddenMembershipEffects struct{ MembershipEffects }

func TestProtectedMembershipPermissionPrecedesJournal(t *testing.T) {
	for _, method := range []string{botv1.BotService_InstallBotInSpace_FullMethodName, botv1.BotService_UninstallBotFromSpace_FullMethodName} {
		for _, mode := range []string{"deny", "unavailable", "allow"} {
			t.Run(method+"/"+mode, func(t *testing.T) {
				key, err := rsa.GenerateKey(rand.Reader, 2048)
				if err != nil {
					t.Fatal(err)
				}
				issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
				if err != nil {
					t.Fatal(err)
				}
				space, bot, profile := uuid.New(), uuid.New(), uuid.New()
				var request proto.Message = &botv1.InstallBotInSpaceRequest{BotId: bot.String(), SpaceId: space.String()}
				if method == botv1.BotService_UninstallBotFromSpace_FullMethodName {
					request = &botv1.UninstallBotFromSpaceRequest{BotId: bot.String(), SpaceId: space.String()}
				}
				hash, _ := principal.RequestHash(request)
				id := uuid.NewString()
				token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: method, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: profile.String(), SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
				if err != nil {
					t.Fatal(err)
				}
				verifier := &installprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
				journal := new(permissionJournal)
				transport := &permissionTransport{allow: mode == "allow"}
				if mode == "unavailable" {
					transport.err = errors.New("Role unavailable")
				}
				service := &BotGRPC{MembershipCoordinator: &MembershipProducer{Journal: journal, Transport: transport, Effects: forbiddenMembershipEffects{}, Source: uuid.New()}}
				ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
				_, err = verifier.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
					if method == botv1.BotService_InstallBotInSpace_FullMethodName {
						return service.InstallBotInSpace(ctx, request.(*botv1.InstallBotInSpaceRequest))
					}
					return service.UninstallBotFromSpace(ctx, request.(*botv1.UninstallBotFromSpaceRequest))
				})
				want := codes.PermissionDenied
				calls := 0
				if mode == "unavailable" {
					want = codes.Unavailable
				}
				if mode == "allow" {
					want = codes.FailedPrecondition
					calls = 1
				}
				if status.Code(err) != want || journal.calls != calls || transport.calls != 1 || transport.profile != profile || transport.space != space {
					t.Fatalf("permission boundary: err=%v journal=%d transport=%+v", err, journal.calls, transport)
				}
			})
		}
	}
}
