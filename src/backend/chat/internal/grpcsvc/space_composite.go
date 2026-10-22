package grpcsvc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/chat/internal/store"
)

// SpaceChatCreator returns success only after the durable operation has
// finalized against Space's exact committed outcome.
type SpaceChatCreator interface {
	Create(context.Context, *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error)
}

type CompositeStore interface {
	PrepareSpaceComposite(context.Context, *chatv1.CreateChatRequest) (*store.SpaceCompositeOperation, error)
	ResolveSpaceComposite(context.Context, uuid.UUID, *spacev1.SpaceChatCreationOutcome) (*store.SpaceCompositeOperation, error)
	ClaimSpaceComposite(context.Context, time.Duration) (*store.SpaceCompositeClaim, error)
	ResolveClaimedSpaceComposite(context.Context, *store.SpaceCompositeClaim, *spacev1.SpaceChatCreationOutcome) (*store.SpaceCompositeOperation, error)
	RetrySpaceComposite(context.Context, *store.SpaceCompositeClaim, time.Duration) error
	ClaimSpaceCompositeEffect(context.Context, time.Duration) (*store.SpaceCompositeEffect, error)
	AckSpaceCompositeEffect(context.Context, *store.SpaceCompositeEffect) error
	RetrySpaceCompositeEffect(context.Context, *store.SpaceCompositeEffect, time.Duration) error
	MinimizeSpaceCompositeEvidence(context.Context, int) (int64, error)
}

type CompositeRemote interface {
	// The implementation uses a fresh, request-bound service:chat principal
	// over the protected Space transport for every attempt, never caller auth.
	Commit(context.Context, *spacev1.SpaceChatCreationBinding) (*spacev1.SpaceChatCreationOutcome, error)
}
type CompositePublicScope interface {
	Check(context.Context, chatcreation.Initiator, *store.SpaceCompositeOperation) error
}
type CompositeEffectSink interface {
	Deliver(context.Context, *store.SpaceCompositeEffect) error
}

type SpaceCompositeCoordinator struct {
	Store   CompositeStore
	Remote  CompositeRemote
	Public  CompositePublicScope
	Effects CompositeEffectSink
	Logger  *slog.Logger
}

func compositeError(err error) error {
	switch {
	case errors.Is(err, store.ErrChatCreateRequestConflict):
		return status.Error(codes.AlreadyExists, "create request conflicts")
	case errors.Is(err, store.ErrSpaceCompositeExpired):
		return status.Error(codes.FailedPrecondition, "COMPOSITE_RESULT_EXPIRED")
	case errors.Is(err, store.ErrSpaceLifecycleState):
		return status.Error(codes.FailedPrecondition, "Space is unavailable")
	case errors.Is(err, chatcreation.ErrInitiator):
		return status.Error(codes.Unauthenticated, "verified initiator required")
	default:
		return status.Error(codes.Unavailable, "composite creation is unresolved")
	}
}

func (c *SpaceCompositeCoordinator) Create(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	actor, err := chatcreation.Creator(ctx, req)
	if err != nil {
		return nil, compositeError(err)
	}
	if c == nil || c.Store == nil || c.Remote == nil || c.Public == nil {
		return nil, status.Error(codes.Unavailable, "composite creation is unavailable")
	}
	op, err := c.Store.PrepareSpaceComposite(ctx, req)
	if err != nil {
		return nil, compositeError(err)
	}
	if op == nil {
		return nil, compositeError(store.ErrSpaceCompositeEvidence)
	}
	if op.State == "PREPARED" {
		outcome, callErr := c.Remote.Commit(ctx, op.Binding)
		if callErr != nil || outcome == nil {
			return nil, status.Error(codes.Unavailable, "composite creation is unresolved")
		}
		operation, parseErr := uuid.Parse(op.Binding.GetOperationId())
		if parseErr != nil {
			return nil, compositeError(store.ErrSpaceCompositeEvidence)
		}
		op, err = c.Store.ResolveSpaceComposite(ctx, operation, outcome)
		if err != nil {
			return nil, compositeError(err)
		}
	}
	if op.State == "ABORTED" {
		return nil, status.Error(op.FailureCode, "Space rejected composite creation")
	}
	if op.State != "COMPLETE" {
		return nil, compositeError(store.ErrSpaceCompositeEvidence)
	}
	// This public check is deliberately after durable rollforward. Losing ACL
	// cannot undo a committed Space result or block autonomous recovery.
	actor, err = chatcreation.Creator(ctx, req)
	if err != nil {
		return nil, compositeError(err)
	}
	if err = c.Public.Check(ctx, actor, op); err != nil {
		return nil, err
	}
	if op.LegacyChat != nil {
		return &chatv1.CreateChatResponse{Chat: chatRowToProto(op.LegacyChat)}, nil
	}
	return op.Response, nil
}

// RecoverOne never runs under the original caller's credential/session. The
// stored immutable initiating fact is bound into a fresh service request.
func (c *SpaceCompositeCoordinator) RecoverOne(ctx context.Context) (bool, error) {
	if c == nil || c.Store == nil || c.Remote == nil {
		return false, status.Error(codes.Unavailable, "composite recovery unavailable")
	}
	claim, err := c.Store.ClaimSpaceComposite(ctx, time.Minute)
	if err != nil || claim == nil {
		return false, err
	}
	outcome, err := c.Remote.Commit(ctx, claim.Operation.Binding)
	if err == nil && outcome != nil {
		_, err = c.Store.ResolveClaimedSpaceComposite(ctx, claim, outcome)
	}
	if err != nil || outcome == nil {
		// Timeout, unknown, malformed evidence and a lost lease all preserve
		// PREPARED. Retry metadata is operational, never an abort decision.
		retryErr := c.Store.RetrySpaceComposite(ctx, claim, time.Second)
		if err != nil {
			return true, err
		}
		return true, retryErr
	}
	return true, nil
}

func (c *SpaceCompositeCoordinator) DeliverOne(ctx context.Context) (bool, error) {
	if c == nil || c.Store == nil || c.Effects == nil {
		return false, status.Error(codes.Unavailable, "composite delivery unavailable")
	}
	effect, err := c.Store.ClaimSpaceCompositeEffect(ctx, time.Minute)
	if err != nil || effect == nil {
		return false, err
	}
	if err = c.Effects.Deliver(ctx, effect); err != nil {
		_ = c.Store.RetrySpaceCompositeEffect(ctx, effect, time.Second)
		return true, err
	}
	return true, c.Store.AckSpaceCompositeEffect(ctx, effect)
}

func (c *SpaceCompositeCoordinator) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	log := c.Logger
	if log == nil {
		log = slog.Default()
	}
	cycles := 0
	for {
		if _, err := c.RecoverOne(ctx); err != nil && ctx.Err() == nil {
			log.Warn("Chat composite recovery unresolved", slog.String("error", err.Error()))
		}
		if _, err := c.DeliverOne(ctx); err != nil && ctx.Err() == nil {
			log.Warn("Chat composite delivery unacknowledged", slog.String("error", err.Error()))
		}
		if cycles%30 == 0 && c.Store != nil {
			if _, err := c.Store.MinimizeSpaceCompositeEvidence(ctx, 10); err != nil && ctx.Err() == nil {
				log.Warn("Chat composite evidence minimization failed", slog.String("error", err.Error()))
			}
		}
		cycles++
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
