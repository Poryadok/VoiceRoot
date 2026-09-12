package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	chatv1 "voice.app/voice/chat/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

type policyChatClient struct {
	chat       *chatv1.Chat
	members    []*chatv1.ChatMember
	getErr     error
	membersErr error
}

func (c *policyChatClient) GetChat(context.Context, *chatv1.GetChatRequest, ...grpc.CallOption) (*chatv1.GetChatResponse, error) {
	return &chatv1.GetChatResponse{Chat: c.chat}, c.getErr
}

func (c *policyChatClient) ListMembers(context.Context, *chatv1.ListMembersRequest, ...grpc.CallOption) (*chatv1.ListMembersResponse, error) {
	return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{Members: c.members}}, c.membersErr
}

type policyUserClient struct {
	accounts map[string]string
	err      error
	calls    int
}

func (c *policyUserClient) GetProfile(_ context.Context, req *userv1.GetProfileRequest, _ ...grpc.CallOption) (*userv1.GetProfileResponse, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	profileID := req.GetProfileId()
	return &userv1.GetProfileResponse{Profile: &userv1.Profile{Id: profileID, AccountId: c.accounts[profileID]}}, nil
}

type policySocialClient struct {
	blocked map[string]bool
	err     error
	pairs   [][2]string
}

type blockingPolicySocialClient struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingPolicySocialClient) IsBlocked(_ context.Context, _ *socialv1.IsBlockedRequest, _ ...grpc.CallOption) (*socialv1.IsBlockedResponse, error) {
	c.once.Do(func() {
		close(c.started)
		<-c.release
	})
	return &socialv1.IsBlockedResponse{}, nil
}

func (c *policySocialClient) IsBlocked(_ context.Context, req *socialv1.IsBlockedRequest, _ ...grpc.CallOption) (*socialv1.IsBlockedResponse, error) {
	c.pairs = append(c.pairs, [2]string{req.GetAccountIdA(), req.GetAccountIdB()})
	if c.err != nil {
		return nil, c.err
	}
	return &socialv1.IsBlockedResponse{Blocked: c.blocked[req.GetAccountIdA()+"/"+req.GetAccountIdB()]}, nil
}

func TestChatSubscriptionPolicyChecksSocialForDMOnlyAndRemembersPair(t *testing.T) {
	accountA, accountB := uuid.NewString(), uuid.NewString()
	profileA, profileB, dmChatID, groupChatID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	hub := newWSHub()
	user := &policyUserClient{accounts: map[string]string{profileB: accountB}}
	social := &policySocialClient{blocked: map[string]bool{}}
	dm := newChatSubscriptionPolicy(
		&policyChatClient{chat: &chatv1.Chat{Id: dmChatID, Type: chatv1.ChatType_CHAT_TYPE_DM}, members: []*chatv1.ChatMember{{ProfileId: profileA}, {ProfileId: profileB}}},
		user, social, hub,
	)
	reg := hub.attachAccountConn("instance", "conn", accountA, profileA, 1)
	registrar, ok := dm.(chatSubscriptionRegistrar)
	if !ok {
		t.Fatal("DM policy does not support atomic subscription registration")
	}
	if err := registrar.AuthorizeAndAddChat(context.Background(), accountA, profileA, dmChatID, reg); err != nil {
		t.Fatalf("allowed DM rejected: %v", err)
	}
	if user.calls != 1 {
		t.Fatalf("user calls=%d, want 1", user.calls)
	}
	wantPairs := [][2]string{{accountA, accountB}, {accountB, accountA}}
	if !slices.Equal(social.pairs, wantPairs) {
		t.Fatalf("social pairs=%v, want %v", social.pairs, wantPairs)
	}
	hub.revokeAccountPairDMChats(accountA, accountB)
	if hub.hasChat(reg, dmChatID) {
		t.Fatal("allowed DM pair was not remembered for later block revocation")
	}

	user.calls = 0
	social.pairs = nil
	group := newChatSubscriptionPolicy(
		&policyChatClient{chat: &chatv1.Chat{Id: groupChatID, Type: chatv1.ChatType_CHAT_TYPE_GROUP}},
		user, social, hub,
	)
	if err := group.AuthorizeChat(context.Background(), accountA, profileA, groupChatID); err != nil {
		t.Fatalf("group subscription rejected: %v", err)
	}
	if user.calls != 0 || len(social.pairs) != 0 {
		t.Fatalf("non-DM invoked DM policy: user=%d social=%v", user.calls, social.pairs)
	}
}

