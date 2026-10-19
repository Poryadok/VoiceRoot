package automodwire

import (
	"bytes"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func validCore() *automodv1.CommittedAccountCore {
	return &automodv1.CommittedAccountCore{ProtocolVersion: Version,
		Subject:     &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: uuid.NewString(), SubjectId: uuid.NewString(), LifecycleRevision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE},
		SourceEpoch: uuid.NewString(), Sequence: 1, EventId: uuid.NewString(), RecordedAt: timestamppb.New(time.Unix(1000, 123000)),
		Eligibility: automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_ELIGIBLE,
		Digests:     []*automodv1.PatternDigest{{KeyId: "current", HmacSha256: bytes.Repeat([]byte{1}, 32)}, {KeyId: "next", HmacSha256: bytes.Repeat([]byte{2}, 32)}},
		Producer:    automodv1.AccountCoreProducer_ACCOUNT_CORE_PRODUCER_USER}
}

func TestCoreCanonicalHashAndStrictDecode(t *testing.T) {
	core := validCore()
	payload, hash, err := EncodeCore(core)
	if err != nil {
		t.Fatal(err)
	}
	decoded, roundTrip, err := DecodeCore(payload)
	if err != nil || !proto.Equal(core, decoded) || hash != roundTrip {
		t.Fatalf("round trip failed: %v", err)
	}
	if hash == sha256.Sum256(payload) {
		t.Fatal("core hash must include the protocol domain")
	}
	changed := proto.Clone(core).(*automodv1.CommittedAccountCore)
	changed.EventId = uuid.NewString()
	_, changedHash, err := EncodeCore(changed)
	if err != nil || changedHash == hash {
		t.Fatal("different committed identity reused hash")
	}
	// A duplicate known field decodes to the same value but is not canonical storage.
	duplicate := protowire.AppendTag(append([]byte{}, payload...), 1, protowire.VarintType)
	duplicate = protowire.AppendVarint(duplicate, Version)
	if _, _, err := DecodeCore(duplicate); err == nil {
		t.Fatal("accepted noncanonical bytes")
	}
}

func TestCoreRejectsOriginSmugglingAtEveryNestedLevel(t *testing.T) {
	origin := []byte(uuid.NewString())
	unknown := protowire.AppendTag(nil, 100, protowire.BytesType)
	unknown = protowire.AppendBytes(unknown, origin)
	for _, level := range []string{"core", "subject", "recorded", "digest", "purge"} {
		t.Run(level, func(t *testing.T) {
			core := validCore()
			var target proto.Message
			switch level {
			case "core":
				target = core
			case "subject":
				target = core.Subject
			case "recorded":
				target = core.RecordedAt
			case "digest":
				target = core.Digests[0]
			case "purge":
				core.Subject.State = automodv1.SubjectState_SUBJECT_STATE_DELETED
				cycle := uuid.NewString()
				core.Subject.DeletionCycleId = &cycle
				core.Subject.PurgeAt = timestamppb.New(time.Unix(2000, 0))
				target = core.Subject.PurgeAt
			}
			target.ProtoReflect().SetUnknown(unknown)
			if _, _, err := EncodeCore(core); err == nil {
				t.Fatal("accepted origin-bearing unknown bytes")
			}
			payload, err := proto.Marshal(core)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := DecodeCore(payload); err == nil {
				t.Fatal("decoded unknown bytes")
			}
		})
	}
}

func TestCoreRejectsUnsupportedAuthorityAndPartialRing(t *testing.T) {
	for _, kind := range []string{"v1", "producer", "bot", "user", "partial", "duplicate", "empty", "timestamp", "metadata"} {
		t.Run(kind, func(t *testing.T) {
			core := validCore()
			switch kind {
			case "v1":
				core.ProtocolVersion = 1
			case "producer":
				core.Producer = 99
			case "bot":
				core.Producer = automodv1.AccountCoreProducer_ACCOUNT_CORE_PRODUCER_BOT
			case "user":
				core.OwnershipRevision = 1
			case "partial":
				core.Digests = core.Digests[:1]
			case "duplicate":
				core.Digests[1].KeyId = core.Digests[0].KeyId
			case "empty":
				core.Digests[0].HmacSha256 = make([]byte, 32)
			case "timestamp":
				core.RecordedAt.Nanos++
			case "metadata":
				core.Eligibility = automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_E2E
			}
			if _, _, err := EncodeCore(core); err == nil {
				t.Fatal("accepted invalid core")
			}
		})
	}
	core := validCore()
	core.Producer = automodv1.AccountCoreProducer_ACCOUNT_CORE_PRODUCER_BOT
	core.OwnershipRevision = 2
	if err := ValidateCore(core); err != nil {
		t.Fatal(err)
	}
	core.Eligibility = automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_E2E
	core.Digests = nil
	if err := ValidateCore(core); err != nil {
		t.Fatal(err)
	}
}

func TestExpiredReceiptBindsOriginalCoreAndRejectsUnknown(t *testing.T) {
	core := validCore()
	_, hash, err := EncodeCore(core)
	if err != nil {
		t.Fatal(err)
	}
	receipt := &automodv1.ExpiredAccountCore{ProtocolVersion: Version, Subject: core.Subject, SourceEpoch: core.SourceEpoch, Sequence: core.Sequence, EventId: core.EventId, RecordedAt: core.RecordedAt, OriginalCoreSha256: hash[:]}
	if err := ValidateExpired(receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Subject.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 100, protowire.VarintType), 1))
	if err := ValidateExpired(receipt); err == nil {
		t.Fatal("accepted hidden receipt authority")
	}
	receipt.Subject.ProtoReflect().SetUnknown(nil)
	receipt.OriginalCoreSha256 = make([]byte, 32)
	if err := ValidateExpired(receipt); err == nil {
		t.Fatal("accepted empty original core commitment")
	}
}
