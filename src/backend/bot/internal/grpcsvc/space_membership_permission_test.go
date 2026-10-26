package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
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

type installBoundaryJournal struct {
	MembershipJournal
	record       store.MembershipRecord
	completed    bool
	acknowledged int
}

func (j *installBoundaryJournal) PrepareInstall(context.Context, uuid.UUID, installprincipal.Administrator, *botv1.InstallBotInSpaceRequest) (store.MembershipRecord, error) {
	return j.record, nil
}
func (j *installBoundaryJournal) ClaimMembershipAttempt(_ context.Context, _ uuid.UUID, r store.MembershipRecord, _ time.Duration) (store.MembershipRecord, error) {
	return r, nil
}
func (j *installBoundaryJournal) RecordAddAcknowledged(context.Context, uuid.UUID, store.MembershipRecord) (store.MembershipRecord, error) {
	j.acknowledged++
	return j.record, nil
}
func (j *installBoundaryJournal) CompleteInstallation(context.Context, uuid.UUID, store.MembershipRecord) (uuid.UUID, error) {
	j.completed = true
	return j.record.InstallationID, nil
}

type installBoundaryTransport struct {
	MembershipTransport
	request        *spacev1.AddBotMemberRequest
	err            error
	checkedProfile uuid.UUID
	checkedSpace   uuid.UUID
}

func (t *installBoundaryTransport) CheckManageBots(_ context.Context, space, profile uuid.UUID) (bool, error) {
	t.checkedSpace = space
	t.checkedProfile = profile
	return true, nil
}
func (t *installBoundaryTransport) Add(_ context.Context, r *spacev1.AddBotMemberRequest) error {
	t.request = proto.Clone(r).(*spacev1.AddBotMemberRequest)
	return t.err
}

type installBoundaryEffects struct{ MembershipEffects }

func (installBoundaryEffects) ValidateChats(context.Context, store.MembershipRecord, *botv1.InstallBotInSpaceRequest) error {
	return nil
}

func TestChannelInstallSpaceAddFailureStopsBeforeLocalCompletion(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	requireNoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	requireNoError(t, err)
	spaceID, botID, adminProfileID, actorProfileID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	chatID := uuid.New()
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	req := &botv1.InstallBotInSpaceRequest{BotId: botID.String(), SpaceId: spaceID.String(), AllowedChats: []*chatv1.ChatRef{{Id: chatID.String(), Type: &chatType}}}
	hash, err := principal.RequestHash(req)
	requireNoError(t, err)
	requestID := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: botv1.BotService_InstallBotInSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: adminProfileID.String(), SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
	requireNoError(t, err)
	verifier := &installprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	var admin installprincipal.Administrator
	_, err = verifier.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: botv1.BotService_InstallBotInSpace_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		admin, err = installprincipal.Require(ctx, botv1.BotService_InstallBotInSpace_FullMethodName, req)
		return nil, err
	})
	requireNoError(t, err)

	addBytes, err := proto.Marshal(&spacev1.AddBotMemberRequest{SpaceId: spaceID.String(), ProfileId: actorProfileID.String(), InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: botID.String(), InstallationId: uuid.NewString(), SpaceId: spaceID.String(), ActorProfileId: actorProfileID.String(), Generation: 1}})
	requireNoError(t, err)
	var add spacev1.AddBotMemberRequest
	requireNoError(t, proto.Unmarshal(addBytes, &add))
	var intent spacev1.BotInstallationIntent
	intent = *add.GetInstallationIntent()
	record := store.MembershipRecord{SpaceID: spaceID, BotID: botID, InstallationID: uuid.MustParse(intent.GetInstallationId()), ActorProfileID: actorProfileID, AddOperationID: uuid.MustParse(intent.GetOperationId()), Generation: 1, AddBytes: addBytes}
	journal := &installBoundaryJournal{record: record}
	transport := &installBoundaryTransport{err: errors.New("Space rejected member add")}
	producer := &MembershipProducer{Journal: journal, Transport: transport, Effects: installBoundaryEffects{}, Source: uuid.New()}
	_, err = producer.Install(ctx, admin, req)
	requireErrorCode(t, err, codes.Unavailable)
	if transport.checkedSpace != spaceID || transport.checkedProfile != adminProfileID {
		t.Fatalf("authorization check used wrong identity: got space=%s profile=%s", transport.checkedSpace, transport.checkedProfile)
	}
	if transport.request == nil || transport.request.GetSpaceId() != spaceID.String() || transport.request.GetProfileId() != actorProfileID.String() {
		t.Fatalf("Space add did not target the bot actor in the requested Space: %+v", transport.request)
	}
	if !proto.Equal(transport.request.GetInstallationIntent(), &intent) {
		t.Fatalf("Space add changed installation intent: got %+v want %+v", transport.request.GetInstallationIntent(), &intent)
	}
	if journal.acknowledged != 0 || journal.completed {
		t.Fatalf("local installation finalized after Space rejected add: ack=%d complete=%v", journal.acknowledged, journal.completed)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func requireErrorCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	if status.Code(err) != code {
		t.Fatalf("status code = %v, want %v (err=%v)", status.Code(err), code, err)
	}
}
