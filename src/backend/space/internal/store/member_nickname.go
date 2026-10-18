package store

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrInvalidMemberNickname = errors.New("invalid member nickname")
var ErrNicknameOwnerProtected = errors.New("cannot change space owner nickname")
var ErrNicknameActorNotMember = errors.New("nickname actor is not a space member")
var ErrNicknameExternallyManaged = errors.New("nickname target is externally managed")

// CanonicalMemberNickname implements BE-234-nickname-v1 value handling.
func CanonicalMemberNickname(value string) (*string, error) {
	if !utf8.ValidString(value) {
		return nil, ErrInvalidMemberNickname
	}
	for _, r := range value {
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Bidi_Control, r) || r == '\u2028' || r == '\u2029' {
			return nil, ErrInvalidMemberNickname
		}
	}
	value = strings.TrimFunc(value, unicode.IsSpace)
	if value == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(value) > 64 {
		return nil, ErrInvalidMemberNickname
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return &value, nil
		}
	}
	return nil, ErrInvalidMemberNickname
}

// UpdateMemberNickname changes only an existing local membership. The caller
// must have checked the global permission; local actor membership, current
// ownership and lifecycle are checked again under the shared Space lock.
func (s *SpaceStore) UpdateMemberNickname(ctx context.Context, spaceID, target, actor uuid.UUID, value string) (*MembershipRow, error) {
	nickname, err := CanonicalMemberNickname(value)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) (*MembershipRow, error) {
			return scoped.UpdateMemberNickname(ctx, spaceID, target, actor, value)
		})
	}
	// Community succession uses its authority row before the spaces row. Take
	// that same lock order so a recovery cannot change Owner between our check
	// and update, or let a local Owner bypass its pending authority freeze.
	var communityStatus string
	err = s.db().QueryRow(ctx, `SELECT status FROM community_owner_authority WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&communityStatus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil && communityStatus != "active" {
		return nil, ErrOwnershipFrozen
	}
	member, err := s.IsSpaceMember(ctx, spaceID, actor)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNicknameActorNotMember
	}
	var owner uuid.UUID
	if err = s.db().QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1 FOR UPDATE`, spaceID).Scan(&owner); err != nil {
		return nil, err
	}
	if target == owner && actor != owner {
		return nil, ErrNicknameOwnerProtected
	}
	row, err := scanMembershipRow(s.db().QueryRow(ctx, `SELECT space_id,profile_id,joined_at,nickname FROM space_members WHERE space_id=$1 AND profile_id=$2 FOR UPDATE`, spaceID, target))
	if errors.Is(err, pgx.ErrNoRows) {
		external, lookupErr := s.IsSpaceMember(ctx, spaceID, target)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if external {
			return nil, ErrNicknameExternallyManaged
		}
	}
	if err != nil {
		return nil, err
	}
	if row.Nickname == nil && nickname == nil || row.Nickname != nil && nickname != nil && *row.Nickname == *nickname {
		return row, nil
	}
	row, err = scanMembershipRow(s.db().QueryRow(ctx, `UPDATE space_members SET nickname=$3 WHERE space_id=$1 AND profile_id=$2 RETURNING space_id,profile_id,joined_at,nickname`, spaceID, target, nickname))
	if err != nil {
		return nil, err
	}
	_, err = s.db().Exec(ctx, `INSERT INTO audit_log(space_id,actor_profile_id,action,target_type,target_id,details)
 VALUES($1,$2,'member_nickname_updated','profile',$3,'{"changed_fields":["nickname"]}'::jsonb)`, spaceID, actor, target)
	return row, err
}
