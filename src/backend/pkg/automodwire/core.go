// Package automodwire validates the minimized private account-core protocol.
// No origin identifier or original full-envelope commitment belongs here.
package automodwire

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math"
	"regexp"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

const Version = 2
const MaxCoreBytes = 16384
const coreDomain = "voice.automod.account-core.v2\x00"

var ErrCore = errors.New("invalid or unsupported minimized automod account core")
var keyID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// RejectUnknown includes every nested message/list/map. Unknown protobuf bytes
// cannot carry origin IDs, full-envelope hashes, or alternate identity fields.
func RejectUnknown(message proto.Message) error {
	if message == nil || !message.ProtoReflect().IsValid() || hasUnknown(message.ProtoReflect()) {
		return ErrCore
	}
	return nil
}
func hasUnknown(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsMap() {
			if field.MapValue().Kind() == protoreflect.MessageKind {
				value.Map().Range(func(_ protoreflect.MapKey, v protoreflect.Value) bool {
					unknown = hasUnknown(v.Message())
					return !unknown
				})
			}
		}
		if field.IsList() {
			if field.Kind() == protoreflect.MessageKind {
				list := value.List()
				for i := 0; i < list.Len(); i++ {
					if hasUnknown(list.Get(i).Message()) {
						unknown = true
						break
					}
				}
			}
		} else if !field.IsMap() && field.Kind() == protoreflect.MessageKind {
			unknown = hasUnknown(value.Message())
		}
		return !unknown
	})
	return unknown
}
func canonicalUUID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}
func validIdentity(version uint32, subject *automodv1.SubjectBinding, epoch string, sequence uint64, event string, recorded *timestamppb.Timestamp) bool {
	if version != Version || subject == nil || subject.GetProtocolVersion() != 1 || !canonicalUUID(subject.GetAccountId()) || !canonicalUUID(subject.GetSubjectId()) || subject.GetLifecycleRevision() == 0 || subject.GetLifecycleRevision() > math.MaxInt64 || !canonicalUUID(epoch) || sequence == 0 || sequence > math.MaxInt64 || !canonicalUUID(event) || recorded == nil || recorded.CheckValid() != nil || recorded.GetNanos()%1000 != 0 {
		return false
	}
	switch subject.GetState() {
	case automodv1.SubjectState_SUBJECT_STATE_LIVE:
		return subject.PurgeAt == nil && subject.DeletionCycleId == nil
	case automodv1.SubjectState_SUBJECT_STATE_DELETED:
		return subject.GetPurgeAt() != nil && subject.GetPurgeAt().CheckValid() == nil && canonicalUUID(subject.GetDeletionCycleId())
	default:
		return false
	}
}
func ValidateCore(core *automodv1.CommittedAccountCore) error {
	if core == nil || RejectUnknown(core) != nil || proto.Size(core) > MaxCoreBytes || !validIdentity(core.GetProtocolVersion(), core.GetSubject(), core.GetSourceEpoch(), core.GetSequence(), core.GetEventId(), core.GetRecordedAt()) {
		return ErrCore
	}
	switch core.GetProducer() {
	case automodv1.AccountCoreProducer_ACCOUNT_CORE_PRODUCER_USER:
		if core.GetOwnershipRevision() != 0 {
			return ErrCore
		}
	case automodv1.AccountCoreProducer_ACCOUNT_CORE_PRODUCER_BOT:
		if core.GetOwnershipRevision() == 0 || core.GetOwnershipRevision() > math.MaxInt64 {
			return ErrCore
		}
	default:
		return ErrCore
	}
	if core.GetEligibility() == automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_ELIGIBLE {
		if len(core.GetDigests()) < 2 || len(core.GetDigests()) > 4 {
			return ErrCore
		}
		seen := map[string]bool{}
		for _, digest := range core.GetDigests() {
			if digest == nil || !keyID.MatchString(digest.GetKeyId()) || seen[digest.GetKeyId()] || len(digest.GetHmacSha256()) != 32 || bytes.Equal(digest.GetHmacSha256(), make([]byte, 32)) {
				return ErrCore
			}
			seen[digest.GetKeyId()] = true
		}
		return nil
	}
	if (core.GetEligibility() == automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_NO_TEXT || core.GetEligibility() == automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_E2E) && len(core.GetDigests()) == 0 && core.GetLinkCount() == 0 {
		return nil
	}
	return ErrCore
}
func EncodeCore(core *automodv1.CommittedAccountCore) ([]byte, [32]byte, error) {
	if err := ValidateCore(core); err != nil {
		return nil, [32]byte{}, err
	}
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(core)
	if err != nil {
		return nil, [32]byte{}, ErrCore
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(coreDomain))
	_, _ = hash.Write(payload)
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return payload, digest, nil
}
func DecodeCore(payload []byte) (*automodv1.CommittedAccountCore, [32]byte, error) {
	if len(payload) == 0 || len(payload) > MaxCoreBytes {
		return nil, [32]byte{}, ErrCore
	}
	var core automodv1.CommittedAccountCore
	if err := proto.Unmarshal(payload, &core); err != nil {
		return nil, [32]byte{}, ErrCore
	}
	canonical, hash, err := EncodeCore(&core)
	if err != nil || !bytes.Equal(canonical, payload) {
		return nil, [32]byte{}, ErrCore
	}
	return &core, hash, nil
}
func ValidateExpired(receipt *automodv1.ExpiredAccountCore) error {
	if receipt == nil || RejectUnknown(receipt) != nil || !validIdentity(receipt.GetProtocolVersion(), receipt.GetSubject(), receipt.GetSourceEpoch(), receipt.GetSequence(), receipt.GetEventId(), receipt.GetRecordedAt()) || len(receipt.GetOriginalCoreSha256()) != 32 || bytes.Equal(receipt.GetOriginalCoreSha256(), make([]byte, 32)) {
		return ErrCore
	}
	return nil
}
