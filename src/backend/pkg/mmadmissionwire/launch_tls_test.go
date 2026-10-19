package mmadmissionwire

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"testing"
	"time"
)

func launchTestCertificate(t *testing.T, server bool, expired bool) tls.Certificate {
	t.Helper()
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	until := now.Add(time.Hour)
	if expired {
		until = now.Add(-time.Hour)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "isolated MM IPC test"}, NotBefore: now.Add(-2 * time.Hour), NotAfter: until, KeyUsage: x509.KeyUsageDigitalSignature}
	if server {
		template.DNSNames = []string{"mm-controller.test"}
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	der, e := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	if e != nil {
		t.Fatal(e)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
func launchTestBinding(t *testing.T, server, client tls.Certificate) ReadbackPeerBindingV1 {
	d, _, _ := testObservation()
	h, _ := BindingHash(d.Binding)
	sc, _ := x509.ParseCertificate(server.Certificate[0])
	cc, _ := x509.ParseCertificate(client.Certificate[0])
	p := ReadbackPeerBindingV1{Version: 1, Transport: LaunchTransport, AuthorityScope: d.AuthorityScope, ScopeID: d.ScopeID, EnvironmentID: d.Binding.EnvironmentID, InstallationID: d.Binding.InstallationID, ControllerID: d.ControllerID, BindingHash: h, ConsumerReplicaID: d.ReplicaID, ConsumerProcessLaunchID: d.ProcessLaunchID, ControllerProcessLaunchID: ID{40}, ControllerLaunchObservationHash: Hash{41}, ConsumerLaunchObservationHash: Hash{35}, SocketMountObservationHash: Hash{43}, ControllerExpectedKernelPeer: NamespacedKernelPeer{1, 0, 0, ID{44}}, ConsumerExpectedKernelPeer: NamespacedKernelPeer{2, 65532, 65532, ID{44}}, ControllerImageDigest: d.AdmittedImageDigest, ControllerExecutableHash: Hash{45}, ConsumerImageDigest: d.AdmittedImageDigest, ConsumerExecutableHash: d.AdmittedExecutableHash, ControllerCertificateDERHash: sha256.Sum256(sc.Raw), ControllerSPKIHash: sha256.Sum256(sc.RawSubjectPublicKeyInfo), ConsumerCertificateDERHash: sha256.Sum256(cc.Raw), ConsumerSPKIHash: sha256.Sum256(cc.RawSubjectPublicKeyInfo), ControllerCertificateDER: sc.Raw, ConsumerCertificateDER: cc.Raw, ControllerServerName: "mm-controller.test", LaunchKeyCustodyContractHash: Hash{46}}
	var e error
	p.PeerBindingHash, e = PeerBindingHash(p)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func tlsPair(t *testing.T, server, client *tls.Config) (error, error) {
	t.Helper()
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	_ = a.SetDeadline(time.Now().Add(time.Second))
	_ = b.SetDeadline(time.Now().Add(time.Second))
	ch := make(chan error, 1)
	go func() { ch <- tls.Server(a, server).HandshakeContext(context.Background()) }()
	e := tls.Client(b, client).HandshakeContext(context.Background())
	return <-ch, e
}
func TestLaunchTLSNormalVerificationAndExactPins(t *testing.T) {
	server, client := launchTestCertificate(t, true, false), launchTestCertificate(t, false, false)
	p := launchTestBinding(t, server, client)
	sc, e := launchTLSConfig(p, server, true)
	if e != nil {
		t.Fatal(e)
	}
	cc, e := launchTLSConfig(p, client, false)
	if e != nil {
		t.Fatal(e)
	}
	if a, b := tlsPair(t, sc, cc); a != nil || b != nil {
		t.Fatal("exact pins failed", a, b)
	}
	for _, kind := range []string{"SAN", "ALPN", "unknown-client", "expired", "extra-certificate"} {
		t.Run(kind, func(t *testing.T) {
			s, c := sc.Clone(), cc.Clone()
			switch kind {
			case "SAN":
				c.ServerName = "wrong.test"
			case "ALPN":
				c.NextProtos = []string{"wrong"}
			case "unknown-client":
				c.Certificates = []tls.Certificate{launchTestCertificate(t, false, false)}
			case "expired":
				expired := launchTestCertificate(t, true, true)
				q := launchTestBinding(t, expired, client)
				s, _ = launchTLSConfig(q, expired, true)
				c, _ = launchTLSConfig(q, client, false)
			case "extra-certificate":
				c.Certificates = []tls.Certificate{client}
				c.Certificates[0].Certificate = append(append([][]byte(nil), client.Certificate...), server.Certificate[0])
			}
			if a, b := tlsPair(t, s, c); a == nil && b == nil {
				t.Fatal("invalid transport authenticated")
			}
		})
	}
	if _, e := launchTLSConfig(p, launchTestCertificate(t, false, false), false); e == nil {
		t.Fatal("wrong own launch key accepted")
	}
}
func TestPeerBindingStrictAndIndependentExpectations(t *testing.T) {
	server, client := launchTestCertificate(t, true, false), launchTestCertificate(t, false, false)
	p := launchTestBinding(t, server, client)
	d, _, _ := testObservation()
	x := LaunchExpectations{p.ControllerProcessLaunchID, p.ControllerLaunchObservationHash, p.ConsumerLaunchObservationHash, p.SocketMountObservationHash, p.LaunchKeyCustodyContractHash, p.ControllerExpectedKernelPeer.ObserverNamespaceID, p.ConsumerExpectedKernelPeer.ObserverNamespaceID}
	if _, e := validatePeerBinding(d, p, x); e != nil {
		t.Fatal(e)
	}
	x.ControllerObservationHash[0]++
	if _, e := validatePeerBinding(d, p, x); e == nil {
		t.Fatal("copied record replaces independent observation")
	}
	raw, _ := json.Marshal(p)
	var out ReadbackPeerBindingV1
	if Decode(raw, &out) != nil {
		t.Fatal("record roundtrip")
	}
	p.ControllerCertificateDER[0] ^= 1
	if p.Validate() == nil {
		t.Fatal("changed certificate accepted")
	}
	if _, e := NewReadbackClient(d, ValidatedPeerBinding{}, client, time.Second); e == nil {
		t.Fatal("missing protected binding accepted")
	}
	if _, e := (Client{Descriptor: d, ControllerPeer: PeerIdentity{PID: 1, ExecutableHash: Hash{1}}, Timeout: time.Second}).ObserveStartup(context.Background()); e == nil {
		t.Fatal("legacy raw constructor remained operational")
	}
}
func TestTLSExchangeRequiresAuthenticatedCloseNotify(t *testing.T) {
	server, client := launchTestCertificate(t, true, false), launchTestCertificate(t, false, false)
	p := launchTestBinding(t, server, client)
	sc, _ := launchTLSConfig(p, server, true)
	cc, _ := launchTLSConfig(p, client, false)
	for _, kind := range []string{"clean", "raw-EOF", "trailing-data"} {
		t.Run(kind, func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			_ = a.SetDeadline(time.Now().Add(time.Second))
			_ = b.SetDeadline(time.Now().Add(time.Second))
			s := tls.Server(requireTLSCloseNotifyConn{a}, sc)
			c := tls.Client(requireTLSCloseNotifyConn{b}, cc)
			result := make(chan error, 1)
			_, _, out := testObservation()
			go func() {
				if e := s.Handshake(); e != nil {
					result <- e
					return
				}
				if e := WriteFrame(s, out); e != nil {
					result <- e
					return
				}
				switch kind {
				case "clean":
					result <- s.CloseWrite()
				case "raw-EOF":
					result <- a.Close()
				case "trailing-data":
					_, e := s.Write([]byte{1})
					result <- e
					_ = s.CloseWrite()
				}
			}()
			if e := c.Handshake(); e != nil {
				t.Fatal(e)
			}
			var response ObserveResponseV1
			if e := ReadFrame(c, &response); e != nil {
				t.Fatal(e)
			}
			var tail [1]byte
			n, e := c.Read(tail[:])
			if kind == "clean" {
				if n != 0 || e != io.EOF {
					t.Fatal("clean notify rejected", n, e)
				}
			} else if n == 0 && e == io.EOF {
				t.Fatal("unauthenticated/trailing termination accepted")
			}
			if e := <-result; e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestPeerBindingGolden(t *testing.T) {
	makeCert := func(server bool) tls.Certificate {
		seed := make([]byte, ed25519.SeedSize)
		seed[0] = 1
		if server {
			seed[0] = 2
		}
		key := ed25519.NewKeyFromSeed(seed)
		tpl := &x509.Certificate{SerialNumber: big.NewInt(int64(seed[0])), Subject: pkix.Name{CommonName: "MM IPC golden"}, NotBefore: time.Unix(1700000000, 0), NotAfter: time.Unix(4102444800, 0), KeyUsage: x509.KeyUsageDigitalSignature}
		if server {
			tpl.DNSNames = []string{"mm-controller.test"}
			tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		} else {
			tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}
		der, e := x509.CreateCertificate(rand.Reader, tpl, tpl, key.Public(), key)
		if e != nil {
			t.Fatal(e)
		}
		return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	}
	p := launchTestBinding(t, makeCert(true), makeCert(false))
	if p.PeerBindingHash.String() != "17adc525f22b8d1afc5c0cd33bc3ac2f0f8932f988c8e9542f981fb434fc53b5" {
		t.Fatal("peer binding golden changed")
	}
	t.Log("peer golden", p.PeerBindingHash.String())
}
