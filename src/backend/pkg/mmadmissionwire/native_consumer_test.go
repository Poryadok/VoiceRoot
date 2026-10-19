package mmadmissionwire

import (
	"testing"
	"time"
)

func TestNativeConsumerStartupFactsAndOpaqueControllerPeer(t *testing.T) {
	d, _, response := testObservation()
	observation := StartupObservation{response: response}
	want := AdmissionStampV1{Version: 1, BindingHash: response.BindingHash, DatabaseLineageID: response.DatabaseLineageID, ContinuityHistoryRootHash: response.ContinuityReceiptsRootHash}
	if observation.Stamp() != want || observation.LaunchObservationHash() != response.LaunchObservationHash {
		t.Fatal("authenticated startup material was not exposed exactly")
	}
	server, own := launchTestCertificate(t, true, false), launchTestCertificate(t, false, false)
	p := launchTestBinding(t, server, own)
	x := LaunchExpectations{p.ControllerProcessLaunchID, p.ControllerLaunchObservationHash, p.ConsumerLaunchObservationHash, p.SocketMountObservationHash, p.LaunchKeyCustodyContractHash, p.ControllerExpectedKernelPeer.ObserverNamespaceID, p.ConsumerExpectedKernelPeer.ObserverNamespaceID}
	validated, err := validatePeerBinding(d, p, x)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewReadbackClient(d, validated, own, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	actual := p.ControllerExpectedKernelPeer.Kernel()
	if client.CheckControllerPeer(actual) != nil {
		t.Fatal("exact independently bound controller peer rejected")
	}
	for _, kind := range []string{"pid", "uid", "gid", "descriptor", "literal", "zero"} {
		t.Run(kind, func(t *testing.T) {
			c, peer := client, actual
			switch kind {
			case "pid":
				peer.PID++
			case "uid":
				peer.UID++
			case "gid":
				peer.GID++
			case "descriptor":
				c.Descriptor.ControllerID[0]++
			case "literal":
				c = Client{Descriptor: d, ControllerPeer: PeerIdentity{PID: actual.PID, UID: actual.UID, GID: actual.GID}, Timeout: time.Second}
			case "zero":
				peer = KernelPeer{}
			}
			if c.CheckControllerPeer(peer) == nil {
				t.Fatal("unqualified controller peer accepted")
			}
		})
	}
	if (StartupObservation{}).Stamp().Validate() == nil || (StartupObservation{}).LaunchObservationHash() != (Hash{}) {
		t.Fatal("zero observation supplied startup authority")
	}
}
