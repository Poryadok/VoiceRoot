package automodwire

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func lifecycleFixture() *automodv1.AccountLifecycleFact {
	return &automodv1.AccountLifecycleFact{ProtocolVersion: 1,
		AuthorityGeneration: "00000000-0000-4000-8000-000000000001", EventId: "00000000-0000-4000-8000-000000000002",
		SubjectId: "80000000-0000-4000-8000-000000000003", State: automodv1.SubjectState_SUBJECT_STATE_LIVE, LifecycleRevision: 7}
}

func TestLifecycleCanonicalOwnerFactAndIndependentVector(t *testing.T) {
	fact := lifecycleFixture()
	body, hash, err := EncodeLifecycleFact(fact)
	if err != nil || len(body) != 120 || hex.EncodeToString(hash[:]) != "53043311ff246eba73d39ef71db56fba6a1d299e7fae115fb9ccfcec8190655a" {
		t.Fatalf("independently hand-framed owner vector: bytes=%d digest=%x err=%v", len(body), hash, err)
	}
	decoded, actual, err := DecodeLifecycleFact(body)
	if err != nil || !proto.Equal(decoded, fact) || actual != hash {
		t.Fatal("canonical owner fact did not round-trip")
	}
	// Even equal protobuf values with duplicate fields are not a second immutable
	// representation of the event. Raw/private fields cannot hide in unknowns.
	duplicate := append(append([]byte(nil), body...), 0x08, 0x01)
	if _, _, err := DecodeLifecycleFact(duplicate); err == nil {
		t.Fatal("duplicate representation accepted")
	}
	unknown := protowire.AppendString(protowire.AppendTag(nil, 99, protowire.BytesType), "raw-account-or-content")
	if _, _, err := DecodeLifecycleFact(append(append([]byte(nil), body...), unknown...)); err == nil {
		t.Fatal("unknown private carrier accepted")
	}
	if _, _, err := DecodeLifecycleFact(bytes.Repeat([]byte{1}, MaxLifecycleFactBytes+1)); err == nil {
		t.Fatal("unbounded fact accepted")
	}
}

func TestLifecycleDeletedIsNotTerminalEvenWhenItsDeadlineHasPassed(t *testing.T) {
	fact := lifecycleFixture()
	fact.State = automodv1.SubjectState_SUBJECT_STATE_DELETED
	fact.DeletionCycleId = proto.String("00000000-0000-4000-8000-000000000004")
	fact.PurgeAt = timestamppb.New(time.Unix(1, 1000))
	if err := ValidateLifecycleFact(fact); err != nil {
		t.Fatal("a due owner schedule must remain representable without inventing terminality", err)
	}
	if fact.GetState() == automodv1.SubjectState_SUBJECT_STATE_PURGED {
		t.Fatal("validation terminalized a stale schedule")
	}
	fact.State = automodv1.SubjectState_SUBJECT_STATE_PURGED
	if ValidateLifecycleFact(fact) == nil {
		t.Fatal("terminal fact invented a revision/cycle/deadline")
	}
	fact.LifecycleRevision, fact.DeletionCycleId, fact.PurgeAt = 0, nil, nil
	if err := ValidateLifecycleFact(fact); err != nil {
		t.Fatal("positive ledger fact must not invent erased raw lifecycle fields", err)
	}
}

func TestLifecycleInvalidOwnerBindings(t *testing.T) {
	cases := map[string]func(*automodv1.AccountLifecycleFact){
		"version":               func(f *automodv1.AccountLifecycleFact) { f.ProtocolVersion = 2 },
		"generation":            func(f *automodv1.AccountLifecycleFact) { f.AuthorityGeneration = "" },
		"nil UUID":              func(f *automodv1.AccountLifecycleFact) { f.SubjectId = "00000000-0000-0000-0000-000000000000" },
		"alternate UUID":        func(f *automodv1.AccountLifecycleFact) { f.EventId = "00000000000040008000000000000002" },
		"revision zero":         func(f *automodv1.AccountLifecycleFact) { f.LifecycleRevision = 0 },
		"revision overflow":     func(f *automodv1.AccountLifecycleFact) { f.LifecycleRevision = math.MaxUint64 },
		"unknown state":         func(f *automodv1.AccountLifecycleFact) { f.State = 99 },
		"missing state":         func(f *automodv1.AccountLifecycleFact) { f.State = 0 },
		"live empty cycle":      func(f *automodv1.AccountLifecycleFact) { f.DeletionCycleId = proto.String("") },
		"live deadline":         func(f *automodv1.AccountLifecycleFact) { f.PurgeAt = timestamppb.New(time.Unix(1, 0)) },
		"deleted missing tuple": func(f *automodv1.AccountLifecycleFact) { f.State = automodv1.SubjectState_SUBJECT_STATE_DELETED },
		"deleted nonmicrosecond": func(f *automodv1.AccountLifecycleFact) {
			f.State = automodv1.SubjectState_SUBJECT_STATE_DELETED
			f.DeletionCycleId = proto.String("00000000-0000-4000-8000-000000000004")
			f.PurgeAt = timestamppb.New(time.Unix(1, 1))
		},
		"unknown nested": func(f *automodv1.AccountLifecycleFact) { f.ProtoReflect().SetUnknown([]byte{0x80, 0x06, 0x01}) },
	}
	if ValidateLifecycleFact(nil) == nil {
		t.Fatal("nil fact accepted")
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			fact := lifecycleFixture()
			mutate(fact)
			if ValidateLifecycleFact(fact) == nil {
				t.Fatal("invalid owner fact accepted")
			}
		})
	}
}

