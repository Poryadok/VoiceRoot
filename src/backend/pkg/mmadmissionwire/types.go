package mmadmissionwire

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const DescriptorPath = "/run/voice/installation/matchmaking-native-v1.json"
const ReadbackSocket = "/run/voice/installation/mm-admission-readback-v1.sock"
const Protocol = "MM_ADMISSION_READBACK_V1"
const NativePurpose = "MATCHMAKING_NATIVE_SERVING"

type ID [16]byte
type Hash [32]byte

func (id ID) String() string {
	h := hex.EncodeToString(id[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
func ParseID(s string) (ID, error) {
	var id ID
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' || s != strings.ToLower(s) {
		return id, ErrInvalid
	}
	b, e := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	if e != nil || len(b) != 16 {
		return id, ErrInvalid
	}
	copy(id[:], b)
	return id, nil
}
func (id ID) MarshalJSON() ([]byte, error) { return json.Marshal(id.String()) }
func (id *ID) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) != nil {
		return ErrInvalid
	}
	v, e := ParseID(s)
	if e == nil {
		*id = v
	}
	return e
}
func (h Hash) String() string { return hex.EncodeToString(h[:]) }
func ParseHash(s string) (Hash, error) {
	var h Hash
	if len(s) != 64 || s != strings.ToLower(s) {
		return h, ErrInvalid
	}
	b, e := hex.DecodeString(s)
	if e != nil {
		return h, ErrInvalid
	}
	copy(h[:], b)
	return h, nil
}
func (h Hash) MarshalJSON() ([]byte, error) { return json.Marshal(h.String()) }
func (h *Hash) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) != nil {
		return ErrInvalid
	}
	v, e := ParseHash(s)
	if e == nil {
		*h = v
	}
	return e
}

type Scope string

const LocalOwned Scope = "LOCAL_OWNED"
const Installed Scope = "INSTALLED"

type State string

const Closed State = "CLOSED"
const Preparing State = "PREPARING"
const Armed State = "ARMED"
const Open State = "OPEN"
const Closing State = "CLOSING"
const RecoveryRequired State = "RECOVERY_REQUIRED"

type Purpose string

const Startup Purpose = "STARTUP"
const FreshAction Purpose = "FRESH_ACTION"
const ReadOnly Purpose = "READ_ONLY"

// Binding preserves the existing MM guard's field order and values.
// ID and Hash are fixed arrays; adapters explicitly convert existing UUID values.
type Binding struct {
	ProtocolVersion           uint32 `json:"protocol_version"`
	EnvironmentID             ID     `json:"environment_id"`
	InstallationID            ID     `json:"installation_id"`
	AuthorityIncarnation      ID     `json:"authority_incarnation"`
	TransitionID              ID     `json:"transition_id"`
	AdmissionGeneration       uint64 `json:"admission_generation,string"`
	CredentialCohortID        ID     `json:"credential_cohort_id"`
	DatabaseBindingHash       Hash   `json:"database_binding_sha256"`
	SchemaContractHash        Hash   `json:"schema_contract_sha256"`
	SourceFactoryManifestHash Hash   `json:"source_factory_manifest_sha256"`
	FreshRouteManifestHash    Hash   `json:"fresh_route_manifest_sha256"`
	AdmissionReceiptHash      Hash   `json:"admission_receipt_sha256"`
	LaunchManifestHash        Hash   `json:"launch_manifest_sha256"`
	ExpectedLoginRoleOID      uint32 `json:"expected_login_role_oid"`
	ExpectedLoginRoleName     string `json:"expected_login_role_name"`
}

func (b Binding) Validate() error {
	if b.ProtocolVersion != 1 || b.EnvironmentID == (ID{}) || b.InstallationID == (ID{}) || b.AuthorityIncarnation == (ID{}) || b.TransitionID == (ID{}) || b.CredentialCohortID == (ID{}) || b.AdmissionGeneration == 0 || b.AdmissionGeneration > math.MaxInt64 || b.ExpectedLoginRoleOID == 0 || !text(b.ExpectedLoginRoleName, 63) {
		return ErrInvalid
	}
	for _, h := range []Hash{b.DatabaseBindingHash, b.SchemaContractHash, b.SourceFactoryManifestHash, b.FreshRouteManifestHash, b.AdmissionReceiptHash, b.LaunchManifestHash} {
		if h == (Hash{}) {
			return ErrInvalid
		}
	}
	return nil
}
func text(s string, n int) bool {
	return len(s) > 0 && len(s) <= n && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}
