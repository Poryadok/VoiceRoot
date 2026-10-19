package grpcsvc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/ordinarycall"
	voicestore "voice/backend/voice/internal/store"
)

// This fixture proves handler orchestration, not real PG fence serialization.
type ordinaryHandlerJournal struct {
	mu    sync.Mutex
	rooms map[uuid.UUID][]byte
}

func (j *ordinaryHandlerJournal) Create(_ context.Context, r *ordinarycall.Room) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.rooms == nil {
		j.rooms = map[uuid.UUID][]byte{}
	}
	b, err := json.Marshal(r)
	j.rooms[r.Intent.RoomID] = b
	return err
}
func (j *ordinaryHandlerJournal) Load(_ context.Context, id uuid.UUID) (*ordinarycall.Room, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.decode(id)
}
func (j *ordinaryHandlerJournal) decode(id uuid.UUID) (*ordinarycall.Room, error) {
	b, ok := j.rooms[id]
	if !ok {
		return nil, ordinarycall.ErrNotFound
	}
	var r ordinarycall.Room
	err := json.Unmarshal(b, &r)
	return &r, err
}
func (j *ordinaryHandlerJournal) Mutate(_ context.Context, id uuid.UUID, change func(*ordinarycall.Room) error) (*ordinarycall.Room, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	r, err := j.decode(id)
	if err != nil {
		return nil, err
	}
	if err = change(r); err != nil {
		return nil, err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	j.rooms[id] = b
	return r, nil
}
func (*ordinaryHandlerJournal) MarkProjected(context.Context, uuid.UUID, uint64) error { return nil }
func (*ordinaryHandlerJournal) CheckExecution(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (j *ordinaryHandlerJournal) FindForProfile(_ context.Context, profile uuid.UUID) (*ordinarycall.Room, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for id := range j.rooms {
		r, err := j.decode(id)
		if err != nil {
			return nil, err
		}
		m, ok := r.Members[profile.String()]
		if ok && r.Stage != ordinarycall.StageClosed && m.State != ordinarycall.MemberLeft && m.State != ordinarycall.MemberAborted {
			return r, nil
		}
	}
	return nil, ordinarycall.ErrNotFound
}
func (j *ordinaryHandlerJournal) FindActiveGroupForChat(_ context.Context, chat uuid.UUID) (*ordinarycall.Room, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for id := range j.rooms {
		r, err := j.decode(id)
		if err != nil {
			return nil, err
		}
		if r.Intent.Group && r.Intent.ChatID == chat && r.Stage != ordinarycall.StageClosed {
			return r, nil
		}
	}
	return nil, ordinarycall.ErrNotFound
}

func (j *ordinaryHandlerJournal) AuthorizeMedia(ctx context.Context, binding ordinarycall.MediaBinding) (string, error) {
	r, err := j.Load(ctx, binding.RoomID)
	if err != nil {
		return "", err
	}
	actual, err := r.MediaBinding(binding.ProfileID)
	if err != nil {
		return "", err
	}
	if actual != binding {
		return "", ordinarycall.ErrStaleGeneration
	}
	return r.LiveKitRoomName, nil
}

type ordinaryHandlerMedia struct {
	ensure func(context.Context, string) error
}

func (m *ordinaryHandlerMedia) EnsureRoom(ctx context.Context, name string) error {
	if m.ensure != nil {
		return m.ensure(ctx, name)
	}
	return nil
}
func (*ordinaryHandlerMedia) CloseRoom(context.Context, string) error                 { return nil }
func (*ordinaryHandlerMedia) RemoveParticipant(context.Context, string, string) error { return nil }
func (*ordinaryHandlerMedia) ListParticipantIdentities(context.Context, string) ([]string, error) {
	return nil, nil
}

func TestOrdinaryStartHandlerJournalsBeforeSDKAndUsesDurableIntentInsteadOfDirectPublisher(t *testing.T) {
	now := time.Now().UTC()
	events := &recordingEvents{}
	s := newTestVoiceService(now, events)
	caller, callee, account, calleeAccount := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	s.AccountVoiceProfiles = accountResolverStub{caller: account, callee: calleeAccount}
	s.SessionEpochChecker = currentSessionCheckerFunc(func(context.Context, string, int64) error { return nil })
	j, m := &ordinaryHandlerJournal{}, &ordinaryHandlerMedia{}
	s.OrdinaryCalls = &ordinarycall.Coordinator{Journal: j, Media: m, Projection: &ordinaryProjection{Calls: s.Calls.(voicestore.OrdinaryProjectionStore), Journal: j}, Now: func() time.Time { return now }}
	m.ensure = func(_ context.Context, name string) error {
		require.Len(t, j.rooms, 1)
		for id := range j.rooms {
			r, err := j.Load(context.Background(), id)
			require.NoError(t, err)
			require.Equal(t, ordinarycall.StagePreparing, r.Stage)
			require.Empty(t, r.Events)
			require.Equal(t, r.LiveKitRoomName, name)
			require.False(t, r.CanIssueToken(caller))
		}
		return nil
	}
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "voice", AccountID: account.String(), ProfileID: caller.String(), SessionEpoch: 7})
	response, err := s.StartCall(ctx, &callsv1.StartCallRequest{LinkedChat: &chatv1.ChatRef{Id: uuid.NewString()}, CalleeProfileId: strPtr(callee.String()), MediaKind: mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO)})
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_RINGING, response.GetCallSession().GetStatus())
	r, err := j.Load(context.Background(), uuid.MustParse(response.GetCallSession().GetRoomId()))
	require.NoError(t, err)
	require.Len(t, r.Events, 1)
	require.Equal(t, ordinarycall.EventIncoming, r.Events[0].Kind)
	require.Empty(t, events.incoming, "compatibility event must be delivered from committed outbox, not directly by handler")
	require.Equal(t, int64(7), r.Members[caller.String()].SessionEpoch)
	require.Zero(t, r.Members[callee.String()].SessionEpoch)
}

