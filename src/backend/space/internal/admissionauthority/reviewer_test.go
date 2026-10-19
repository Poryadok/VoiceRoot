package admissionauthority

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type permissionRPC func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)

func (f permissionRPC) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return f(ctx, r)
}

func TestReviewerPermissionExactProtectedDecisionAndEarliestOwnerBudget(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	for _, phase := range []authv1.SpaceAdmissionPhase{authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH} {
		for _, name := range []string{"valid", "denied", "unknown response", "nil response", "unavailable", "service denied", "request mutated", "two seconds"} {
			t.Run(phase.String()+"/"+name, func(t *testing.T) {
				now := time.Now()
				clock := func() time.Time { return now }
				expected := subjectRequest()
				expected.Phase = phase
				expected.SubjectRole = authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER
				if phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH {
					expected.InteractiveSessionEpoch = nil
				}
				subjectClient := &Client{Issuer: issuer, Clock: clock, Auth: subjectRPC(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
					return eligibleSubject(r), nil
				})}
				subject, err := subjectClient.Observe(context.Background(), expected)
				require.NoError(t, err)
				firstStart := now
				now = now.Add(time.Second)
				roleClient := &ReviewerClient{Issuer: issuer, Clock: clock, Role: permissionRPC(func(ctx context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
					require.True(t, proto.Equal(&rolev1.CheckPermissionRequest{SpaceId: expected.SpaceId, ProfileId: expected.ProfileId, PermissionName: "SPACE_APPROVE_JOIN_REQUESTS"}, r), "no chat/voice selectors or alternate permission")
					md, _ := metadata.FromOutgoingContext(ctx)
					require.Empty(t, md.Get("x-voice-profile-id"))
					transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), md))
					require.NoError(t, err)
					hash, err := principal.RequestHash(r)
					require.NoError(t, err)
					actor, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckPermission_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
					require.NoError(t, err)
					require.Equal(t, "service:space", actor.Subject)
					response := &rolev1.CheckPermissionResponse{Allowed: true}
					switch name {
					case "denied":
						response.Allowed = false
					case "unknown response":
						response.ProtoReflect().SetUnknown([]byte{0x78, 1})
					case "nil response":
						return nil, nil
					case "unavailable":
						return nil, status.Error(codes.Unavailable, "offline")
					case "service denied":
						return nil, status.Error(codes.PermissionDenied, "caller config")
					case "request mutated":
						r.ProfileId = uuid.NewString()
					case "two seconds":
						now = now.Add(2 * time.Second)
					}
					return response, nil
				})}
				proof, err := roleClient.Observe(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString())), subject, expected)
				if name == "valid" {
					require.NoError(t, err)
					require.NoError(t, proof.Check(expected))
					require.Equal(t, firstStart, proof.StartedAt())
					require.Equal(t, firstStart.Add(5*time.Second), proof.Deadline())
					transcript, err := proof.Transcript(expected)
					require.NoError(t, err)
					require.NotEqual(t, uuid.Nil, transcript.RequestID)
					require.Equal(t, rolev1.RoleService_CheckPermission_FullMethodName, transcript.RPC)
					changed := proto.Clone(expected).(*authv1.ObserveSpaceAdmissionSubjectRequest)
					changed.AdmissionBindingSha256[0] = 1
					require.ErrorIs(t, proof.Check(changed), ErrBinding)
					if phase != authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH {
						changed = proto.Clone(expected).(*authv1.ObserveSpaceAdmissionSubjectRequest)
						changed.Phase = authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH
						changed.InteractiveSessionEpoch = nil
						require.ErrorIs(t, proof.Check(changed), ErrBinding)
					}
					now = firstStart.Add(5*time.Second - time.Nanosecond)
					require.NoError(t, proof.Check(expected))
					now = firstStart.Add(5 * time.Second)
					require.ErrorIs(t, proof.Check(expected), ErrUnavailable, "Role's later start cannot reset the subject budget")
				} else if name == "denied" {
					require.ErrorIs(t, err, ErrIneligible)
				} else {
					require.ErrorIs(t, err, ErrUnavailable)
				}
			})
		}
	}
	require.ErrorIs(t, (&VerifiedReviewerPermission{}).Check(subjectRequest()), ErrBinding)
	_, err = (&ReviewerClient{}).Observe(context.Background(), nil, subjectRequest())
	require.ErrorIs(t, err, ErrBinding)
	read := subjectRequest()
	read.Phase = authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ
	read.SubjectRole = authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER
	read.InteractiveSessionEpoch = nil
	_, err = (&ReviewerClient{}).Observe(context.Background(), nil, read)
	require.ErrorIs(t, err, ErrBinding, "reviewer READ requires the live caller epoch")
}
