package grpcsvc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func (s *BotGRPC) applyPurgeNative(ctx context.Context, claim *store.BotPurgeClaim, bot, operation uuid.UUID) (bool, error) {
	effect, err := s.Store.SealBotPurgeNativeRequest(ctx, claim, bot, operation)
	if err != nil {
		return false, err
	}
	if len(effect.ReceiptBytes) > 0 {
		return false, nil
	} // store revalidated the own typed result
	if s.InstallationSpace == nil || s.PrincipalIssuer == nil {
		return false, status.Error(codes.Unavailable, "protected P3 native reconciliation unavailable")
	}
	q := new(spacev1.ReconcileBotPurgeInstallationRequest)
	if proto.Unmarshal(effect.RequestBytes, q) != nil {
		return false, store.ErrSpaceLifecycleConflict
	}
	hash, err := principal.RequestHash(q)
	if err != nil {
		return false, err
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName, RequestID: q.OperationId, RequestHash: hash})
	if err != nil {
		return false, status.Error(codes.Unavailable, "Bot P3 native principal unavailable")
	}
	call, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", q.OperationId))
	r, err := s.InstallationSpace.ReconcileBotPurgeInstallation(call, q)
	if err != nil {
		return true, err
	}
	if store.ValidateBotPurgeNativeReceipt(q, r) != nil {
		return true, status.Error(codes.Unavailable, "native P3 result does not match its sealed source")
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	if err != nil {
		return true, err
	}
	return true, s.Store.SaveBotPurgeNativeReceipt(ctx, claim, bot, operation, effect.RequestBytes, body)
}

func (s *BotGRPC) processPurgeNativeClaim(ctx context.Context, claim *store.BotPurgeClaim) error {
	calls := 0
	for _, source := range claim.Sources() {
		requests, err := claim.NativeDrainRequests(source.BotID)
		if err != nil {
			return err
		}
		for _, q := range requests {
			if calls >= 8 {
				return store.ErrDeletionPending
			}
			op, err := uuid.Parse(q.OperationId)
			if err != nil {
				return store.ErrSpaceLifecycleConflict
			}
			called, err := s.applyPurgeNative(ctx, claim, source.BotID, op)
			if called {
				calls++
			}
			if err != nil {
				return err
			}
		}
	}
	// Own P3-native outcomes do not by themselves retire local accepted work.
	// The next explicit atomic coverage/adoption transition still owns that step.
	return store.ErrDeletionPending
}
