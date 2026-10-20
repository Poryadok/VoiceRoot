package store

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrInstallationConflict = errors.New("bot installation operation conflicts with current state")

// InstallationIntent is committed before any remote membership mutation.
// OperationID and identity survive lost responses; CancelOperationID is a
// separate stable operation bound to the original add, never a replacement add.
type InstallationIntent struct {
	OperationID, BotID, InstallationID, SpaceID, ActorProfileID, InstallerProfileID uuid.UUID
	Generation                                                                      int64
	State                                                                           string
	Chats                                                                           []uuid.UUID
	CancelOperationID                                                               uuid.UUID
	LegacyEvidence                                                                  bool
}

const intentColumns = `operation_id,bot_id,installation_id,space_id,actor_profile_id,installed_by_profile_id,generation,state,chat_ids,cancel_operation_id,legacy_evidence`

func scanInstallationIntent(row pgx.Row) (*InstallationIntent, error) {
	var i InstallationIntent
	var chats []byte
	var cancel *uuid.UUID
	err := row.Scan(&i.OperationID, &i.BotID, &i.InstallationID, &i.SpaceID, &i.ActorProfileID, &i.InstallerProfileID, &i.Generation, &i.State, &chats, &cancel, &i.LegacyEvidence)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(chats, &i.Chats); err != nil {
		return nil, err
	}
	if cancel != nil {
		i.CancelOperationID = *cancel
	}
	return &i, nil
}

func normalizedInstallationChats(chats []uuid.UUID) []uuid.UUID {
	result := append([]uuid.UUID{}, chats...)
	sort.Slice(result, func(a, b int) bool { return result[a].String() < result[b].String() })
	return slices.Compact(result)
}

func (s *BotStore) lockInstallation(ctx context.Context, tx pgx.Tx, botID, spaceID uuid.UUID, admitting bool) (*BotRow, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,71431))`, spaceID.String()); err != nil {
		return nil, err
	}
	bot, err := s.GetBotByIDForUpdate(ctx, tx, botID)
	if err != nil {
		return nil, err
	}
	if admitting {
		pending, err := s.pendingUninstallCleanup(ctx, tx, botID, spaceID)
		if err != nil {
			return nil, err
		}
		if pending {
			return nil, ErrInstallationConflict
		}
		if bot.Status != "live" || bot.ActorProfileID == uuid.Nil {
			return nil, ErrInstallationConflict
		}
		var state string
		err = tx.QueryRow(ctx, `SELECT desired_state FROM bot_space_lifecycle_heads WHERE space_id=$1`, spaceID).Scan(&state)
		if err == nil && state != "LIVE" {
			return nil, ErrSpaceLifecycleConflict
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	return bot, nil
}

func (s *BotStore) PrepareInstallation(ctx context.Context, botID, spaceID, installer uuid.UUID, chats []uuid.UUID) (*InstallationIntent, error) {
	if botID == uuid.Nil || spaceID == uuid.Nil || installer == uuid.Nil {
		return nil, ErrInstallationConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bot, err := s.lockInstallation(ctx, tx, botID, spaceID, true)
	if err != nil {
		return nil, err
	}
	i, err := scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE bot_id=$1 AND space_id=$2 AND state<>'CANCELED' FOR UPDATE`, botID, spaceID))
	normalized := normalizedInstallationChats(chats)
	if err == nil {
		if i.ActorProfileID != bot.ActorProfileID || i.State == "CANCELING" || (i.State == "PREPARING" && !slices.Equal(i.Chats, normalized)) {
			return nil, ErrInstallationConflict
		}
		if _, err = ensureInstallationAttachments(ctx, tx, i); err != nil {
			return nil, err
		}
		return i, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	i, err = s.newInstallationIntent(ctx, tx, bot, spaceID, installer, normalized)
	if err != nil {
		return nil, err
	}
	if err = persistInitialInstallationScopes(ctx, tx, i); err != nil {
		return nil, err
	}
	if _, err = ensureInstallationAttachments(ctx, tx, i); err != nil {
		return nil, err
	}
	return i, tx.Commit(ctx)
}

func (s *BotStore) newInstallationIntent(ctx context.Context, tx pgx.Tx, bot *BotRow, spaceID, installer uuid.UUID, chats []uuid.UUID) (*InstallationIntent, error) {
	var previous int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(generation),0) FROM bot_installation_intents WHERE bot_id=$1 AND space_id=$2`, bot.ID, spaceID).Scan(&previous); err != nil {
		return nil, err
	}
	if previous == math.MaxInt64 {
		return nil, ErrInstallationConflict
	}
	installationID := uuid.New()
	// Preserve the actual persisted ID when qualifying a pre-journal installation.
	err := tx.QueryRow(ctx, `SELECT id FROM bot_space_installations WHERE bot_id=$1 AND space_id=$2 FOR UPDATE`, bot.ID, spaceID).Scan(&installationID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	i := &InstallationIntent{OperationID: uuid.New(), BotID: bot.ID, InstallationID: installationID, SpaceID: spaceID, ActorProfileID: bot.ActorProfileID, InstallerProfileID: installer, Generation: previous + 1, State: "PREPARING", Chats: chats}
	i.LegacyEvidence = err == nil
	raw, err := json.Marshal(chats)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_installation_intents (`+intentColumns+`) VALUES($1,$2,$3,$4,$5,$6,$7,'PREPARING',$8::jsonb,NULL,$9)`, i.OperationID, i.BotID, i.InstallationID, i.SpaceID, i.ActorProfileID, i.InstallerProfileID, i.Generation, raw, i.LegacyEvidence)
	return i, err
}

