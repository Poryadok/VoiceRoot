package profileprojection

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
)

// StoreAdapter atomically persists the inbox/revision fence and selected Search projection.
type StoreAdapter struct{ Pool *pgxpool.Pool }

func (s *StoreAdapter) Checkpoint(ctx context.Context) (uint64, error) {
	var offset uint64
	err := s.Pool.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_checkpoint WHERE singleton=true`).Scan(&offset)
	return offset, err
}

func (s *StoreAdapter) StoreCheckpoint(ctx context.Context, offset uint64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE search_user_profile_checkpoint SET journal_offset=GREATEST(journal_offset,$1),updated_at=now() WHERE singleton=true`, offset)
	return err
}

func (s *StoreAdapter) Apply(ctx context.Context, envelope *userv1.SearchProfileProjectionEvent) (ApplyResult, error) {
	return s.apply(ctx, envelope, 0)
}

// ApplyAndCheckpoint commits a journal event and the durable replay offset in
// one database transaction. A restart therefore either sees neither change or
// replays an already-applied event through the inbox/revision fence.
func (s *StoreAdapter) ApplyAndCheckpoint(ctx context.Context, envelope *userv1.SearchProfileProjectionEvent, checkpoint uint64) (ApplyResult, error) {
	if checkpoint == 0 {
		return Quarantined, fmt.Errorf("journal checkpoint is required")
	}
	return s.apply(ctx, envelope, checkpoint)
}

func (s *StoreAdapter) apply(ctx context.Context, envelope *userv1.SearchProfileProjectionEvent, checkpoint uint64) (ApplyResult, error) {
	if s == nil || s.Pool == nil {
		return Quarantined, fmt.Errorf("profile projection store unavailable")
	}
	event, err := eventFromProto(envelope)
	if err != nil {
		return Quarantined, err
	}
	payload, err := proto.Marshal(envelope)
	if err != nil {
		return Quarantined, err
	}
	digest := sha256.Sum256(payload)
	eventID, err := uuid.Parse(envelope.GetEventId())
	if err != nil {
		return Quarantined, err
	}
	profileID, err := uuid.Parse(envelope.GetProfileId())
	if err != nil {
		return Quarantined, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Quarantined, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	state := State{ProfileID: event.ProfileID}
	var revision int64
	var hash []byte
	var tombstoned bool
	err = tx.QueryRow(ctx, `SELECT source_revision,payload_sha256,tombstoned_at IS NOT NULL FROM search_user_profile_inbox WHERE profile_id=$1 ORDER BY source_revision DESC LIMIT 1 FOR UPDATE`, profileID).Scan(&revision, &hash, &tombstoned)
	if err != nil && err != pgx.ErrNoRows {
		return Quarantined, err
	}
	if err == nil {
		state.SourceRevision = uint64(revision)
		state.PayloadHash = fmt.Sprintf("%x", hash)
		state.Tombstoned = tombstoned
	}
	result, applyErr := Apply(&state, event, fmt.Sprintf("%x", digest[:]))
	if applyErr != nil {
		_, _ = tx.Exec(ctx, `INSERT INTO search_user_profile_inbox(event_id,profile_id,source_revision,payload_sha256,quarantined_at,quarantine_reason) VALUES($1,$2,$3,$4,now(),$5) ON CONFLICT(event_id) DO NOTHING`, eventID, profileID, event.SourceRevision, digest[:], applyErr.Error())
		_ = tx.Commit(ctx)
		return result, applyErr
	}
	_, err = tx.Exec(ctx, `INSERT INTO search_user_profile_inbox(event_id,profile_id,source_revision,payload_sha256) VALUES($1,$2,$3,$4) ON CONFLICT(event_id) DO NOTHING`, eventID, profileID, event.SourceRevision, digest[:])
	if err != nil {
		return Quarantined, err
	}
	if result == Applied {
		if event.Kind == Delete {
			_, err = tx.Exec(ctx, `UPDATE profile_search_documents SET source_revision=$2,tombstoned_at=now(),updated_at=now() WHERE profile_id=$1`, profileID, event.SourceRevision)
		} else {
			u := envelope.GetUpsert()
			accountID, parseErr := uuid.Parse(u.GetAccountId())
			if parseErr != nil {
				return Quarantined, parseErr
			}
			_, err = tx.Exec(ctx, `INSERT INTO profile_search_documents(profile_id,account_id,username,discriminator,display_name,username_lower,verification_type,username_search_key,display_name_search_key,normalization_version,source_revision,tombstoned_at,updated_at) VALUES($1,$2,$3,$4,$5,lower($3),$6,$7,$8,$9,$10,NULL,now()) ON CONFLICT(profile_id) DO UPDATE SET account_id=EXCLUDED.account_id,username=EXCLUDED.username,discriminator=EXCLUDED.discriminator,display_name=EXCLUDED.display_name,username_lower=EXCLUDED.username_lower,verification_type=EXCLUDED.verification_type,username_search_key=EXCLUDED.username_search_key,display_name_search_key=EXCLUDED.display_name_search_key,normalization_version=EXCLUDED.normalization_version,source_revision=EXCLUDED.source_revision,tombstoned_at=NULL,updated_at=now()`, profileID, accountID, event.Username, u.GetDiscriminator(), event.DisplayName, u.GetVerificationType(), event.UsernameSearchKey, event.DisplayNameSearchKey, event.NormalizationVersion, event.SourceRevision)
		}
		if err != nil {
			return Quarantined, err
		}
	}
	if checkpoint != 0 {
		if _, err = tx.Exec(ctx, `UPDATE search_user_profile_checkpoint SET journal_offset=GREATEST(journal_offset,$1),updated_at=now() WHERE singleton=true`, checkpoint); err != nil {
			return Quarantined, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Quarantined, err
	}
	return result, nil
}
func eventFromProto(event *userv1.SearchProfileProjectionEvent) (Event, error) {
	if event == nil {
		return Event{}, fmt.Errorf("missing profile projection event")
	}
	out := Event{ProtocolVersion: int(event.GetProtocolVersion()), EventID: event.GetEventId(), ProfileID: event.GetProfileId(), SourceRevision: event.GetSourceRevision()}
	if u := event.GetUpsert(); u != nil {
		out.Kind = Upsert
		out.Username = u.GetUsername()
		out.DisplayName = u.GetDisplayName()
		out.UsernameSearchKey = u.GetUsernameSearchKey()
		out.DisplayNameSearchKey = u.GetDisplayNameSearchKey()
		out.NormalizationVersion = int(u.GetNormalizationVersion())
	} else if event.GetDelete() != nil {
		out.Kind = Delete
	}
	return out, ValidateEvent(out)
}
