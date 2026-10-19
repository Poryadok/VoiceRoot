package automodwire

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func TestTerminalSealCanonicalSetAndFiniteCompleteness(t *testing.T) {
	ids := []string{"00000000-0000-4000-8000-000000000001", "80000000-0000-4000-8000-000000000002"}
	hash, err := TerminalDigest(ids)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(hash[:]) != "a9a0ec3fc5e87c551a11f7533f3c8eecade74bfaa63e030350b0d16471197c25" {
		t.Fatal("cross-language terminal hash contract changed")
	}
	snapshot := &automodv1.ListTerminalModerationSubjectsResponse{Complete: true, SubjectIds: ids, Seal: &automodv1.ModerationAuthoritySnapshotSeal{ProtocolVersion: 2, AuthorityGeneration: uuid.NewString(), SnapshotId: uuid.NewString(), Cut: timestamppb.New(time.Unix(1000, 0)), TerminalHighWater: 2, ItemCount: 2, CanonicalSha256: hash[:], Complete: true}}
	if err := ValidateTerminalSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	for _, defect := range []string{"missing", "incomplete", "count", "highwater", "legacy-union", "behind-water", "hash", "order", "duplicate", "generation", "unknown"} {
		t.Run(defect, func(t *testing.T) {
			bad := proto.Clone(snapshot).(*automodv1.ListTerminalModerationSubjectsResponse)
			switch defect {
			case "missing":
				bad.Seal = nil
			case "incomplete":
				bad.Seal.Complete = false
			case "count":
				bad.Seal.ItemCount = 1
			case "highwater":
				bad.Seal.TerminalHighWater = 3
			case "legacy-union":
				bad.Seal.ProtocolVersion = 1
			case "behind-water":
				bad.Seal.TerminalHighWater = 1
			case "hash":
				bad.Seal.CanonicalSha256[0] ^= 1
			case "order":
				bad.SubjectIds[0], bad.SubjectIds[1] = bad.SubjectIds[1], bad.SubjectIds[0]
			case "duplicate":
				bad.SubjectIds[1] = bad.SubjectIds[0]
			case "generation":
				bad.Seal.AuthorityGeneration = ""
			case "unknown":
				bad.Seal.ProtoReflect().SetUnknown([]byte{0x50, 0x01})
			}
			if ValidateTerminalSnapshot(bad) == nil {
				t.Fatal("accepted incomplete authority cut")
			}
		})
	}
	emptyHash, err := TerminalDigest(nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.SubjectIds = nil
	snapshot.Seal.ItemCount = 0
	snapshot.Seal.TerminalHighWater = 0
	snapshot.Seal.CanonicalSha256 = emptyHash[:]
	if err := ValidateTerminalSnapshot(snapshot); err != nil {
		t.Fatal("complete empty view must be explicit", err)
	}
}
