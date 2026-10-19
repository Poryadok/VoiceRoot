package admissionauthority

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestSettingsSubjectPurposeIsFiniteAndDoesNotReplaceReviewer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	for _, name := range []string{"settings", "wrong role", "wrong phase", "missing epoch", "zero epoch", "overflow epoch"} {
		t.Run(name, func(t *testing.T) {
			request := subjectRequest()
			request.Phase = authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY
			request.SubjectRole = authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR
			switch name {
			case "wrong role":
				request.SubjectRole = authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER
			case "wrong phase":
				request.Phase = authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ
			case "missing epoch":
				request.InteractiveSessionEpoch = nil
			case "zero epoch":
				v := uint64(0)
				request.InteractiveSessionEpoch = &v
			case "overflow epoch":
				v := uint64(1) << 63
				request.InteractiveSessionEpoch = &v
			}
			calls := 0
			client := &Client{Issuer: issuer, Auth: subjectRPC(func(context.Context, *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
				calls++
				return eligibleSubject(request), nil
			})}
			observed, err := client.Observe(context.Background(), request)
			if name == "settings" {
				require.NoError(t, err)
				_, err = observed.Check(request)
				require.NoError(t, err)
				require.Equal(t, 1, calls)
				require.False(t, reviewerRequest(request), "settings is never review/approval42 authority")
			} else {
				require.ErrorIs(t, err, ErrBinding)
				require.Zero(t, calls)
			}
		})
	}
}

// Controlled responses exercise the concrete signed consumer. Genuine RolePG
// authorization and protected mTLS assembly remain a separate producer proof.
func TestSettingsPermissionExactPurposeAndOwnerBudget(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, mode := range []string{"valid", "guest", "denied", "nil response", "unknown response", "changed body", "two seconds"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now()
			clock := func() time.Time { return now }
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: clock})
			require.NoError(t, err)
			request := subjectRequest()
			request.Phase = authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY
			request.SubjectRole = authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR
			owner := &Client{Issuer: issuer, Clock: clock, Auth: subjectRPC(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
				response := eligibleSubject(r)
				if mode == "guest" {
					response.AccountType = "guest"
				}
				return response, nil
			})}
			subject, err := owner.Observe(context.Background(), request)
			require.NoError(t, err)
			calls := 0
			client := &SettingsClient{Issuer: issuer, Clock: clock, Role: permissionRPC(func(ctx context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
				calls++
				require.Equal(t, ManageEntryPolicyPermission, r.PermissionName)
				require.Equal(t, request.SpaceId, r.SpaceId)
				require.Equal(t, request.ProfileId, r.ProfileId)
				require.True(t, proto.Equal(r, &rolev1.CheckPermissionRequest{PermissionName: ManageEntryPolicyPermission, SpaceId: request.SpaceId, ProfileId: request.ProfileId}), "exact fixed global settings request has no extra selectors")
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
				md, _ := metadata.FromOutgoingContext(ctx)
				require.Empty(t, md.Get("x-voice-profile-id"))
				transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(ctx, md))
				require.NoError(t, err)
				hash, err := principal.RequestHash(r)
				require.NoError(t, err)
				_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckPermission_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock})
				require.NoError(t, err)
				response := &rolev1.CheckPermissionResponse{Allowed: true}
				switch mode {
				case "denied":
					response.Allowed = false
				case "nil response":
					return nil, nil
				case "unknown response":
					response.ProtoReflect().SetUnknown([]byte{0x78, 1})
				case "changed body":
					r.PermissionName = ReviewJoinPermission
				case "two seconds":
					now = now.Add(2 * time.Second)
				}
				return response, nil
			})}
			verified, err := client.Observe(context.Background(), subject, request)
			switch mode {
			case "denied":
				require.ErrorIs(t, err, ErrIneligible)
			case "nil response", "unknown response", "changed body", "two seconds":
				require.ErrorIs(t, err, ErrUnavailable)
			default:
				require.NoError(t, err)
				require.NoError(t, verified.Check(request))
				require.Equal(t, subject.StartedAt(), verified.StartedAt())
				require.Equal(t, subject.Deadline(), verified.Deadline())
				transcript, err := verified.Transcript(request)
				require.NoError(t, err)
				frame := []byte("voice.space.entry-policy-settings-permission-observation.v1\x00")
				frame = append(frame, request.AdmissionBindingSha256...)
				frame = append(frame, transcript.Subject.ObservationSHA256[:]...)
				frame = append(frame, transcript.RequestID[:]...)
				frame = append(frame, transcript.RequestSHA256[:]...)
				frame = append(frame, transcript.ResponseSHA256[:]...)
				require.Equal(t, sha256.Sum256(frame), transcript.ObservationSHA256)
				changed := proto.Clone(request).(*authv1.ObserveSpaceAdmissionSubjectRequest)
				changed.AdmissionBindingSha256[0] ^= 1
				require.ErrorIs(t, verified.Check(changed), ErrBinding)
				_, err = (&ReviewerClient{}).Observe(context.Background(), subject, request)
				require.ErrorIs(t, err, ErrBinding)
				_, err = (&SettingsClient{}).Observe(context.Background(), subject, request)
				require.ErrorIs(t, err, ErrUnavailable)
				now = now.Add(5 * time.Second)
				require.ErrorIs(t, verified.Check(request), ErrUnavailable, "earliest owner budget equality is expired")
			}
			require.Equal(t, 1, calls)
		})
	}
	var empty VerifiedSettingsPermission
	require.ErrorIs(t, empty.Check(subjectRequest()), ErrBinding)
}
