package admissionauthority

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

const ManageEntryPolicyPermission = "SPACE_MANAGE_SETTINGS"

// SettingsClient is a fixed-purpose consumer, separate from approval42. It
// requires the protected Role transport; no caller selects a permission name.
type SettingsClient struct {
	Role   PermissionClient
	Issuer *principal.Issuer
	Clock  func() time.Time
}
type VerifiedSettingsPermission struct {
	subject                       *VerifiedSubject
	expected                      *authv1.ObserveSpaceAdmissionSubjectRequest
	requestID                     uuid.UUID
	requestSHA256, responseSHA256 [32]byte
	started                       time.Time
	clock                         func() time.Time
}
type SettingsTranscript struct {
	Subject                                          SubjectTranscript
	RequestID                                        uuid.UUID
	RPC                                              string
	RequestSHA256, ResponseSHA256, ObservationSHA256 [32]byte
}

func settingsRequest(r *authv1.ObserveSpaceAdmissionSubjectRequest) bool {
	return validRequest(r) && r.Phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY && r.SubjectRole == authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR
}
func (v *VerifiedSettingsPermission) StartedAt() time.Time {
	if v == nil || v.subject == nil {
		return time.Time{}
	}
	return v.subject.StartedAt()
}
func (v *VerifiedSettingsPermission) Deadline() time.Time {
	if v == nil || v.subject == nil || v.started.IsZero() {
		return time.Time{}
	}
	d := v.started.Add(5 * time.Second)
	if earlier := v.subject.Deadline(); earlier.Before(d) {
		d = earlier
	}
	return d
}
func (v *VerifiedSettingsPermission) Check(expected *authv1.ObserveSpaceAdmissionSubjectRequest) error {
	if v == nil || v.clock == nil || v.subject == nil || v.requestID == uuid.Nil || !settingsRequest(expected) || !proto.Equal(v.expected, expected) {
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
func (v *VerifiedSettingsPermission) Transcript(expected *authv1.ObserveSpaceAdmissionSubjectRequest) (SettingsTranscript, error) {
	if err := v.Check(expected); err != nil {
		return SettingsTranscript{}, err
	}
	subject, err := v.subject.Transcript(expected)
	if err != nil {
		return SettingsTranscript{}, err
	}
	frame := []byte("voice.space.entry-policy-settings-permission-observation.v1\x00")
	frame = append(frame, expected.AdmissionBindingSha256...)
	frame = append(frame, subject.ObservationSHA256[:]...)
	frame = append(frame, v.requestID[:]...)
	frame = append(frame, v.requestSHA256[:]...)
	frame = append(frame, v.responseSHA256[:]...)
	return SettingsTranscript{Subject: subject, RequestID: v.requestID, RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestSHA256: v.requestSHA256, ResponseSHA256: v.responseSHA256, ObservationSHA256: sha256.Sum256(frame)}, nil
}
func (c *SettingsClient) Observe(ctx context.Context, subject *VerifiedSubject, expected *authv1.ObserveSpaceAdmissionSubjectRequest) (*VerifiedSettingsPermission, error) {
	if !settingsRequest(expected) {
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
	request := &rolev1.CheckPermissionRequest{SpaceId: expected.SpaceId, ProfileId: expected.ProfileId, PermissionName: ManageEntryPolicyPermission}
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
	verified := &VerifiedSettingsPermission{subject: subject, expected: expected, requestID: uuid.MustParse(id), requestSHA256: sha256.Sum256(raw), responseSHA256: sha256.Sum256(responseRaw), started: started, clock: clock}
	if err = verified.Check(expected); err != nil {
		return nil, err
	}
	return verified, nil
}
