package control

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// Model-only protected readers. No fixture record is deployed authority,
// supervisor capability or evidence of actual network/process/media fencing.
func id(n byte) ID {
	const hex = "0123456789abcdef"
	return ID("11111111-1111-4111-8111-1111111111" + string([]byte{hex[n>>4], hex[n&15]}))
}
func ref(issuer, record byte) Ref { return Ref{id(issuer), id(record), Digest{record}} }
func fixtureClose() (CloseIntent, FenceReceipt) {
	owner := LaunchIntent{Ref: ref(1, 2), Binding: OwnerBinding{Authority: id(1), Resource: id(3), Incarnation: id(4), LaunchOperation: id(2), ControlRevision: 1, ImageDigest: Digest{5}}, Supervisor: id(6), Profile: id(7)}
	intent := CloseIntent{Ref: ref(1, 8), Authority: id(1), Resource: id(3), Operation: id(8), SealingRevision: 2, Owners: []LaunchIntent{owner}}
	intent.OwnerCensusHash, _ = Hash("voice.matchsquad.owner-census.v1", intent.Owners)
	receipt := FenceReceipt{Ref: ref(6, 9), Close: intent.Ref, Launch: owner.Ref, Binding: owner.Binding, Profile: owner.Profile, SealBarrier: ref(6, 10), Runtimes: []RuntimeFence{{Runtime: RuntimeIdentity{Host: id(11), Boot: id(12), RuntimeUID: "immutable-runtime-uid", AttachmentUID: "exclusive-attachment-uid", ImageDigest: owner.Binding.ImageDigest}, ForwardingFence: ref(6, 13), Termination: ref(6, 14), Quiescence: ref(6, 15)}}}
	receipt.ProfileWitness = ref(6, 21)
	receipt.BirthCensus = ref(6, 22)
	sealFixtureReceipt(&receipt)
	return intent, receipt
}
func sealFixtureReceipt(f *FenceReceipt) {
	f.RuntimeCensusHash, _ = Hash("voice.matchsquad.runtime-census.v1", f.Runtimes)
	f.CanonicalBytes = nil
	f.Ref.Hash = Digest{}
	raw, _ := json.Marshal(*f)
	hash, _ := Hash("voice.matchsquad.fence-receipt.v1", *f)
	f.CanonicalBytes = raw
	f.Ref.Hash = hash
}
func TestFenceReceiptRejectsSubstitutedReferenceHash(t *testing.T) {
	c, f := fixtureClose()
	if err := f.Validate(c, c.Owners[0]); err != nil {
		t.Fatal(err)
	}
	f.Ref.Hash[0] ^= 1
	if err := f.Validate(c, c.Owners[0]); !errors.Is(err, ErrConflict) {
		t.Fatal("changed receipt hash must conflict")
	}
}

type authorityFixture struct {
	intent     CloseIntent
	records    map[Ref]Ref
	confirmErr error
	commits    int
}

func (f *authorityFixture) ReadCloseIntent(_ context.Context, r Ref) (CloseIntent, error) {
	if r != f.intent.Ref {
		return CloseIntent{}, ErrConflict
	}
	return f.intent, nil
}
func (f *authorityFixture) RecordWorkerFence(_ context.Context, close, launch, fence Ref) error {
	if close != f.intent.Ref {
		return ErrConflict
	}
	if prior, ok := f.records[launch]; ok && prior != fence {
		return ErrConflict
	}
	f.records[launch] = fence
	return nil
}
func (f *authorityFixture) ConfirmClose(_ context.Context, r Ref) (Ref, error) {
	if r != f.intent.Ref {
		return Ref{}, ErrConflict
	}
	if f.confirmErr != nil {
		return Ref{}, f.confirmErr
	}
	for _, owner := range f.intent.Owners {
		if !f.records[owner.Ref].Valid() {
			return Ref{}, ErrPending
		}
	}
	f.commits++
	return ref(1, 16), nil
}
func (f *authorityFixture) ReadCloseAcknowledgement(_ context.Context, r Ref) (CloseAcknowledgement, error) {
	ack := CloseAcknowledgement{Ref: r, Close: f.intent.Ref, Resource: f.intent.Resource, SealingRevision: f.intent.SealingRevision, OwnerCensusHash: f.intent.OwnerCensusHash, Absence: ref(1, 17), WorkerFences: []Ref{}}
	for _, o := range f.intent.Owners {
		ack.WorkerFences = append(ack.WorkerFences, f.records[o.Ref])
	}
	return ack, nil
}

