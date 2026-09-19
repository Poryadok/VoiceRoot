package roomlifecycle

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MembershipState string

const (
	MembershipJoining      MembershipState = "JOINING"
	MembershipJoined       MembershipState = "JOINED"
	MembershipReconnecting MembershipState = "RECONNECTING"
	MembershipLeaving      MembershipState = "LEAVING"
	MembershipLeft         MembershipState = "LEFT"
	MembershipEjected      MembershipState = "EJECTED"
)

// MatchmakingMembershipIdentity is stored evidence, not an authenticated
// principal or a current eligibility decision. Snapshot activation must verify
// Auth epoch validity and reconnect expiry within its roster transaction.
type MatchmakingMembershipIdentity struct {
	ProfileID          uuid.UUID
	RoomID             uuid.UUID
	MediaEpoch         uuid.UUID
	AccountID          uuid.UUID
	SessionEpoch       int64
	State              MembershipState
	ReconnectStartedAt *time.Time
	ReconnectDeadline  *time.Time
}

type MatchmakingRoomIdentity struct {
	RoomID                uuid.UUID
	RoomType              string
	Purpose               string
	SpaceID               *uuid.UUID
	VoiceRoomID           *uuid.UUID
	ChatID                *uuid.UUID
	OwnerID               *uuid.UUID
	CreationOperationID   *uuid.UUID
	CreationManifestHash  []byte
	CreationReceiptID     *uuid.UUID
	ChatCreationReceiptID *uuid.UUID
	RosterVersion         int64
	Active                bool
}

func validateMatchmakingMembership(row MatchmakingMembershipIdentity) error {
	if anyNilUUID(row.ProfileID, row.RoomID, row.MediaEpoch, row.AccountID) || row.SessionEpoch <= 0 {
		return ErrInvariant
	}
	switch row.State {
	case MembershipJoining, MembershipJoined, MembershipLeaving, MembershipLeft, MembershipEjected:
		if row.ReconnectStartedAt != nil || row.ReconnectDeadline != nil {
			return ErrInvariant
		}
	case MembershipReconnecting:
		if row.ReconnectStartedAt == nil || row.ReconnectDeadline == nil || row.ReconnectStartedAt.IsZero() ||
			!row.ReconnectDeadline.After(*row.ReconnectStartedAt) || row.ReconnectDeadline.Sub(*row.ReconnectStartedAt) > 30*time.Second {
			return ErrInvariant
		}
	default:
		return ErrInvariant
	}
	return nil
}

const matchmakingRoomColumns = `room_id,room_type,purpose,space_id,voice_room_id,chat_id,
owner_id,creation_operation_id,creation_manifest_hash,creation_receipt_id,chat_creation_receipt_id,roster_version,state`
const matchmakingMembershipColumns = `profile_id,room_id,media_epoch,account_id,session_epoch,membership_state,reconnect_started_at,reconnect_deadline`

// CheckMatchmakingMembershipSchema is deliberately separate from R22 readiness.
// Nothing calls these APIs from a handler or startup path until activation.
func (store *PostgresLifecycleStore) CheckMatchmakingMembershipSchema(ctx context.Context) error {
	if store == nil || store.pool == nil {
		return ErrUnavailable
	}
	for _, query := range []string{
		`SELECT ` + matchmakingRoomColumns + ` FROM voice_room_instances LIMIT 0`,
		`SELECT ` + matchmakingMembershipColumns + ` FROM voice_room_memberships LIMIT 0`,
	} {
		rows, err := store.pool.Query(ctx, query)
		if err != nil {
			return ErrUnavailable
		}
		rows.Close()
		if rows.Err() != nil {
			return ErrUnavailable
		}
	}
	return nil
}

