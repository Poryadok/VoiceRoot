package matchsquad

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/voice/internal/gameprovision"
)

// NewCutoverWorkStore supplies the fixed OWNED_ROOM source contract. It is
// separate from the serving MatchSquad provider: no issuer, create/refresh or
// recovery-forward effects are installed by this maintenance-only adapter.
// A sealed work reference does not claim teardown or retirement completion.
func NewCutoverWorkStore(pool *pgxpool.Pool, authority gameprovision.CutoverMaintenanceAuthority) *gameprovision.CutoverWorkStore {
	return &gameprovision.CutoverWorkStore{Pool: pool, Authority: authority, SourceKind: gameprovision.OwnedRoomSource}
}
