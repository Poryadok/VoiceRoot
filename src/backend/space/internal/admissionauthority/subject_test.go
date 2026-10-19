package admissionauthority

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
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
	"voice/backend/pkg/principal"
)

type subjectRPC func(context.Context, *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error)

func (f subjectRPC) ObserveSpaceAdmissionSubject(ctx context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest, _ ...grpc.CallOption) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
	return f(ctx, r)
}

func subjectRequest() *authv1.ObserveSpaceAdmissionSubjectRequest {
	epoch := uint64(7)
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: uuid.NewString(), ProfileId: uuid.NewString(), SpaceId: uuid.NewString(), AdmissionBindingSha256: make([]byte, 32), InteractiveSessionEpoch: &epoch}
}
func eligibleSubject(r *authv1.ObserveSpaceAdmissionSubjectRequest) *authv1.ObserveSpaceAdmissionSubjectResponse {
	raw, _ := proto.MarshalOptions{Deterministic: true}.Marshal(r)
	hash := sha256.Sum256(raw)
	return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: hash[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 2, ProfileRevision: 3, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}
}

func TestSubjectObservationExactSignedSpaceScopeAndMalformedResponses(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	for _, name := range []string{"valid", "guest", "wrong echo", "wrong account", "wrong profile", "unknown type", "zero account revision", "zero profile revision", "zero floor", "revoked epoch", "unknown decision", "unknown fields", "nil response", "owner unavailable", "owner unauthenticated", "owner denied", "exact two seconds"} {
		t.Run(name, func(t *testing.T) {
			now := time.Now()
			clock := func() time.Time { return now }
			request := subjectRequest()
			client := &Client{Issuer: issuer, Clock: clock, Auth: subjectRPC(func(ctx context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
				md, _ := metadata.FromOutgoingContext(ctx)
				require.Empty(t, md.Get("x-voice-profile-id"))
				transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), md))
				require.NoError(t, err)
				hash, err := principal.RequestHash(r)
				require.NoError(t, err)
				actor, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "auth", ExpectedRPC: authv1.AuthService_ObserveSpaceAdmissionSubject_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
				require.NoError(t, err)
				require.Equal(t, "service:space", actor.Subject)
				response := eligibleSubject(r)
				switch name {
				case "guest":
					response.AccountType = "guest"
				case "wrong echo":
					response.RequestBindingSha256 = make([]byte, 32)
				case "wrong account":
					response.AccountId = uuid.NewString()
				case "wrong profile":
					response.ProfileId = uuid.NewString()
				case "unknown type":
					response.AccountType = "sdk-account"
				case "zero account revision":
					response.AccountSecurityRevision = 0
				case "zero profile revision":
					response.ProfileRevision = 0
				case "zero floor":
					response.SessionEpochFloor = 0
				case "revoked epoch":
					response.SessionEpochFloor = 8
				case "unknown decision":
					response.Decision = authv1.SpaceAdmissionDecision(99)
				case "unknown fields":
					response.ProtoReflect().SetUnknown([]byte{0x78, 1})
				case "nil response":
					return nil, nil
				case "owner unavailable":
					return nil, status.Error(codes.Unavailable, "offline")
				case "owner unauthenticated":
					return nil, status.Error(codes.Unauthenticated, "bad service")
				case "owner denied":
					return nil, status.Error(codes.PermissionDenied, "denied")
				case "exact two seconds":
					now = now.Add(2 * time.Second)
				}
				return response, nil
			})}
			ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString()))
			observed, err := client.Observe(ctx, request)
			if name == "valid" || name == "guest" {
				require.NoError(t, err)
				identity, err := observed.Check(request)
				require.NoError(t, err)
				require.Equal(t, request.AccountId, identity.AccountID.String())
				now = now.Add(5*time.Second - time.Nanosecond)
				_, err = observed.Check(request)
				require.NoError(t, err)
				now = now.Add(time.Nanosecond)
				_, err = observed.Check(request)
				require.ErrorIs(t, err, ErrUnavailable)
			} else if name == "owner denied" {
				require.ErrorIs(t, err, ErrIneligible)
			} else {
				require.ErrorIs(t, err, ErrUnavailable)
			}
		})
	}
}

func TestSubjectObservationRetainsImmutableScopeAcrossCallerAndTransportMutation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	request := subjectRequest()
	expected := proto.Clone(request).(*authv1.ObserveSpaceAdmissionSubjectRequest)
	var transportRequest *authv1.ObserveSpaceAdmissionSubjectRequest
	client := &Client{Issuer: issuer, Auth: subjectRPC(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		transportRequest = r
		return eligibleSubject(r), nil
	})}
	observed, err := client.Observe(context.Background(), request)
	require.NoError(t, err)
	request.ProfileId = uuid.NewString()
	_, err = observed.Check(request)
	require.ErrorIs(t, err, ErrBinding)
	transportRequest.ProfileId = uuid.NewString()
	_, err = observed.Check(expected)
	require.NoError(t, err, "the private scope must not retain the RPC adapter's mutable request pointer")
	transcript, err := observed.Transcript(expected)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, transcript.RequestID)
	require.Equal(t, authv1.AuthService_ObserveSpaceAdmissionSubject_FullMethodName, transcript.RPC)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(expected)
	require.NoError(t, err)
	require.Equal(t, sha256.Sum256(raw), transcript.RequestSHA256)
	require.False(t, observed.StartedAt().IsZero())
	require.Equal(t, observed.StartedAt().Add(5*time.Second), observed.Deadline())
	transcript.RequestSHA256[0] ^= 1
	unchanged, err := observed.Transcript(expected)
	require.NoError(t, err)
	require.Equal(t, sha256.Sum256(raw), unchanged.RequestSHA256, "journal projection cannot mutate the capability")
	second, err := client.Observe(context.Background(), expected)
	require.NoError(t, err)
	newTranscript, err := second.Transcript(expected)
	require.NoError(t, err)
	require.NotEqual(t, unchanged.RequestID, newTranscript.RequestID)
	require.NotEqual(t, unchanged.ObservationSHA256, newTranscript.ObservationSHA256, "a new owner read has a fresh transcript identity even for the same request")
	_, err = (&VerifiedSubject{}).Check(expected)
	require.ErrorIs(t, err, ErrBinding)
}

func TestSubjectObservationValidatesPhaseRoleAndInteractiveEpochBeforeRPC(t *testing.T) {
	for _, phase := range []authv1.SpaceAdmissionPhase{authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH} {
		for _, role := range []authv1.SpaceAdmissionSubjectRole{authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER} {
			r := subjectRequest()
			r.Phase, r.SubjectRole = phase, role
			interactive := phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START || phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ || phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT || (phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE && role == authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER)
			if !interactive {
				r.InteractiveSessionEpoch = nil
			}
			want := !((phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START || phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT) && role != authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT)
			require.Equal(t, want, validRequest(r))
			if interactive {
				r.InteractiveSessionEpoch = nil
			} else {
				epoch := uint64(7)
				r.InteractiveSessionEpoch = &epoch
			}
			require.False(t, validRequest(r))
		}
	}
}