func TestOrdinaryHandlersAcceptTokenAndEndUseOneNativeJournal(t *testing.T) {
	now := time.Now().UTC()
	events := &recordingEvents{}
	s := newTestVoiceService(now, events)
	caller, callee, account, calleeAccount := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	s.AccountVoiceProfiles = accountResolverStub{caller: account, callee: calleeAccount}
	s.SessionEpochChecker = currentSessionCheckerFunc(func(context.Context, string, int64) error { return nil })
	j, m := &ordinaryHandlerJournal{}, &ordinaryHandlerMedia{}
	s.OrdinaryCalls = &ordinarycall.Coordinator{Journal: j, Media: m, Projection: &ordinaryProjection{Calls: s.Calls.(voicestore.OrdinaryProjectionStore), Journal: j}, Now: func() time.Time { return now }}
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "voice", AccountID: account.String(), ProfileID: caller.String(), SessionEpoch: 7})
	peerCtx := principal.WithVerified(context.Background(), principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "voice", AccountID: calleeAccount.String(), ProfileID: callee.String(), SessionEpoch: 9})
	started, err := s.StartCall(ctx, &callsv1.StartCallRequest{LinkedChat: &chatv1.ChatRef{Id: uuid.NewString()}, CalleeProfileId: strPtr(callee.String()), MediaKind: mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO)})
	require.NoError(t, err)
	roomID := started.GetCallSession().GetRoomId()
	active, err := s.GetActiveCall(ctx, &callsv1.GetActiveCallRequest{})
	require.NoError(t, err)
	require.Equal(t, roomID, active.GetCallSession().GetRoomId())
	_, err = s.GetJoinToken(ctx, &callsv1.GetJoinTokenRequest{RoomId: roomID})
	require.Error(t, err, "ringing reservation must not authorize media")
	accepted, err := s.AcceptCall(peerCtx, &callsv1.AcceptCallRequest{RoomId: roomID})
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, accepted.GetCallSession().GetStatus())
	stored, err := j.Load(context.Background(), uuid.MustParse(roomID))
	require.NoError(t, err)
	require.Equal(t, ordinarycall.StageActive, stored.Stage)
	require.Equal(t, uint64(2), stored.RosterVersion)
	// Losing every Redis projection must recover from the native locator,
	// without inventing absence or another logical join/event.
	s.Calls = voicestore.NewMemoryCallStore()
	s.OrdinaryCalls.Projection = NewOrdinaryProjection(s.Calls.(voicestore.OrdinaryProjectionStore), j)
	active, err = s.GetActiveCall(ctx, &callsv1.GetActiveCallRequest{})
	require.NoError(t, err)
	require.Equal(t, roomID, active.GetCallSession().GetRoomId())
	token, err := s.GetJoinToken(ctx, &callsv1.GetJoinTokenRequest{RoomId: roomID})
	require.NoError(t, err)
	parts := strings.Split(token.GetJwt(), ".")
	require.Len(t, parts, 3)
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(claimsBytes, &claims))
	require.Equal(t, stored.Members[caller.String()].MediaIdentity, claims["sub"])
	_, err = s.EndCall(ctx, &callsv1.EndCallRequest{RoomId: roomID})
	require.NoError(t, err)
	stored, err = j.Load(context.Background(), uuid.MustParse(roomID))
	require.NoError(t, err)
	require.Equal(t, ordinarycall.StageClosed, stored.Stage)
	require.Equal(t, uint64(4), stored.RosterVersion)
	_, err = s.GetJoinToken(ctx, &callsv1.GetJoinTokenRequest{RoomId: roomID})
	require.Error(t, err)
	require.Empty(t, events.accepted)
	require.Empty(t, events.ended, "ordinary facts belong to the native outbox")
	_, err = s.GetActiveCall(ctx, &callsv1.GetActiveCallRequest{})
	require.Error(t, err)
}
