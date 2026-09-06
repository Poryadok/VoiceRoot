package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"voice/backend/pkg/correlation"

	userv1 "voice.app/voice/user/v1"
)

type lifecycleOwnerClientForTest struct {
	ctx  context.Context
	req  *userv1.ResolveAccountIDForProfileRequest
	resp *userv1.ResolveAccountIDForProfileResponse
	err  error
}

func (c *lifecycleOwnerClientForTest) ResolveAccountIDForProfile(
	ctx context.Context,
	req *userv1.ResolveAccountIDForProfileRequest,
	_ ...grpc.CallOption,
) (*userv1.ResolveAccountIDForProfileResponse, error) {
	c.ctx = ctx
	c.req = req
	return c.resp, c.err
}

func TestUserGRPCLifecycleOwners_UsesFreshChatMetadataAndBoundedDeadline(t *testing.T) {
	profileID, accountID := uuid.New(), uuid.New()
	client := &lifecycleOwnerClientForTest{
		resp: &userv1.ResolveAccountIDForProfileResponse{AccountId: accountID.String()},
	}
	adapter := &UserGRPCLifecycleOwners{Client: client}
	viewerID := uuid.New()
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer viewer-token",
		"x-voice-user-id", uuid.NewString(),
		"x-voice-profile-id", viewerID.String(),
		"x-voice-internal-caller", "untrusted-forwarded-caller",
		correlation.GRPCMetadataKey, "request-42",
	))
	polluted := metadata.NewOutgoingContext(incoming, metadata.Pairs(
		"authorization", "Bearer stale-outgoing-token",
		"x-voice-user-id", uuid.NewString(),
		"x-voice-profile-id", uuid.NewString(),
		"x-voice-internal-caller", "stale-outgoing-caller",
		"x-unrelated", "must-not-forward",
	))

	before := time.Now()
	got, err := adapter.AccountIDByProfileID(polluted, profileID)
	require.NoError(t, err)
	require.Equal(t, accountID, got)
	require.NotNil(t, client.req)
	require.Equal(t, profileID.String(), client.req.GetProfileId())

	deadline, ok := client.ctx.Deadline()
	require.True(t, ok)
	require.WithinDuration(t, before.Add(2*time.Second), deadline, 250*time.Millisecond)

	md, ok := metadata.FromOutgoingContext(client.ctx)
	require.True(t, ok)
	require.Equal(t, metadata.MD{
		"x-voice-internal-caller":   {"chat"},
		correlation.GRPCMetadataKey: {"request-42"},
	}, md)
}

func TestUserGRPCLifecycleOwners_OnlyForwardsSingleSafeRequestIDAndPreservesEarlierDeadline(t *testing.T) {
	profileID, accountID := uuid.New(), uuid.New()
	for _, tt := range []struct {
		name           string
		requestIDs     []string
		wantRequestIDs []string
	}{
		{name: "single safe request id", requestIDs: []string{"request-42"}, wantRequestIDs: []string{"request-42"}},
		{name: "missing request id"},
		{name: "unsafe request id", requestIDs: []string{"request id with spaces"}},
		{name: "multiple request ids", requestIDs: []string{"request-42", "request-43"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &lifecycleOwnerClientForTest{resp: &userv1.ResolveAccountIDForProfileResponse{AccountId: accountID.String()}}
			adapter := &UserGRPCLifecycleOwners{Client: client}
			ctx := context.Background()
			if len(tt.requestIDs) > 0 {
				ctx = metadata.NewIncomingContext(ctx, metadata.MD{correlation.GRPCMetadataKey: tt.requestIDs})
			}

			got, err := adapter.AccountIDByProfileID(ctx, profileID)
			require.NoError(t, err)
			require.Equal(t, accountID, got)
			md, ok := metadata.FromOutgoingContext(client.ctx)
			require.True(t, ok)
			require.Equal(t, tt.wantRequestIDs, md.Get(correlation.GRPCMetadataKey))
		})
	}

	client := &lifecycleOwnerClientForTest{resp: &userv1.ResolveAccountIDForProfileResponse{AccountId: accountID.String()}}
	adapter := &UserGRPCLifecycleOwners{Client: client}
	parentDeadline := time.Now().Add(300 * time.Millisecond)
	parent, cancel := context.WithDeadline(context.Background(), parentDeadline)
	t.Cleanup(cancel)
	_, err := adapter.AccountIDByProfileID(parent, profileID)
	require.NoError(t, err)
	gotDeadline, ok := client.ctx.Deadline()
	require.True(t, ok)
	require.WithinDuration(t, parentDeadline, gotDeadline, 50*time.Millisecond)
}

func TestUserGRPCLifecycleOwners_FailsClosedForCanceledAndMalformedResponses(t *testing.T) {
	profileID := uuid.New()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	canceledClient := &lifecycleOwnerClientForTest{}
	canceledAdapter := &UserGRPCLifecycleOwners{Client: canceledClient}
	_, err := canceledAdapter.AccountIDByProfileID(canceled, profileID)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, canceledClient.req, "a canceled snapshot must not make a remote lifecycle lookup")

	for _, tt := range []struct {
		name string
		resp *userv1.ResolveAccountIDForProfileResponse
		err  error
	}{
		{name: "nil response"},
		{name: "missing account id", resp: &userv1.ResolveAccountIDForProfileResponse{}},
		{name: "invalid account id", resp: &userv1.ResolveAccountIDForProfileResponse{AccountId: "not-a-uuid"}},
		{name: "nil account id", resp: &userv1.ResolveAccountIDForProfileResponse{AccountId: uuid.Nil.String()}},
		{name: "upstream error preserved", err: errors.New("user lifecycle unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &lifecycleOwnerClientForTest{resp: tt.resp, err: tt.err}
			adapter := &UserGRPCLifecycleOwners{Client: client}

			got, err := adapter.AccountIDByProfileID(context.Background(), profileID)
			require.Error(t, err)
			require.Equal(t, uuid.Nil, got)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
			}
		})
	}

	for name, adapter := range map[string]*UserGRPCLifecycleOwners{
		"nil adapter":   nil,
		"empty adapter": {},
		"nil client":    {Client: nil},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := adapter.AccountIDByProfileID(context.Background(), profileID)
			require.Error(t, err)
		})
	}

	client := &lifecycleOwnerClientForTest{}
	adapter := &UserGRPCLifecycleOwners{Client: client}
	_, err = adapter.AccountIDByProfileID(context.Background(), uuid.Nil)
	require.Error(t, err)
	require.Nil(t, client.req, "nil profile id must not be sent to User")
}
