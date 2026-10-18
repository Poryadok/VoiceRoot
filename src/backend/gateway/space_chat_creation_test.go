package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
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
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type compositeChatClient struct {
	chatv1.ChatServiceClient
	create func(context.Context, *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error)
}

func (c compositeChatClient) CreateChat(ctx context.Context, r *chatv1.CreateChatRequest, _ ...grpc.CallOption) (*chatv1.CreateChatResponse, error) {
	return c.create(ctx, r)
}

type compositeSpaceClient struct {
	spacev1.SpaceServiceClient
	get     func(context.Context, *spacev1.GetSpaceChatCreationOutcomeRequest) (*spacev1.GetSpaceChatCreationOutcomeResponse, error)
	upserts *int
}

func (c compositeSpaceClient) GetSpaceChatCreationOutcome(ctx context.Context, r *spacev1.GetSpaceChatCreationOutcomeRequest, _ ...grpc.CallOption) (*spacev1.GetSpaceChatCreationOutcomeResponse, error) {
	return c.get(ctx, r)
}
func (c compositeSpaceClient) UpsertTreeNode(context.Context, *spacev1.UpsertTreeNodeRequest, ...grpc.CallOption) (*spacev1.UpsertTreeNodeResponse, error) {
	*c.upserts++
	return nil, status.Error(codes.Internal, "legacy Upsert forbidden")
}

func TestSpaceChatCompositeRouteUsesProtectedOriginalResultWithoutUpsert(t *testing.T) {
	for _, scenario := range []string{"success", "unknown", "aborted", "changed node", "changed creator", "missing Chat completion"} {
		t.Run(scenario, func(t *testing.T) {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
			require.NoError(t, err)
			claims := tokenClaims{UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}
			spaceID, chatID, nodeID, opID, requestID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
			verify := func(ctx context.Context, message proto.Message, method, audience, id string) {
				md, ok := metadata.FromOutgoingContext(ctx)
				require.True(t, ok)
				require.Len(t, md, 2)
				require.Equal(t, []string{id}, md.Get("x-request-id"))
				hash, err := principal.RequestHash(message)
				require.NoError(t, err)
				p, err := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: audience, ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, SessionEpochChecker: func(_ context.Context, account string, epoch int64) error {
					require.Equal(t, claims.UserID, account)
					require.Equal(t, claims.SessionEpoch, epoch)
					return nil
				}})
				require.NoError(t, err)
				require.Equal(t, claims.ProfileID, p.ProfileID)
				require.Equal(t, "regular", p.AccountType)
			}
			creates, lookups, upserts := 0, 0, 0
			chatClient := compositeChatClient{create: func(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
				creates++
				require.Equal(t, spaceID, req.GetSpaceId())
				require.Equal(t, requestID, req.RequestId)
				verify(ctx, req, chatv1.ChatService_CreateChat_FullMethodName, "chat", requestID)
				name := "general"
				result := &chatv1.CreateChatResponse{Chat: &chatv1.Chat{Id: chatID, Type: chatv1.ChatType_CHAT_TYPE_GROUP, SpaceId: &spaceID, Name: &name}, CompositeOperationId: opID, CompositeNodeId: nodeID}
				if scenario == "missing Chat completion" {
					result.CompositeOperationId = ""
				}
				return result, nil
			}}
			spaceClient := compositeSpaceClient{upserts: &upserts, get: func(ctx context.Context, req *spacev1.GetSpaceChatCreationOutcomeRequest) (*spacev1.GetSpaceChatCreationOutcomeResponse, error) {
				lookups++
				require.Equal(t, spaceID, req.SpaceId)
				require.Equal(t, opID, req.OperationId)
				verify(ctx, req, spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName, "space", opID)
				if scenario == "unknown" {
					return nil, status.Error(codes.Unavailable, "outcome unknown")
				}
				kind := chatv1.ChatType_CHAT_TYPE_GROUP
				binding := &spacev1.SpaceChatCreationBinding{ProtocolVersion: 1, OperationId: opID, CreatorProfileId: claims.ProfileID, SpaceId: spaceID, ChatId: chatID, ChatType: kind, NodeId: nodeID, AuditEventId: uuid.NewString(), CreateRequestSha256: make([]byte, 32), Initiator: &spacev1.SpaceChatCreationBinding_User{User: &spacev1.SpaceChatCreationUser{AccountId: claims.UserID, SessionEpoch: 7, AccountType: "regular"}}}
				out := &spacev1.SpaceChatCreationOutcome{Binding: binding, State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED, Node: &spacev1.SpaceTreeNode{Id: nodeID, SpaceId: spaceID, Kind: "text_chat", SortOrder: 42, LinkedChat: &chatv1.ChatRef{Id: chatID, Type: &kind}}}
				switch scenario {
				case "aborted":
					out.State = spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED
					out.Node = nil
					out.FailureCode = uint32(codes.PermissionDenied)
				case "changed node":
					out.Node.Id = uuid.NewString()
				case "changed creator":
					binding.CreatorProfileId = uuid.NewString()
				}
				return &spacev1.GetSpaceChatCreationOutcomeResponse{Outcome: out}, nil
			}}
			transcoder := &transcoder{lifecycleIssuer: issuer, clients: grpcClients{chatAudit: chatClient, spaceLifecycle: spaceClient, space: spaceClient}}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+spaceID+"/chats", strings.NewReader(`{"type":"CHAT_TYPE_GROUP","name":"general","requestId":"`+requestID+`"}`))
			r = r.WithContext(context.WithValue(r.Context(), verifiedUserClaimsKey{}, claims))
			r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
			w := httptest.NewRecorder()
			require.True(t, transcoder.serveSpacesTree(w, r, spaceID+"/chats"))
			require.Equal(t, 1, creates)
			require.Zero(t, upserts)
			if scenario == "success" {
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.Contains(t, w.Body.String(), `"sort_order":42`)
				require.Contains(t, w.Body.String(), nodeID)
			} else {
				require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
			}
			if scenario == "missing Chat completion" {
				require.Zero(t, lookups)
			} else {
				require.Equal(t, 1, lookups)
			}
		})
	}
}
