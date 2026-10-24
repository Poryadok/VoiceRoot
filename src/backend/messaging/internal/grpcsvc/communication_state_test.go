package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/spacecommunication"
)

type communicationScopeStub struct {
	policy *store.ChatThreadPolicy
	err    error
}

func (s communicationScopeStub) Load(context.Context, uuid.UUID) (*store.ChatThreadPolicy, error) {
	return s.policy, s.err
}

type communicationStateStub struct {
	calls     int
	state     int32
	err       error
	malformed bool
	deadline  time.Time
}

func (s *communicationStateStub) ResolveMemberCommunicationState(ctx context.Context, space, profile string) (spacecommunication.Snapshot, error) {
	s.calls++
	s.deadline, _ = ctx.Deadline()
	now := time.Now()
	snapshot := spacecommunication.Snapshot{Version: 1, SpaceID: space, ProfileID: profile, Epoch: 1, EvaluatedAt: timestamppb.New(now), State: s.state}
	if s.state == spacecommunication.TimedOut {
		snapshot.Until = timestamppb.New(now.Add(time.Hour))
	}
	if s.malformed {
		snapshot.ProfileID = uuid.NewString()
	}
	return snapshot, s.err
}

func TestCommunicationActionRequiresFreshBoundSpaceDecision(t *testing.T) {
	space, chat, profile := uuid.New(), uuid.New(), uuid.New()
	authority := &communicationStateStub{state: spacecommunication.Clear}
	svc := &MessagingGRPC{CommunicationScope: communicationScopeStub{policy: &store.ChatThreadPolicy{ChatType: "channel", SpaceID: &space}}, CommunicationStates: authority}
	for i := 1; i <= 2; i++ {
		ctx, cancel, err := svc.beginCommunicationAction(t.Context(), chat, profile)
		require.NoError(t, err)
		require.Equal(t, i, authority.calls)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.Equal(t, deadline, authority.deadline)
		require.WithinDuration(t, time.Now().Add(2*time.Second), deadline, 100*time.Millisecond)
		cancel()
	}
	authority.state = spacecommunication.TimedOut
	_, _, err := svc.beginCommunicationAction(t.Context(), chat, profile)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, 3, authority.calls)
	authority.state = spacecommunication.Clear
	authority.malformed = true
	_, _, err = svc.beginCommunicationAction(t.Context(), chat, profile)
	require.Equal(t, codes.Unavailable, status.Code(err))
	authority.malformed = false
	authority.err = errors.New("provider unavailable")
	_, _, err = svc.beginCommunicationAction(t.Context(), chat, profile)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestCommunicationActionMissingCapabilityAndUnknownScopeFailClosed(t *testing.T) {
	space := uuid.New()
	for _, policy := range []*store.ChatThreadPolicy{nil, {ChatType: ""}, {ChatType: "dm", SpaceID: &space}, {ChatType: "channel", SpaceID: &space}} {
		svc := &MessagingGRPC{CommunicationScope: communicationScopeStub{policy: policy}}
		_, _, err := svc.beginCommunicationAction(t.Context(), uuid.New(), uuid.New())
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
}

func TestCommunicationActionLeavesOtherScopesOutsideTimeoutAuthority(t *testing.T) {
	for _, typ := range []string{"dm", "group", "channel"} {
		svc := &MessagingGRPC{CommunicationScope: communicationScopeStub{policy: &store.ChatThreadPolicy{ChatType: typ}}}
		ctx, cancel, err := svc.beginCommunicationAction(t.Context(), uuid.New(), uuid.New())
		require.NoError(t, err)
		require.Equal(t, t.Context(), ctx)
		cancel()
	}
}
