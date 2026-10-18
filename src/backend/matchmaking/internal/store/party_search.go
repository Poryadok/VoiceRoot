package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"voice/backend/pkg/partyauthority"
)

var (
	ErrSourcePartyInvalid      = errors.New("invalid authoritative search party")
	ErrSourcePartyUnavailable  = errors.New("authoritative search party unavailable")
	ErrSourcePartyNotFound     = errors.New("authoritative search party not found")
	ErrSourcePartyChanged      = errors.New("captured Voice party changed")
	ErrSearchOperationConflict = errors.New("search operation binding conflict")
)

// SourceParty is an immutable native Voice snapshot attached to one MM-owned
// party. LeaderProfileID is the initiator; no client-supplied roster is used.
type SourceParty struct {
	Party
	SearchOperationID      uuid.UUID
	SearchRequestHash      [32]byte
	InitiatorAccountID     uuid.UUID
	InitiatorSessionEpoch  int64
	ProtocolVersion        uint32
	SourceKind             string
	SourceRoomID           *uuid.UUID
	SourceRosterVersion    uint64
	MemberManifestHash     [32]byte
	MemberAuthorityVersion uint32
	MemberAuthorities      []partyauthority.Member
	MemberAuthorityHash    [32]byte
	SourceAuthorityHash    [32]byte
}

type CreateSourcePartyParams struct {
	SearchOperationID      uuid.UUID
	SearchRequestHash      [32]byte
	InitiatorAccountID     uuid.UUID
	InitiatorProfileID     uuid.UUID
	InitiatorSessionEpoch  int64
	ProtocolVersion        uint32
	SourceKind             string
	SourceRoomID           *uuid.UUID
	SourceRosterVersion    uint64
	MemberProfileIDs       []uuid.UUID
	MemberAuthorityVersion uint32
	MemberAuthorities      []partyauthority.Member
	SourceAuthorityHash    [32]byte
	GameID                 uuid.UUID
	Mode, Criteria         string
	SpaceID                *uuid.UUID
	TimeoutAt              time.Time
}

type SourcePartySearch struct {
	Party    SourceParty
	Sessions []SearchSession
	Replay   bool
}

const sourcePartyCols = `id,leader_profile_id,member_profile_ids,game_id,mode,criteria::text,created_at,disbanded_at,
search_operation_id,search_request_sha256,initiator_account_id,initiator_session_epoch,
voice_snapshot_protocol_version,voice_source_kind,voice_source_room_id,voice_source_roster_version,voice_member_manifest_sha256,
voice_member_authority_version,voice_member_authority,voice_member_authority_sha256,voice_source_authority_sha256`

