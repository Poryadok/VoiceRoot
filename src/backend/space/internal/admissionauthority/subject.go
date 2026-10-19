// Package admissionauthority consumes current Auth-owned admission observations.
package admissionauthority

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

var (
	ErrUnavailable = errors.New("admission owner authority unavailable")
	ErrIneligible  = errors.New("admission subject denied")
	ErrBinding     = errors.New("admission observation binding invalid")
)

type AuthClient interface {
	ObserveSpaceAdmissionSubject(context.Context, *authv1.ObserveSpaceAdmissionSubjectRequest, ...grpc.CallOption) (*authv1.ObserveSpaceAdmissionSubjectResponse, error)
}

// Client requires the protected Auth mTLS connection. There is no ordinary,
// cached, Redis-only, SDK or Role/Voice floor fallback.
type Client struct {
	Auth   AuthClient
	Issuer *principal.Issuer
	Clock  func() time.Time
}

type SubjectIdentity struct {
	AccountID, ProfileID                                        uuid.UUID
	AccountType                                                 string
	AccountSecurityRevision, SessionEpochFloor, ProfileRevision uint64
}

// VerifiedSubject cannot be deserialized or constructed with public fields.
// It proves one current owner observation, not membership, reviewer ACL,
// immutable profile lifetime, or a native publication continuation.
type VerifiedSubject struct {
	request                       *authv1.ObserveSpaceAdmissionSubjectRequest
	identity                      SubjectIdentity
	requestID                     uuid.UUID
	requestSHA256, responseSHA256 [32]byte
	started                       time.Time
	clock                         func() time.Time
}

// SubjectTranscript is diagnostic historical evidence, never a replayable
// eligibility permit. All fields are copies derived from an actual checked RPC.
type SubjectTranscript struct {
	RequestID                                        uuid.UUID
	RPC                                              string
	RequestSHA256, ResponseSHA256, ObservationSHA256 [32]byte
	Identity                                         SubjectIdentity
}

// StartedAt and Deadline are local monotonic instants for this process only.
// Persisting either does not preserve authority across restart.
func (v *VerifiedSubject) StartedAt() time.Time {
	if v == nil {
		return time.Time{}
	}
	return v.started
}

func (v *VerifiedSubject) Transcript(expected *authv1.ObserveSpaceAdmissionSubjectRequest) (SubjectTranscript, error) {
	identity, err := v.Check(expected)
	if err != nil {
		return SubjectTranscript{}, err
	}
	if v.requestID == uuid.Nil {
		return SubjectTranscript{}, ErrBinding
	}
	// The domain fixes service:space -> Auth.ObserveSpaceAdmissionSubject v1.
	framed := []byte("voice.space.admission-subject-observation.v1\x00")
	framed = append(framed, v.requestID[:]...)
	framed = append(framed, v.requestSHA256[:]...)
	framed = append(framed, v.responseSHA256[:]...)
	return SubjectTranscript{RequestID: v.requestID, RPC: authv1.AuthService_ObserveSpaceAdmissionSubject_FullMethodName, RequestSHA256: v.requestSHA256, ResponseSHA256: v.responseSHA256, ObservationSHA256: sha256.Sum256(framed), Identity: identity}, nil
}

func validID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}

func validRequest(r *authv1.ObserveSpaceAdmissionSubjectRequest) bool {
	if r == nil || len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || !validID(r.AccountId) || !validID(r.ProfileId) || !validID(r.SpaceId) || len(r.AdmissionBindingSha256) != 32 {
		return false
	}
	if r.SubjectRole != authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT && r.SubjectRole != authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER {
		return false
	}
	interactive := false
	switch r.Phase {
	case authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT:
		if r.SubjectRole != authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT {
			return false
		}
		interactive = true
	case authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ:
		interactive = true
	case authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE:
		interactive = r.SubjectRole == authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER
	case authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH:
	default:
		return false
	}
	return interactive == (r.InteractiveSessionEpoch != nil) && (r.InteractiveSessionEpoch == nil || (r.GetInteractiveSessionEpoch() > 0 && r.GetInteractiveSessionEpoch() <= math.MaxInt64))
}

