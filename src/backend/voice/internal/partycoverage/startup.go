package partycoverage

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// CheckStartup must run before any legacy/native fallback decision, including
// when an operator removed the binding configuration after a cutover.
func CheckStartup(ctx context.Context, tx pgx.Tx, binding *RuntimeBinding) error {
	if tx == nil {
		return ErrUnavailable
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('voice_party_coverage_head') IS NOT NULL`).Scan(&exists); err != nil {
		return ErrUnavailable
	}
	if !exists {
		if binding != nil {
			return ErrUnavailable
		}
		return nil
	}
	if binding == nil {
		var begun bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_party_coverage_head)`).Scan(&begun); err != nil || begun {
			return ErrUnavailable
		}
		return nil
	}
	if err := CheckRuntimePrivileges(ctx, tx); err != nil {
		return err
	}
	_, err := ReadCoverage(ctx, tx, *binding)
	return err
}
