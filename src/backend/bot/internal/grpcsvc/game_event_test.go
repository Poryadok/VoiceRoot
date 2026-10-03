package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"voice/backend/pkg/principal"

	botv1 "voice.app/voice/bot/v1"
	gameintegrationv1 "voice.app/voice/gameintegration/v1"
)

func TestPublishGameEventRequiresExactGISPrincipalAndFailClosesWithoutDownstream(t *testing.T) {
	request := validPublishGameEventRequest()
	service := &BotGRPC{}
	_, err := service.PublishGameEvent(context.Background(), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	ctx := publishGameEventPrincipalContext(t, request, func(p *principal.Principal) { p.Issuer = "bot" })
	_, err = service.PublishGameEvent(ctx, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	ctx = publishGameEventPrincipalContext(t, request, nil)
	_, err = service.PublishGameEvent(ctx, request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestPublishGameEventAcceptsBoundedV2CardIntentBeforeOwnerDependencies(t *testing.T) {
	request := validPublishGameEventRequest()
	request.Intent.SchemaVersion = 2
	request.Intent.Card = &gameintegrationv1.GameCard{SchemaVersion: 1, Revision: 1, Title: "Relic found", SafeSummary: "A relic was found",
		Actions: []*gameintegrationv1.GameCardAction{{ActionId: uuid.NewString(), ActionType: "relic.inspect", Label: "Inspect", ArgumentsJson: `{"target":"relic"}`}}}
	ctx := publishGameEventPrincipalContext(t, request, nil)
	_, err := (&BotGRPC{}).PublishGameEvent(ctx, request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "v2 card survives validation and reaches configured dependencies")
}

func validPublishGameEventRequest() *botv1.PublishGameEventRequest {
	now := time.Now().UTC().Truncate(time.Second)
	return &botv1.PublishGameEventRequest{Intent: &gameintegrationv1.VerifiedGameEventIntent{
		OperationId: uuid.NewString(), AppId: uuid.NewString(), AppOwnerAccountId: uuid.NewString(),
		EnvironmentId: uuid.NewString(), InstallationId: uuid.NewString(), BotId: uuid.NewString(),
		EventId: uuid.NewString(), PayloadHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Recipient: &gameintegrationv1.VerifiedGameEventIntent_Binding{Binding: &gameintegrationv1.BindingEventRecipient{
			BindingId: uuid.NewString(), ResolvedChatId: uuid.NewString(),
		}}, AuthorityRevision: 1, SchemaVersion: 1,
		OccurredAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(time.Minute)),
		FallbackText: "A game event occurred.", ClientMessageId: uuid.NewString(), EventType: "match.completed",
	}}
}

func publishGameEventPrincipalContext(t *testing.T, request *botv1.PublishGameEventRequest, edit func(*principal.Principal)) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	verified := principal.Principal{Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration", Audience: "bot",
		RPC: botv1.BotService_PublishGameEvent_FullMethodName, RequestID: "request-1", RequestHash: hash}
	if edit != nil {
		edit(&verified)
	}
	return principal.WithVerified(context.Background(), verified)
}
