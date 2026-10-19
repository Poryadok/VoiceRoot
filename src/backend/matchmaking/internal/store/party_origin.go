package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	admission "voice/backend/matchmaking/internal/mmadmission"
	wire "voice/backend/pkg/mmadmissionwire"
)

func copyNativeBinding(binding *admission.Binding) *admission.Binding {
	if binding == nil {
		return nil
	}
	copy := *binding
	return &copy
}

func readNativeHistory(ctx context.Context, tx pgx.Tx, binding *admission.Binding) (*admission.HistoryCut, error) {
	if binding == nil {
		return nil, nil
	}
	cut, err := admission.ReadHistory(ctx, tx, admission.WireBinding(*binding))
	if err != nil {
		return nil, err
	}
	return &cut, nil
}

// nativePrivateManifest binds the actual Voice source kind and complete c16
// authority, independently of rollout generation/current installation stamp.
func nativePrivateManifest(p SourceParty) [32]byte {
	var b bytes.Buffer
	b.WriteString("voice.mm.native-source-private-manifest.v1\x00")
	_ = binary.Write(&b, binary.BigEndian, p.ProtocolVersion)
	b.Write(p.SearchOperationID[:])
	b.Write(p.SearchRequestHash[:])
	b.Write(p.InitiatorAccountID[:])
	b.Write(p.LeaderProfileID[:])
	_ = binary.Write(&b, binary.BigEndian, uint64(p.InitiatorSessionEpoch))
	_ = binary.Write(&b, binary.BigEndian, uint32(len(p.SourceKind)))
	b.WriteString(p.SourceKind)
	if p.SourceRoomID == nil {
		b.WriteByte(0)
	} else {
		b.WriteByte(1)
		b.Write(p.SourceRoomID[:])
	}
	_ = binary.Write(&b, binary.BigEndian, p.SourceRosterVersion)
	b.Write(p.MemberManifestHash[:])
	_ = binary.Write(&b, binary.BigEndian, p.MemberAuthorityVersion)
	b.Write(p.MemberAuthorityHash[:])
	b.Write(p.SourceAuthorityHash[:])
	return sha256.Sum256(b.Bytes())
}

func insertNativeOrigin(ctx context.Context, tx pgx.Tx, p SourceParty, cut *admission.HistoryCut, binding *admission.Binding) error {
	if cut == nil || binding == nil {
		return admission.ErrUnavailable
	}
	stamp := cut.Stamp()
	b := admission.WireBinding(*binding)
	boundHash, err := wire.BindingHash(b)
	if err != nil || boundHash != stamp.BindingHash {
		return admission.ErrUnavailable
	}
	o := wire.OriginV1{Version: 1, EnvironmentID: b.EnvironmentID, InstallationID: b.InstallationID, AuthorityIncarnation: b.AuthorityIncarnation, DatabaseLineageID: stamp.DatabaseLineageID, OriginTransitionID: b.TransitionID, OriginAdmissionGeneration: b.AdmissionGeneration, OriginAdmissionReceiptHash: b.AdmissionReceiptHash, SourceKind: "NATIVE_VOICE", OwnerSourceReceiptOrCaptureID: wire.ID(p.SearchOperationID), PartyID: wire.ID(p.ID), CreationOperationID: wire.ID(p.SearchOperationID), CreationRequestHash: wire.Hash(p.SearchRequestHash), PrivateSourceManifestHash: wire.Hash(nativePrivateManifest(p))}
	hash, err := wire.OriginHash(o)
	if err != nil {
		return admission.ErrUnavailable
	}
	o.OriginHash = hash
	if !cut.AllowsOrigin(o, admission.NativeSourceVersion) {
		return admission.ErrUnavailable
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return admission.ErrUnavailable
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.mm_native_party_origins(party_id,origin_sha256,origin_bytes,original_receipt_sha256,database_lineage_id,creation_operation_id,private_source_manifest_sha256) VALUES($1,$2,$3,$4,$5,$6,$7)`, p.ID, hash[:], raw, o.OriginAdmissionReceiptHash[:], uuid.UUID(o.DatabaseLineageID), p.SearchOperationID, o.PrivateSourceManifestHash[:])
	return err
}

// readNativeOrigin binds canonical history to this exact immutable party. It
// cannot manufacture an origin for historical or partially upgraded captures.
func readNativeOrigin(ctx context.Context, tx pgx.Tx, p SourceParty, cut *admission.HistoryCut) (wire.OriginV1, error) {
	if cut == nil {
		return wire.OriginV1{}, admission.ErrUnavailable
	}
	var raw, hash, receipt, manifest []byte
	var lineage, operation uuid.UUID
	err := tx.QueryRow(ctx, `SELECT origin_bytes,origin_sha256,original_receipt_sha256,database_lineage_id,creation_operation_id,private_source_manifest_sha256 FROM public.mm_native_party_origins WHERE party_id=$1`, p.ID).Scan(&raw, &hash, &receipt, &lineage, &operation, &manifest)
	var o wire.OriginV1
	if err != nil || wire.Decode(raw, &o) != nil || !bytes.Equal(hash, o.OriginHash[:]) || !bytes.Equal(receipt, o.OriginAdmissionReceiptHash[:]) || o.DatabaseLineageID != wire.ID(lineage) || o.CreationOperationID != wire.ID(operation) || !bytes.Equal(manifest, o.PrivateSourceManifestHash[:]) {
		return wire.OriginV1{}, admission.ErrUnavailable
	}
	if o.SourceKind != "NATIVE_VOICE" || o.PartyID != wire.ID(p.ID) || o.CreationOperationID != wire.ID(p.SearchOperationID) || o.OwnerSourceReceiptOrCaptureID != wire.ID(p.SearchOperationID) || o.CreationRequestHash != wire.Hash(p.SearchRequestHash) || o.PrivateSourceManifestHash != wire.Hash(nativePrivateManifest(p)) || !cut.AllowsOrigin(o, admission.NativeSourceVersion) {
		return wire.OriginV1{}, admission.ErrUnavailable
	}
	return o, nil
}