func scope(s Scope) bool { return s == LocalOwned || s == Installed }
func state(s State) bool {
	switch s {
	case Closed, Preparing, Armed, Open, Closing, RecoveryRequired:
		return true
	}
	return false
}

type DescriptorV1 struct {
	ProtocolVersion              uint32   `json:"protocol_version"`
	Purpose                      string   `json:"purpose"`
	AuthorityScope               Scope    `json:"authority_scope"`
	ScopeID                      ID       `json:"scope_id"`
	ControllerID                 ID       `json:"controller_id"`
	Binding                      Binding  `json:"binding"`
	DatabaseLineageID            ID       `json:"database_lineage_id"`
	ReplicaID                    ID       `json:"replica_id"`
	ProcessLaunchID              ID       `json:"process_launch_id"`
	AdmittedImageDigest          string   `json:"admitted_image_digest"`
	AdmittedExecutableHash       Hash     `json:"admitted_executable_sha256"`
	AdmittedArgv                 []string `json:"admitted_argv"`
	ConfigContractHash           Hash     `json:"config_contract_sha256"`
	CredentialRefsManifestHash   Hash     `json:"credential_refs_manifest_sha256"`
	UpstreamContractManifestHash Hash     `json:"upstream_contract_manifest_sha256"`
	BrokerContractManifestHash   Hash     `json:"broker_contract_manifest_sha256"`
	ReadbackProtocol             string   `json:"readback_protocol"`
}

func (d DescriptorV1) Validate() error {
	if d.ProtocolVersion != 1 || d.Purpose != NativePurpose || !scope(d.AuthorityScope) || d.ScopeID == (ID{}) || d.ControllerID == (ID{}) || d.DatabaseLineageID == (ID{}) || d.ReplicaID == (ID{}) || d.ProcessLaunchID == (ID{}) || d.ReadbackProtocol != Protocol || d.Binding.Validate() != nil || !strings.HasPrefix(d.AdmittedImageDigest, "sha256:") {
		return ErrInvalid
	}
	if _, e := ParseHash(strings.TrimPrefix(d.AdmittedImageDigest, "sha256:")); e != nil {
		return e
	}
	if len(d.AdmittedArgv) == 0 || len(d.AdmittedArgv) > 64 {
		return ErrInvalid
	}
	for _, s := range d.AdmittedArgv {
		if !text(s, 4096) {
			return ErrInvalid
		}
	}
	for _, h := range []Hash{d.AdmittedExecutableHash, d.ConfigContractHash, d.CredentialRefsManifestHash, d.UpstreamContractManifestHash, d.BrokerContractManifestHash} {
		if h == (Hash{}) {
			return ErrInvalid
		}
	}
	return nil
}

type ObserveRequestV1 struct {
	ProtocolVersion     uint32  `json:"protocol_version"`
	Nonce               Hash    `json:"nonce"`
	Purpose             Purpose `json:"purpose"`
	EnvironmentID       ID      `json:"environment_id"`
	InstallationID      ID      `json:"installation_id"`
	ReplicaID           ID      `json:"replica_id"`
	ProcessLaunchID     ID      `json:"process_launch_id"`
	ExpectedBindingHash Hash    `json:"expected_binding_sha256"`
	ActionID            ID      `json:"action_id"`
	ActionRequestHash   Hash    `json:"action_request_sha256"`
}

