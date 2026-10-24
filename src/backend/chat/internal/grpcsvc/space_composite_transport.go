package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
	Chats  interface {
		FindChatByID(context.Context, uuid.UUID) (*store.ChatRow, error)
	}
}

// Check performs a new owner-side public disclosure decision on every call.
// The current actor never replaces the immutable initiating creation facts.
func (r *SignedSpaceCompositeRemote) Check(ctx context.Context, actor chatcreation.Initiator, op *store.SpaceCompositeOperation) error {
	if r == nil || r.Client == nil || op == nil || !chatcreation.ValidBinding(op.Binding) {
		return status.Error(codes.Unavailable, "composite public authority unavailable")
	}
	current := &spacev1.SpaceChatCreationPublicActor{ProfileId: actor.ProfileID.String()}
	if actor.User != nil {
		current.Initiator = &spacev1.SpaceChatCreationPublicActor_User{User: proto.Clone(actor.User).(*spacev1.SpaceChatCreationUser)}
	} else if actor.Bot != nil {
		current.Initiator = &spacev1.SpaceChatCreationPublicActor_Bot{Bot: proto.Clone(actor.Bot).(*spacev1.SpaceChatCreationBot)}
	} else {
		return status.Error(codes.Unauthenticated, "verified public actor required")
	}
	req := &spacev1.GetSpaceChatCreationOutcomeRequest{SpaceId: op.Binding.SpaceId, OperationId: op.Binding.OperationId, PublicActor: current}
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	signed, err := ownCompositePrincipal(call, r.Issuer, spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName, req.OperationId, req)
	if err != nil {
		return status.Error(codes.Unavailable, "composite public credential unavailable")
	}
	out, err := r.Client.GetSpaceChatCreationOutcome(signed, req)
	if err != nil {
		return err
	}
	if out == nil || chatcreation.HasUnknown(out) || out.Outcome == nil || !proto.Equal(out.Outcome.Binding, op.Binding) || out.Outcome.State != spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED {
		return status.Error(codes.Unavailable, "composite public evidence conflicts")
	}
	// Space owns its admission flag; Chat independently owns the current chat
	// flag. A saved create response cannot establish today's guest access.
	if actor.User != nil && actor.User.AccountType == "guest" {
		if r.Chats == nil {
			return status.Error(codes.Unavailable, "Chat guest authority unavailable")
		}
		row, err := r.Chats.FindChatByID(ctx, uuid.MustParse(op.Binding.ChatId))
		if err != nil {
			return status.Error(codes.Unavailable, "Chat guest authority unavailable")
		}
		if row == nil || row.SpaceID == nil || row.SpaceID.String() != op.Binding.SpaceId || !row.AllowGuests {
			return status.Error(codes.NotFound, "composite outcome not found")
		}
	}
	return nil
}

func ownCompositePrincipal(ctx context.Context, issuer *principal.Issuer, method, id string, request proto.Message) (context.Context, error) {
	if issuer == nil {
		return nil, errors.New("chat signing issuer unavailable")
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
			return errors.New("chat created publisher unavailable")
		}
		return s.Created.PublishChatCreatedBytes(ctx, e.EventID.String(), e.Payload)
	case "AUDIT":
		if s.Audit == nil {
			return errors.New("space audit receiver unavailable")
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
			return errors.New("space audit acknowledgement absent")
		}
		return nil
	default:
		return store.ErrSpaceCompositeEvidence
	}
}
