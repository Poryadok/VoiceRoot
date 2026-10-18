package reporttargetclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"testing"
	commonv1 "voice.app/voice/common/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/reporttargetprincipal"
)

type reportOwnerInvoker func(context.Context, string, any, any, ...grpc.CallOption) error

func (f reportOwnerInvoker) Invoke(ctx context.Context, method string, in, out any, options ...grpc.CallOption) error {
	return f(ctx, method, in, out, options...)
}
func (reportOwnerInvoker) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("unexpected streaming")
}
func TestOwnerClientExactBindingAndMalformedResponses(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "moderation", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	report, target := uuid.New(), uuid.New()
	seen := map[string]bool{}
	for _, fixture := range []struct {
		name    string
		state   commonv1.ReportTargetState
		account string
		valid   bool
	}{
		{"live", commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE, uuid.NewString(), true},
		{"missing", commonv1.ReportTargetState_REPORT_TARGET_STATE_MISSING, "", true},
		{"deleted", commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED, "", true},
		{"unspecified", commonv1.ReportTargetState_REPORT_TARGET_STATE_UNSPECIFIED, "", false},
		{"unknown", commonv1.ReportTargetState(77), "", false},
		{"missing_account", commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE, "", false},
		{"malformed_account", commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE, "bad", false},
		{"deleted_account", commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED, uuid.NewString(), false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			conn := reportOwnerInvoker(func(ctx context.Context, method string, in, out any, _ ...grpc.CallOption) error {
				request := in.(*userv1.ResolveReportTargetRequest)
				require.Equal(t, report.String(), request.ReportId)
				require.Equal(t, target.String(), request.ProfileId)
				require.Equal(t, reporttargetprincipal.Method("user"), method)
				md, ok := metadata.FromOutgoingContext(ctx)
				require.True(t, ok)
				require.Len(t, md, 2)
				transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), md))
				require.NoError(t, err)
				require.False(t, seen[transport.RequestID])
				seen[transport.RequestID] = true
				hash, err := principal.RequestHash(in.(proto.Message))
				require.NoError(t, err)
				verified, err := principal.VerifyService(context.Background(), transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "moderation", ExpectedAudience: "user", ExpectedRPC: method, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
				require.NoError(t, err)
				require.Equal(t, "service:moderation", verified.Subject)
				response := out.(*userv1.ResolveReportTargetResponse)
				response.State = fixture.state
				response.AccountId = fixture.account
				return nil
			})
			client := &Client{Issuer: issuer, Connections: map[string]grpc.ClientConnInterface{"user": conn}}
			state, account, err := client.Resolve(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString())), report, "user", target)
			if fixture.valid {
				require.NoError(t, err)
				require.NotEmpty(t, state)
				if state == "LIVE" {
					require.Equal(t, fixture.account, account.String())
				} else {
					require.Equal(t, uuid.Nil, account)
				}
			} else {
				require.Error(t, err)
				require.Empty(t, state)
				require.Equal(t, uuid.Nil, account)
			}
		})
	}
}