func TestLifecycleReceiptBindsTheExactRequestWithoutCertifyingOwnerCurrentness(t *testing.T) {
	fact := lifecycleFixture()
	_, digest, _ := EncodeLifecycleFact(fact)
	request := &automodv1.ApplyLifecycleFactRequest{Fact: fact}
	receipt := &automodv1.ApplyLifecycleFactResponse{ProtocolVersion: 1, AuthorityGeneration: fact.GetAuthorityGeneration(),
		SubjectId: fact.GetSubjectId(), RequestedEventId: fact.GetEventId(), LifecycleRevision: fact.GetLifecycleRevision(), CurrentFactSha256: digest[:]}
	if err := ValidateLifecycleReceipt(request, receipt); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*automodv1.ApplyLifecycleFactResponse){
		"version":                 func(r *automodv1.ApplyLifecycleFactResponse) { r.ProtocolVersion = 2 },
		"generation":              func(r *automodv1.ApplyLifecycleFactResponse) { r.AuthorityGeneration = fact.GetEventId() },
		"subject":                 func(r *automodv1.ApplyLifecycleFactResponse) { r.SubjectId = fact.GetEventId() },
		"event":                   func(r *automodv1.ApplyLifecycleFactResponse) { r.RequestedEventId = fact.GetSubjectId() },
		"lower revision":          func(r *automodv1.ApplyLifecycleFactResponse) { r.LifecycleRevision-- },
		"overflow":                func(r *automodv1.ApplyLifecycleFactResponse) { r.LifecycleRevision = math.MaxUint64 },
		"missing hash":            func(r *automodv1.ApplyLifecycleFactResponse) { r.CurrentFactSha256 = nil },
		"equal changed bytes":     func(r *automodv1.ApplyLifecycleFactResponse) { r.CurrentFactSha256 = bytes.Repeat([]byte{1}, 32) },
		"terminal keeps revision": func(r *automodv1.ApplyLifecycleFactResponse) { r.Terminal = true; r.CurrentFactSha256 = nil },
		"terminal keeps hash":     func(r *automodv1.ApplyLifecycleFactResponse) { r.Terminal = true; r.LifecycleRevision = 0 },
		"unknown":                 func(r *automodv1.ApplyLifecycleFactResponse) { r.ProtoReflect().SetUnknown([]byte{0x80, 0x06, 0x01}) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := proto.Clone(receipt).(*automodv1.ApplyLifecycleFactResponse)
			mutate(r)
			if ValidateLifecycleReceipt(request, r) == nil {
				t.Fatal("unbound receipt accepted")
			}
		})
	}
	newer := proto.Clone(receipt).(*automodv1.ApplyLifecycleFactResponse)
	newer.LifecycleRevision++
	if err := ValidateLifecycleReceipt(request, newer); err != nil {
		t.Fatal("typed ahead response must reach current-owner comparison", err)
	}
	terminal := proto.Clone(receipt).(*automodv1.ApplyLifecycleFactResponse)
	terminal.Terminal, terminal.LifecycleRevision, terminal.CurrentFactSha256 = true, 0, nil
	if err := ValidateLifecycleReceipt(request, terminal); err != nil {
		t.Fatal("positive fence response must reach owner-ledger comparison", err)
	}
	purged := proto.Clone(request).(*automodv1.ApplyLifecycleFactRequest)
	purged.Fact.State, purged.Fact.LifecycleRevision = automodv1.SubjectState_SUBJECT_STATE_PURGED, 0
	if ValidateLifecycleReceipt(purged, receipt) == nil {
		t.Fatal("nonterminal receipt cancelled positive owner terminal")
	}
	request.ProtoReflect().SetUnknown([]byte{0x80, 0x06, 0x01})
	if ValidateLifecycleReceipt(request, receipt) == nil {
		t.Fatal("unknown request carrier accepted")
	}
}
