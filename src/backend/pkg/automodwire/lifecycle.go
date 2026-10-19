package automodwire

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math"

	"google.golang.org/protobuf/proto"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

const MaxLifecycleFactBytes = 1024
const lifecycleDomain = "voice.automod.lifecycle.v1\x00"

var ErrLifecycle = errors.New("invalid private Auth lifecycle fact or receipt")

// ValidateLifecycleFact authenticates no sender and asserts no currentness.
// The private receiver must verify Auth/mTLS and the current boot authority.
// A due DELETED fact remains nonterminal: only positive PURGED can fence it.
func ValidateLifecycleFact(fact *automodv1.AccountLifecycleFact) error {
	if fact == nil || RejectUnknown(fact) != nil || proto.Size(fact) > MaxLifecycleFactBytes ||
		fact.GetProtocolVersion() != 1 || !canonicalUUID(fact.GetAuthorityGeneration()) ||
		!canonicalUUID(fact.GetEventId()) || !canonicalUUID(fact.GetSubjectId()) {
		return ErrLifecycle
	}
	switch fact.GetState() {
	case automodv1.SubjectState_SUBJECT_STATE_LIVE:
		if fact.GetLifecycleRevision() > 0 && fact.GetLifecycleRevision() <= math.MaxInt64 && fact.DeletionCycleId == nil && fact.PurgeAt == nil {
			return nil
		}
	case automodv1.SubjectState_SUBJECT_STATE_DELETED:
		if fact.GetLifecycleRevision() > 0 && fact.GetLifecycleRevision() <= math.MaxInt64 &&
			canonicalUUID(fact.GetDeletionCycleId()) && fact.PurgeAt != nil && fact.PurgeAt.CheckValid() == nil && fact.PurgeAt.GetNanos()%1000 == 0 {
			return nil
		}
	case automodv1.SubjectState_SUBJECT_STATE_PURGED:
		if fact.GetLifecycleRevision() == 0 && fact.DeletionCycleId == nil && fact.PurgeAt == nil {
			return nil
		}
	}
	return ErrLifecycle
}

func EncodeLifecycleFact(fact *automodv1.AccountLifecycleFact) ([]byte, [32]byte, error) {
	if err := ValidateLifecycleFact(fact); err != nil {
		return nil, [32]byte{}, err
	}
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(fact)
	if err != nil {
		return nil, [32]byte{}, ErrLifecycle
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(lifecycleDomain))
	_, _ = hash.Write(body)
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return body, digest, nil
}

// DecodeLifecycleFact accepts only the canonical immutable representation.
func DecodeLifecycleFact(body []byte) (*automodv1.AccountLifecycleFact, [32]byte, error) {
	if len(body) == 0 || len(body) > MaxLifecycleFactBytes {
		return nil, [32]byte{}, ErrLifecycle
	}
	fact := &automodv1.AccountLifecycleFact{}
	if err := proto.Unmarshal(body, fact); err != nil {
		return nil, [32]byte{}, ErrLifecycle
	}
	canonical, digest, err := EncodeLifecycleFact(fact)
	if err != nil || !bytes.Equal(body, canonical) {
		return nil, [32]byte{}, ErrLifecycle
	}
	return fact, digest, nil
}

// ValidateLifecycleReceipt checks structural/request binding only. A newer
// revision/hash MUST be compared with CURRENT owner state by the sender before
// any intent is cleared. Terminal is acceptable only with its own positive
// owner ledger; this function cannot derive terminality from a peer or timer.
func ValidateLifecycleReceipt(request *automodv1.ApplyLifecycleFactRequest, receipt *automodv1.ApplyLifecycleFactResponse) error {
	if request == nil || RejectUnknown(request) != nil || ValidateLifecycleFact(request.GetFact()) != nil ||
		receipt == nil || RejectUnknown(receipt) != nil || proto.Size(receipt) > MaxLifecycleFactBytes {
		return ErrLifecycle
	}
	fact := request.GetFact()
	if receipt.GetProtocolVersion() != 1 || receipt.GetAuthorityGeneration() != fact.GetAuthorityGeneration() ||
		receipt.GetSubjectId() != fact.GetSubjectId() || receipt.GetRequestedEventId() != fact.GetEventId() {
		return ErrLifecycle
	}
	if receipt.GetTerminal() {
		if receipt.GetLifecycleRevision() != 0 || len(receipt.GetCurrentFactSha256()) != 0 {
			return ErrLifecycle
		}
		return nil
	}
	if fact.GetState() == automodv1.SubjectState_SUBJECT_STATE_PURGED || receipt.GetLifecycleRevision() < fact.GetLifecycleRevision() ||
		receipt.GetLifecycleRevision() > math.MaxInt64 || len(receipt.GetCurrentFactSha256()) != sha256.Size {
		return ErrLifecycle
	}
	if receipt.GetLifecycleRevision() == fact.GetLifecycleRevision() {
		_, expected, err := EncodeLifecycleFact(fact)
		if err != nil || !bytes.Equal(expected[:], receipt.GetCurrentFactSha256()) {
			return ErrLifecycle
		}
	}
	return nil
}
