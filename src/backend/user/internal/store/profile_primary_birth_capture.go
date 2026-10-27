package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"voice/backend/user/internal/identityassociation"
)

func (s *ProfileStore) ensurePrimaryProfileWithBirth(ctx context.Context, account uuid.UUID, profileID *uuid.UUID, displayHint string, guest bool) (*ProfileRow, error) {
	boundary := s.birthCapture
	proof, err := boundary.parents.Verify(ctx, account)
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	if profileID != nil {
		id = *profileID
	}
	prepared, err := boundary.identities.PrepareBirthSelectors(id, account)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < maxDiscriminatorAttempts; attempt++ {
		tx, err := boundary.identities.Begin(ctx)
		if err != nil {
			return nil, err
		}
		// I serializes the lookup with other qualified births. Returning an
		// existing primary must not append another BIRTH or search event.
		primary, err := scanProfile(tx.SQL().QueryRow(ctx, `SELECT `+profileSelectCols+` FROM profiles WHERE account_id=$1 AND is_primary=true`, account))
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = boundary.identities.CreateBirthTx(ctx, tx, prepared, proof)
			if err == nil {
				var created bool
				primary, created, err = s.ensurePrimaryProfileTx(ctx, tx.SQL(), account, &id, displayHint, guest)
				if err == nil && !created {
					err = identityassociation.ErrConflict
				}
			}
			if err == nil {
				event, eventErr := searchProjectionForProfile(ctx, tx.SQL(), primary)
				err = eventErr
				if err == nil {
					err = AppendSearchProjection(ctx, tx.SQL(), event)
				}
			}
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err == nil {
			return primary, nil
		}
		_ = tx.Rollback(context.Background())
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			lastErr = err
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("username/discriminator exhausted: %w", lastErr)
}
