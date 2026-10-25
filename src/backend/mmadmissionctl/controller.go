package mmadmissionctl

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	w "voice/backend/pkg/mmadmissionwire"
)

type Identity struct {
	ScopeID              w.ID `json:"scope_id"`
	EnvironmentID        w.ID `json:"environment_id"`
	InstallationID       w.ID `json:"installation_id"`
	ControllerID         w.ID `json:"controller_id"`
	AuthorityIncarnation w.ID `json:"authority_incarnation"`
	DatabaseLineageID    w.ID `json:"database_lineage_id"`
}
type Snapshot struct {
	ReceiptHistory    []w.ArmedReceiptV1     `json:"receipt_history"`
	ContinuityHistory []w.ContinuityRecordV1 `json:"continuity_history"`
	Version           uint32                 `json:"version"`
	Identity          Identity               `json:"identity"`
	State             w.State                `json:"state"`
	Generation        uint64                 `json:"generation,string"`
	Binding           w.Binding              `json:"binding"`
	Spec              *w.LaunchSpecV1        `json:"spec"`
	ArmedReceipt      *w.ArmedReceiptV1      `json:"armed_receipt"`
	OpenReceipt       *w.OpenReceiptV1       `json:"open_receipt"`
	HistoryRoot       w.Hash                 `json:"history_root"`
	Observation       Evidence               `json:"observation"`
	HeadInstalled     bool                   `json:"head_installed"`
	IngressConfirmed  bool                   `json:"ingress_confirmed"`
}
type Evidence struct {
	ScopeID                   w.ID          `json:"scope_id"`
	LaunchSpecHash            w.Hash        `json:"launch_spec_sha256"`
	DatabaseBindingHash       w.Hash        `json:"database_binding_sha256"`
	SchemaContractHash        w.Hash        `json:"schema_contract_sha256"`
	SourceFactoryManifestHash w.Hash        `json:"source_factory_manifest_sha256"`
	FreshRouteManifestHash    w.Hash        `json:"fresh_route_manifest_sha256"`
	LoginRoleOID              uint32        `json:"login_role_oid"`
	LoginRoleName             string        `json:"login_role_name"`
	CredentialCohortID        w.ID          `json:"credential_cohort_id"`
	ClosureHash               w.Hash        `json:"closure_sha256"`
	ResourcesHash             w.Hash        `json:"resources_sha256"`
	LaunchesHash              w.Hash        `json:"launches_sha256"`
	PreparedHash              w.Hash        `json:"prepared_sha256"`
	Peers                     []ReplicaPeer `json:"peers"`
	UnresolvedOperations      []w.ID        `json:"unresolved_operations"`
}
type ReplicaPeer struct {
	ReplicaID      w.ID   `json:"replica_id"`
	LaunchID       w.ID   `json:"launch_id"`
	PID            int32  `json:"pid"`
	UID            uint32 `json:"uid"`
	GID            uint32 `json:"gid"`
	ExecutableHash w.Hash `json:"executable_sha256"`
}
type Driver interface {
	Scope() w.Scope
	ObserveClosed(context.Context, w.LaunchSpecV1, w.ID) (Evidence, error)
	InstallHead(context.Context, Snapshot, w.ID) error
	StartPrepared(context.Context, Snapshot, w.ID, func(Evidence) error) (Evidence, error)
	ObservePrepared(context.Context, Snapshot, w.ID) (Evidence, error)
	OpenIngress(context.Context, Snapshot, w.ID) error
	CloseAndDrain(context.Context, Snapshot, w.ID) (Evidence, error)
	ReconcileClosed(context.Context, Snapshot, w.ID) (Evidence, error)
}
type Command struct {
	Version                uint32          `json:"version"`
	Kind                   string          `json:"kind"`
	OperationID            w.ID            `json:"operation_id"`
	ExpectedRevision       uint64          `json:"expected_revision,string"`
	TransitionID           w.ID            `json:"transition_id"`
	LaunchSpec             *w.LaunchSpecV1 `json:"launch_spec"`
	InterruptedOperationID w.ID            `json:"interrupted_operation_id"`
}
type Result struct {
	Revision uint64   `json:"revision,string"`
	Snapshot Snapshot `json:"snapshot"`
	Error    string   `json:"error"`
}
type Controller struct {
	Journal *Journal
	Driver  Driver
}

