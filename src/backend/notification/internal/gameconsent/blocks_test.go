package gameconsent

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

type profileAccountStub map[string]string

func (s profileAccountStub) GetProfile(_ context.Context, req *userv1.GetProfileRequest, _ ...grpc.CallOption) (*userv1.GetProfileResponse, error) {
	profileID := req.GetBy().(*userv1.GetProfileRequest_ProfileId).ProfileId
	accountID, ok := s[profileID]
	if !ok {
		return nil, errors.New("profile unavailable")
	}
	return &userv1.GetProfileResponse{Profile: &userv1.Profile{Id: profileID, AccountId: accountID}}, nil
}

type blockPairStub struct {
	blocked map[string]bool
	err     error
	calls   int
}

func (s *blockPairStub) IsBlocked(_ context.Context, req *socialv1.IsBlockedRequest, _ ...grpc.CallOption) (*socialv1.IsBlockedResponse, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return &socialv1.IsBlockedResponse{Blocked: s.blocked[req.GetAccountIdA()+":"+req.GetAccountIdB()]}, nil
}

func TestBlockCheckerSuppressesEitherDirectedAccountBlock(t *testing.T) {
	profileA, profileB, accountA, accountB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	blocks := &blockPairStub{blocked: map[string]bool{accountB.String() + ":" + accountA.String(): true}}
	checker := &BlockChecker{Profiles: profileAccountStub{profileA.String(): accountA.String(), profileB.String(): accountB.String()}, Blocks: blocks}
	blocked, err := checker.IsGamePushBlocked(context.Background(), profileA, profileB)
	require.NoError(t, err)
	require.True(t, blocked)
	require.Equal(t, 2, blocks.calls, "block lookup checks both directions")
}

func TestBlockCheckerFailsClosedWhenProfileOrBlockAuthorityIsUnavailable(t *testing.T) {
	checker := &BlockChecker{Profiles: profileAccountStub{}, Blocks: &blockPairStub{}}
	blocked, err := checker.IsGamePushBlocked(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, ErrUnavailable)
	require.False(t, blocked)

	profileA, profileB, accountA, accountB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	checker = &BlockChecker{Profiles: profileAccountStub{profileA.String(): accountA.String(), profileB.String(): accountB.String()}, Blocks: &blockPairStub{err: errors.New("social unavailable")}}
	blocked, err = checker.IsGamePushBlocked(context.Background(), profileA, profileB)
	require.ErrorIs(t, err, ErrUnavailable)
	require.False(t, blocked)
}
