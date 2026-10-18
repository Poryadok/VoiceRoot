package store

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math"
	"time"
	"voice/backend/messaging/internal/automodevidence"
	"voice/backend/pkg/automodclient"
	"voice/backend/pkg/automodwire"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

// AutoModAdmission is supplied only after authenticated account/actor ownership
// and private Auth/Moderation admission. It is never serialized into an intent,
// public message event, or accepted from client identity metadata.
type AutoModAdmission struct {
	Subject               *automodv1.SubjectBinding
	SenderProfileID       uuid.UUID
	OperationID           uuid.UUID
	BotID, InstallationID *uuid.UUID
	OwnershipRevision     uint64
	SessionEpoch          uint64
}

type AutoModProducer struct{ Ring *automodevidence.Ring }

// ConfigureAutoMod is a startup-only operation before handlers/workers start.
// Registration never retires an omitted key or resets an existing source epoch.
// Public activation still requires current-boot terminal/cursor proof.
func (s *MessagesStore) ConfigureAutoMod(ctx context.Context, ring *automodevidence.Ring) (uuid.UUID, error) {
	if s == nil || s.Pool == nil || ring == nil || len(ring.KeyIDs()) < 2 {
		return uuid.Nil, automodevidence.ErrKeyRing
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('messaging-automod-keyring',0))`); err != nil {
		return uuid.Nil, err
	}
	for _, id := range ring.KeyIDs() {
		fingerprint, err := ring.KeyFingerprint(id)
		if err != nil {
			return uuid.Nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO automod_hmac_keys(key_id,fingerprint) VALUES($1,$2) ON CONFLICT(key_id) DO NOTHING`, id, fingerprint[:]); err != nil {
			return uuid.Nil, err
		}
		var actual []byte
		var retired *time.Time
		if err = tx.QueryRow(ctx, `SELECT fingerprint,retired_at FROM automod_hmac_keys WHERE key_id=$1`, id).Scan(&actual, &retired); err != nil {
			return uuid.Nil, err
		}
		if retired != nil || !bytes.Equal(actual, fingerprint[:]) {
			return uuid.Nil, automodevidence.ErrKeyRing
		}
	}
	var active int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM automod_hmac_keys WHERE retired_at IS NULL`).Scan(&active); err != nil {
		return uuid.Nil, err
	}
	if active != len(ring.KeyIDs()) {
		return uuid.Nil, automodevidence.ErrKeyRing
	}
	var epoch uuid.UUID
	var version int
	if err = tx.QueryRow(ctx, `SELECT source_epoch,format_version FROM automod_producer_state WHERE singleton`).Scan(&epoch, &version); err != nil {
		return uuid.Nil, err
	}
	if version != automodwire.Version {
		return uuid.Nil, automodwire.ErrCore
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	s.AutoMod = &AutoModProducer{Ring: ring}
	return epoch, nil
}

type automodAdmissionKey struct{}

// WithAutoModAdmission carries the already verified producer decision through
// protected game/attachment APIs without adding public identity fields.
func WithAutoModAdmission(ctx context.Context, admission *AutoModAdmission) context.Context {
	return context.WithValue(ctx, automodAdmissionKey{}, admission)
}

var ErrAutoModAdmission = errors.New("verified live account moderation admission is required")

// appendAutoModEvidenceTx is called only on a genuinely new primary insertion,
// after Chat/Space locks. It makes no RPC and commits no independent transaction.
func (s *MessagesStore) appendAutoModEvidenceTx(ctx context.Context, tx pgx.Tx, row MessageRow) error {
	if s.AutoMod == nil {
		return nil
	} // Active database coverage guard denies forgotten production writers.
	a := row.AutoModAdmission
	if a == nil {
		a, _ = ctx.Value(automodAdmissionKey{}).(*AutoModAdmission)
	}
	if a == nil || a.Subject == nil || a.SessionEpoch == 0 || a.SessionEpoch > math.MaxInt64 || a.SenderProfileID != row.SenderProfileID || a.OperationID == uuid.Nil {
		return ErrAutoModAdmission
	}
	account, err := uuid.Parse(a.Subject.GetAccountId())
	if err != nil || account == uuid.Nil {
		return ErrAutoModAdmission
	}
	subject, err := uuid.Parse(a.Subject.GetSubjectId())
	if err != nil || subject == uuid.Nil || !automodclient.ValidSubject(a.Subject, account, subject) || a.Subject.GetState() != automodv1.SubjectState_SUBJECT_STATE_LIVE {
		return ErrAutoModAdmission
	}
	if a.BotID != nil && (*a.BotID == uuid.Nil || a.OwnershipRevision == 0 || a.OwnershipRevision > math.MaxInt64) || a.BotID == nil && (a.OwnershipRevision != 0 || a.InstallationID != nil) || a.InstallationID != nil && *a.InstallationID == uuid.Nil {
		return ErrAutoModAdmission
	}
	if row.GameBotID != nil && (a.BotID == nil || *row.GameBotID != *a.BotID) || row.GameInstallationID != nil && (a.InstallationID == nil || *row.GameInstallationID != *a.InstallationID) {
		return ErrAutoModAdmission
	}
	commentary := ""
	if row.ForwardCommentary != nil {
		commentary = row.ForwardCommentary.Content
	}
	projection, err := s.AutoMod.Ring.Project(subject, row.Content, commentary, row.IsE2E)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('messaging-automod-account:'||$1::text,0))`, account); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('messaging-automod-subject:'||$1::text,0))`, subject); err != nil {
		return err
	}
	var terminal bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject).Scan(&terminal); err != nil {
		return err
	}
	if terminal {
		return ErrAutoModAdmission
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended('messaging-automod-keyring',0))`); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT key_id,fingerprint FROM automod_hmac_keys WHERE retired_at IS NULL ORDER BY key_id FOR SHARE`)
	if err != nil {
		return err
	}
	seen := 0
	for rows.Next() {
		var id string
		var fingerprint []byte
		if err = rows.Scan(&id, &fingerprint); err != nil {
			rows.Close()
			return err
		}
		expected, e := s.AutoMod.Ring.KeyFingerprint(id)
		if e != nil || !bytes.Equal(expected[:], fingerprint) {
			rows.Close()
			return automodevidence.ErrKeyRing
		}
		seen++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if seen != len(s.AutoMod.Ring.KeyIDs()) {
		return automodevidence.ErrKeyRing
	}
	if _, err = tx.Exec(ctx, `INSERT INTO automod_source_subjects(subject_id,account_id,lifecycle_revision) VALUES($1,$2,$3) ON CONFLICT(subject_id) DO NOTHING`, subject, account, int64(a.Subject.GetLifecycleRevision())); err != nil {
		return err
	}
	var actual uuid.UUID
	var lifecycle int64
	if err = tx.QueryRow(ctx, `SELECT account_id,lifecycle_revision FROM automod_source_subjects WHERE subject_id=$1 FOR UPDATE`, subject).Scan(&actual, &lifecycle); err != nil {
		return err
	}
	if actual != account || lifecycle > int64(a.Subject.GetLifecycleRevision()) {
		return ErrAutoModAdmission
	}
	var sequence uint64
	var sourceAt time.Time
	var epoch uuid.UUID
	if err = tx.QueryRow(ctx, `UPDATE automod_source_subjects SET lifecycle_revision=$2,through_sequence=through_sequence+1,last_source_at=GREATEST(clock_timestamp(),COALESCE(last_source_at+interval '1 microsecond','-infinity'::timestamptz)) WHERE subject_id=$1 RETURNING through_sequence,last_source_at`, subject, int64(a.Subject.GetLifecycleRevision())).Scan(&sequence, &sourceAt); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT source_epoch FROM automod_producer_state WHERE singleton`).Scan(&epoch); err != nil {
		return err
	}
	evidence := &automodv1.CommittedAccountCore{ProtocolVersion: automodwire.Version, Subject: proto.Clone(a.Subject).(*automodv1.SubjectBinding), SourceEpoch: epoch.String(), Sequence: sequence, EventId: uuid.NewString(), RecordedAt: timestamppb.New(sourceAt), Eligibility: projection.Eligibility, Digests: projection.Digests, LinkCount: projection.LinkCount, OwnershipRevision: a.OwnershipRevision, Producer: automodv1.AccountCoreProducer_ACCOUNT_CORE_PRODUCER_USER}
	if a.BotID != nil {
		evidence.Producer = automodv1.AccountCoreProducer_ACCOUNT_CORE_PRODUCER_BOT
	}
	payload, digest, err := automodwire.EncodeCore(evidence)
	if err != nil {
		return err
	}
	var companion any
	if row.ForwardCommentary != nil {
		companion = row.ForwardCommentary.ID
	}
	if _, err = tx.Exec(ctx, `INSERT INTO automod_evidence_outbox(event_id,subject_id,sequence,source_at,core_sha256,payload) VALUES($1,$2,$3,$4,$5,$6)`, evidence.GetEventId(), subject, sequence, sourceAt, digest[:], payload); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO automod_message_coverage(event_id,message_id,companion_id,chat_id) VALUES($1,$2,$3,$4)`, evidence.GetEventId(), row.ID, companion, row.ChatID); err != nil {
		return err
	}
	for _, digest := range projection.Digests {
		if _, err = tx.Exec(ctx, `INSERT INTO automod_hmac_usage(subject_id,key_id,last_source_at) VALUES($1,$2,$3) ON CONFLICT(subject_id,key_id) DO UPDATE SET last_source_at=EXCLUDED.last_source_at`, subject, digest.GetKeyId(), sourceAt); err != nil {
			return err
		}
	}
	return nil
}