// Deadline is a local monotonic bound for combining independent owner/Role
// observations. Check remains mandatory; a timestamp alone is not authority.
func (v *VerifiedSubject) Deadline() time.Time {
	if v == nil || v.started.IsZero() {
		return time.Time{}
	}
	return v.started.Add(5 * time.Second)
}

// Check binds the observation to a freshly reconstructed server-owned context.
// Call again after local lock waits. The earliest required observation must
// also bound the final decision when multiple owners participate.
func (v *VerifiedSubject) Check(expected *authv1.ObserveSpaceAdmissionSubjectRequest) (SubjectIdentity, error) {
	if v == nil || v.clock == nil || v.request == nil || !validRequest(expected) || !proto.Equal(v.request, expected) {
		return SubjectIdentity{}, ErrBinding
	}
	now := v.clock()
	if v.started.IsZero() || now.Before(v.started) || !now.Before(v.Deadline()) {
		return SubjectIdentity{}, ErrUnavailable
	}
	return v.identity, nil
}

func (c *Client) Observe(ctx context.Context, request *authv1.ObserveSpaceAdmissionSubjectRequest) (*VerifiedSubject, error) {
	if !validRequest(request) {
		return nil, ErrBinding
	}
	if c == nil || c.Auth == nil || c.Issuer == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	request = proto.Clone(request).(*authv1.ObserveSpaceAdmissionSubjectRequest)
	clock := c.Clock
	if clock == nil {
		clock = time.Now
	}
	started := clock()
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, ErrBinding
	}
	expectedEcho := sha256.Sum256(raw)
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, ErrBinding
	}
	id := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: authv1.AuthService_ObserveSpaceAdmissionSubject_FullMethodName, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, ErrUnavailable
	}
	ownerCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ownerCtx = metadata.NewOutgoingContext(ownerCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	wireRequest := proto.Clone(request).(*authv1.ObserveSpaceAdmissionSubjectRequest)
	response, err := c.Auth.ObserveSpaceAdmissionSubject(ownerCtx, wireRequest)
	now := clock()
	if ownerCtx.Err() != nil || now.Before(started) || !now.Before(started.Add(2*time.Second)) || !proto.Equal(wireRequest, request) {
		return nil, ErrUnavailable
	}
	if err != nil {
		// A denial is not authority to retire a native candidate. Service caller
		// configuration and subject decisions may both use coarse status codes.
		if status.Code(err) == codes.PermissionDenied {
			return nil, ErrIneligible
		}
		return nil, ErrUnavailable
	}
	if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Decision != authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE || !bytes.Equal(response.RequestBindingSha256, expectedEcho[:]) || response.AccountId != request.AccountId || response.ProfileId != request.ProfileId || (response.AccountType != "regular" && response.AccountType != "guest") || response.AccountSecurityRevision == 0 || response.AccountSecurityRevision > math.MaxInt64 || response.ProfileRevision == 0 || response.ProfileRevision > math.MaxInt64 || response.SessionEpochFloor == 0 || response.SessionEpochFloor > math.MaxInt64 {
		return nil, ErrUnavailable
	}
	if request.InteractiveSessionEpoch != nil && request.GetInteractiveSessionEpoch() < response.SessionEpochFloor {
		return nil, ErrUnavailable
	}
	responseRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return nil, ErrUnavailable
	}
	verified := &VerifiedSubject{request: request, identity: SubjectIdentity{AccountID: uuid.MustParse(response.AccountId), ProfileID: uuid.MustParse(response.ProfileId), AccountType: response.AccountType, AccountSecurityRevision: response.AccountSecurityRevision, SessionEpochFloor: response.SessionEpochFloor, ProfileRevision: response.ProfileRevision}, requestID: uuid.MustParse(id), requestSHA256: expectedEcho, responseSHA256: sha256.Sum256(responseRaw), started: started, clock: clock}
	if _, err = verified.Check(request); err != nil {
		return nil, err
	}
	return verified, nil
}
