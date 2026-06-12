package grpcsvc

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"

	chatv1 "voice.app/voice/chat/v1"
)

func ctxWithProfile(profileID uuid.UUID) context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", profileID.String()))
}

type stubMessageSearch struct {
	lastInChatQuery string
	lastInChatChat  uuid.UUID
	lastGlobalQuery string
	lastPageSize    int32
	inChatHits      []MessageHit
	globalHits      []MessageHit
}

func (s *stubMessageSearch) SearchInChat(_ context.Context, chatID uuid.UUID, query string, _ *string, limit int) ([]MessageHit, string, error) {
	s.lastInChatChat = chatID
	s.lastInChatQuery = query
	if limit == 0 {
		limit = 20
	}
	s.lastPageSize = int32(limit)
	return s.inChatHits, "", nil
}

func (s *stubMessageSearch) SearchGlobalMessages(_ context.Context, _ uuid.UUID, query string, _ *string, limit int, _ []uuid.UUID) ([]MessageHit, string, error) {
	s.lastGlobalQuery = query
	if limit == 0 {
		limit = 20
	}
	s.lastPageSize = int32(limit)
	return s.globalHits, "", nil
}

type stubProfileSearch struct {
	lastQuery            string
	lastExcludeAccounts  []uuid.UUID
	profileIDs           []uuid.UUID
}

func (s *stubProfileSearch) SearchProfiles(_ context.Context, _ uuid.UUID, query string, exclude []uuid.UUID, _ int) ([]uuid.UUID, error) {
	s.lastQuery = query
	s.lastExcludeAccounts = exclude
	return s.profileIDs, nil
}

type stubSpaceSearch struct {
	lastQuery string
	spaceIDs  []uuid.UUID
}

func (s *stubSpaceSearch) SearchSpaces(_ context.Context, query string, _ *string, _ int) ([]uuid.UUID, string, error) {
	s.lastQuery = query
	return s.spaceIDs, "", nil
}

type stubRoleChecker struct {
	denyRead bool
	lastChat uuid.UUID
}

func (s *stubRoleChecker) CanReadMessages(_ context.Context, _ uuid.UUID, chatID uuid.UUID) (bool, error) {
	s.lastChat = chatID
	return !s.denyRead, nil
}

type stubBlockList struct {
	blockedAccounts []uuid.UUID
}

func (s *stubBlockList) BlockedAccountIDs(_ context.Context) ([]uuid.UUID, error) {
	return s.blockedAccounts, nil
}

type stubChatAccess struct {
	accessible []uuid.UUID
}

func (s *stubChatAccess) AccessibleChatIDs(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return s.accessible, nil
}

func (s *stubChatAccess) SearchChats(_ context.Context, _ string, _ int) ([]uuid.UUID, error) {
	return s.accessible, nil
}