// CreateSourceParty commits the captured source, every member's unique active
// session and the queue intent together. Redis IO always runs after COMMIT.
// A lost commit acknowledgment is resolved by this exact operation's replay.
func (s *PartyStore) CreateSourceParty(ctx context.Context, p CreateSourcePartyParams) (SourcePartySearch, error) {
	p.MemberProfileIDs = append([]uuid.UUID(nil), p.MemberProfileIDs...)
	p.MemberAuthorities = append([]partyauthority.Member(nil), p.MemberAuthorities...)
	raw, manifest, err := encodeSourceBinding(p)
	if err != nil || p.TimeoutAt.IsZero() {
		return SourcePartySearch{}, ErrSourcePartyInvalid
	}
	authorityRaw, authorityHash, err := encodePrivateSourceAuthority(p)
	if err != nil {
		return SourcePartySearch{}, ErrSourcePartyInvalid
	}
	if s == nil || s.Pool == nil {
		return SourcePartySearch{}, ErrSourcePartyUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	defer rollbackPartySearch(tx)
	accounts := make([]uuid.UUID, 0, len(p.MemberAuthorities))
	for _, member := range p.MemberAuthorities {
		accounts = append(accounts, member.AccountID)
	}
	guard, err := AcquirePlatformBanGuard(ctx, tx, accounts)
	if err != nil {
		return SourcePartySearch{}, err
	}
	if err := guard.Check(ctx); err != nil {
		return SourcePartySearch{}, err
	}
	var observedAt time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&observedAt); err != nil {
		return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	if !p.TimeoutAt.After(observedAt) {
		return SourcePartySearch{}, ErrSourcePartyInvalid
	}
	party, err := scanSourceParty(tx.QueryRow(ctx, `INSERT INTO parties
(id,leader_profile_id,member_profile_ids,game_id,mode,criteria,search_operation_id,search_request_sha256,
initiator_account_id,initiator_session_epoch,voice_snapshot_protocol_version,voice_source_kind,
voice_source_room_id,voice_source_roster_version,voice_member_manifest_sha256,
voice_member_authority_version,voice_member_authority,voice_member_authority_sha256,voice_source_authority_sha256)
VALUES($1,$2,$3::jsonb,$4,$5,$6::jsonb,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
ON CONFLICT(search_operation_id) DO NOTHING RETURNING `+sourcePartyCols,
		uuid.New(), p.InitiatorProfileID, string(raw), p.GameID, p.Mode, p.Criteria, p.SearchOperationID, p.SearchRequestHash[:],
		p.InitiatorAccountID, p.InitiatorSessionEpoch, int16(p.ProtocolVersion), p.SourceKind, p.SourceRoomID, int64(p.SourceRosterVersion), manifest[:], int16(p.MemberAuthorityVersion), authorityRaw, authorityHash[:], p.SourceAuthorityHash[:]))
	if errors.Is(err, pgx.ErrNoRows) {
		party, err = scanSourceParty(tx.QueryRow(ctx, `SELECT `+sourcePartyCols+` FROM parties WHERE search_operation_id=$1 FOR UPDATE`, p.SearchOperationID))
		if err != nil {
			return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		if party.SearchRequestHash != p.SearchRequestHash || party.InitiatorAccountID != p.InitiatorAccountID || party.LeaderProfileID != p.InitiatorProfileID || party.InitiatorSessionEpoch != p.InitiatorSessionEpoch || party.GameID != p.GameID || party.Mode != p.Mode {
			return SourcePartySearch{}, ErrSearchOperationConflict
		}
		var sameCriteria bool
		if err := tx.QueryRow(ctx, `SELECT criteria=$2::jsonb FROM parties WHERE id=$1`, party.ID, p.Criteria).Scan(&sameCriteria); err != nil {
			return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		if !sameCriteria {
			return SourcePartySearch{}, ErrSearchOperationConflict
		}
		if party.DisbandedAt != nil || party.ProtocolVersion != p.ProtocolVersion || party.SourceKind != p.SourceKind || !samePartyUUID(party.SourceRoomID, p.SourceRoomID) || party.SourceRosterVersion != p.SourceRosterVersion || party.MemberManifestHash != manifest || party.MemberAuthorityVersion != p.MemberAuthorityVersion || party.MemberAuthorityHash != authorityHash || party.SourceAuthorityHash != p.SourceAuthorityHash {
			return SourcePartySearch{}, ErrSourcePartyChanged
		}
		sessions, err := loadSourceSessions(ctx, tx, party.ID)
		if err != nil {
			return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		if len(sessions) != len(p.MemberProfileIDs) {
			return SourcePartySearch{}, ErrSourcePartyUnavailable
		}
		for i, session := range sessions {
			if session.ProfileID != p.MemberProfileIDs[i] || session.GameID != p.GameID || session.Mode != p.Mode || !samePartyUUID(session.PartyID, &party.ID) || !samePartyUUID(session.SpaceID, p.SpaceID) {
				return SourcePartySearch{}, ErrSearchOperationConflict
			}
			if session.Status == SessionStatusCancelled || session.Status == SessionStatusTimeout || (session.TimeoutAt != nil && !session.TimeoutAt.After(observedAt)) {
				return SourcePartySearch{}, ErrSourcePartyChanged
			}
		}
		var intent bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM matchmaking_party_search_queue_intents WHERE party_id=$1)`, party.ID).Scan(&intent); err != nil || !intent {
			return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		return SourcePartySearch{Party: party, Sessions: sessions, Replay: true}, nil
	}
	if err != nil {
		return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	sessions := make([]SearchSession, 0, len(p.MemberProfileIDs))
	// The global UUID order also orders conflicting unique-profile insert locks.
	for _, profile := range p.MemberProfileIDs {
		session, err := scanSession(tx.QueryRow(ctx, `INSERT INTO search_sessions
(id,profile_id,party_id,game_id,mode,criteria,status,timeout_at,space_id,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6::jsonb,'searching',$7,$8,$9,$9) RETURNING `+sessionSelectCols,
			uuid.New(), profile, party.ID, p.GameID, p.Mode, p.Criteria, p.TimeoutAt, p.SpaceID, party.CreatedAt))
		if err != nil {
			var constraint *pgconn.PgError
			if errors.As(err, &constraint) && constraint.Code == "23505" && constraint.ConstraintName == "search_sessions_one_active_per_profile" {
				return SourcePartySearch{}, ErrActiveSearchExists
			}
			return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		sessions = append(sessions, session)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO matchmaking_party_search_queue_intents(party_id,action) VALUES($1,'ENQUEUE')`, party.ID); err != nil {
		return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return SourcePartySearch{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	return SourcePartySearch{Party: party, Sessions: sessions}, nil
}

func (s *PartyStore) GetSourceParty(ctx context.Context, id uuid.UUID) (SourceParty, error) {
	if s == nil || s.Pool == nil || id == uuid.Nil {
		return SourceParty{}, ErrSourcePartyUnavailable
	}
	party, err := scanSourceParty(s.Pool.QueryRow(ctx, `SELECT `+sourcePartyCols+` FROM parties WHERE id=$1 AND search_operation_id IS NOT NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return SourceParty{}, ErrSourcePartyNotFound
	}
	if err != nil {
		return SourceParty{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	return party, nil
}

func encodeSourceBinding(p CreateSourcePartyParams) ([]byte, [32]byte, error) {
	invalid := func() ([]byte, [32]byte, error) { return nil, [32]byte{}, ErrSourcePartyInvalid }
	if p.SearchOperationID == uuid.Nil || p.SearchRequestHash == [32]byte{} || p.InitiatorAccountID == uuid.Nil || p.InitiatorProfileID == uuid.Nil || p.InitiatorSessionEpoch <= 0 || p.ProtocolVersion != 1 || p.GameID == uuid.Nil || strings.TrimSpace(p.Mode) == "" || len(p.MemberProfileIDs) == 0 {
		return invalid()
	}
	if _, _, err := encodePrivateSourceAuthority(p); err != nil {
		return invalid()
	}
	if p.SpaceID != nil && *p.SpaceID == uuid.Nil {
		return invalid()
	}
	var criteria map[string]json.RawMessage
	if json.Unmarshal([]byte(p.Criteria), &criteria) != nil || criteria == nil {
		return invalid()
	}
	if p.SourceKind == "SOLO" {
		if p.SourceRoomID != nil || p.SourceRosterVersion != 0 || len(p.MemberProfileIDs) != 1 {
			return invalid()
		}
	} else if p.SourceKind != "VOICE_ROSTER" || p.SourceRoomID == nil || *p.SourceRoomID == uuid.Nil || p.SourceRosterVersion == 0 || p.SourceRosterVersion > math.MaxInt64 {
		return invalid()
	}
	ids := make([]string, 0, len(p.MemberProfileIDs))
	canonical := make([]byte, 0, len(p.MemberProfileIDs)*16)
	initiator := false
	for i, id := range p.MemberProfileIDs {
		if id == uuid.Nil || (i > 0 && bytes.Compare(p.MemberProfileIDs[i-1][:], id[:]) >= 0) {
			return invalid()
		}
		if id == p.InitiatorProfileID {
			initiator = true
		}
		ids = append(ids, id.String())
		canonical = append(canonical, id[:]...)
	}
	if !initiator {
		return invalid()
	}
	raw, err := json.Marshal(ids)
	return raw, sha256.Sum256(canonical), err
}

func encodePrivateSourceAuthority(p CreateSourcePartyParams) ([]byte, [32]byte, error) {
	if p.MemberAuthorityVersion != partyauthority.Version || p.SourceAuthorityHash == [32]byte{} || len(p.MemberAuthorities) != len(p.MemberProfileIDs) {
		return nil, [32]byte{}, ErrSourcePartyInvalid
	}
	raw, hash, err := partyauthority.Encode(p.MemberAuthorities)
	if err != nil {
		return nil, [32]byte{}, ErrSourcePartyInvalid
	}
	for i, member := range p.MemberAuthorities {
		if member.ProfileID != p.MemberProfileIDs[i] || (member.ProfileID == p.InitiatorProfileID && (member.AccountID != p.InitiatorAccountID || (p.SourceKind == "SOLO" && member.SessionEpoch != p.InitiatorSessionEpoch))) {
			return nil, [32]byte{}, ErrSourcePartyInvalid
		}
	}
	return raw, hash, nil
}

func scanSourceParty(row pgx.Row) (SourceParty, error) {
	var p SourceParty
	var members, requestHash, manifest, authorityRaw, authorityHash, sourceHash []byte
	err := row.Scan(&p.ID, &p.LeaderProfileID, &members, &p.GameID, &p.Mode, &p.Criteria, &p.CreatedAt, &p.DisbandedAt,
		&p.SearchOperationID, &requestHash, &p.InitiatorAccountID, &p.InitiatorSessionEpoch, &p.ProtocolVersion, &p.SourceKind, &p.SourceRoomID, &p.SourceRosterVersion, &manifest, &p.MemberAuthorityVersion, &authorityRaw, &authorityHash, &sourceHash)
	if err != nil {
		return SourceParty{}, err
	}
	if len(requestHash) != 32 || len(manifest) != 32 || len(authorityHash) != 32 || len(sourceHash) != 32 {
		return SourceParty{}, ErrSourcePartyInvalid
	}
	copy(p.SearchRequestHash[:], requestHash)
	copy(p.MemberManifestHash[:], manifest)
	copy(p.MemberAuthorityHash[:], authorityHash)
	copy(p.SourceAuthorityHash[:], sourceHash)
	p.MemberAuthorities, err = partyauthority.Decode(authorityRaw, p.MemberAuthorityHash)
	if err != nil {
		return SourceParty{}, ErrSourcePartyInvalid
	}
	var ids []string
	if err := json.Unmarshal(members, &ids); err != nil {
		return SourceParty{}, ErrSourcePartyInvalid
	}
	for _, text := range ids {
		id, err := uuid.Parse(text)
		if err != nil || id == uuid.Nil || id.String() != text {
			return SourceParty{}, ErrSourcePartyInvalid
		}
		p.MemberProfileIDs = append(p.MemberProfileIDs, id)
	}
	_, hash, err := encodeSourceBinding(CreateSourcePartyParams{SearchOperationID: p.SearchOperationID, SearchRequestHash: p.SearchRequestHash, InitiatorAccountID: p.InitiatorAccountID, InitiatorProfileID: p.LeaderProfileID, InitiatorSessionEpoch: p.InitiatorSessionEpoch, ProtocolVersion: p.ProtocolVersion, SourceKind: p.SourceKind, SourceRoomID: p.SourceRoomID, SourceRosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash, GameID: p.GameID, Mode: p.Mode, Criteria: p.Criteria})
	if err != nil || hash != p.MemberManifestHash {
		return SourceParty{}, ErrSourcePartyInvalid
	}
	return p, nil
}

func loadSourceSessions(ctx context.Context, tx pgx.Tx, party uuid.UUID) ([]SearchSession, error) {
	rows, err := tx.Query(ctx, `SELECT `+sessionSelectCols+` FROM search_sessions WHERE party_id=$1 ORDER BY profile_id FOR UPDATE`, party)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := []SearchSession{}
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func samePartyUUID(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func rollbackPartySearch(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
