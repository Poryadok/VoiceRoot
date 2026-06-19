package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestEnsureCallPrivacy_NilCheckerSkips(t *testing.T) {
	t.Parallel()
	s := &VoiceGRPC{Privacy: nil}
	require.NoError(t, s.ensureCallPrivacy(context.Background(), uuid.NewString(), uuid.NewString()))
}

func TestEnsureCallPrivacy_InvalidCaller(t *testing.T) {
	t.Parallel()
	s := &VoiceGRPC{Privacy: callPrivacyStub{}}
	err := s.ensureCallPrivacy(context.Background(), "not-uuid", uuid.NewString())
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestEnsureCallPrivacy_StrangerDenied(t *testing.T) {
	t.Parallel()
	callee := uuid.New()
	s := &VoiceGRPC{
		Privacy: callPrivacyStub{friendsOnly: map[uuid.UUID]bool{callee: true}},
		Friends: callNoFriendsStub{},
	}
	err := s.ensureCallPrivacy(context.Background(), uuid.NewString(), callee.String())
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestEnsureCallPrivacy_FriendsAllowed(t *testing.T) {
	t.Parallel()
	caller := uuid.New()
	callee := uuid.New()
	s := &VoiceGRPC{
		Privacy: callPrivacyStub{friendsOnly: map[uuid.UUID]bool{callee: true}},
		Friends: callFriendsStub{friends: map[[2]uuid.UUID]bool{{caller, callee}: true}},
	}
	require.NoError(t, s.ensureCallPrivacy(context.Background(), caller.String(), callee.String()))
}

func TestEnsureCallPrivacy_PrivacyDepsUnavailable(t *testing.T) {
	t.Parallel()
	s := &VoiceGRPC{Privacy: failingCallPrivacyStub{}}
	err := s.ensureCallPrivacy(context.Background(), uuid.NewString(), uuid.NewString())
	require.Equal(t, codes.Internal, status.Code(err))
}

type callFriendsStub struct {
	friends map[[2]uuid.UUID]bool
}

func (s callFriendsStub) AreFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	if s.friends[[2]uuid.UUID{a, b}] || s.friends[[2]uuid.UUID{b, a}] {
		return true, nil
	}
	return false, nil
}

func (callFriendsStub) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
