package grpcsvc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaudit"
)

type SignedSpaceCompositeRemote struct {
	Client spacev1.SpaceServiceClient
	Issuer *principal.Issuer
}

func ownCompositePrincipal(ctx context.Context, issuer *principal.Issuer, method, id string, request proto.Message) (context.Context, error) {
	if issuer == nil {
		return nil, errors.New("Chat signing issuer unavailable")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, err
	}
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, err
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), nil
}

func (r *SignedSpaceCompositeRemote) Commit(ctx context.Context, binding *spacev1.SpaceChatCreationBinding) (*spacev1.SpaceChatCreationOutcome, error) {
	if r == nil || r.Client == nil || !chatcreation.ValidBinding(binding) {
		return nil, store.ErrSpaceCompositeEvidence
	}
	req := &spacev1.CommitSpaceChatCreationRequest{Binding: proto.Clone(binding).(*spacev1.SpaceChatCreationBinding)}
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	signed, err := ownCompositePrincipal(call, r.Issuer, spacev1.SpaceService_CommitSpaceChatCreation_FullMethodName, binding.OperationId, req)
	if err != nil {
		return nil, err
	}
	response, err := r.Client.CommitSpaceChatCreation(signed, req)
	if err != nil {
		return nil, err
	}
	if response == nil || response.Outcome == nil || chatcreation.HasUnknown(response) {
		return nil, store.ErrSpaceCompositeEvidence
	}
	// Store repeats full binding/state validation in the finalization TX.
	return response.Outcome, nil
}

type CompositeCreatedPublisher interface {
	PublishChatCreatedBytes(context.Context, string, []byte) error
}
type SignedSpaceCompositeEffects struct {
	Audit   spacev1.SpaceServiceClient
	Issuer  *principal.Issuer
	Created CompositeCreatedPublisher
}

func (s *SignedSpaceCompositeEffects) Deliver(ctx context.Context, e *store.SpaceCompositeEffect) error {
	if s == nil || e == nil || len(e.Payload) == 0 {
		return store.ErrSpaceCompositeEvidence
	}
	switch e.Kind {
	case "CHAT_CREATED":
		if s.Created == nil {
			return errors.New("Chat created publisher unavailable")
		}
		return s.Created.PublishChatCreatedBytes(ctx, e.EventID.String(), e.Payload)
	case "AUDIT":
		if s.Audit == nil {
			return errors.New("Space audit receiver unavailable")
		}
		req := new(spacev1.AppendAuditEventRequest)
		if proto.Unmarshal(e.Payload, req) != nil || chatcreation.HasUnknown(req) || req.AuditEventId != e.EventID.String() {
			return store.ErrSpaceCompositeEvidence
		}
		if err := spaceaudit.ValidateEvent("chat", spaceaudit.ReplayEvent{AuditEventID: req.AuditEventId, SpaceID: req.SpaceId, ActorProfileID: req.ActorProfileId, Action: req.Action, TargetType: req.TargetType, TargetID: req.TargetId, DetailsJSON: req.DetailsJson, OccurredAt: req.OccurredAt}); err != nil {
			return err
		}
		call, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		signed, err := ownCompositePrincipal(call, s.Issuer, spacev1.SpaceService_AppendAuditEvent_FullMethodName, req.AuditEventId, req)
		if err != nil {
			return err
		}
		response, err := s.Audit.AppendAuditEvent(signed, req)
		if err != nil {
			return err
		}
		if response == nil {
			return errors.New("Space audit acknowledgement absent")
		}
		return nil
	default:
		return store.ErrSpaceCompositeEvidence
	}
}
