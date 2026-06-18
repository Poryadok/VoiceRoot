package privacy

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrDenied = errors.New("privacy denied")

func CheckAllowed(m Matcher, ctx context.Context, ownerProfile, viewerProfile uuid.UUID, audience Audience, viewerIsGuest bool) error {
	ok, err := m.Allowed(ctx, ownerProfile, viewerProfile, audience, viewerIsGuest)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDenied
	}
	return nil
}
