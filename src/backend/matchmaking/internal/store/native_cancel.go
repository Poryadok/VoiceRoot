package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/matchmaking/internal/mmadmission"
)

func (s *MatchStore) CancelNativeSearch(ctx context.Context, account, profile, sessionID uuid.UUID) error {
	if s == nil || s.Pool == nil || s.NativeAdmission == nil || ctx == nil || account == uuid.Nil || profile == uuid.Nil || sessionID == uuid.Nil {
		return ErrSourcePartyUnavailable
	}
	binding := copyNativeBinding(s.NativeAdmission)
	for range 4 {
		err := s.cancelNativeSearchAttempt(ctx, binding, account, profile, sessionID)
		if errors.Is(err, errVoiceRosterCutChanged) {
			continue
		}
		return err
	}
	return ErrSourcePartyUnavailable
}

func (s *MatchStore) cancelNativeSearchAttempt(ctx context.Context, binding *mmadmission.Binding, account, profile, sessionID uuid.UUID) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer rollbackPartySearch(tx)
	if err = mmadmission.AcquireTerminal(ctx, tx, binding); err != nil {
		return err
	}
	session, party, err := readNativeCancellationSubject(ctx, tx, binding, account, profile, sessionID)
	if err != nil {
		return err
	}
	if session.Status == SessionStatusCancelled {
		return tx.Commit(ctx)
	}
	if session.Status != SessionStatusSearching {
		return ErrSessionNotSearchable
	}
	_, err = resetCapturedRosterPartiesChecked(ctx, tx, map[uuid.UUID]bool{party.ID: true}, func() error {
		current, currentParty, e := readNativeCancellationSubject(ctx, tx, binding, account, profile, sessionID)
		if e != nil {
			return e
		}
		if current.PartyID == nil || currentParty.ID != party.ID || current.Status != SessionStatusSearching {
			return ErrSessionNotSearchable
		}
		var count int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='searching'`, party.ID).Scan(&count); e != nil {
			return e
		}
		if count != len(currentParty.MemberProfileIDs) {
			return ErrSourcePartyUnavailable
		}
		return nil
	}, "")
	if err != nil {
		return err
	}
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM search_sessions WHERE id=$1`, sessionID).Scan(&status); err != nil {
		return err
	}
	if status != SessionStatusCancelled {
		return ErrSessionNotSearchable
	}
	return tx.Commit(ctx)
}

func readNativeCancellationSubject(ctx context.Context, tx pgx.Tx, binding *mmadmission.Binding, account, profile, sessionID uuid.UUID) (SearchSession, SourceParty, error) {
	session, err := scanSession(tx.QueryRow(ctx, `SELECT `+sessionSelectCols+` FROM search_sessions WHERE id=$1`, sessionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return SearchSession{}, SourceParty{}, ErrSessionNotFound
	}
	if err != nil || session.ProfileID != profile || session.PartyID == nil {
		return SearchSession{}, SourceParty{}, ErrSourcePartyUnavailable
	}
	party, err := scanSourceParty(tx.QueryRow(ctx, `SELECT `+sourcePartyCols+` FROM parties WHERE id=$1 AND search_operation_id IS NOT NULL`, *session.PartyID))
	if err != nil {
		return SearchSession{}, SourceParty{}, ErrSourcePartyUnavailable
	}
	bound := false
	for _, member := range party.MemberAuthorities {
		if member.AccountID == account && member.ProfileID == profile {
			bound = true
		}
	}
	if !bound {
		return SearchSession{}, SourceParty{}, ErrSourcePartyUnavailable
	}
	if _, err = readCandidateOriginsMode(ctx, tx, binding, []SourceParty{party}, true); err != nil {
		return SearchSession{}, SourceParty{}, ErrSourcePartyUnavailable
	}
	return session, party, nil
}
