package grpcsvc

import (
	"context"

	"github.com/google/uuid"
)

type fakeShadowBanMod struct {
	shadowBanned map[uuid.UUID]bool
}

func (f fakeShadowBanMod) IsShadowBanned(_ context.Context, accountID uuid.UUID) (bool, error) {
	return f.shadowBanned[accountID], nil
}

func (f fakeShadowBanMod) CheckMessageAllowed(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
