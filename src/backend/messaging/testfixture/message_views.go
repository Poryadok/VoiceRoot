package testfixture

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/messaging/internal/messageviews"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

// CompactProfileViews exercises production authority validation and persistence
// from cross-service integration tests without exposing internal packages.
func CompactProfileViews(ctx context.Context, pool *pgxpool.Pool, client userv1.UserServiceClient, issuer *principal.Issuer) error {
	return (&messageviews.Compactor{Store: &store.MessagesStore{Pool: pool}, Checker: &s2s.UserGRPCProfileDeletion{Client: client, Issuer: issuer}}).Sweep(ctx)
}
