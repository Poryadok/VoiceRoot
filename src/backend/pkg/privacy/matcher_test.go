package privacy

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubSocial struct {
	friends map[string]bool
	fof     map[string]bool
}

func (s stubSocial) AreFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	return s.friends[pairKey(a, b)], nil
}

func (s stubSocial) AreFriendsOfFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	return s.fof[pairKey(a, b)], nil
}

type stubSpace struct {
	co map[string]bool
}

func (s stubSpace) AreCoMembers(_ context.Context, a, b uuid.UUID, _ []string) (bool, error) {
	return s.co[pairKey(a, b)], nil
}

func pairKey(a, b uuid.UUID) string {
	if a.String() < b.String() {
		return a.String() + ":" + b.String()
	}
	return b.String() + ":" + a.String()
}

func TestMatcher_UnionFriendsOrFoF(t *testing.T) {
	owner := uuid.New()
	viewer := uuid.New()
	m := Matcher{Social: stubSocial{
		friends: map[string]bool{},
		fof:     map[string]bool{pairKey(viewer, owner): true},
	}}
	ok, err := m.Allowed(context.Background(), owner, viewer, FriendsAndFoF(), false)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestMatcher_FriendsOnly(t *testing.T) {
	owner := uuid.New()
	viewer := uuid.New()
	m := Matcher{Social: stubSocial{friends: map[string]bool{pairKey(viewer, owner): true}}}
	ok, err := m.Allowed(context.Background(), owner, viewer, FriendsOnly(), false)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestMatcher_NobodyDenied(t *testing.T) {
	owner := uuid.New()
	viewer := uuid.New()
	m := Matcher{Social: stubSocial{friends: map[string]bool{pairKey(viewer, owner): true}}}
	ok, err := m.Allowed(context.Background(), owner, viewer, Nobody(), false)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcher_EveryoneShortcutAllowsStranger(t *testing.T) {
	owner := uuid.New()
	stranger := uuid.New()
	m := Matcher{}
	ok, err := m.Allowed(context.Background(), owner, stranger, EveryoneWithGuests(), false)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestMatcher_GuestIncludeGuests(t *testing.T) {
	owner := uuid.New()
	guest := uuid.New()
	m := Matcher{}
	ok, err := m.Allowed(context.Background(), owner, guest, Audience{IncludeGuests: true}, true)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestMatcher_IncludeGuestsOnly_StrangerDenied(t *testing.T) {
	owner := uuid.New()
	stranger := uuid.New()
	m := Matcher{}
	ok, err := m.Allowed(context.Background(), owner, stranger, Audience{IncludeGuests: true}, false)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcher_IncludeGuestsOnly_FriendDenied(t *testing.T) {
	owner := uuid.New()
	friend := uuid.New()
	m := Matcher{Social: stubSocial{friends: map[string]bool{pairKey(friend, owner): true}}}
	ok, err := m.Allowed(context.Background(), owner, friend, Audience{IncludeGuests: true}, false)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcher_GuestDeniedWhenIncludeGuestsFalse(t *testing.T) {
	owner := uuid.New()
	guest := uuid.New()
	m := Matcher{}
	ok, err := m.Allowed(context.Background(), owner, guest, FriendsOnly(), true)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcher_SpaceMembers(t *testing.T) {
	owner := uuid.New()
	viewer := uuid.New()
	m := Matcher{Space: stubSpace{co: map[string]bool{pairKey(viewer, owner): true}}}
	ok, err := m.Allowed(context.Background(), owner, viewer, SpaceMembersOnly(), false)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestMatcher_SpaceS2SDownFailClosed(t *testing.T) {
	owner := uuid.New()
	viewer := uuid.New()
	m := Matcher{Space: nil}
	ok, err := m.Allowed(context.Background(), owner, viewer, SpaceMembersOnly(), false)
	require.NoError(t, err)
	require.False(t, ok)
}
