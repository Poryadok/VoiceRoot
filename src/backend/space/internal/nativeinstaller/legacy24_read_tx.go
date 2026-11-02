package nativeinstaller

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Read-only snapshot cleanup is required even when the original context was
// cancelled. A transaction already closed by Commit needs no rollback.
func finishLegacy24Read(tx pgx.Tx, result *error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		*result = errors.Join(*result, errors.New("HOLD: legacy24 read snapshot cleanup failed"))
	}
}
