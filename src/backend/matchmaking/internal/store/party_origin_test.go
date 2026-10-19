package store

import (
	"encoding/hex"
	"github.com/google/uuid"
	"testing"
)

func TestNativePrivateManifestGolden(t *testing.T) {
	room := uuid.UUID{5}
	p := SourceParty{Party: Party{LeaderProfileID: uuid.UUID{4}}, SearchOperationID: uuid.UUID{1}, SearchRequestHash: [32]byte{2}, InitiatorAccountID: uuid.UUID{3}, InitiatorSessionEpoch: 7, ProtocolVersion: 1, SourceKind: "VOICE_ROSTER", SourceRoomID: &room, SourceRosterVersion: 9, MemberManifestHash: [32]byte{6}, MemberAuthorityVersion: 1, MemberAuthorityHash: [32]byte{8}, SourceAuthorityHash: [32]byte{9}}
	got := nativePrivateManifest(p)
	// Independently encoded using Python struct.pack fixed-width BE grammar.
	if hex.EncodeToString(got[:]) != "aa75f1ac650c34b11440287e23c1d0ad5072ea1abbce772b01bec550e94374d8" {
		t.Fatal("native private manifest grammar changed")
	}
	changed := p
	changed.SourceKind = "SOLO"
	if nativePrivateManifest(changed) == got {
		t.Fatal("source kind not bound")
	}
	changed = p
	changed.MemberAuthorityHash[0]++
	if nativePrivateManifest(changed) == got {
		t.Fatal("private account authority not bound")
	}
	changed = p
	changed.SourceAuthorityHash[0]++
	if nativePrivateManifest(changed) == got {
		t.Fatal("captured source cut not bound")
	}
}