func TestDMSubscriptionCheckEventAddRaceFailsClosed(t *testing.T) {
	accountA, accountB := uuid.NewString(), uuid.NewString()
	profileA, profileB, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	hub := newWSHub()
	social := &blockingPolicySocialClient{started: make(chan struct{}), release: make(chan struct{})}
	checker := newChatSubscriptionPolicy(
		&policyChatClient{
			chat:    &chatv1.Chat{Id: chatID, Type: chatv1.ChatType_CHAT_TYPE_DM},
			members: []*chatv1.ChatMember{{ProfileId: profileA}, {ProfileId: profileB}},
		},
		&policyUserClient{accounts: map[string]string{profileB: accountB}},
		social,
		hub,
	)
	reg := hub.attachAccountConn("instance", "conn", accountA, profileA, 1)
	result := make(chan error, 1)
	go func() {
		result <- checker.(chatSubscriptionRegistrar).AuthorizeAndAddChat(context.Background(), accountA, profileA, chatID, reg)
	}()
	<-social.started
	hub.revokeAccountPairDMChats(accountA, accountB)
	close(social.release)

	if err := <-result; err == nil {
		t.Fatal("an event racing the authoritative check authorized add")
	}
	if hub.hasChat(reg, chatID) {
		t.Fatal("racing subscription was added")
	}
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if len(hub.dmPairByChat) != 0 || len(hub.dmChatsByPair) != 0 {
		t.Fatalf("race left pair index state: by_chat=%d by_pair=%d", len(hub.dmPairByChat), len(hub.dmChatsByPair))
	}
}

func TestChatSubscriptionPolicyDeniesBlocksAndDependencyFailures(t *testing.T) {
	accountA, accountB := uuid.NewString(), uuid.NewString()
	profileA, profileB, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	newPolicy := func(userErr, socialErr error, blocked map[string]bool) chatSubscriptionChecker {
		return newChatSubscriptionPolicy(
			&policyChatClient{chat: &chatv1.Chat{Id: chatID, Type: chatv1.ChatType_CHAT_TYPE_DM}, members: []*chatv1.ChatMember{{ProfileId: profileA}, {ProfileId: profileB}}},
			&policyUserClient{accounts: map[string]string{profileB: accountB}, err: userErr},
			&policySocialClient{blocked: blocked, err: socialErr},
			newWSHub(),
		)
	}
	for name, policy := range map[string]chatSubscriptionChecker{
		"blocked-forward":    newPolicy(nil, nil, map[string]bool{accountA + "/" + accountB: true}),
		"blocked-reverse":    newPolicy(nil, nil, map[string]bool{accountB + "/" + accountA: true}),
		"user-unavailable":   newPolicy(status.Error(codes.Unavailable, "user down"), nil, nil),
		"social-unavailable": newPolicy(nil, status.Error(codes.Unavailable, "social down"), nil),
	} {
		t.Run(name, func(t *testing.T) {
			err := policy.AuthorizeChat(context.Background(), accountA, profileA, chatID)
			if err == nil {
				t.Fatal("DM policy failure authorized subscription")
			}
			if strings.HasPrefix(name, "blocked-") && status.Code(err) != codes.PermissionDenied {
				t.Fatalf("blocked error code=%s, want PermissionDenied", status.Code(err))
			}
		})
	}
}

type recordingGetChatServer struct {
	chatv1.UnimplementedChatServiceServer
	mu       sync.Mutex
	metadata metadata.MD
	err      error
	delay    time.Duration
	chatID   string
}

func (s *recordingGetChatServer) GetChat(ctx context.Context, req *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error) {
	s.mu.Lock()
	s.metadata, _ = metadata.FromIncomingContext(ctx)
	err, delay := s.err, s.delay
	s.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	chatID := s.chatID
	if chatID == "" {
		chatID = req.GetChatId()
	}
	return &chatv1.GetChatResponse{Chat: &chatv1.Chat{Id: chatID}}, nil
}

func (s *recordingGetChatServer) incomingMetadata() metadata.MD {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metadata.Copy()
}

func newBufconnChatSubscriptionChecker(t *testing.T, server *recordingGetChatServer) chatSubscriptionChecker {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	chatv1.RegisterChatServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})
	conn, err := grpc.NewClient("passthrough:///chat-subscription-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return newGRPCChatSubscriptionChecker(conn)
}

