package store

import (
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"math"
	eventsv1 "voice.app/voice/events/v1"
)

var ErrVoiceRosterContract = errors.New("voice roster contract mismatch")
var errVoiceRosterCutChanged = errors.New("voice roster reset source changed")

type voiceRosterFact struct {
	EventID, RoomID, ProfileID uuid.UUID
	Version                    int64
	Joined                     bool
	Hash                       [32]byte
}
type VoiceRosterResetResult struct {
	ContractMismatch  bool
	CancelledSessions int
}

func decodeVoiceRosterFact(raw []byte) (voiceRosterFact, error) {
	f := voiceRosterFact{Hash: sha256.Sum256(raw)}
	env := new(eventsv1.VoiceStreamEvent)
	if len(raw) > 2<<20 || proto.Unmarshal(raw, env) != nil {
		return f, ErrVoiceRosterContract
	}
	f.EventID, _ = uuid.Parse(env.GetEventId())
	fact := env.GetVoiceRosterMembershipChanged()
	if fact == nil {
		return f, ErrVoiceRosterContract
	}
	f.RoomID, _ = uuid.Parse(fact.GetRoomId())
	f.ProfileID, _ = uuid.Parse(fact.GetProfileId())
	if fact.GetRosterVersion() <= math.MaxInt64 {
		f.Version = int64(fact.GetRosterVersion())
	}
	f.Joined = fact.GetChange() == eventsv1.VoiceRosterMembershipChanged_CHANGE_JOINED
	if f.EventID == uuid.Nil || f.RoomID == uuid.Nil || f.ProfileID == uuid.Nil || f.Version <= 0 || fact.GetProtocolVersion() != 1 || env.GetOccurredAt() == nil || env.GetOccurredAt().CheckValid() != nil ||
		(!f.Joined && fact.GetChange() != eventsv1.VoiceRosterMembershipChanged_CHANGE_LEFT) {
		return f, ErrVoiceRosterContract
	}
	return f, nil
}