func freshID() w.ID {
	var id w.ID
	if _, e := rand.Read(id[:]); e != nil {
		panic("MM controller entropy unavailable")
	}
	id[6] = (id[6] & 15) | 64
	id[8] = (id[8] & 63) | 128
	return id
}
func NewLocalController(j *Journal, d Driver, scope, environment, installation w.ID) (*Controller, error) {
	if j == nil || d == nil || d.Scope() != w.LocalOwned || scope == (w.ID{}) || environment == (w.ID{}) || installation == (w.ID{}) {
		return nil, ErrConflict
	}
	s := Snapshot{Version: 1, State: w.Closed, Identity: Identity{ScopeID: scope, EnvironmentID: environment, InstallationID: installation, ControllerID: freshID(), AuthorityIncarnation: freshID(), DatabaseLineageID: freshID()}}
	raw, _ := json.Marshal(s)
	r, e := j.db.Exec(`UPDATE head SET snapshot=? WHERE singleton=1 AND revision=0 AND length(snapshot)=0`, raw)
	if e != nil {
		return nil, e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return nil, ErrConflict
	}
	return &Controller{Journal: j, Driver: d}, nil
}
func (c *Controller) Read(ctx context.Context) (Result, error) {
	h, e := c.Journal.Read(ctx)
	if e != nil {
		return Result{}, e
	}
	var s Snapshot
	if json.Unmarshal(h.Snapshot, &s) != nil || s.Version != 1 || s.Identity.ScopeID == (w.ID{}) || s.Identity.ControllerID == (w.ID{}) {
		return Result{}, ErrPending
	}
	if h.Pending != "" {
		s.State = w.RecoveryRequired
		s.IngressConfirmed = false
	}
	return Result{Revision: h.Revision, Snapshot: s}, nil
}
func (c *Controller) Execute(ctx context.Context, cmd Command) (Result, error) {
	unlock, lockerr := c.Journal.operationLock()
	if lockerr != nil {
		return Result{}, lockerr
	}
	defer unlock()
	if c.Driver == nil || c.Driver.Scope() != w.LocalOwned || cmd.Version != 1 || cmd.OperationID == (w.ID{}) {
		return Result{}, ErrConflict
	}
	raw, e := json.Marshal(cmd)
	if e != nil {
		return Result{}, e
	}
	request := sha256.Sum256(append([]byte("voice.mm.admission.command.v1\x00"), raw...))
	in := Intent{OperationID: cmd.OperationID, ExpectedRevision: cmd.ExpectedRevision, RequestHash: request, Kind: cmd.Kind, TargetOperationID: cmd.InterruptedOperationID}
	a, e := c.Journal.Begin(ctx, in)
	if e != nil {
		return Result{}, e
	}
	if a.Replay {
		if a.Pending {
			return Result{}, ErrPending
		}
		var r Result
		if json.Unmarshal(a.Result, &r) != nil {
			return Result{}, ErrPending
		}
		if r.Error != "" {
			return r, ErrConflict
		}
		return r, nil
	}
	head, e := c.Journal.Read(ctx)
	if e != nil {
		return Result{}, e
	}
	var s Snapshot
	if json.Unmarshal(head.Snapshot, &s) != nil || s.Version != 1 {
		return Result{}, ErrPending
	}
	if cmd.Kind == "RECONCILE" && cmd.InterruptedOperationID != (w.ID{}) {
		s.State = w.RecoveryRequired
		s.IngressConfirmed = false
	}
	finish := func(cause error) (Result, error) {
		r := Result{Revision: head.Revision + 1, Snapshot: s}
		if cause != nil {
			r.Error = cause.Error()
		}
		b, _ := json.Marshal(s)
		result, _ := json.Marshal(r)
		rev, err := c.Journal.Complete(ctx, cmd.OperationID, b, result)
		if err != nil {
			return Result{}, err
		}
		r.Revision = rev
		return r, cause
	}
	persist := func(phase string) error {
		b, err := json.Marshal(s)
		if err != nil {
			return err
		}
		if err = c.Journal.SetPendingSnapshot(ctx, cmd.OperationID, b); err != nil {
			return err
		}
		if err = c.Journal.RecordEffect(ctx, cmd.OperationID, phase, b); err != nil {
			return err
		}
		head, err = c.Journal.Read(ctx)
		return err
	}
	fail := func(err error) (Result, error) {
		s.State = w.RecoveryRequired
		s.IngressConfirmed = false
		return finish(errors.Join(ErrPending, err))
	}
	if cmd.Kind != "PREPARE" && (s.Binding.TransitionID == (w.ID{}) || cmd.TransitionID != s.Binding.TransitionID) {
		return finish(ErrConflict)
	}
	switch cmd.Kind {
	case "PREPARE":
		if s.State != w.Closed || cmd.LaunchSpec == nil || cmd.LaunchSpec.Validate() != nil || cmd.LaunchSpec.AuthorityScope != w.LocalOwned || cmd.LaunchSpec.ScopeID != s.Identity.ScopeID || s.Generation >= uint64(1<<63-2) {
			return finish(ErrConflict)
		}
		// Copy through canonical JSON so caller-owned slices cannot mutate an intent.
		specbytes, _ := json.Marshal(cmd.LaunchSpec)
		var spec w.LaunchSpecV1
		_ = json.Unmarshal(specbytes, &spec)
		s.Spec = &spec
		s.State = w.Preparing
		s.IngressConfirmed = false
		s.HeadInstalled = false
		s.OpenReceipt = nil
		s.Generation++
		s.Binding.TransitionID = cmd.OperationID
		if e = persist("PREPARE_INTENT"); e != nil {
			return Result{}, e
		}
		obs, err := c.Driver.ObserveClosed(ctx, spec, cmd.OperationID)
		if err != nil {
			return fail(err)
		}
		if e = validateEvidence(spec, obs, false); e != nil {
			return fail(e)
		}
		s.Observation = obs
		s.Binding = w.Binding{ProtocolVersion: 1, EnvironmentID: s.Identity.EnvironmentID, InstallationID: s.Identity.InstallationID, AuthorityIncarnation: s.Identity.AuthorityIncarnation, TransitionID: cmd.OperationID, AdmissionGeneration: s.Generation, CredentialCohortID: obs.CredentialCohortID, DatabaseBindingHash: obs.DatabaseBindingHash, SchemaContractHash: obs.SchemaContractHash, SourceFactoryManifestHash: obs.SourceFactoryManifestHash, FreshRouteManifestHash: obs.FreshRouteManifestHash, LaunchManifestHash: obs.LaunchSpecHash, ExpectedLoginRoleOID: obs.LoginRoleOID, ExpectedLoginRoleName: obs.LoginRoleName}
		return finish(nil)
	case "ARM":
		if s.State != w.Preparing || s.Spec == nil {
			return finish(ErrConflict)
		}
		if e = persist("ARM_OBSERVE_INTENT"); e != nil {
			return Result{}, e
		}
		obs, err := c.Driver.ObserveClosed(ctx, *s.Spec, cmd.OperationID)
		if err != nil {
			return fail(err)
		}
		if e = validateEvidence(*s.Spec, obs, false); e != nil {
			return fail(e)
		}
		if !sameResourceBinding(s.Observation, obs) {
			return fail(ErrConflict)
		}
		s.Observation = obs
		var previous *w.Hash
		if s.ArmedReceipt != nil {
			h, err := w.ArmedHash(*s.ArmedReceipt)
			if err != nil {
				return fail(err)
			}
			previous = &h
		}
		receipt := w.ArmedReceiptV1{Version: 1, AuthorityScope: w.LocalOwned, ScopeID: s.Identity.ScopeID, ControllerID: s.Identity.ControllerID, TransitionID: s.Binding.TransitionID, PredecessorReceiptHash: previous, DatabaseLineageID: s.Identity.DatabaseLineageID, Binding: s.Binding, LaunchSpecHash: s.Binding.LaunchManifestHash, ClosureEvidenceHash: obs.ClosureHash, PreparationEvidenceHash: obs.ResourcesHash, PreviousReceiptHash: previous}
		hash, err := w.ArmedHash(receipt)
		if err != nil {
			return fail(err)
		}
		s.Binding.AdmissionReceiptHash = hash
		s.ArmedReceipt = &receipt
		s.State = w.Armed
		if previous == nil {
			s.HistoryRoot, err = w.HistoryGenesisHash(hash, s.Identity.DatabaseLineageID)
		} else {
			record := w.ContinuityRecordV1{Version: 1, EnvironmentID: s.Identity.EnvironmentID, InstallationID: s.Identity.InstallationID, AuthorityIncarnation: s.Identity.AuthorityIncarnation, DatabaseLineageID: s.Identity.DatabaseLineageID, PredecessorReceiptHash: *previous, SuccessorReceiptHash: hash, SupportedSourceVersions: []string{"NATIVE_VOICE_C16_V1_ORIGIN_V1"}, PreviousHistoryRootHash: s.HistoryRoot}
			s.HistoryRoot, err = w.ContinuityHash(record)
			s.ContinuityHistory = append(s.ContinuityHistory, record)
		}
		if err != nil {
			return fail(err)
		}
		s.ReceiptHistory = append(s.ReceiptHistory, receipt)
		// ARMED receipt is durable before installation; pending readback is closed.
		if e = persist("ARMED_HEAD_INSTALL_INTENT"); e != nil {
			return Result{}, e
		}
		if err = c.Driver.InstallHead(ctx, s, cmd.OperationID); err != nil {
			return fail(err)
		}
		s.HeadInstalled = true
		if e = persist("HEAD_INSTALLED_START_INTENT"); e != nil {
			return Result{}, e
		}
		// A source-matched readback producer may expose STARTUP at this checkpoint,
		// never FRESH_ACTION; domain ingress remains closed until OPEN completes.
		obs, err = c.Driver.StartPrepared(ctx, s, cmd.OperationID, func(live Evidence) error {
			if validateEvidence(*s.Spec, live, false) != nil || !sameResourceBinding(s.Observation, live) || len(live.Peers) != len(s.Spec.Replicas) || live.LaunchesHash == (w.Hash{}) {
				return ErrPending
			}
			for i, p := range live.Peers {
				r := s.Spec.Replicas[i]
				if p.ReplicaID != r.ReplicaID || p.LaunchID != r.LaunchID || p.PID <= 0 || p.UID != r.RuntimeUID || p.ExecutableHash != r.ExecutableHash {
					return ErrPending
				}
			}
			s.Observation = live
			return persist("STARTUP_PEERS_BOUND")
		})
		if err != nil {
			return fail(err)
		}
		if e = validateEvidence(*s.Spec, obs, true); e != nil || !sameResourceBinding(s.Observation, obs) {
			return fail(ErrConflict)
		}
		s.Observation = obs
		return finish(nil)
	case "OPEN":
		if s.State != w.Armed || !s.HeadInstalled || s.Spec == nil {
			return finish(ErrConflict)
		}
		if e = persist("OPEN_OBSERVE_INTENT"); e != nil {
			return Result{}, e
		}
		obs, err := c.Driver.ObservePrepared(ctx, s, cmd.OperationID)
		if err != nil {
			return fail(err)
		}
		if e = validateEvidence(*s.Spec, obs, true); e != nil || !sameResourceBinding(s.Observation, obs) {
			return fail(ErrConflict)
		}
		s.Observation = obs
		s.OpenReceipt = &w.OpenReceiptV1{Version: 1, ArmedReceiptHash: s.Binding.AdmissionReceiptHash, ObservedLaunchesHash: obs.LaunchesHash, PreparedAcknowledgementsHash: obs.PreparedHash, StateRevision: head.Revision + 1}
		if _, err = w.OpenHash(*s.OpenReceipt); err != nil {
			return fail(err)
		}
		s.State = w.Open
		if e = persist("OPEN_INGRESS_INTENT"); e != nil {
			return Result{}, e
		}
		if err = c.Driver.OpenIngress(ctx, s, cmd.OperationID); err != nil {
			return fail(err)
		}
		s.IngressConfirmed = true
		return finish(nil)
	case "CLOSE":
		if s.Spec == nil || (s.State != w.Open && s.State != w.Armed && s.State != w.Preparing) {
			return finish(ErrConflict)
		}
		s.State = w.Closing
		s.IngressConfirmed = false
		if e = persist("CLOSE_DRAIN_INTENT"); e != nil {
			return Result{}, e
		}
		obs, err := c.Driver.CloseAndDrain(ctx, s, cmd.OperationID)
		if err != nil {
			return fail(err)
		}
		if e = validateEvidence(*s.Spec, obs, false); e != nil {
			return fail(e)
		}
		s.State = w.Closed
		s.HeadInstalled = false
		s.Observation = obs
		return finish(nil)
	case "RECONCILE":
		if s.State != w.RecoveryRequired || s.Spec == nil {
			return finish(ErrConflict)
		}
		if e = persist("RECONCILE_CLOSED_INTENT"); e != nil {
			return Result{}, e
		}
		obs, err := c.Driver.ReconcileClosed(ctx, s, cmd.OperationID)
		if err != nil {
			return fail(err)
		}
		if e = validateEvidence(*s.Spec, obs, false); e != nil {
			return fail(e)
		}
		s.State = w.Closed
		s.HeadInstalled = false
		s.IngressConfirmed = false
		s.Observation = obs
		return finish(nil)
	default:
		return finish(ErrConflict)
	}
}
func sameResourceBinding(a, b Evidence) bool {
	return a.ScopeID == b.ScopeID && a.LaunchSpecHash == b.LaunchSpecHash && a.DatabaseBindingHash == b.DatabaseBindingHash && a.SchemaContractHash == b.SchemaContractHash && a.SourceFactoryManifestHash == b.SourceFactoryManifestHash && a.FreshRouteManifestHash == b.FreshRouteManifestHash && a.LoginRoleOID == b.LoginRoleOID && a.LoginRoleName == b.LoginRoleName && a.CredentialCohortID == b.CredentialCohortID
}
func validateEvidence(spec w.LaunchSpecV1, o Evidence, prepared bool) error {
	if len(o.UnresolvedOperations) > 0 {
		return ErrPending
	}
	h, e := w.LaunchHash(spec)
	if e != nil || o.ScopeID != spec.ScopeID || o.LaunchSpecHash != h || o.DatabaseBindingHash == (w.Hash{}) || o.SchemaContractHash == (w.Hash{}) || o.SourceFactoryManifestHash == (w.Hash{}) || o.FreshRouteManifestHash == (w.Hash{}) || o.CredentialCohortID == (w.ID{}) || o.LoginRoleOID == 0 || o.LoginRoleName == "" || o.ClosureHash == (w.Hash{}) || o.ResourcesHash == (w.Hash{}) {
		return ErrPending
	}
	if !prepared {
		return nil
	}
	if o.LaunchesHash == (w.Hash{}) || o.PreparedHash == (w.Hash{}) || len(o.Peers) != len(spec.Replicas) {
		return ErrPending
	}
	for i, r := range spec.Replicas {
		p := o.Peers[i]
		if p.ReplicaID != r.ReplicaID || p.LaunchID != r.LaunchID || p.PID <= 0 || p.UID != r.RuntimeUID || p.ExecutableHash != r.ExecutableHash {
			return ErrPending
		}
	}
	return nil
}