func TestGRPCChatSubscriptionCheckerForwardsCallerMetadataAndFailsClosed(t *testing.T) {
	accountID, profileID, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for name, setup := range map[string]func(*recordingGetChatServer){
		"allowed":         func(*recordingGetChatServer) {},
		"mismatched-chat": func(s *recordingGetChatServer) { s.chatID = uuid.NewString() },
		"not-found":       func(s *recordingGetChatServer) { s.err = status.Error(codes.NotFound, "not found") },
		"nonmember":       func(s *recordingGetChatServer) { s.err = status.Error(codes.PermissionDenied, "not member") },
		"role-denied":     func(s *recordingGetChatServer) { s.err = status.Error(codes.PermissionDenied, "role denied") },
		"internal":        func(s *recordingGetChatServer) { s.err = status.Error(codes.Internal, "dependency detail") },
		"unavailable":     func(s *recordingGetChatServer) { s.err = status.Error(codes.Unavailable, "unavailable") },
	} {
		t.Run(name, func(t *testing.T) {
			server := &recordingGetChatServer{}
			setup(server)
			ctx := metadata.AppendToOutgoingContext(context.Background(), grpcMDVoiceInternalCaller, "must-not-forward")
			err := newBufconnChatSubscriptionChecker(t, server).AuthorizeChat(ctx, accountID, profileID, chatID)
			if name == "allowed" && err != nil {
				t.Fatalf("allowed checker error: %v", err)
			}
			if name != "allowed" && err == nil {
				t.Fatalf("%s unexpectedly authorized", name)
			}
			md := server.incomingMetadata()
			if got := md.Get(grpcMDVoiceUserID); len(got) != 1 || got[0] != accountID {
				t.Fatalf("user metadata = %v, want %q", got, accountID)
			}
			if got := md.Get(grpcMDVoiceProfileID); len(got) != 1 || got[0] != profileID {
				t.Fatalf("profile metadata = %v, want %q", got, profileID)
			}
			if got := md.Get(grpcMDVoiceInternalCaller); len(got) != 0 {
				t.Fatalf("GetChat unexpectedly has internal caller metadata: %v", got)
			}
		})
	}
}

func TestGRPCChatSubscriptionCheckerAcceptsCanonicalResponseForUppercaseRequest(t *testing.T) {
	accountID, profileID, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	server := &recordingGetChatServer{chatID: chatID}
	if err := newBufconnChatSubscriptionChecker(t, server).AuthorizeChat(context.Background(), accountID, profileID, strings.ToUpper(chatID)); err != nil {
		t.Fatalf("canonical response rejected uppercase UUID request: %v", err)
	}
}

func TestGRPCChatSubscriptionCheckerTimeoutAndNilClientFailClosed(t *testing.T) {
	oldTimeout := chatSubscriptionCheckTimeout
	chatSubscriptionCheckTimeout = 20 * time.Millisecond
	t.Cleanup(func() { chatSubscriptionCheckTimeout = oldTimeout })
	checker := newBufconnChatSubscriptionChecker(t, &recordingGetChatServer{delay: time.Second})
	if err := checker.AuthorizeChat(context.Background(), uuid.NewString(), uuid.NewString(), uuid.NewString()); err == nil {
		t.Fatal("timeout authorized subscription")
	}
	if err := newGRPCChatSubscriptionChecker(nil).AuthorizeChat(context.Background(), uuid.NewString(), uuid.NewString(), uuid.NewString()); err == nil {
		t.Fatal("nil client authorized subscription")
	}
}

type recordingSubscriptionChecker struct {
	mu    sync.Mutex
	calls int
	err   error
}

type subscriptionCheckerFunc func(context.Context, string, string, string) error

func (f subscriptionCheckerFunc) AuthorizeChat(ctx context.Context, accountID, profileID, chatID string) error {
	return f(ctx, accountID, profileID, chatID)
}

func (c *recordingSubscriptionChecker) AuthorizeChat(context.Context, string, string, string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.err
}

func (c *recordingSubscriptionChecker) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func newACLTestRealtimeHandlerWithChecker(tv tokenValidator, checker chatSubscriptionChecker) (*wsHub, *httptest.Server) {
	hub := newWSHub()
	hub.subscriptionChecker = checker
	return hub, httptest.NewServer(newServiceHandler(serviceName, tv, nil, hub, nil, "acl-checker-test", readinessDeps{}))
}

func TestWSSubscribeACLCheckerFailureMapsToGenericDenialAndConnectionStaysOpen(t *testing.T) {
	accountID, profileID, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for name, checkerErr := range map[string]error{
		"not-found":        status.Error(codes.NotFound, "chat not found"),
		"deleted-for-self": status.Error(codes.NotFound, "deleted for self"),
		"nonmember":        status.Error(codes.PermissionDenied, "not a member"),
		"internal":         status.Error(codes.Internal, "role backend detail"),
		"unavailable":      status.Error(codes.Unavailable, "chat unavailable"),
		"timeout":          context.DeadlineExceeded,
		"misconfigured":    errors.New("checker unavailable"),
	} {
		t.Run(name, func(t *testing.T) {
			checker := &recordingSubscriptionChecker{err: checkerErr}
			hub, srv := newACLTestRealtimeHandlerWithChecker(staticTokenValidator{
				"member": {UserID: accountID, ProfileID: profileID},
			}, checker)
			t.Cleanup(srv.Close)
			c := dialACLTestConn(t, srv, "member", profileID)
			if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
				t.Fatalf("subscribe: %v", err)
			}
			assertSubscriptionDenied(t, readACLEnvelope(t, c), 2)
			if checker.callCount() != 1 {
				t.Fatalf("checker calls = %d, want 1", checker.callCount())
			}
			hub.mu.RLock()
			_, registered := hub.byChat[chatID]
			hub.mu.RUnlock()
			if registered {
				t.Fatal("denied subscribe registered a hub chat")
			}
			if err := c.WriteJSON(map[string]any{"op": "heartbeat"}); err != nil {
				t.Fatalf("heartbeat: %v", err)
			}
			if got := readACLEnvelope(t, c); got.Op != "heartbeat_ack" || got.S != 3 {
				t.Fatalf("denied connection did not stay open: %+v", got)
			}
		})
	}
}

