package timeout

import (
	"context"
	"errors"
	"voice/backend/matchmaking/internal/mmadmission"
)

// RunNativeOnce preserves timeout/recovery intent transitions, while positive
// Redis requeue belongs exclusively to the qualified whole-party projector.
// No native queue RPC runs inside ApplyPendingRecoveryEffects SQL row locks.
func (s *Sweeper) RunNativeOnce(ctx context.Context) error {
	if s == nil || s.Matches == nil || s.Matches.NativeAdmission == nil || s.Sessions == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("native search timeout runtime unavailable")
	}
	if binding := mmadmission.WireBinding(*s.Matches.NativeAdmission); binding.Validate() != nil {
		return errors.New("native search timeout runtime unavailable")
	}
	return s.runOnce(ctx, true)
}