type supervisorFixture struct {
	receipt    FenceReceipt
	lostReply  bool
	calls      int
	inspectErr error
}

func (f *supervisorFixture) FenceWorker(_ context.Context, close, launch Ref) (Ref, error) {
	f.calls++
	if close != f.receipt.Close || launch != f.receipt.Launch {
		return Ref{}, ErrConflict
	}
	if f.lostReply {
		f.lostReply = false
		return Ref{}, ErrUnavailable
	}
	return f.receipt.Ref, nil
}
func (f *supervisorFixture) InspectFence(_ context.Context, r Ref) (FenceReceipt, error) {
	if f.inspectErr != nil {
		return FenceReceipt{}, f.inspectErr
	}
	if r != f.receipt.Ref {
		return FenceReceipt{}, ErrConflict
	}
	return f.receipt, nil
}

func TestCloseLostFenceReplyKeepsNativeCompletionPending(t *testing.T) {
	intent, receipt := fixtureClose()
	a := &authorityFixture{intent: intent, records: map[Ref]Ref{}}
	s := &supervisorFixture{receipt: receipt, lostReply: true}
	c, err := New(a, map[ID]Supervisor{id(6): s})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Close(context.Background(), intent.Ref); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if a.commits != 0 || len(a.records) != 0 {
		t.Fatal("lost reply cannot complete native close")
	}
	ack, err := c.Close(context.Background(), intent.Ref)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := c.Close(context.Background(), intent.Ref)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := json.Marshal(ack)
	y, _ := json.Marshal(retry)
	if string(x) != string(y) {
		t.Fatal("replay changed acknowledgement")
	}
	if s.calls != 3 {
		t.Fatal("reference retries must re-observe real supervisor receipt")
	}
}

func TestCloseRejectsIncompleteOrWrongWorkerEvidence(t *testing.T) {
	for _, kind := range []string{"wrong_incarnation", "missing_termination", "wrong_profile", "duplicate_runtime", "missing_supervisor", "unobserved_receipt", "projection_failure", "missing_birth_census", "missing_profile_witness"} {
		t.Run(kind, func(t *testing.T) {
			intent, receipt := fixtureClose()
			a := &authorityFixture{intent: intent, records: map[Ref]Ref{}}
			s := &supervisorFixture{receipt: receipt}
			supervisors := map[ID]Supervisor{id(6): s}
			switch kind {
			case "wrong_incarnation":
				s.receipt.Binding.Incarnation = id(18)
			case "missing_termination":
				s.receipt.Runtimes[0].Termination = Ref{}
			case "wrong_profile":
				s.receipt.Profile = id(19)
			case "duplicate_runtime":
				s.receipt.Runtimes = append(s.receipt.Runtimes, s.receipt.Runtimes[0])
			case "missing_supervisor":
				supervisors = map[ID]Supervisor{id(20): s}
			case "unobserved_receipt":
				s.inspectErr = ErrUnavailable
			case "projection_failure":
				a.confirmErr = ErrPending
			case "missing_birth_census":
				s.receipt.BirthCensus = Ref{}
			case "missing_profile_witness":
				s.receipt.ProfileWitness = Ref{}
			}
			sealFixtureReceipt(&s.receipt)
			c, err := New(a, supervisors)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = c.Close(context.Background(), intent.Ref); err == nil {
				t.Fatal("invalid/unconfirmed fence completed close")
			}
			if a.commits != 0 {
				t.Fatal("invalid evidence must not commit native completion")
			}
		})
	}
}
