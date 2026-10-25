package mmadmissionwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"unicode/utf8"
)

type encoder struct {
	bytes.Buffer
	err error
}

func newEncoder(domain string) *encoder {
	e := &encoder{}
	e.WriteString(domain)
	e.WriteByte(0)
	return e
}
func (e *encoder) u32(n uint32) { _ = binary.Write(&e.Buffer, binary.BigEndian, n) }
func (e *encoder) u64(n uint64) { _ = binary.Write(&e.Buffer, binary.BigEndian, n) }
func (e *encoder) id(id ID)     { e.Write(id[:]) }
func (e *encoder) hash(h Hash)  { e.Write(h[:]) }
func (e *encoder) str(s string) {
	if len(s) > MaxFrame || !utf8.ValidString(s) {
		e.err = ErrInvalid
	}
	e.u32(uint32(len(s)))
	e.WriteString(s)
}
func (e *encoder) strings(a []string) {
	e.u32(uint32(len(a)))
	for _, s := range a {
		e.str(s)
	}
}
func (e *encoder) sum() (Hash, error) {
	if e.err != nil {
		return Hash{}, e.err
	}
	return sha256.Sum256(e.Bytes()), nil
}
func bindingBytes(e *encoder, b Binding, receipt bool) {
	e.u32(b.ProtocolVersion)
	for _, id := range []ID{b.EnvironmentID, b.InstallationID, b.AuthorityIncarnation, b.TransitionID} {
		e.id(id)
	}
	e.u64(b.AdmissionGeneration)
	e.id(b.CredentialCohortID)
	for _, h := range []Hash{b.DatabaseBindingHash, b.SchemaContractHash, b.SourceFactoryManifestHash, b.FreshRouteManifestHash} {
		e.hash(h)
	}
	if receipt {
		e.hash(b.AdmissionReceiptHash)
	}
	e.hash(b.LaunchManifestHash)
	e.u32(b.ExpectedLoginRoleOID)
	e.str(b.ExpectedLoginRoleName)
}
func BindingBytes(b Binding) ([]byte, error) {
	if b.Validate() != nil {
		return nil, ErrInvalid
	}
	e := newEncoder("voice.mm.admission.binding.v1")
	bindingBytes(e, b, true)
	return e.Bytes(), e.err
}
func BindingHash(b Binding) (Hash, error) {
	raw, e := BindingBytes(b)
	if e != nil {
		return Hash{}, e
	}
	return sha256.Sum256(raw), nil
}
func OriginHash(o OriginV1) (Hash, error) {
	if o.Version != 1 || o.OriginAdmissionGeneration == 0 || o.OriginAdmissionReceiptHash == (Hash{}) || o.CreationRequestHash == (Hash{}) || o.PrivateSourceManifestHash == (Hash{}) || (o.SourceKind != "NATIVE_VOICE" && o.SourceKind != "MM_LFP_CONSENT") {
		return Hash{}, ErrInvalid
	}
	for _, id := range []ID{o.EnvironmentID, o.InstallationID, o.AuthorityIncarnation, o.DatabaseLineageID, o.OriginTransitionID, o.OwnerSourceReceiptOrCaptureID, o.PartyID, o.CreationOperationID} {
		if id == (ID{}) {
			return Hash{}, ErrInvalid
		}
	}
	e := newEncoder("voice.mm.source-origin.v1")
	e.u32(o.Version)
	for _, id := range []ID{o.EnvironmentID, o.InstallationID, o.AuthorityIncarnation, o.DatabaseLineageID, o.OriginTransitionID} {
		e.id(id)
	}
	e.u64(o.OriginAdmissionGeneration)
	e.hash(o.OriginAdmissionReceiptHash)
	e.str(o.SourceKind)
	for _, id := range []ID{o.OwnerSourceReceiptOrCaptureID, o.PartyID, o.CreationOperationID} {
		e.id(id)
	}
	e.hash(o.CreationRequestHash)
	e.hash(o.PrivateSourceManifestHash)
	return e.sum()
}
func StampHash(s AdmissionStampV1) (Hash, error) {
	if s.Validate() != nil {
		return Hash{}, ErrInvalid
	}
	e := newEncoder("voice.mm.admission.stamp.v1")
	e.u32(s.Version)
	e.hash(s.BindingHash)
	e.id(s.DatabaseLineageID)
	e.hash(s.ContinuityHistoryRootHash)
	return e.sum()
}
func ContinuityHash(c ContinuityRecordV1) (Hash, error) {
	if c.Version != 1 || c.PredecessorReceiptHash == (Hash{}) || c.SuccessorReceiptHash == (Hash{}) || c.PredecessorReceiptHash == c.SuccessorReceiptHash || !registry(c.SupportedSourceVersions) {
		return Hash{}, ErrInvalid
	}
	e := newEncoder("voice.mm.admission.continuity.v1")
	e.u32(c.Version)
	for _, id := range []ID{c.EnvironmentID, c.InstallationID, c.AuthorityIncarnation, c.DatabaseLineageID} {
		if id == (ID{}) {
			return Hash{}, ErrInvalid
		}
		e.id(id)
	}
	e.hash(c.PredecessorReceiptHash)
	e.hash(c.SuccessorReceiptHash)
	e.strings(c.SupportedSourceVersions)
	e.hash(c.PreviousHistoryRootHash)
	return e.sum()
}

// HistoryGenesisHash starts a continuous lineage with its first immutable receipt.
func HistoryGenesisHash(receipt Hash, lineage ID) (Hash, error) {
	if receipt == (Hash{}) || lineage == (ID{}) {
		return Hash{}, ErrInvalid
	}
	e := newEncoder("voice.mm.admission.history-genesis.v1")
	e.u32(1)
	e.hash(receipt)
	e.id(lineage)
	return e.sum()
}
func (c ContinuityRecordV1) Validate() error { _, e := ContinuityHash(c); return e }
func registry(s []string) bool {
	if len(s) == 0 || len(s) > 128 || !sort.StringsAreSorted(s) {
		return false
	}
	for i, v := range s {
		if !text(v, 128) || (i > 0 && s[i-1] == v) {
			return false
		}
	}
	return true
}
