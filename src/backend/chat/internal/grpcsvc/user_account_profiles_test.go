package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	userv1 "voice.app/voice/user/v1"
)

type accountProfileIDsClientForTest struct {
	ctx  context.Context
	req  *userv1.ListProfileIDsForAccountRequest
	resp *userv1.ListProfileIDsForAccountResponse
	err  error
}

func (c *accountProfileIDsClientForTest) ListProfileIDsForAccount(
	ctx context.Context,
	req *userv1.ListProfileIDsForAccountRequest,
	_ ...grpc.CallOption,
) (*userv1.ListProfileIDsForAccountResponse, error) {
	c.ctx = ctx
	c.req = req
	return c.resp, c.err
}

func TestUserGRPCAccountProfiles_ListProfileIDsForAccountUsesChatS2SAndStrictUUIDs(t *testing.T) {
	accountID, profileA, profileB := uuid.New(), uuid.New(), uuid.New()
	client := &accountProfileIDsClientForTest{resp: &userv1.ListProfileIDsForAccountResponse{
		ProfileIds: []string{profileA.String(), profileB.String()},
	}}
	adapter := &UserGRPCAccountProfiles{Client: client}

	got, err := adapter.ListProfileIDsForAccount(context.Background(), accountID.String())
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{profileA, profileB}, got)
	require.Equal(t, accountID.String(), client.req.GetAccountId())
	md, ok := metadata.FromOutgoingContext(client.ctx)
	require.True(t, ok)
	require.Equal(t, []string{"chat"}, md.Get("x-voice-internal-caller"))
}

func TestUserGRPCAccountProfiles_ListProfileIDsForAccountRejectsInvalidIDs(t *testing.T) {
	t.Run("invalid account is not sent", func(t *testing.T) {
		client := &accountProfileIDsClientForTest{}
		adapter := &UserGRPCAccountProfiles{Client: client}

		_, err := adapter.ListProfileIDsForAccount(context.Background(), "not-a-uuid")
		require.Error(t, err)
		require.Nil(t, client.req)
	})

	t.Run("invalid returned profile fails the whole lookup", func(t *testing.T) {
		client := &accountProfileIDsClientForTest{resp: &userv1.ListProfileIDsForAccountResponse{
			ProfileIds: []string{uuid.NewString(), "not-a-uuid"},
		}}
		adapter := &UserGRPCAccountProfiles{Client: client}

		_, err := adapter.ListProfileIDsForAccount(context.Background(), uuid.NewString())
		require.Error(t, err)
	})

	t.Run("upstream error is preserved", func(t *testing.T) {
		want := errors.New("user unavailable")
		client := &accountProfileIDsClientForTest{err: want}
		adapter := &UserGRPCAccountProfiles{Client: client}

		_, err := adapter.ListProfileIDsForAccount(context.Background(), uuid.NewString())
		require.ErrorIs(t, err, want)
	})
}
