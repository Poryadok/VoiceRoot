package mmadmissionwire

import (
	"reflect"
	"time"
)

// Stamp and LaunchObservationHash expose defensive values from an already
// authenticated startup exchange. They do not construct observations or admit
// any action. Native consumers compare the stamp with their same local SQL cut.
func (o StartupObservation) Stamp() AdmissionStampV1 {
	return AdmissionStampV1{Version: 1, BindingHash: o.response.BindingHash, DatabaseLineageID: o.response.DatabaseLineageID, ContinuityHistoryRootHash: o.response.ContinuityReceiptsRootHash}
}
func (o StartupObservation) LaunchObservationHash() Hash { return o.response.LaunchObservationHash }

// CheckControllerPeer accepts only actual SO_PEERCRED from the private workload
// control connection and the exact independently authenticated launch binding.
// It is for close/status, never an alternate readback/forward authorization.
// A Client literal or mutated exported descriptor remains nonoperational.
func (c Client) CheckControllerPeer(actual KernelPeer) error {
	if c.peerBinding == nil || c.peerBinding.Validate() != nil || c.Descriptor.Validate() != nil || !reflect.DeepEqual(c.Descriptor, c.admittedDescriptor) || c.Timeout <= 0 || c.Timeout > 10*time.Second {
		return ErrInvalid
	}
	p := c.peerBinding
	h, err := BindingHash(c.Descriptor.Binding)
	if err != nil || h != p.BindingHash || c.Descriptor.ReplicaID != p.ConsumerReplicaID || c.Descriptor.ProcessLaunchID != p.ConsumerProcessLaunchID || c.Descriptor.ControllerID != p.ControllerID || c.Descriptor.AuthorityScope != p.AuthorityScope || c.Descriptor.ScopeID != p.ScopeID || actual != p.ControllerExpectedKernelPeer.Kernel() {
		return ErrInvalid
	}
	return nil
}