func (r ObserveRequestV1) Validate() error {
	if r.ProtocolVersion != 1 || r.Nonce == (Hash{}) || r.EnvironmentID == (ID{}) || r.InstallationID == (ID{}) || r.ReplicaID == (ID{}) || r.ProcessLaunchID == (ID{}) || r.ExpectedBindingHash == (Hash{}) {
		return ErrInvalid
	}
	switch r.Purpose {
	case FreshAction:
		if r.ActionID == (ID{}) || r.ActionRequestHash == (Hash{}) {
			return ErrInvalid
		}
	case Startup, ReadOnly:
		if r.ActionID != (ID{}) || r.ActionRequestHash != (Hash{}) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

type ObserveResponseV1 struct {
	ProtocolVersion            uint32  `json:"protocol_version"`
	Nonce                      Hash    `json:"nonce"`
	ControllerID               ID      `json:"controller_id"`
	AuthorityScope             Scope   `json:"authority_scope"`
	ScopeID                    ID      `json:"scope_id"`
	State                      State   `json:"state"`
	StateRevision              uint64  `json:"state_revision,string"`
	TransitionID               ID      `json:"transition_id"`
	DatabaseLineageID          ID      `json:"database_lineage_id"`
	ExactBinding               Binding `json:"exact_binding"`
	BindingHash                Hash    `json:"binding_sha256"`
	ArmedReceiptHash           Hash    `json:"armed_receipt_sha256"`
	OpenReceiptHash            *Hash   `json:"open_receipt_sha256"`
	LaunchObservationHash      Hash    `json:"launch_observation_sha256"`
	ContinuityReceiptsRootHash Hash    `json:"continuity_receipts_root_sha256"`
	ActionID                   ID      `json:"action_id"`
	ActionRequestHash          Hash    `json:"action_request_sha256"`
}

func (r ObserveResponseV1) Validate() error {
	h, e := BindingHash(r.ExactBinding)
	if e != nil || r.ProtocolVersion != 1 || r.Nonce == (Hash{}) || r.ControllerID == (ID{}) || !scope(r.AuthorityScope) || r.ScopeID == (ID{}) || !state(r.State) || r.StateRevision == 0 || r.TransitionID != r.ExactBinding.TransitionID || r.DatabaseLineageID == (ID{}) || h != r.BindingHash || r.ArmedReceiptHash != r.ExactBinding.AdmissionReceiptHash || r.LaunchObservationHash == (Hash{}) || r.ContinuityReceiptsRootHash == (Hash{}) {
		return ErrInvalid
	}
	if r.State == Open && (r.OpenReceiptHash == nil || *r.OpenReceiptHash == (Hash{})) {
		return ErrInvalid
	}
	return nil
}

type OriginV1 struct {
	Version                       uint32 `json:"version"`
	EnvironmentID                 ID     `json:"environment_id"`
	InstallationID                ID     `json:"installation_id"`
	AuthorityIncarnation          ID     `json:"authority_incarnation"`
	DatabaseLineageID             ID     `json:"database_lineage_id"`
	OriginTransitionID            ID     `json:"origin_transition_id"`
	OriginAdmissionGeneration     uint64 `json:"origin_admission_generation,string"`
	OriginAdmissionReceiptHash    Hash   `json:"origin_admission_receipt_sha256"`
	SourceKind                    string `json:"source_kind"`
	OwnerSourceReceiptOrCaptureID ID     `json:"owner_source_receipt_or_capture_id"`
	PartyID                       ID     `json:"party_id"`
	CreationOperationID           ID     `json:"creation_operation_id"`
	CreationRequestHash           Hash   `json:"creation_request_sha256"`
	PrivateSourceManifestHash     Hash   `json:"private_source_manifest_sha256"`
	OriginHash                    Hash   `json:"origin_sha256"`
}
type AdmissionStampV1 struct {
	Version                   uint32 `json:"version"`
	BindingHash               Hash   `json:"binding_sha256"`
	DatabaseLineageID         ID     `json:"database_lineage_id"`
	ContinuityHistoryRootHash Hash   `json:"continuity_history_root_sha256"`
}
type ContinuityRecordV1 struct {
	Version                 uint32   `json:"version"`
	EnvironmentID           ID       `json:"environment_id"`
	InstallationID          ID       `json:"installation_id"`
	AuthorityIncarnation    ID       `json:"authority_incarnation"`
	DatabaseLineageID       ID       `json:"database_lineage_id"`
	PredecessorReceiptHash  Hash     `json:"predecessor_receipt_sha256"`
	SuccessorReceiptHash    Hash     `json:"successor_receipt_sha256"`
	SupportedSourceVersions []string `json:"supported_source_versions"`
	PreviousHistoryRootHash Hash     `json:"previous_history_root_sha256"`
}

func (o OriginV1) Validate() error {
	h, e := OriginHash(o)
	if e != nil || h != o.OriginHash {
		return ErrInvalid
	}
	return nil
}
func (s AdmissionStampV1) Validate() error {
	if s.Version != 1 || s.BindingHash == (Hash{}) || s.DatabaseLineageID == (ID{}) || s.ContinuityHistoryRootHash == (Hash{}) {
		return ErrInvalid
	}
	return nil
}
func invalid(what string) error { return fmt.Errorf("%w: %s", ErrInvalid, what) }
