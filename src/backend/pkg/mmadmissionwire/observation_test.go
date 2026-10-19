package mmadmissionwire

import "testing"

func testObservation() (DescriptorV1, ObserveRequestV1, ObserveResponseV1) {
	b := testBinding()
	h, _ := BindingHash(b)
	d := DescriptorV1{ProtocolVersion: 1, Purpose: NativePurpose, AuthorityScope: LocalOwned, ScopeID: ID{20}, ControllerID: ID{21}, Binding: b, DatabaseLineageID: ID{22}, ReplicaID: ID{23}, ProcessLaunchID: ID{24}, AdmittedImageDigest: "sha256:" + Hash{25}.String(), AdmittedExecutableHash: Hash{26}, AdmittedArgv: []string{"/service-native"}, ConfigContractHash: Hash{27}, CredentialRefsManifestHash: Hash{28}, UpstreamContractManifestHash: Hash{29}, BrokerContractManifestHash: Hash{30}, ReadbackProtocol: Protocol}
	r := ObserveRequestV1{ProtocolVersion: 1, Nonce: Hash{31}, Purpose: FreshAction, EnvironmentID: b.EnvironmentID, InstallationID: b.InstallationID, ReplicaID: d.ReplicaID, ProcessLaunchID: d.ProcessLaunchID, ExpectedBindingHash: h, ActionID: ID{32}, ActionRequestHash: Hash{33}}
	open := Hash{34}
	out := ObserveResponseV1{ProtocolVersion: 1, Nonce: r.Nonce, ControllerID: d.ControllerID, AuthorityScope: d.AuthorityScope, ScopeID: d.ScopeID, State: Open, StateRevision: 1, TransitionID: b.TransitionID, DatabaseLineageID: d.DatabaseLineageID, ExactBinding: b, BindingHash: h, ArmedReceiptHash: b.AdmissionReceiptHash, OpenReceiptHash: &open, LaunchObservationHash: Hash{35}, ContinuityReceiptsRootHash: Hash{36}, ActionID: r.ActionID, ActionRequestHash: r.ActionRequestHash}
	return d, r, out
}
func TestObservationBindsIdentityAndExactAction(t *testing.T) {
	d, r, good := testObservation()
	if e := ValidateObservation(d, r, good); e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*ObserveResponseV1){"nonce": func(o *ObserveResponseV1) { o.Nonce[0]++ }, "controller": func(o *ObserveResponseV1) { o.ControllerID[0]++ }, "scope": func(o *ObserveResponseV1) { o.AuthorityScope = Installed }, "lineage": func(o *ObserveResponseV1) { o.DatabaseLineageID[0]++ }, "action": func(o *ObserveResponseV1) { o.ActionID[0]++ }, "request": func(o *ObserveResponseV1) { o.ActionRequestHash[0]++ }, "closed": func(o *ObserveResponseV1) { o.State = Closed }, "armed": func(o *ObserveResponseV1) { o.State = Armed }, "binding": func(o *ObserveResponseV1) { o.ExactBinding.AdmissionGeneration++ }} {
		t.Run(name, func(t *testing.T) {
			bad := good
			change(&bad)
			if ValidateObservation(d, r, bad) == nil {
				t.Fatal("accepted mismatched readback")
			}
		})
	}
	r.Purpose = Startup
	r.ActionID = ID{}
	r.ActionRequestHash = Hash{}
	good.ActionID = ID{}
	good.ActionRequestHash = Hash{}
	good.State = Armed
	good.OpenReceiptHash = nil
	if ValidateObservation(d, r, good) != nil {
		t.Fatal("ARMED must permit startup only")
	}
}
func TestOriginCannotBeReboundOrSilentlyAdopted(t *testing.T) {
	o := OriginV1{Version: 1, EnvironmentID: ID{1}, InstallationID: ID{2}, AuthorityIncarnation: ID{3}, DatabaseLineageID: ID{4}, OriginTransitionID: ID{5}, OriginAdmissionGeneration: 6, OriginAdmissionReceiptHash: Hash{7}, SourceKind: "NATIVE_VOICE", OwnerSourceReceiptOrCaptureID: ID{8}, PartyID: ID{9}, CreationOperationID: ID{10}, CreationRequestHash: Hash{11}, PrivateSourceManifestHash: Hash{12}}
	o.OriginHash, _ = OriginHash(o)
	if o.Validate() != nil {
		t.Fatal("valid origin")
	}
	changed := o
	changed.PartyID[0]++
	if changed.Validate() == nil {
		t.Fatal("copied origin accepted")
	}
	changed = o
	changed.DatabaseLineageID[0]++
	if changed.Validate() == nil {
		t.Fatal("copied lineage accepted")
	}
	changed = o
	changed.PrivateSourceManifestHash[0]++
	if changed.Validate() == nil {
		t.Fatal("changed source accepted")
	}
	if (OriginV1{}).Validate() == nil {
		t.Fatal("historical no-origin adopted")
	}
	t.Log("origin golden", o.OriginHash.String())
}
