// Package spacecommunication validates fresh Space-owned communication evidence.
package spacecommunication

import (
	"errors"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	Clear    int32 = 1
	TimedOut int32 = 2
)

// Snapshot has no local cache or expiry authority. Both Space RPC adapters use
// this same shape; zero-valued legacy responses are deliberately invalid.
type Snapshot struct {
	Version            uint32
	SpaceID, ProfileID string
	Epoch              uint64
	EvaluatedAt        *timestamppb.Timestamp
	State              int32
	Until              *timestamppb.Timestamp
}

func (s Snapshot) Validate(spaceID, profileID string) (bool, error) {
	space, err := uuid.Parse(spaceID)
	if err != nil || space == uuid.Nil {
		return false, errors.New("invalid expected Space identity")
	}
	profile, err := uuid.Parse(profileID)
	if err != nil || profile == uuid.Nil {
		return false, errors.New("invalid expected profile identity")
	}
	if s.Version != 1 || s.SpaceID != spaceID || s.ProfileID != profileID || s.Epoch == 0 || s.EvaluatedAt == nil || s.EvaluatedAt.CheckValid() != nil || s.EvaluatedAt.Seconds <= 0 {
		return false, errors.New("invalid Space communication snapshot")
	}
	switch s.State {
	case Clear:
		if s.Until != nil {
			return false, errors.New("CLEAR snapshot contains timeout")
		}
		return false, nil
	case TimedOut:
		if s.Until == nil || s.Until.CheckValid() != nil || !s.Until.AsTime().After(s.EvaluatedAt.AsTime()) {
			return false, errors.New("invalid timeout snapshot")
		}
		return true, nil
	default:
		return false, errors.New("unsupported communication state")
	}
}