func TestWSSubscribeACLRechecksDuplicateWithoutPositiveCache(t *testing.T) {
	accountID, profileID, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	checker := &recordingSubscriptionChecker{}
	_, srv := newACLTestRealtimeHandlerWithChecker(staticTokenValidator{
		"member": {UserID: accountID, ProfileID: profileID},
	}, checker)
	t.Cleanup(srv.Close)
	c := dialACLTestConn(t, srv, "member", profileID)
	for sequence := int64(2); sequence <= 3; sequence++ {
		if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		if got := readACLEnvelope(t, c); got.Op != "subscribe_ack" || got.S != sequence {
			t.Fatalf("subscribe response = %+v", got)
		}
	}
	if checker.callCount() != 2 {
		t.Fatalf("checker calls = %d, want 2 to prove no positive cache", checker.callCount())
	}
}

func TestWSSubscribeACLMalformedUUIDDoesNotCallChat(t *testing.T) {
	accountID, profileID := uuid.NewString(), uuid.NewString()
	checker := &recordingSubscriptionChecker{}
	_, srv := newACLTestRealtimeHandlerWithChecker(staticTokenValidator{
		"member": {UserID: accountID, ProfileID: profileID},
	}, checker)
	t.Cleanup(srv.Close)
	c := dialACLTestConn(t, srv, "member", profileID)
	if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": "not-a-uuid"}}); err != nil {
		t.Fatalf("malformed subscribe: %v", err)
	}
	got := readACLEnvelope(t, c)
	if got.Op != "error" || got.S != 2 {
		t.Fatalf("malformed response = %+v", got)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(got.D, &body); err != nil || body.Code != "invalid_subscribe" {
		t.Fatalf("malformed response body = %s, err=%v", got.D, err)
	}
	if checker.callCount() != 0 {
		t.Fatalf("malformed UUID invoked Chat checker %d times", checker.callCount())
	}
}

func TestWSUnsubscribeDoesNotCheckChat(t *testing.T) {
	accountID, profileID, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	checker := &recordingSubscriptionChecker{}
	_, srv := newACLTestRealtimeHandlerWithChecker(staticTokenValidator{
		"member": {UserID: accountID, ProfileID: profileID},
	}, checker)
	t.Cleanup(srv.Close)
	c := dialACLTestConn(t, srv, "member", profileID)
	if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if got := readACLEnvelope(t, c); got.Op != "subscribe_ack" || got.S != 2 {
		t.Fatalf("subscribe response = %+v", got)
	}
	if err := c.WriteJSON(map[string]any{"op": "unsubscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	if got := readACLEnvelope(t, c); got.Op != "unsubscribe_ack" || got.S != 3 {
		t.Fatalf("unsubscribe response = %+v", got)
	}
	if checker.callCount() != 1 {
		t.Fatalf("unsubscribe invoked Chat checker; calls = %d", checker.callCount())
	}
}

func TestWSSubscribeACLDoesNotShareMembershipBetweenProfilesOfOneAccount(t *testing.T) {
	accountID, memberProfileID, otherProfileID, chatID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	checker := subscriptionCheckerFunc(func(_ context.Context, account, profile, chat string) error {
		if account != accountID || chat != chatID || profile != memberProfileID {
			return status.Error(codes.PermissionDenied, "not a chat member")
		}
		return nil
	})
	_, srv := newACLTestRealtimeHandlerWithChecker(staticTokenValidator{
		"member": {UserID: accountID, ProfileID: memberProfileID},
		"other":  {UserID: accountID, ProfileID: otherProfileID},
	}, checker)
	t.Cleanup(srv.Close)
	member := dialACLTestConn(t, srv, "member", memberProfileID)
	if err := member.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("member subscribe: %v", err)
	}
	if got := readACLEnvelope(t, member); got.Op != "subscribe_ack" || got.S != 2 {
		t.Fatalf("member subscribe response = %+v", got)
	}
	other := dialACLTestConn(t, srv, "other", otherProfileID)
	if err := other.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("other subscribe: %v", err)
	}
	assertSubscriptionDenied(t, readACLEnvelope(t, other), 2)
}
