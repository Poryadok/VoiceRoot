package authoritysource

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
)

// UserState accounts for every requested profile ID, including opaque SDK
// actors with no profile. Missing rows never establish permission or inactivity.
// Historical aliases are permanent exact actor/account deny facts, not grants.
type UserState struct {
	SchemaVersion uint32                `json:"schema_version"`
	Revision      uint64                `json:"revision"`
	Profiles      []UserProfile         `json:"profiles"`
	Tombstones    []UserAuthorTombstone `json:"tombstones"`
}
type UserProfile struct {
	ProfileID       string `json:"profile_id"`
	Exists          bool   `json:"exists"`
	AccountID       string `json:"account_id"`
	ProfileRevision uint64 `json:"profile_revision"`
	Deleted         bool   `json:"deleted"`
	Frozen          bool   `json:"frozen"`
	AccountInactive bool   `json:"account_inactive"`
}
type UserAuthorTombstone struct {
	SourceAccountID   string `json:"source_account_id"`
	SourceActorID     string `json:"source_actor_id"`
	TargetAccountID   string `json:"target_account_id"`
	TargetProfileID   string `json:"target_profile_id"`
	ProfileRevision   uint64 `json:"profile_revision"`
	TombstoneRevision uint64 `json:"tombstone_revision"`
}

func DecodeUserState(raw []byte) (UserState, error) {
	var state UserState
	if len(raw) > MaxStateBytes || json.Unmarshal(raw, &state) != nil {
		return UserState{}, errors.New("invalid User source state")
	}
	canonical, err := json.Marshal(state)
	if err != nil || !bytes.Equal(raw, canonical) || state.SchemaVersion != SchemaVersion || state.Revision == 0 || state.Revision > math.MaxInt64 || state.Profiles == nil || state.Tombstones == nil || len(state.Profiles) > MaxSubjects || len(state.Tombstones) > MaxSubjects {
		return UserState{}, errors.New("noncanonical User source state")
	}
	subjects := map[string]bool{}
	last := ""
	for _, item := range state.Profiles {
		if !canonicalID(item.ProfileID) || item.ProfileID <= last {
			return UserState{}, errors.New("invalid User profile set")
		}
		last = item.ProfileID
		subjects[item.ProfileID] = true
		if item.Exists {
			if !canonicalID(item.AccountID) || item.ProfileRevision == 0 || item.ProfileRevision > math.MaxInt64 {
				return UserState{}, errors.New("invalid User profile authority")
			}
		} else if item.AccountID != "" || item.ProfileRevision != 0 || item.Deleted || item.Frozen || item.AccountInactive {
			return UserState{}, errors.New("missing User profile carries authority")
		}
	}
	last = ""
	revisions := map[uint64]bool{}
	for _, item := range state.Tombstones {
		key := item.SourceActorID + "/" + item.SourceAccountID
		if !subjects[item.SourceActorID] || !canonicalID(item.SourceAccountID) || !canonicalID(item.TargetAccountID) || !canonicalID(item.TargetProfileID) || key <= last || item.ProfileRevision == 0 || item.ProfileRevision > math.MaxInt64 || item.TombstoneRevision == 0 || item.TombstoneRevision > math.MaxInt64 || revisions[item.TombstoneRevision] {
			return UserState{}, errors.New("invalid User actor tombstone")
		}
		last = key
		revisions[item.TombstoneRevision] = true
	}
	return state, nil
}

func (state UserState) ActorRetired(actor, account string) bool {
	for _, item := range state.Tombstones {
		if item.SourceActorID == actor && item.SourceAccountID == account {
			return true
		}
	}
	return false
}
func (state UserState) EligibleProfile(profile, account string) bool {
	if state.ActorRetired(profile, account) {
		return false
	}
	for _, item := range state.Profiles {
		if item.ProfileID == profile {
			return item.Exists && item.AccountID == account && !item.Deleted && !item.Frozen && !item.AccountInactive
		}
	}
	return false
}
