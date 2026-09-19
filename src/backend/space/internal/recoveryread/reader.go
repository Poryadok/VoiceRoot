// Package recoveryread prepares the authenticated A2 frozen-owner read.
// It has no production wiring; activation requires the complete public vertical.
package recoveryread

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/store"
)

type Store interface {
	GetLifecycleRecoverySpace(context.Context, uuid.UUID, uuid.UUID) (*spacev1.Space, error)
}

// Reader requires all cryptographic and revocation dependencies explicitly.
// No legacy metadata or generic pre-populated context principal is trusted.
type Reader struct {
	Store               Store
	KeyResolver         principal.KeyResolver
	ReplayGuard         principal.ReplayGuard
	SessionEpochChecker principal.SessionEpochChecker
	Clock               func() time.Time
}

func (r Reader) GetSpace(ctx context.Context, req *spacev1.GetSpaceRequest) (*spacev1.GetSpaceResponse, error) {
	deny := status.Error(codes.Unauthenticated, "invalid recovery principal")
	if r.KeyResolver == nil || r.ReplayGuard == nil || r.SessionEpochChecker == nil {
		return nil, deny
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid space request")
	}
	spaceID, err := canonicalID(req.GetSpaceId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid space request")
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return nil, deny
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, deny
	}
	verified, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_GetSpace_FullMethodName,
		ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: r.KeyResolver, ReplayGuard: r.ReplayGuard, SessionEpochChecker: r.SessionEpochChecker, Clock: r.Clock,
	})
	if err != nil {
		return nil, deny
	}
	if _, err := canonicalID(verified.AccountID); err != nil {
		return nil, deny
	}
	actor, err := canonicalID(verified.ProfileID)
	if err != nil {
		return nil, deny
	}
	if r.Store == nil {
		return nil, status.Error(codes.Unavailable, "space recovery unavailable")
	}
	projection, err := r.Store.GetLifecycleRecoverySpace(ctx, spaceID, actor)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, status.Error(codes.NotFound, "space not found")
	case errors.Is(err, store.ErrLifecycleStateTransition):
		return nil, status.Error(codes.FailedPrecondition, "space is not frozen")
	case err != nil:
		return nil, status.Error(codes.Unavailable, "space recovery unavailable")
	}
	if !validProjection(projection, req.SpaceId) {
		return nil, status.Error(codes.Unavailable, "space recovery unavailable")
	}
	return &spacev1.GetSpaceResponse{Space: proto.Clone(projection).(*spacev1.Space)}, nil
}

func canonicalID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, errors.New("invalid UUID")
	}
	return id, nil
}

func validProjection(value *spacev1.Space, spaceID string) bool {
	if value == nil || value.Id != spaceID || value.Name == "" {
		return false
	}
	minimal := &spacev1.Space{Id: value.Id, Name: value.Name, DeletionScheduledAt: value.DeletionScheduledAt, PurgeAfter: value.PurgeAfter}
	if !proto.Equal(value, minimal) {
		return false
	}
	if value.DeletionScheduledAt == nil || value.PurgeAfter == nil {
		return value.DeletionScheduledAt == nil && value.PurgeAfter == nil
	}
	return value.DeletionScheduledAt.CheckValid() == nil && value.PurgeAfter.CheckValid() == nil &&
		value.PurgeAfter.AsTime().Equal(value.DeletionScheduledAt.AsTime().Add(7*24*time.Hour))
}