func startSearchGRPCTestServer(t *testing.T, svc *SearchGRPC) searchv1.SearchServiceClient {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	searchv1.RegisterSearchServiceServer(srv, svc)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("grpc serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return searchv1.NewSearchServiceClient(conn)
}

func TestSearchInChat_EmptyQuery_InvalidArgument(t *testing.T) {
	t.Parallel()
	client := startSearchGRPCTestServer(t, &SearchGRPC{
		Messages: &stubMessageSearch{},
	})
	_, err := client.SearchInChat(ctxWithProfile(uuid.New()), &searchv1.SearchInChatRequest{
		Chat:  &chatv1.ChatRef{Id: uuid.New().String()},
		Query: "   ",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSearchInChat_RequiresReadPermission(t *testing.T) {
	t.Parallel()
	chatID := uuid.New()
	roles := &stubRoleChecker{denyRead: true}
	client := startSearchGRPCTestServer(t, &SearchGRPC{
		Messages: &stubMessageSearch{},
		Roles:    roles,
	})
	_, err := client.SearchInChat(ctxWithProfile(uuid.New()), &searchv1.SearchInChatRequest{
		Chat:  &chatv1.ChatRef{Id: chatID.String()},
		Query: "hello",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, chatID, roles.lastChat)
}

func TestSearchInChat_DefaultPageSize20(t *testing.T) {
	t.Parallel()
	msgs := &stubMessageSearch{}
	client := startSearchGRPCTestServer(t, &SearchGRPC{Messages: msgs, Roles: &stubRoleChecker{}})
	chatID := uuid.New()
	_, err := client.SearchInChat(ctxWithProfile(uuid.New()), &searchv1.SearchInChatRequest{
		Chat:  &chatv1.ChatRef{Id: chatID.String()},
		Query: "hello",
		Page:  &commonv1.CursorPageRequest{},
	})
	require.NoError(t, err)
	require.Equal(t, int32(20), msgs.lastPageSize)
	require.Equal(t, chatID, msgs.lastInChatChat)
}

func TestSearchGlobal_ExcludesBlockedUsers(t *testing.T) {
	t.Parallel()
	blockedAccount := uuid.New()
	visibleProfile := uuid.New()
	profiles := &stubProfileSearch{profileIDs: []uuid.UUID{visibleProfile}}
	client := startSearchGRPCTestServer(t, &SearchGRPC{
		Messages: &stubMessageSearch{},
		Profiles: profiles,
		Spaces:   &stubSpaceSearch{},
		Blocks:   &stubBlockList{blockedAccounts: []uuid.UUID{blockedAccount}},
		Chats:    &stubChatAccess{},
	})
	resp, err := client.SearchGlobal(ctxWithProfile(uuid.New()), &searchv1.SearchGlobalRequest{
		Query: "raid",
		Page:  &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	require.Equal(t, []string{visibleProfile.String()}, resp.GetGlobalSearchResults().GetProfileIds())
	require.Contains(t, profiles.lastExcludeAccounts, blockedAccount)
}

func TestSearchUsers_EmptyQuery_InvalidArgument(t *testing.T) {
	t.Parallel()
	client := startSearchGRPCTestServer(t, &SearchGRPC{Profiles: &stubProfileSearch{}})
	_, err := client.SearchUsers(ctxWithProfile(uuid.New()), &searchv1.SearchUsersRequest{Query: ""})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSearchSpaces_ForwardsQuery(t *testing.T) {
	t.Parallel()
	spaces := &stubSpaceSearch{spaceIDs: []uuid.UUID{uuid.New()}}
	client := startSearchGRPCTestServer(t, &SearchGRPC{Spaces: spaces})
	resp, err := client.SearchSpaces(ctxWithProfile(uuid.New()), &searchv1.SearchSpacesRequest{
		Query: "public guild",
		Page:  &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	require.Equal(t, "public guild", spaces.lastQuery)
	require.Len(t, resp.GetSpaceSearchResults().GetSpaceIds(), 1)
}

func TestSearchInChat_UnavailableWithoutStore(t *testing.T) {
	t.Parallel()
	client := startSearchGRPCTestServer(t, &SearchGRPC{})
	_, err := client.SearchInChat(ctxWithProfile(uuid.New()), &searchv1.SearchInChatRequest{
		Chat:  &chatv1.ChatRef{Id: uuid.New().String()},
		Query: "hello",
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSearchUsers_UnavailableWithoutProfiles(t *testing.T) {
	t.Parallel()
	client := startSearchGRPCTestServer(t, &SearchGRPC{})
	_, err := client.SearchUsers(ctxWithProfile(uuid.New()), &searchv1.SearchUsersRequest{Query: "alice"})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSearchGlobal_ReturnsMessageHitsForAccessibleChats(t *testing.T) {
	t.Parallel()
	chatID := uuid.New()
	msgID := uuid.New()
	msgs := &stubMessageSearch{
		globalHits: []MessageHit{{MessageID: msgID, ChatID: chatID, Snippet: "hit", Score: 1}},
	}
	client := startSearchGRPCTestServer(t, &SearchGRPC{
		Messages: msgs,
		Chats:    &stubChatAccess{accessible: []uuid.UUID{chatID}},
	})
	resp, err := client.SearchGlobal(ctxWithProfile(uuid.New()), &searchv1.SearchGlobalRequest{
		Query: "raid",
		Page:  &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetGlobalSearchResults().GetMessages(), 1)
	require.Equal(t, msgID.String(), resp.GetGlobalSearchResults().GetMessages()[0].GetMessageId())
}

func TestSearchInChat_Unauthenticated(t *testing.T) {
	t.Parallel()
	client := startSearchGRPCTestServer(t, &SearchGRPC{Messages: &stubMessageSearch{}})
	_, err := client.SearchInChat(context.Background(), &searchv1.SearchInChatRequest{
		Chat:  &chatv1.ChatRef{Id: uuid.New().String()},
		Query: "hello",
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
