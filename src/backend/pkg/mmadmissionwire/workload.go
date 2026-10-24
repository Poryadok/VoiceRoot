package mmadmissionwire

const WorkloadSocketDir = "/run/voice/mm-workload-control"

type WorkloadRequestV1 struct {
	ProtocolVersion uint32  `json:"protocol_version"`
	Method          string  `json:"method"`
	Nonce           Hash    `json:"nonce"`
	ReplicaID       ID      `json:"replica_id"`
	ProcessLaunchID ID      `json:"process_launch_id"`
	TransitionID    ID      `json:"transition_id"`
	ExpectedBinding Binding `json:"expected_binding"`
}
type WorkloadResponseV1 struct {
	ProtocolVersion                uint32 `json:"protocol_version"`
	Nonce                          Hash   `json:"nonce"`
	ReplicaID                      ID     `json:"replica_id"`
	ProcessLaunchID                ID     `json:"process_launch_id"`
	TransitionID                   ID     `json:"transition_id"`
	BindingHash                    Hash   `json:"binding_sha256"`
	State                          State  `json:"state"`
	InFlightWorkIDs                []ID   `json:"in_flight_work_ids"`
	UnresolvedProviderOperationIDs []ID   `json:"unresolved_provider_operation_ids"`
	PreparedContractsHash          Hash   `json:"prepared_contracts_sha256"`
	ObservedLaunchHash             Hash   `json:"observed_launch_sha256"`
}

func (r WorkloadRequestV1) Validate() error {
	if r.ProtocolVersion != 1 || (r.Method != "CLOSE_REPLICA" && r.Method != "READ_REPLICA_STATUS") || r.Nonce == (Hash{}) || r.ReplicaID == (ID{}) || r.ProcessLaunchID == (ID{}) || r.TransitionID == (ID{}) || r.ExpectedBinding.Validate() != nil || r.TransitionID != r.ExpectedBinding.TransitionID {
		return ErrInvalid
	}
	return nil
}
func (r WorkloadResponseV1) Validate() error {
	if r.ProtocolVersion != 1 || r.Nonce == (Hash{}) || r.ReplicaID == (ID{}) || r.ProcessLaunchID == (ID{}) || r.TransitionID == (ID{}) || r.BindingHash == (Hash{}) || r.ObservedLaunchHash == (Hash{}) || (r.State != Closed && r.State != Armed && r.State != Open) || len(r.InFlightWorkIDs) > 1024 || len(r.UnresolvedProviderOperationIDs) > 1024 {
		return ErrInvalid
	}
	if r.State == Armed && r.PreparedContractsHash == (Hash{}) {
		return ErrInvalid
	}
	for _, ids := range [][]ID{r.InFlightWorkIDs, r.UnresolvedProviderOperationIDs} {
		for i, id := range ids {
			if id == (ID{}) || (i > 0 && string(ids[i-1][:]) >= string(id[:])) {
				return ErrInvalid
			}
		}
	}
	return nil
}
