// Package rosteroutbox persists A3 eligibility facts in the native source transaction.
package rosteroutbox

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

const Subject = "voice.roster_membership_changed"

// Append requires the owner already to hold its native room/head serialization
// lock and to have advanced its independent roster counter in this transaction.
func Append(ctx context.Context, tx pgx.Tx, room, profile uuid.UUID, version int64, joined bool) error {
	if tx == nil || room == uuid.Nil || profile == uuid.Nil || version <= 0 {
		return errors.New("invalid roster membership fact")
	}
	var at time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
		return err
	}
	change, label := eventsv1.VoiceRosterMembershipChanged_CHANGE_LEFT, "LEFT"
	if joined {
		change, label = eventsv1.VoiceRosterMembershipChanged_CHANGE_JOINED, "JOINED"
	}
	id := uuid.New()
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(&eventsv1.VoiceStreamEvent{
		EventId: id.String(), OccurredAt: timestamppb.New(at),
		Payload: &eventsv1.VoiceStreamEvent_VoiceRosterMembershipChanged{VoiceRosterMembershipChanged: &eventsv1.VoiceRosterMembershipChanged{
			ProtocolVersion: 1, RoomId: room.String(), RosterVersion: uint64(version), Change: change, ProfileId: profile.String(),
		}},
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO voice_roster_outbox(event_id,room_id,roster_version,profile_id,change,occurred_at,payload) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, room, version, profile, label, at, payload)
	return err
}
