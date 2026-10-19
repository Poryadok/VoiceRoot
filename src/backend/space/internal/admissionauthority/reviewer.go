package admissionauthority

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

const ReviewJoinPermission = "SPACE_APPROVE_JOIN_REQUESTS"

type PermissionClient interface {
	CheckPermission(context.Context, *rolev1.CheckPermissionRequest, ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error)
}

// ReviewerClient uses only the protected Role transport and exact Space-level
// permission. It cannot substitute an Owner name, invite/settings permission,
// chat selector, or a persisted historical approval for the online decision.
type ReviewerClient struct {
	Role   PermissionClient
	Issuer *principal.Issuer
	Clock  func() time.Time
}

type ReviewerTranscript struct {
	Subject                                          SubjectTranscript
	RequestID                                        uuid.UUID
	RPC                                              string
	RequestSHA256, ResponseSHA256, ObservationSHA256 [32]byte
}

type VerifiedReviewerPermission struct {
	subject                       *VerifiedSubject
	expected                      *authv1.ObserveSpaceAdmissionSubjectRequest
	requestID                     uuid.UUID
	requestSHA256, responseSHA256 [32]byte
	started                       time.Time
	clock                         func() time.Time
}

func reviewerRequest(request *authv1.ObserveSpaceAdmissionSubjectRequest) bool {
	return validRequest(request) && request.SubjectRole == authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER && (request.Phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ || request.Phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE || request.Phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH)
}

func (v *VerifiedReviewerPermission) StartedAt() time.Time {
	if v == nil || v.subject == nil {
		return time.Time{}
	}
	return v.subject.StartedAt()
}
func (v *VerifiedReviewerPermission) Deadline() time.Time {
	if v == nil || v.subject == nil || v.started.IsZero() {
		return time.Time{}
	}
	deadline := v.started.Add(5 * time.Second)
	if earlier := v.subject.Deadline(); earlier.Before(deadline) {
		deadline = earlier
	}
	return deadline
}

// Check is pure and must run after final Space lock waits alongside current
// local reviewer membership. A previous READ or APPROVE proof cannot authorize
// FIRST_PUBLISH even within the same five-second interval.
func (v *VerifiedReviewerPermission) Check(expected *authv1.ObserveSpaceAdmissionSubjectRequest) error {
	if v == nil || v.clock == nil || v.subject == nil || v.requestID == uuid.Nil || !reviewerRequest(expected) || !proto.Equal(v.expected, expected) {
		return ErrBinding
	}
	if _, err := v.subject.Check(expected); err != nil {
		return err
	}
	now := v.clock()
	if v.started.IsZero() || now.Before(v.started) || !now.Before(v.Deadline()) {
		return ErrUnavailable
	}
	return nil
}

func (v *VerifiedReviewerPermission) Transcript(expected *authv1.ObserveSpaceAdmissionSubjectRequest) (ReviewerTranscript, error) {
	if err := v.Check(expected); err != nil {
		return ReviewerTranscript{}, err
	}
	subject, err := v.subject.Transcript(expected)
	if err != nil {
		return ReviewerTranscript{}, err
	}
	// Domain fixes Space->Role.CheckPermission/SPACE_APPROVE_JOIN_REQUESTS.
	framed := []byte("voice.space.join-review-permission-observation.v1\x00")
	framed = append(framed, subject.ObservationSHA256[:]...)
	framed = append(framed, v.requestID[:]...)
	framed = append(framed, v.requestSHA256[:]...)
	framed = append(framed, v.responseSHA256[:]...)
	return ReviewerTranscript{Subject: subject, RequestID: v.requestID, RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestSHA256: v.requestSHA256, ResponseSHA256: v.responseSHA256, ObservationSHA256: sha256.Sum256(framed)}, nil
}

func (c *ReviewerClient) Observe(ctx context.Context, subject *VerifiedSubject, expected *authv1.ObserveSpaceAdmissionSubjectRequest) (*VerifiedReviewerPermission, error) {
	if !reviewerRequest(expected) {
		return nil, ErrBinding
	}
	expected = proto.Clone(expected).(*authv1.ObserveSpaceAdmissionSubjectRequest)
	if _, err := subject.Check(expected); err != nil {
		return nil, err
	}
	if c == nil || c.Role == nil || c.Issuer == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	clock := c.Clock
	if clock == nil {
		clock = time.Now
	}
	started := clock()
	request := &rolev1.CheckPermissionRequest{SpaceId: expected.SpaceId, ProfileId: expected.ProfileId, PermissionName: ReviewJoinPermission}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, ErrBinding
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, ErrBinding
	}
	id := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, ErrUnavailable
	}
	ownerCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ownerCtx = metadata.NewOutgoingContext(ownerCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	wire := proto.Clone(request).(*rolev1.CheckPermissionRequest)
	response, err := c.Role.CheckPermission(ownerCtx, wire)
	now := clock()
	if err != nil || ownerCtx.Err() != nil || now.Before(started) || !now.Before(started.Add(2*time.Second)) || !proto.Equal(wire, request) || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrUnavailable
	}
	if _, err = subject.Check(expected); err != nil {
		return nil, err
	}
	if !response.Allowed {
		return nil, ErrIneligible
	}
	responseRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return nil, ErrUnavailable
	}
	verified := &VerifiedReviewerPermission{subject: subject, expected: expected, requestID: uuid.MustParse(id), requestSHA256: sha256.Sum256(raw), responseSHA256: sha256.Sum256(responseRaw), started: started, clock: clock}
	if err = verified.Check(expected); err != nil {
		return nil, err
	}
	return verified, nil
}
