// Package testfixture exposes actual User entrypoints for isolated cross-service
// acceptance. Callers supply private databases and authenticated transports.
package testfixture

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"voice/backend/user/internal/grpcsvc"
	"voice/backend/user/internal/store"
)

// RegisterMessagingDeletionStates registers the production protected Messaging
// entrypoint. The registrar must use the production Phase-0 verifier and TLS.
func RegisterMessagingDeletionStates(registrar grpc.ServiceRegistrar, pool *pgxpool.Pool) {
	grpcsvc.RegisterMessagingScheduledPresenceServer(registrar, &grpcsvc.UserGRPC{Profiles: store.NewProfileStore(pool)})
}
