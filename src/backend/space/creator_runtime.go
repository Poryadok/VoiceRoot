package main

import (
	"context"
	"errors"
	"time"

	"voice/backend/pkg/principal"
	"voice/backend/space/internal/creatorauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

type creatorReplayStore interface {
	ClaimCreatorPrincipalReplay(context.Context, string, string, time.Time) error
}

// Creator readiness belongs to its own public receiver dependencies, not the
// independent Role-only member-authority listener or its cleanup worker.
func creatorMembershipDependencies(replays creatorReplayStore, auth creatorauthority.Client, issuer *principal.Issuer) lifecycleprincipal.MembershipDependencies {
	if replays == nil || auth == nil || issuer == nil {
		return lifecycleprincipal.MembershipDependencies{}
	}
	checker := &creatorauthority.Checker{Client: auth, Issuer: issuer}
	return lifecycleprincipal.MembershipDependencies{
		CreatorEpoch: func(ctx context.Context, account string, epoch int64) error {
			err := checker.CheckEpoch(ctx, account, epoch)
			if errors.Is(err, creatorauthority.ErrUnavailable) {
				return lifecycleprincipal.ErrDependencyUnavailable
			}
			return err
		},
		CreatorReplay: func(ctx context.Context, issuer, jti string, expiry time.Time) error {
			err := replays.ClaimCreatorPrincipalReplay(ctx, issuer, jti, expiry)
			if errors.Is(err, store.ErrMemberAuthorityUnavailable) {
				return lifecycleprincipal.ErrDependencyUnavailable
			}
			return err
		},
	}
}
