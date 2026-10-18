package outboxdelivery

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaudit"
)

type auditRPC interface {
	AppendAuditEvent(context.Context, *spacev1.AppendAuditEventRequest, ...grpc.CallOption) (*spacev1.AppendAuditEventResponse, error)
}

// SignedAuditSink is the Role-only protected ingestion client. Transport TLS is
// established by the caller; each immutable fact gets a fresh service credential.
type SignedAuditSink struct {
	Client auditRPC
	Issuer *principal.Issuer
}

var ErrAuditPayloadInvalid = errors.New("invalid frozen audit request")

func (s *SignedAuditSink) Append(ctx context.Context, eventID uuid.UUID, payload []byte) error {
	if s == nil || s.Client == nil || s.Issuer == nil || ctx == nil {
		return errors.New("signed audit sink is not configured")
	}
	var request spacev1.AppendAuditEventRequest
	if eventID == uuid.Nil || len(payload) == 0 || proto.Unmarshal(payload, &request) != nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return ErrAuditPayloadInvalid
	}
	parsedID, err := uuid.Parse(request.GetAuditEventId())
	if err != nil || parsedID != eventID {
		return ErrAuditPayloadInvalid
	}
	event := spaceaudit.ReplayEvent{
		AuditEventID: request.GetAuditEventId(), SpaceID: request.GetSpaceId(), ActorProfileID: request.GetActorProfileId(),
		Action: request.GetAction(), TargetType: request.GetTargetType(), TargetID: request.GetTargetId(),
		DetailsJSON: request.GetDetailsJson(), OccurredAt: request.GetOccurredAt(),
	}
	if err := spaceaudit.ValidateEvent("role", event); err != nil {
		return errors.Join(ErrAuditPayloadInvalid, err)
	}
	hash, err := principal.RequestHash(&request)
	if err != nil {
		return err
	}
	token, err := s.Issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_AppendAuditEvent_FullMethodName, RequestID: eventID.String(), RequestHash: hash})
	if err != nil {
		return err
	}
	// Replace inherited metadata, including stale credentials/raw identity. The
	// context retains cancellation/deadline without acquiring caller authority.
	callCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", eventID.String()))
	response, err := s.Client.AppendAuditEvent(callCtx, &request)
	if err != nil {
		return err
	}
	if response == nil {
		return errors.New("audit append returned no acknowledgement")
	}
	// Empty response accepts unknown fields for forward-compatible ACKs.
	return nil
}
