package evidence

import (
	"context"
	"voice/backend/moderation/internal/automod"
	"voice/backend/pkg/automodwire"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/socialprincipal"
)

type LifecycleService struct {
	automodv1.UnimplementedAccountLifecycleReceiverServiceServer
	Store *automod.Store
	Gate  *automod.BootGate
}

func (s *LifecycleService) ApplyLifecycleFact(ctx context.Context, request *automodv1.ApplyLifecycleFactRequest) (*automodv1.ApplyLifecycleFactResponse, error) {
	if err := socialprincipal.RequireAutoModEvidence(ctx, automodv1.AccountLifecycleReceiverService_ApplyLifecycleFact_FullMethodName, request); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, mapError(automod.ErrBootClosed)
	}
	check, err := s.Gate.AcquirePrivateRepair(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	if automodwire.RejectUnknown(request) != nil || automodwire.ValidateLifecycleFact(request.GetFact()) != nil {
		return nil, invalid()
	}
	response, err := s.Store.ApplyLifecycleFact(ctx, request.GetFact(), check)
	if err != nil {
		return nil, mapError(err)
	}
	return response, nil
}
