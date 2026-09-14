package grpcsvc

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/pkg/privacy"

	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"

	chatv1 "voice.app/voice/chat/v1"
)

func ctxWithProfile(profileID uuid.UUID) context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", profileID.String()))
}

func ctxWithProfileAndAccount(profileID, accountID uuid.UUID) context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		"x-voice-profile-id", profileID.String(),
		"x-voice-user-id", accountID.String(),
	))
}

type stubMessageSearch struct {
	calls           int
	lastInChatQuery string
	lastInChatChat  uuid.UUID
	lastGlobalQuery string
	lastPageSize    int32
	inChatHits      []MessageHit
	globalHits      []MessageHit
}

func (s *stubMessageSearch) SearchInChat(_ context.Context, chatID uuid.UUID, query string, _ *string, limit int) ([]MessageHit, string, error) {
	s.calls++
	s.lastInChatChat = chatID
	s.lastInChatQuery = query
	if limit == 0 {
		limit = 20
	}
	s.lastPageSize = int32(limit)
	return s.inChatHits, "", nil
}

func (s *stubMessageSearch) SearchGlobalMessages(_ context.Context, _ uuid.UUID, query string, _ *string, limit int, _ []uuid.UUID) ([]MessageHit, string, error) {
	s.calls++
	s.lastGlobalQuery = query
	if limit == 0 {
		limit = 20
	}
	s.lastPageSize = int32(limit)
	return s.globalHits, "", nil
}

type stubProfileSearch struct {
	calls               int
	lastQuery           string
	lastExcludeAccounts []uuid.UUID
	hits                []ProfileSearchHit
}

func (s *stubProfileSearch) SearchProfiles(_ context.Context, _ uuid.UUID, query string, exclude []uuid.UUID, _ int) ([]ProfileSearchHit, error) {
	s.calls++
	s.lastQuery = query
	s.lastExcludeAccounts = exclude
	return s.hits, nil
}

type stubSpaceSearch struct {
	calls     int
	lastQuery string
	spaceIDs  []uuid.UUID
}

func (s *stubSpaceSearch) SearchSpaces(_ context.Context, query string, _ *string, _ int) ([]uuid.UUID, string, error) {
	s.calls++
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
	pairBlocked     map[uuid.UUID]bool
}

func (s *stubBlockList) BlockedAccountIDs(_ context.Context) ([]uuid.UUID, error) {
	return s.blockedAccounts, nil
}

func (s *stubBlockList) AccountPairBlocked(_ context.Context, _, other uuid.UUID) (bool, error) {
	if s.pairBlocked == nil {
		return false, nil
	}
	return s.pairBlocked[other], nil
}

type stubChatAccess struct {
	accessible  []uuid.UUID
	searchChats []uuid.UUID
}

func (s *stubChatAccess) AccessibleChatIDs(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return s.accessible, nil
}