func sameInstallationIdentity(a, b *InstallationIntent) bool {
	return a != nil && b != nil && a.OperationID == b.OperationID && a.BotID == b.BotID && a.InstallationID == b.InstallationID && a.SpaceID == b.SpaceID && a.ActorProfileID == b.ActorProfileID && a.Generation == b.Generation
}

func (s *BotStore) CompleteInstallation(ctx context.Context, want *InstallationIntent, chats []uuid.UUID) error {
	if want == nil {
		return ErrInstallationConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bot, err := s.lockInstallation(ctx, tx, want.BotID, want.SpaceID, true)
	if err != nil {
		return err
	}
	got, err := scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE installation_id=$1 FOR UPDATE`, want.InstallationID))
	if err != nil {
		return err
	}
	if !sameInstallationIdentity(got, want) || bot.ActorProfileID != got.ActorProfileID || (got.State != "PREPARING" && got.State != "INSTALLED") {
		return ErrInstallationConflict
	}
	attachmentIDs, err := ensureInstallationAttachments(ctx, tx, got)
	if err != nil {
		return err
	}
	chats = normalizedInstallationChats(chats)
	if got.State == "PREPARING" && !slices.Equal(got.Chats, chats) {
		return ErrInstallationConflict
	}
	if got.State == "INSTALLED" && slices.Equal(got.Chats, chats) {
		// Exact native acknowledgement recovery must not replace a whitelist
		// subsequently changed through its separate authorized writer.
		return tx.Commit(ctx)
	}
	var rawScopeIDs []byte
	if err = tx.QueryRow(ctx, `SELECT to_jsonb(i)->'chat_scope_observation_ids' FROM bot_installation_intents i WHERE operation_id=$1`, got.OperationID).Scan(&rawScopeIDs); err != nil {
		return err
	}
	var scopeIDs []uuid.UUID
	if len(rawScopeIDs) > 0 {
		if err = json.Unmarshal(rawScopeIDs, &scopeIDs); err != nil {
			return ErrChatScopeObservation
		}
	}
	if got.State == "INSTALLED" || len(scopeIDs) == 0 {
		scopeIDs, err = persistChatScopeObservations(ctx, tx, got.BotID, got.SpaceID, chats, &got.OperationID, "whitelist_admission")
		if err != nil {
			return err
		}
	}
	if len(scopeIDs) > 0 && len(scopeIDs) != len(chats) {
		return ErrChatScopeObservation
	}
	var actual uuid.UUID
	err = tx.QueryRow(ctx, `INSERT INTO bot_space_installations(id,bot_id,space_id,installed_by_profile_id) VALUES($1,$2,$3,$4) ON CONFLICT(bot_id,space_id) DO UPDATE SET installed_by_profile_id=EXCLUDED.installed_by_profile_id RETURNING id`, got.InstallationID, got.BotID, got.SpaceID, got.InstallerProfileID).Scan(&actual)
	if err != nil {
		return err
	}
	if actual != got.InstallationID {
		return ErrInstallationConflict
	}
	var preferred map[uuid.UUID]uuid.UUID
	if got.State == "PREPARING" {
		preferred, err = preferredAttachmentsByChat(ctx, tx, got, attachmentIDs)
		if err != nil {
			return err
		}
	}
	if err := replaceInstallationWhitelist(ctx, tx, got.BotID, got.SpaceID, got.InstallerProfileID, chats, scopeIDs, preferred); err != nil {
		return err
	}
	if err = attachChatScopeObservations(ctx, tx, got.BotID, got.SpaceID, scopeIDs); err != nil {
		return err
	}
	// This acknowledgement qualifies the new exact native request only. A
	// historical legacy source remains a separate provenance/cleanup obligation.
	if _, err = tx.Exec(ctx, `UPDATE bot_installation_intents SET state='INSTALLED',updated_at=now() WHERE installation_id=$1`, got.InstallationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replaceInstallationWhitelist(ctx context.Context, tx pgx.Tx, botID, spaceID, installer uuid.UUID, chats []uuid.UUID, scopeIDs []uuid.UUID, preferred map[uuid.UUID]uuid.UUID) error {
	chats = normalizedInstallationChats(chats)
	scopes, err := chatScopeIDsByChat(ctx, tx, botID, spaceID, scopeIDs)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `DELETE FROM bot_chat_whitelist WHERE bot_id=$1 AND space_id=$2 AND NOT(chat_id=ANY($3::uuid[])) RETURNING chat_id`, botID, spaceID, chats)
	if err != nil {
		return err
	}
	var removed []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if !slices.Contains(chats, id) {
			removed = append(removed, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(removed) > 0 {
		if _, err = tx.Exec(ctx, `UPDATE bot_message_deliveries SET status='canceled',claimed_until=NULL WHERE bot_id=$1 AND chat_id=ANY($2) AND status='pending'`, botID, removed); err != nil {
			return err
		}
	}
	for _, id := range chats {
		if err = insertWhitelistWithAttachment(ctx, tx, botID, spaceID, id, installer, scopes, preferred); err != nil {
			return err
		}
	}
	return nil
}

func (s *BotStore) BeginInstallationCancellation(ctx context.Context, botID, spaceID, installer uuid.UUID) (*InstallationIntent, error) {
	return s.beginInstallationCancellation(ctx, botID, spaceID, installer, nil)
}

func (s *BotStore) BeginInstallationCancellationWithAdmission(ctx context.Context, botID, spaceID, installer uuid.UUID, admission DeletionAdmission) (*InstallationIntent, error) {
	return s.beginInstallationCancellation(ctx, botID, spaceID, installer, &admission)
}

func (s *BotStore) beginInstallationCancellation(ctx context.Context, botID, spaceID, installer uuid.UUID, admission *DeletionAdmission) (*InstallationIntent, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bot, err := s.lockInstallation(ctx, tx, botID, spaceID, false)
	if err != nil {
		return nil, err
	}
	i, err := scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE bot_id=$1 AND space_id=$2 ORDER BY generation DESC LIMIT 1 FOR UPDATE`, botID, spaceID))
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bot_space_installations WHERE bot_id=$1 AND space_id=$2)`, botID, spaceID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, nil
		}
		// A legacy row is captured as a new admitted cancellation intent. This
		// is not proof that its old, unqualified remote add was fenced.
		i, err = s.newInstallationIntent(ctx, tx, bot, spaceID, installer, []uuid.UUID{})
	}
	if err != nil {
		return nil, err
	}
	if i.State == "CANCELED" || i.State == "CANCELING" {
		if i.State == "CANCELING" {
			if _, err = frozenNativeCancelTx(ctx, tx, i, i.CancelOperationID); err != nil {
				return nil, err
			}
		}
		if i.State == "CANCELING" && admission != nil {
			if err = s.ensureUninstallCleanupTx(ctx, tx, i, *admission); err != nil {
				return nil, err
			}
		}
		return i, tx.Commit(ctx)
	}
	rows, err := tx.Query(ctx, `SELECT chat_id FROM bot_chat_whitelist WHERE bot_id=$1 AND space_id=$2 ORDER BY chat_id`, botID, spaceID)
	if err != nil {
		return nil, err
	}
	chats := append([]uuid.UUID{}, i.Chats...)
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		chats = append(chats, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	i.Chats = normalizedInstallationChats(chats)
	i.CancelOperationID, err = frozenNativeCancelTx(ctx, tx, i, uuid.Nil)
	if err != nil {
		return nil, err
	}
	i.State = "CANCELING"
	raw, err := json.Marshal(i.Chats)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_installation_intents SET state='CANCELING',cancel_operation_id=$2,chat_ids=$3::jsonb,updated_at=now() WHERE installation_id=$1`, i.InstallationID, i.CancelOperationID, raw); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_chat_whitelist SET enabled=false WHERE bot_id=$1 AND space_id=$2`, botID, spaceID); err != nil {
		return nil, err
	}
	if admission != nil {
		if err = s.ensureUninstallCleanupTx(ctx, tx, i, *admission); err != nil {
			return nil, err
		}
	}
	return i, tx.Commit(ctx)
}

func (s *BotStore) CompleteInstallationCancellation(ctx context.Context, want *InstallationIntent) error {
	if want == nil || want.CancelOperationID == uuid.Nil {
		return ErrInstallationConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = s.lockInstallation(ctx, tx, want.BotID, want.SpaceID, false); err != nil {
		return err
	}
	if err = s.completeInstallationCancellationTx(ctx, tx, want); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *BotStore) completeInstallationCancellationTx(ctx context.Context, tx pgx.Tx, want *InstallationIntent) error {
	got, err := scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE installation_id=$1 FOR UPDATE`, want.InstallationID))
	if err != nil {
		return err
	}
	if !sameInstallationIdentity(got, want) || got.CancelOperationID != want.CancelOperationID || (got.State != "CANCELING" && got.State != "CANCELED") {
		return ErrInstallationConflict
	}
	if got.State == "CANCELED" {
		return nil
	}
	if err := replaceInstallationWhitelist(ctx, tx, got.BotID, got.SpaceID, got.InstallerProfileID, nil, nil, nil); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM bot_space_installations WHERE id=$1 AND bot_id=$2 AND space_id=$3`, got.InstallationID, got.BotID, got.SpaceID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_installation_intents SET state='CANCELED',updated_at=now() WHERE installation_id=$1`, got.InstallationID); err != nil {
		return err
	}
	return nil
}
