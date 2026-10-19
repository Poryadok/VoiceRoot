package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type matchSquadReceiptMode struct {
	dispatch  bool
	abandoned bool
}

// Receipt-only transitions lock match before operation. They never acquire
// party/session locks afterward and perform no external IO.
func lockMatchSquadReceiptMatchTx(ctx context.Context, tx pgx.Tx, matchID uuid.UUID) error {
	var id uuid.UUID
	return tx.QueryRow(ctx, `SELECT id FROM matches WHERE id=$1 FOR UPDATE`, matchID).Scan(&id)
}

func lockMatchSquadAggregatePrefixTx(ctx context.Context, tx pgx.Tx, aggregateID uuid.UUID) error {
	// Discovery takes no row lock. Aggregate identity is permanent; after
	// discovery all participating writers acquire match -> operation first.
	var matchID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT match_id FROM matchmaking_match_squad_teardowns WHERE aggregate_id=$1`, aggregateID).Scan(&matchID); err != nil {
		if err == pgx.ErrNoRows {
			return ErrMatchSquadPending
		}
		return err
	}
	if err := lockMatchSquadReceiptMatchTx(ctx, tx, matchID); err != nil {
		return err
	}
	return tx.QueryRow(ctx, `SELECT match_id FROM matchmaking_match_squad_operations WHERE match_id=$1 FOR UPDATE`, matchID).Scan(&matchID)
}

func readMatchSquadReceiptModeTx(ctx context.Context, tx pgx.Tx, operation MatchSquadProvisioningOperation, provider string) (matchSquadReceiptMode, error) {
	var disposition, dispatch string
	// JSON field access preserves the predecessor store tests/schema behavior;
	// the enabled new coordinator separately requires protocol021 readiness.
	if err := tx.QueryRow(ctx, `SELECT COALESCE(to_jsonb(o)->>'provision_disposition','LEGACY'),
		COALESCE(to_jsonb(o)->>($2||'_dispatch_state'),'LEGACY')
		FROM matchmaking_match_squad_operations o WHERE match_id=$1`, operation.MatchID, provider).Scan(&disposition, &dispatch); err != nil {
		return matchSquadReceiptMode{}, err
	}
	if disposition == "LEGACY" {
		if operation.State != "provisioning" {
			return matchSquadReceiptMode{}, ErrMatchSquadPending
		}
		return matchSquadReceiptMode{}, nil
	}
	if dispatch != "MAY_HAVE_CREATED" && dispatch != "RECEIPT_KNOWN" {
		return matchSquadReceiptMode{}, ErrMatchSquadConflict
	}
	mode := matchSquadReceiptMode{dispatch: true, abandoned: disposition == "ABANDONED"}
	if disposition == "LIVE" && operation.State == "provisioning" {
		return mode, nil
	}
	if mode.abandoned && (operation.State == "compensating" || (operation.State == "closed" && dispatch == "RECEIPT_KNOWN")) {
		return mode, nil
	}
	return matchSquadReceiptMode{}, ErrMatchSquadPending
}
