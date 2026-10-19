package mmadmissionwire

import (
	"crypto/tls"
	"encoding/json"
	"reflect"
	"testing"
)

func bootstrapFixture(t *testing.T) ([]byte, KernelPeer, LaunchExpectations, tls.Certificate) {
	t.Helper()
	d, _, _ := testObservation()
	raw, _ := json.Marshal(d)
	own := launchTestCertificate(t, false, false)
	return raw, KernelPeer{123, 65532, 65532}, LaunchExpectations{ID{1}, Hash{2}, Hash{3}, Hash{4}, Hash{5}, ID{6}, ID{7}}, own
}
func TestPrivateBootstrapExactIndependentInputs(t *testing.T) {
	descriptor, recipient, expected, own := bootstrapFixture(t)
	raw, e := encodeNativeBootstrap(descriptor, recipient, expected, own)
	if e != nil {
		t.Fatal("complete private launch inputs rejected", e)
	}
	got, cert, e := decodeNativeBootstrap(raw, descriptor, recipient)
	if e != nil || got != expected || !reflect.DeepEqual(cert.Certificate, own.Certificate) {
		t.Fatal("private launch input roundtrip", e)
	}
	changed := append([]byte(nil), descriptor...)
	changed = append(changed, ' ')
	if _, _, e = decodeNativeBootstrap(raw, changed, recipient); e == nil {
		t.Fatal("different descriptor bytes accepted")
	}
	wrong := recipient
	wrong.PID++
	if _, _, e = decodeNativeBootstrap(raw, descriptor, wrong); e == nil {
		t.Fatal("copied launch accepted by another PID")
	}
	if _, _, e = decodeNativeBootstrap(append(raw, []byte(`{}`)...), descriptor, recipient); e == nil {
		t.Fatal("trailing private envelope accepted")
	}
	bad := expected
	bad.ConsumerObservationHash = Hash{}
	if _, e = encodeNativeBootstrap(descriptor, recipient, bad, own); e == nil {
		t.Fatal("absent independent consumer observation accepted")
	}
	wrongKey := own
	wrongKey.PrivateKey = launchTestCertificate(t, false, false).PrivateKey
	if _, e = encodeNativeBootstrap(descriptor, recipient, expected, wrongKey); e == nil {
		t.Fatal("private key does not match own launch certificate")
	}
	serverCertificate := launchTestCertificate(t, true, false)
	if _, e = encodeNativeBootstrap(descriptor, recipient, expected, serverCertificate); e == nil {
		t.Fatal("server/admission credential accepted as serving IPC identity")
	}
	for _, change := range []func(*nativeBootstrapRecord){
		func(r *nativeBootstrapRecord) { r.Version++ },
		func(r *nativeBootstrapRecord) { r.Profile = "different" },
		func(r *nativeBootstrapRecord) { r.Recipient.UID++ },
		func(r *nativeBootstrapRecord) { r.Recipient.GID++ },
		func(r *nativeBootstrapRecord) { r.Expected.ConsumerNamespaceID = ID{} },
		func(r *nativeBootstrapRecord) { r.Expected.CustodyContractHash = Hash{} },
	} {
		var r nativeBootstrapRecord
		if json.Unmarshal(raw, &r) != nil {
			t.Fatal("fixture decode")
		}
		change(&r)
		bad, _ := json.Marshal(r)
		if _, _, e = decodeNativeBootstrap(bad, descriptor, recipient); e == nil {
			t.Fatal("changed private launch record accepted")
		}
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	fields["PROFILE"] = fields["profile"]
	alias, _ := json.Marshal(fields)
	if _, _, e = decodeNativeBootstrap(alias, descriptor, recipient); e == nil {
		t.Fatal("case alias in private record accepted")
	}
}
