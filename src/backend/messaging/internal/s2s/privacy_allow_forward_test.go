package s2s

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
)

type privacyAllowForwardClient struct {
	userv1.UserServiceClient
	resp *userv1.GetPrivacySettingsResponse
	err  error
}

func (c *privacyAllowForwardClient) GetPrivacySettings(context.Context, *userv1.GetPrivacySettingsRequest, ...grpc.CallOption) (*userv1.GetPrivacySettingsResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.resp, nil
}

func TestGRPCUserPrivacy_AllowForward_defaultsTrueWhenUnset(t *testing.T) {
	u := &GRPCUserPrivacy{Client: &privacyAllowForwardClient{
		resp: &userv1.GetPrivacySettingsResponse{
			PrivacySettings: &userv1.PrivacySettings{},
		},
	}}
	ok, err := u.AllowForward(context.Background(), uuid.New())
	require.NoError(t, err)
	require.True(t, ok, "unset allow_forward must default true (privacy.md)")
}

func TestGRPCUserPrivacy_AllowForward_false(t *testing.T) {
	u := &GRPCUserPrivacy{Client: &privacyAllowForwardClient{
		resp: &userv1.GetPrivacySettingsResponse{
			PrivacySettings: &userv1.PrivacySettings{
				AllowForward: proto.Bool(false),
			},
		},
	}}
	ok, err := u.AllowForward(context.Background(), uuid.New())
	require.NoError(t, err)
	require.False(t, ok)
}

func TestGRPCUserPrivacy_AllowForward_nilClientFailsOpen(t *testing.T) {
	u := &GRPCUserPrivacy{}
	ok, err := u.AllowForward(context.Background(), uuid.New())
	require.NoError(t, err)
	require.True(t, ok)
}

func TestGRPCUserPrivacy_ShowReadReceipts_defaultsTrueWhenUnset(t *testing.T) {
	u := &GRPCUserPrivacy{Client: &privacyAllowForwardClient{
		resp: &userv1.GetPrivacySettingsResponse{PrivacySettings: &userv1.PrivacySettings{}},
	}}
	ok, err := u.ShowReadReceipts(context.Background(), uuid.New())
	require.NoError(t, err)
	require.True(t, ok, "older unset settings keep the documented true default")
}

func TestGRPCUserPrivacy_ShowReadReceipts_false(t *testing.T) {
	u := &GRPCUserPrivacy{Client: &privacyAllowForwardClient{
		resp: &userv1.GetPrivacySettingsResponse{PrivacySettings: &userv1.PrivacySettings{
			ShowReadReceipts: proto.Bool(false),
		}},
	}}
	ok, err := u.ShowReadReceipts(context.Background(), uuid.New())
	require.NoError(t, err)
	require.False(t, ok)
}
