package s2s

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/privacy"
)

// Keep the generated clients in the path: assertions cover the exact RPC and
// protobuf request sent by the adapters, without depending on external stores.
type privacyContractConn struct {
	invoke func(context.Context, string, any, any) error
}

func (c privacyContractConn) Invoke(ctx context.Context, method string, req, reply any, _ ...grpc.CallOption) error {
	return c.invoke(ctx, method, req, reply)
}

func (privacyContractConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("unexpected stream")
}

func TestPrivacyAudienceContract(t *testing.T) {
	issuer, _ := testSocialIssuer(t)
	target := uuid.New()
	friend := privacy.Audience{Friends: true, SpaceMembers: true, SpaceIDs: []string{uuid.NewString()}}
	phone := privacy.Audience{FriendsOfFriends: true, IncludeGuests: true}
	for _, field := range []string{"friend", "phone"} {
		t.Run(field, func(t *testing.T) {
			for _, code := range []codes.Code{codes.OK, codes.Unavailable, codes.PermissionDenied, codes.DeadlineExceeded} {
				t.Run(code.String(), func(t *testing.T) {
					calls := 0
					conn := privacyContractConn{invoke: func(_ context.Context, method string, req, reply any) error {
						calls++
						require.Equal(t, userv1.UserService_GetPrivacySettings_FullMethodName, method)
						require.Equal(t, target.String(), req.(*userv1.GetPrivacySettingsRequest).GetProfileId())
						if code != codes.OK {
							return status.Error(code, "privacy lookup failed")
						}
						reply.(*userv1.GetPrivacySettingsResponse).PrivacySettings = &userv1.PrivacySettings{
							AllowFriendRequests: privacy.ToProto(friend), AllowPhoneSearch: privacy.ToProto(phone),
						}
						return nil
					}}
					adapter := &GRPCUserPrivacy{Client: userv1.NewUserServiceClient(conn), Issuer: issuer}
					call, want := adapter.AllowFriendRequestsAudience, friend
					if field == "phone" {
						call, want = adapter.AllowPhoneSearchAudience, phone
					}
					got, err := call(context.Background(), target)
					require.Equal(t, 1, calls)
					require.Equal(t, code, status.Code(err))
					if code == codes.OK {
						require.NoError(t, err)
						require.Equal(t, want, got)
					} else {
						require.True(t, got.IsNobody(), "failed lookup must not grant an audience")
					}
				})
			}
		})
	}
}

func TestPrivacyCoMembershipContract(t *testing.T) {
	issuer, _ := testSocialIssuer(t)
	a, b := uuid.New(), uuid.New()
	spaces := []string{uuid.NewString(), uuid.NewString()}
	for _, tc := range []struct {
		name   string
		member bool
		code   codes.Code
	}{
		{"shared", true, codes.OK}, {"not_shared", false, codes.OK},
		{"unavailable", false, codes.Unavailable}, {"denied", false, codes.PermissionDenied},
		{"deadline", false, codes.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			conn := privacyContractConn{invoke: func(_ context.Context, method string, req, reply any) error {
				calls++
				require.Equal(t, spacev1.SpaceService_AreCoMembers_FullMethodName, method)
				request := req.(*spacev1.AreCoMembersRequest)
				require.Equal(t, a.String(), request.GetProfileIdA())
				require.Equal(t, b.String(), request.GetProfileIdB())
				require.Equal(t, spaces, request.GetSpaceIds())
				if tc.code != codes.OK {
					return status.Error(tc.code, "membership lookup failed")
				}
				reply.(*spacev1.AreCoMembersResponse).CoMembers = tc.member
				return nil
			}}
			adapter := &GRPCSpaceCoMembership{Client: spacev1.NewSpaceServiceClient(conn), Issuer: issuer}
			got, err := adapter.AreCoMembers(context.Background(), a, b, spaces)
			require.Equal(t, 1, calls)
			require.Equal(t, tc.code, status.Code(err))
			require.Equal(t, tc.member, got)
		})
	}
}
