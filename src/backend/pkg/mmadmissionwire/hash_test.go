package mmadmissionwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func testBinding() Binding {
	return Binding{ProtocolVersion: 1, EnvironmentID: ID{1}, InstallationID: ID{2}, AuthorityIncarnation: ID{3}, TransitionID: ID{4}, AdmissionGeneration: 5, CredentialCohortID: ID{6}, DatabaseBindingHash: Hash{7}, SchemaContractHash: Hash{8}, SourceFactoryManifestHash: Hash{9}, FreshRouteManifestHash: Hash{10}, AdmissionReceiptHash: Hash{11}, LaunchManifestHash: Hash{12}, ExpectedLoginRoleOID: 13, ExpectedLoginRoleName: "mm_serving"}
}
func TestBindingGoldenGrammar(t *testing.T) {
	b := testBinding()
	var golden bytes.Buffer
	golden.WriteString("voice.mm.admission.binding.v1\x00")
	_ = binary.Write(&golden, binary.BigEndian, uint32(1))
	for _, n := range []byte{1, 2, 3, 4} {
		golden.WriteByte(n)
		golden.Write(make([]byte, 15))
	}
	_ = binary.Write(&golden, binary.BigEndian, uint64(5))
	golden.WriteByte(6)
	golden.Write(make([]byte, 15))
	for _, n := range []byte{7, 8, 9, 10, 11, 12} {
		golden.WriteByte(n)
		golden.Write(make([]byte, 31))
	}
	_ = binary.Write(&golden, binary.BigEndian, uint32(13))
	_ = binary.Write(&golden, binary.BigEndian, uint32(10))
	golden.WriteString("mm_serving")
	raw, e := BindingBytes(b)
	if e != nil || !bytes.Equal(raw, golden.Bytes()) {
		t.Fatal("binding grammar mismatch", e)
	}
	h, e := BindingHash(b)
	if e != nil || h != sha256.Sum256(golden.Bytes()) {
		t.Fatal("binding digest mismatch", e)
	}
	t.Log("golden", h.String())
	encoded, e := json.Marshal(b)
	if e != nil {
		t.Fatal(e)
	}
	var copy Binding
	if e = Decode(encoded, &copy); e != nil || copy != b {
		t.Fatal("roundtrip", e)
	}
}
func TestCanonicalIdentifiersAndNumbers(t *testing.T) {
	for _, s := range []string{`"01000000-0000-0000-0000-00000000000A"`, `"bad"`, `null`} {
		var id ID
		if json.Unmarshal([]byte(s), &id) == nil {
			t.Errorf("accepted ID %s", s)
		}
	}
	var h Hash
	if json.Unmarshal([]byte(`"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"`), &h) == nil {
		t.Fatal("upper hash")
	}
	b := testBinding()
	raw, _ := json.Marshal(b)
	bad := bytes.Replace(raw, []byte(`"admission_generation":"5"`), []byte(`"admission_generation":5`), 1)
	if Decode(bad, &b) == nil {
		t.Fatal("numeric uint64 accepted")
	}
}
func TestReceiptHashIsAcyclic(t *testing.T) {
	b := testBinding()
	r := ArmedReceiptV1{Version: 1, AuthorityScope: LocalOwned, ScopeID: ID{1}, ControllerID: ID{2}, TransitionID: b.TransitionID, DatabaseLineageID: ID{3}, Binding: b, LaunchSpecHash: b.LaunchManifestHash, ClosureEvidenceHash: Hash{1}, PreparationEvidenceHash: Hash{2}}
	h, e := ArmedHash(r)
	if e != nil {
		t.Fatal(e)
	}
	r.Binding.AdmissionReceiptHash = h
	h2, e := ArmedHash(r)
	if e != nil || h2 != h {
		t.Fatal("receipt hashes itself", e)
	}
}