// LoadMembershipIdentity returns ErrInvariant for a legacy unknown locator,
// preserving the distinction from a genuinely absent membership. This single-row
// read must not be composed with room reads to construct a concurrent MM snapshot.
func (store *PostgresLifecycleStore) LoadMembershipIdentity(ctx context.Context, profileID uuid.UUID) (MatchmakingMembershipIdentity, bool, error) {
	if err := store.CheckMatchmakingMembershipSchema(ctx); err != nil {
		return MatchmakingMembershipIdentity{}, false, err
	}
	if profileID == uuid.Nil {
		return MatchmakingMembershipIdentity{}, false, ErrInvariant
	}
	var row MatchmakingMembershipIdentity
	var account *uuid.UUID
	var epoch *int64
	var state *string
	err := store.pool.QueryRow(ctx, `SELECT `+matchmakingMembershipColumns+` FROM voice_room_memberships WHERE profile_id=$1`, profileID).Scan(
		&row.ProfileID, &row.RoomID, &row.MediaEpoch, &account, &epoch, &state, &row.ReconnectStartedAt, &row.ReconnectDeadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchmakingMembershipIdentity{}, false, nil
	}
	if err != nil {
		return MatchmakingMembershipIdentity{}, false, mapScanError(err)
	}
	if account == nil || epoch == nil || state == nil {
		return MatchmakingMembershipIdentity{}, false, ErrInvariant
	}
	row.AccountID, row.SessionEpoch, row.State = *account, *epoch, MembershipState(*state)
	if err = validateMatchmakingMembership(row); err != nil {
		return MatchmakingMembershipIdentity{}, false, err
	}
	normalizeUTC(row.ReconnectStartedAt)
	normalizeUTC(row.ReconnectDeadline)
	return row, true, nil
}

func (store *PostgresLifecycleStore) LoadMatchmakingRoom(ctx context.Context, roomID uuid.UUID) (MatchmakingRoomIdentity, bool, error) {
	if err := store.CheckMatchmakingMembershipSchema(ctx); err != nil {
		return MatchmakingRoomIdentity{}, false, err
	}
	if roomID == uuid.Nil {
		return MatchmakingRoomIdentity{}, false, ErrInvariant
	}
	var row MatchmakingRoomIdentity
	var state string
	err := store.pool.QueryRow(ctx, `SELECT `+matchmakingRoomColumns+` FROM voice_room_instances WHERE room_id=$1`, roomID).Scan(
		&row.RoomID, &row.RoomType, &row.Purpose, &row.SpaceID, &row.VoiceRoomID, &row.ChatID,
		&row.OwnerID, &row.CreationOperationID, &row.CreationManifestHash, &row.CreationReceiptID, &row.ChatCreationReceiptID, &row.RosterVersion, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchmakingRoomIdentity{}, false, nil
	}
	if err != nil {
		return MatchmakingRoomIdentity{}, false, mapScanError(err)
	}
	if state != "active" && state != "closed" {
		return MatchmakingRoomIdentity{}, false, ErrInvariant
	}
	row.Active = state == "active"
	if err = validateMatchmakingRoom(row); err != nil {
		return MatchmakingRoomIdentity{}, false, err
	}
	return row, true, nil
}

func validateMatchmakingRoom(row MatchmakingRoomIdentity) error {
	if row.RoomID == uuid.Nil || row.RosterVersion < 0 {
		return ErrInvariant
	}
	switch row.RoomType {
	case "voice_room":
		if !validUUIDPointer(row.SpaceID) || !validUUIDPointer(row.VoiceRoomID) || row.ChatID != nil {
			return ErrInvariant
		}
	case "call", "group_voice":
		if row.SpaceID != nil || row.VoiceRoomID != nil || !validUUIDPointer(row.ChatID) {
			return ErrInvariant
		}
	default:
		return ErrInvariant
	}
	switch row.Purpose {
	case "ORDINARY":
		if row.OwnerID != nil || row.CreationOperationID != nil || row.CreationManifestHash != nil || row.CreationReceiptID != nil || row.ChatCreationReceiptID != nil {
			return ErrInvariant
		}
	case "MATCH_SQUAD":
		if row.RoomType != "group_voice" || !validUUIDPointer(row.OwnerID) || !validUUIDPointer(row.CreationOperationID) ||
			len(row.CreationManifestHash) != 32 || !validUUIDPointer(row.CreationReceiptID) || !validUUIDPointer(row.ChatCreationReceiptID) {
			return ErrInvariant
		}
	default:
		return ErrInvariant
	}
	return nil
}
