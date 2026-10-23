package store

import (
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

func frozenMembership(t *testing.T) MembershipRecord {
	t.Helper()
	r := MembershipRecord{SpaceID: uuid.New(), BotID: uuid.New(), InstallationID: uuid.New(), ActorProfileID: uuid.New(), AddOperationID: uuid.New(), CancelOperationID: uuid.New(), Generation: 1}
	var err error
	r.AddBytes, err = proto.MarshalOptions{Deterministic: true}.Marshal(&spacev1.AddBotMemberRequest{SpaceId: r.SpaceID.String(), ProfileId: r.ActorProfileID.String(), InstallationIntent: r.intent(r.AddOperationID)})
	if err != nil {
		t.Fatal(err)
	}
	r.CancelBytes, err = proto.MarshalOptions{Deterministic: true}.Marshal(&spacev1.RemoveBotMemberRequest{SpaceId: r.SpaceID.String(), ProfileId: r.ActorProfileID.String(), InstallationIntent: r.intent(r.CancelOperationID), InstallOperationId: r.AddOperationID.String()})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestMembershipFrozenWirePreservesIndependentOperationRoles(t *testing.T) {
	r := frozenMembership(t)
	add, err := r.AddRequest()
	if err != nil {
		t.Fatal(err)
	}
	remove, err := r.RemoveRequest()
	if err != nil {
		t.Fatal(err)
	}
	if add.GetInstallationIntent().GetOperationId() == remove.GetInstallationIntent().GetOperationId() || remove.GetInstallOperationId() != r.AddOperationID.String() {
		t.Fatal("operation roles exchanged")
	}
	// Corruption cannot turn the original add operation into its own cancellation.
	r.CancelOperationID = r.AddOperationID
	remove.InstallationIntent.OperationId = r.AddOperationID.String()
	r.CancelBytes, _ = proto.Marshal(remove)
	if _, err = r.RemoveRequest(); err == nil {
		t.Fatal("ambiguous add/cancel operation accepted")
	}
}

func TestMembershipFrozenWireRejectsUnknownAndMismatchedActor(t *testing.T) {
	for _, kind := range []string{"actor", "unknown", "zero"} {
		t.Run(kind, func(t *testing.T) {
			r := frozenMembership(t)
			req, _ := r.AddRequest()
			switch kind {
			case "actor":
				req.ProfileId = uuid.NewString()
			case "unknown":
				req.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
			case "zero":
				r.InstallationID = uuid.Nil
				req.InstallationIntent.InstallationId = uuid.Nil.String()
			}
			r.AddBytes, _ = proto.Marshal(req)
			if _, err := r.AddRequest(); err == nil {
				t.Fatal("corrupt immutable binding accepted")
			}
		})
	}
}