func (s *stubChatAccess) SearchChats(_ context.Context, _ string, _ int) ([]uuid.UUID, error) {
	if s.searchChats != nil {
		return s.searchChats, nil
	}
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
	profiles := &stubProfileSearch{hits: []ProfileSearchHit{{ProfileID: visibleProfile, AccountID: uuid.New()}}}
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

func TestSearchGlobal_MatchedChatsIntersectAccessible(t *testing.T) {
	t.Parallel()
	accessibleChat := uuid.New()
	leakedChat := uuid.New()
	client := startSearchGRPCTestServer(t, &SearchGRPC{
		Chats: &stubChatAccess{
			accessible:  []uuid.UUID{accessibleChat},
			searchChats: []uuid.UUID{accessibleChat, leakedChat},
		},
	})
	resp, err := client.SearchGlobal(ctxWithProfile(uuid.New()), &searchv1.SearchGlobalRequest{
		Query: "secret",
		Page:  &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	matched := resp.GetGlobalSearchResults().GetMatchedChats()
	require.Len(t, matched, 1)
	require.Equal(t, accessibleChat.String(), matched[0].GetId())
}

func TestSearchGlobal_ExcludesReverseBlockedUsers(t *testing.T) {
	t.Parallel()
	viewerAccount := uuid.New()
	blockerAccount := uuid.New()
	visibleProfile := uuid.New()
	visibleAccount := uuid.New()
	blockedProfile := uuid.New()
	profiles := &stubProfileSearch{
		hits: []ProfileSearchHit{
			{ProfileID: visibleProfile, AccountID: visibleAccount},
			{ProfileID: blockedProfile, AccountID: blockerAccount},
		},
	}
	client := startSearchGRPCTestServer(t, &SearchGRPC{
		Profiles: profiles,
		Blocks: &stubBlockList{
			pairBlocked: map[uuid.UUID]bool{blockerAccount: true},
		},
	})
	resp, err := client.SearchGlobal(ctxWithProfileAndAccount(uuid.New(), viewerAccount), &searchv1.SearchGlobalRequest{
		Query: "user",
		Page:  &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	require.Equal(t, []string{visibleProfile.String()}, resp.GetGlobalSearchResults().GetProfileIds())
}

type stubDiscoverability struct {
	audienceByProfile map[uuid.UUID]privacy.Audience
	friends           map[[2]uuid.UUID]bool
}

func (s *stubDiscoverability) AllowFriendRequestsAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.audienceByProfile == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	if a, ok := s.audienceByProfile[profileID]; ok {
		return a, nil
	}
	return privacy.EveryoneWithGuests(), nil
}

func (s *stubDiscoverability) AreFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	if s.friends == nil {
		return false, nil
	}
	if s.friends[[2]uuid.UUID{a, b}] || s.friends[[2]uuid.UUID{b, a}] {
		return true, nil
	}
	return false, nil
}

func (s *stubDiscoverability) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (s *stubDiscoverability) AreCoMembers(context.Context, uuid.UUID, uuid.UUID, []string) (bool, error) {
	return false, nil
}

func TestSearchUsers_FiltersByAllowFriendRequestsAudience(t *testing.T) {
	t.Parallel()
	viewer := uuid.New()
	friendTarget := uuid.New()
	strangerTarget := uuid.New()
	nobodyTarget := uuid.New()
	profiles := &stubProfileSearch{
		hits: []ProfileSearchHit{
			{ProfileID: friendTarget, AccountID: uuid.New()},
			{ProfileID: strangerTarget, AccountID: uuid.New()},
			{ProfileID: nobodyTarget, AccountID: uuid.New()},
		},
	}
	disc := &stubDiscoverability{
		audienceByProfile: map[uuid.UUID]privacy.Audience{
			friendTarget:   privacy.FriendsOnly(),
			strangerTarget: privacy.FriendsOnly(),
			nobodyTarget:   privacy.Nobody(),
		},
		friends: map[[2]uuid.UUID]bool{
			{viewer, friendTarget}: true,
		},
	}
	client := startSearchGRPCTestServer(t, &SearchGRPC{
		Profiles:        profiles,
		Discoverability: disc,
		Social:          disc,
		SpaceMembers:    disc,
	})
	resp, err := client.SearchUsers(ctxWithProfile(viewer), &searchv1.SearchUsersRequest{Query: "alice"})
	require.NoError(t, err)
	require.Equal(t, []string{friendTarget.String()}, resp.GetUserSearchResults().GetProfileIds())
}

func TestSearchGlobal_FiltersByAllowFriendRequestsAudience(t *testing.T) {
	t.Parallel()
	viewer := uuid.New()
	openTarget := uuid.New()
	closedTarget := uuid.New()
	profiles := &stubProfileSearch{
		hits: []ProfileSearchHit{
			{ProfileID: openTarget, AccountID: uuid.New()},
			{ProfileID: closedTarget, AccountID: uuid.New()},
		},
	}
	disc := &stubDiscoverability{
		audienceByProfile: map[uuid.UUID]privacy.Audience{
			openTarget:   privacy.EveryoneWithGuests(),
			closedTarget: privacy.Nobody(),
		},
	}
	client := startSearchGRPCTestServer(t, &SearchGRPC{
		Profiles:        profiles,
		Discoverability: disc,
		Social:          disc,
		SpaceMembers:    disc,
		Chats:           &stubChatAccess{},
	})
	resp, err := client.SearchGlobal(ctxWithProfile(viewer), &searchv1.SearchGlobalRequest{
		Query: "raid",
		Page:  &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	require.Equal(t, []string{openTarget.String()}, resp.GetGlobalSearchResults().GetProfileIds())
}

func TestSearchUsers_OwnProfileAlwaysVisible(t *testing.T) {
	t.Parallel()
	viewer := uuid.New()
	profiles := &stubProfileSearch{
		hits: []ProfileSearchHit{{ProfileID: viewer, AccountID: uuid.New()}},
	}
	disc := &stubDiscoverability{
		audienceByProfile: map[uuid.UUID]privacy.Audience{
			viewer: privacy.Nobody(),
		},
	}
	client := startSearchGRPCTestServer(t, &SearchGRPC{
		Profiles:        profiles,
		Discoverability: disc,
		Social:          disc,
		SpaceMembers:    disc,
	})
	resp, err := client.SearchUsers(ctxWithProfile(viewer), &searchv1.SearchUsersRequest{Query: "me"})
	require.NoError(t, err)
	require.Equal(t, []string{viewer.String()}, resp.GetUserSearchResults().GetProfileIds())
}

func TestSearchGlobal_QueryTooLong_InvalidArgument(t *testing.T) {
	t.Parallel()
	client := startSearchGRPCTestServer(t, &SearchGRPC{Messages: &stubMessageSearch{}})
	longQuery := strings.Repeat("a", maxQueryLen+1)
	_, err := client.SearchGlobal(ctxWithProfile(uuid.New()), &searchv1.SearchGlobalRequest{
		Query: longQuery,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRequireQuery_UnicodeRuneLimit(t *testing.T) {
	t.Parallel()

	valid, err := requireQuery(strings.Repeat("界", maxQueryLen))
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("界", maxQueryLen), valid)

	_, err = requireQuery(strings.Repeat("界", maxQueryLen+1))
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSearchRPCs_QueryTooLong_InvalidArgument(t *testing.T) {
	t.Parallel()
	client := startSearchGRPCTestServer(t, &SearchGRPC{})
	ctx := ctxWithProfile(uuid.New())
	longQuery := strings.Repeat("a", maxQueryLen+1)

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "in chat",
			call: func() error {
				_, err := client.SearchInChat(ctx, &searchv1.SearchInChatRequest{
					Chat:  &chatv1.ChatRef{Id: uuid.New().String()},
					Query: longQuery,
				})
				return err
			},
		},
		{
			name: "global",
			call: func() error {
				_, err := client.SearchGlobal(ctx, &searchv1.SearchGlobalRequest{Query: longQuery})
				return err
			},
		},
		{
			name: "users",
			call: func() error {
				_, err := client.SearchUsers(ctx, &searchv1.SearchUsersRequest{Query: longQuery})
				return err
			},
		},
		{
			name: "spaces",
			call: func() error {
				_, err := client.SearchSpaces(ctx, &searchv1.SearchSpacesRequest{Query: longQuery})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, codes.InvalidArgument, status.Code(tt.call()))
		})
	}
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

func TestRequireQuery_UnicodeAndBoundedEnvelope(t *testing.T) {
	t.Parallel()

	family := "👨‍👩‍👧‍👦"
	validFamily := strings.Repeat(family, 18) + "xx"
	invalidFamily := strings.Repeat(family, 18) + "xxx"
	validCombining := strings.Repeat("e\u0301", 64)
	invalidCombining := validCombining + "x"

	tests := []struct {
		name    string
		raw     string
		want    string
		message string
	}{
		{name: "empty", raw: "", message: "query required"},
		{name: "ascii whitespace", raw: " \t\n ", message: "query required"},
		{name: "unicode whitespace", raw: "\u2003\u00a0", message: "query required"},
		{name: "unicode edge whitespace is trimmed", raw: "\u2003界\u00a0", want: "界"},
		{name: "128 ascii", raw: strings.Repeat("a", 128), want: strings.Repeat("a", 128)},
		{name: "129 ascii", raw: strings.Repeat("a", 129), message: "query too long"},
		{name: "128 cjk", raw: strings.Repeat("界", 128), want: strings.Repeat("界", 128)},
		{name: "129 cjk", raw: strings.Repeat("界", 129), message: "query too long"},
		{name: "128 four byte scalars", raw: strings.Repeat("😀", 128), want: strings.Repeat("😀", 128)},
		{name: "129 four byte scalars", raw: strings.Repeat("😀", 129), message: "query too long"},
		{name: "128 combining scalars", raw: validCombining, want: validCombining},
		{name: "129 combining scalars", raw: invalidCombining, message: "query too long"},
		{name: "128 family emoji scalars", raw: validFamily, want: validFamily},
		{name: "129 family emoji scalars", raw: invalidFamily, message: "query too long"},
		{name: "invalid leading byte", raw: "\xff", message: "query invalid UTF-8"},
		{name: "invalid continuation", raw: "\xe2\x28\xa1", message: "query invalid UTF-8"},
		{name: "truncated sequence", raw: "\xe2\x82", message: "query invalid UTF-8"},
		{name: "512 spaces", raw: strings.Repeat(" ", 512), message: "query required"},
		{name: "513 spaces rejects before trim", raw: strings.Repeat(" ", 513), message: "query too long"},
		{name: "513 byte invalid input rejects before UTF-8 validation", raw: strings.Repeat("a", 512) + "\xff", message: "query too long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := requireQuery(tt.raw)
			if tt.message != "" {
				require.Equal(t, codes.InvalidArgument, status.Code(err))
				require.Equal(t, tt.message, status.Convert(err).Message())
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSearchRPCs_UnicodeLimitAndAuthenticationPrecedence(t *testing.T) {
	query128 := strings.Repeat("界", 128)
	query129 := strings.Repeat("界", 129)

	tests := []struct {
		name      string
		newServer func() (searchv1.SearchServiceClient, func() int, func() string)
		call      func(searchv1.SearchServiceClient, context.Context, string) error
	}{
		{
			name: "in chat",
			newServer: func() (searchv1.SearchServiceClient, func() int, func() string) {
				messages := &stubMessageSearch{}
				return startSearchGRPCTestServer(t, &SearchGRPC{Messages: messages}), func() int { return messages.calls }, func() string { return messages.lastInChatQuery }
			},
			call: func(client searchv1.SearchServiceClient, ctx context.Context, query string) error {
				_, err := client.SearchInChat(ctx, &searchv1.SearchInChatRequest{Chat: &chatv1.ChatRef{Id: uuid.New().String()}, Query: query})
				return err
			},
		},
		{
			name: "global",
			newServer: func() (searchv1.SearchServiceClient, func() int, func() string) {
				messages := &stubMessageSearch{}
				chatID := uuid.New()
				return startSearchGRPCTestServer(t, &SearchGRPC{Messages: messages, Chats: &stubChatAccess{accessible: []uuid.UUID{chatID}}}), func() int { return messages.calls }, func() string { return messages.lastGlobalQuery }
			},
			call: func(client searchv1.SearchServiceClient, ctx context.Context, query string) error {
				_, err := client.SearchGlobal(ctx, &searchv1.SearchGlobalRequest{Query: query})
				return err
			},
		},
		{
			name: "users",
			newServer: func() (searchv1.SearchServiceClient, func() int, func() string) {
				profiles := &stubProfileSearch{}
				return startSearchGRPCTestServer(t, &SearchGRPC{Profiles: profiles}), func() int { return profiles.calls }, func() string { return profiles.lastQuery }
			},
			call: func(client searchv1.SearchServiceClient, ctx context.Context, query string) error {
				_, err := client.SearchUsers(ctx, &searchv1.SearchUsersRequest{Query: query})
				return err
			},
		},
		{
			name: "spaces",
			newServer: func() (searchv1.SearchServiceClient, func() int, func() string) {
				spaces := &stubSpaceSearch{}
				return startSearchGRPCTestServer(t, &SearchGRPC{Spaces: spaces}), func() int { return spaces.calls }, func() string { return spaces.lastQuery }
			},
			call: func(client searchv1.SearchServiceClient, ctx context.Context, query string) error {
				_, err := client.SearchSpaces(ctx, &searchv1.SearchSpacesRequest{Query: query})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" accepts 128 multibyte scalars", func(t *testing.T) {
			client, calls, forwarded := tt.newServer()
			err := tt.call(client, ctxWithProfile(uuid.New()), "\u2003"+query128+"\u00a0")
			require.NoError(t, err)
			require.Equal(t, 1, calls())
			require.Equal(t, query128, forwarded())
		})

		t.Run(tt.name+" rejects 129 multibyte scalars before dependencies", func(t *testing.T) {
			client, calls, _ := tt.newServer()
			err := tt.call(client, ctxWithProfile(uuid.New()), query129)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Equal(t, "query too long", status.Convert(err).Message())
			require.Zero(t, calls())
		})

		t.Run(tt.name+" authenticates before oversized query validation", func(t *testing.T) {
			client, calls, _ := tt.newServer()
			err := tt.call(client, context.Background(), strings.Repeat("a", 513))
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.Zero(t, calls())
		})
	}
}
