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
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func (s *BotGRPC) purgeReadContext(ctx context.Context, audience, method, id string, q proto.Message) (context.Context, context.CancelFunc, error) {
	if s.PrincipalIssuer == nil {
		return nil, nil, status.Error(codes.Unavailable, "Bot P3 read principal unavailable")
	}
	hash, err := principal.RequestHash(q)
	if err != nil {
		return nil, nil, err
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, nil, status.Error(codes.Unavailable, "Bot P3 read principal unavailable")
	}
	call, cancel := context.WithTimeout(ctx, 3*time.Second)
	return metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), cancel, nil
}

func (s *BotGRPC) readPurgeHandoff(ctx context.Context, p *commonv1.SpacePurgeRequest) (*store.BotPurgeCoverageProof, error) {
	if s.InstallationSpace == nil {
		return nil, status.Error(codes.Unavailable, "protected P3 handoff source unavailable")
	}
	q := &spacev1.GetBotPurgeHandoffRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding)}
	call, cancel, err := s.purgeReadContext(ctx, "space", spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName, q.RequestId, q)
	if err != nil {
		return nil, err
	}
	defer cancel()
	r, err := s.InstallationSpace.GetBotPurgeHandoff(call, q)
	if err != nil {
		return nil, err
	}
	// Validate mandatory Role and any supplied original Messaging entry before
	// asking for chat membership. This is not native or participant completion.
	return store.ValidateBotPurgeCoverage(store.BotPurgeCoverageProof{Purge: p, HandoffRequest: q, Handoff: r}, nil)
}

func (s *BotGRPC) readPurgeMembership(ctx context.Context, base *store.BotPurgeCoverageProof, chats []string) (store.BotPurgeCoverageProof, error) {
	p := store.BotPurgeCoverageProof{Purge: base.Purge, HandoffRequest: base.HandoffRequest, Handoff: base.Handoff}
	if len(chats) == 0 {
		return p, nil
	}
	if base.Handoff.Messaging == nil {
		return p, store.ErrDeletionPending
	}
	if s.CleanupMessaging == nil {
		return p, status.Error(codes.Unavailable, "protected P3 manifest coverage unavailable")
	}
	q := &messagingv1.GetBotPurgeChatCoverageRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: base.Purge.SpaceId, DeletionOperationId: base.Purge.DeletionOperationId, Generation: base.Purge.Generation, Manifest: proto.Clone(base.Purge.Manifest).(*commonv1.ManifestBinding), ChatIds: append([]string(nil), chats...)}
	call, cancel, err := s.purgeReadContext(ctx, "messaging", messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName, q.RequestId, q)
	if err != nil {
		return p, err
	}
	defer cancel()
	r, err := s.CleanupMessaging.GetBotPurgeChatCoverage(call, q)
	if err != nil {
		return p, err
	}
	p.CoverageRequest, p.Coverage = q, r
	return p, nil
}

func (s *BotGRPC) processPurgeObservationClaim(ctx context.Context, claim *store.BotPurgeClaim) error {
	// Persist after each exact <=1000-chat batch. Large finite sources therefore
	// resume without repeating already accepted reads or imposing a product cap.
	reads := 0
	for _, source := range claim.Sources() {
		proofs, err := claim.CoverageObservation(source.BotID)
		if err != nil {
			return err
		}
		batches, err := claim.ChatBatches(source.BotID)
		if err != nil {
			return err
		}
		if len(proofs) == len(batches) {
			continue
		}
		var base *store.BotPurgeCoverageProof
		if len(proofs) > 0 {
			base = &proofs[0]
		} else {
			base, err = s.readPurgeHandoff(ctx, claim.PurgeRequest())
			if err != nil {
				return err
			}
			reads++
		}
		for len(proofs) < len(batches) {
			if reads >= 8 {
				return store.ErrDeletionPending
			}
			p, err := s.readPurgeMembership(ctx, base, batches[len(proofs)])
			if err != nil {
				return err
			}
			reads++
			proofs = append(proofs, p)
			if err = s.Store.SaveBotPurgeCoverageObservation(ctx, claim, source.BotID, proofs); err != nil {
				return err
			}
		}
	}
	// Native drain and explicit journal retirement are separate obligations.
	// Accepted read observations alone never complete the Bot P3 participant.
	return nil
}

func (s *BotGRPC) ProcessNextBotPurgeHandoff(ctx context.Context) (bool, error) {
	if s == nil || s.Store == nil || s.Store.Pool == nil {
		return false, status.Error(codes.Unavailable, "Bot P3 store unavailable")
	}
	claim, err := s.Store.ClaimNextBotPurgeHandoff(ctx)
	if errors.Is(err, store.ErrDeletionClaimLost) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	work, cancel := context.WithTimeout(ctx, 25*time.Second)
	err = s.processPurgeObservationClaim(work, claim)
	if err == nil {
		err = s.processPurgeNativeClaim(work, claim)
	}
	cancel()
	retry, retryCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer retryCancel()
	if e := s.Store.RetryBotPurgeHandoff(retry, claim); e != nil {
		return true, e
	}
	return true, err
}
