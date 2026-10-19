package automodwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"

	"google.golang.org/protobuf/proto"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

const MaxCensusAccounts = 10000
const censusDomain = "voice.automod.account-census.v1\x00"

// CensusDigest frames one sorted finite Auth-owned binding view. As with the
// terminal seal, integrity does not certify an independently recovered tail.
func CensusDigest(accounts []*automodv1.ResolveModerationSubjectResponse) ([32]byte, error) {
	if len(accounts) > MaxCensusAccounts {
		return [32]byte{}, ErrCore
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(censusDomain))
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(accounts)))
	_, _ = hash.Write(count[:])
	previous := ""
	for _, account := range accounts {
		if !validCensusAccount(account) || account.GetSubject().GetAccountId() <= previous {
			return [32]byte{}, ErrCore
		}
		body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(account)
		if err != nil {
			return [32]byte{}, ErrCore
		}
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(body)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write(body)
		previous = account.GetSubject().GetAccountId()
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func validCensusAccount(account *automodv1.ResolveModerationSubjectResponse) bool {
	if account == nil || RejectUnknown(account) != nil || account.GetSessionEpoch() == 0 || account.GetSessionEpoch() > math.MaxInt64 {
		return false
	}
	binding := account.GetSubject()
	if binding == nil || binding.GetProtocolVersion() != 1 || !canonicalUUID(binding.GetAccountId()) || !canonicalUUID(binding.GetSubjectId()) || binding.GetLifecycleRevision() == 0 || binding.GetLifecycleRevision() > math.MaxInt64 {
		return false
	}
	switch binding.GetState() {
	case automodv1.SubjectState_SUBJECT_STATE_LIVE:
		return binding.PurgeAt == nil && binding.DeletionCycleId == nil
	case automodv1.SubjectState_SUBJECT_STATE_DELETED:
		return account.GetAuthRestricted() && canonicalUUID(binding.GetDeletionCycleId()) && binding.GetPurgeAt() != nil && binding.GetPurgeAt().CheckValid() == nil && binding.GetPurgeAt().GetNanos()%1000 == 0
	default:
		return false
	}
}

func ValidateAccountCensus(census *automodv1.ListModerationAccountCensusResponse) error {
	if census == nil || RejectUnknown(census) != nil || !census.GetComplete() {
		return ErrCore
	}
	seal := census.GetSeal()
	if seal == nil || seal.GetProtocolVersion() != 1 || !seal.GetComplete() || !canonicalUUID(seal.GetAuthorityGeneration()) || !canonicalUUID(seal.GetSnapshotId()) || seal.GetCut() == nil || seal.GetCut().CheckValid() != nil || seal.GetCut().GetNanos()%1000 != 0 || seal.GetItemCount() != uint64(len(census.GetAccounts())) || seal.GetTerminalHighWater() > math.MaxInt64 {
		return ErrCore
	}
	for _, account := range census.GetAccounts() {
		if account.GetSubject().GetPurgeAt() != nil && !account.GetSubject().GetPurgeAt().AsTime().After(seal.GetCut().AsTime()) {
			return ErrCore
		}
	}
	digest, err := CensusDigest(census.GetAccounts())
	if err != nil || !bytes.Equal(digest[:], seal.GetCanonicalSha256()) {
		return ErrCore
	}
	return nil
}
