//go:build linux && amd64

package mmadmissionctl

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	w "voice/backend/pkg/mmadmissionwire"
)

type childInput struct {
	TargetPID    int
	Mode         string
	Journal      string
	Descriptor   w.DescriptorV1
	Binding      w.ReadbackPeerBindingV1
	Expectations w.LaunchExpectations
	PrivateKey   []byte
}
type fixedReadbackRegistry struct{ launch ReadbackRegistration }

func (r fixedReadbackRegistry) Lookup(peer w.KernelPeer) (ReadbackRegistration, error) {
	if peer != r.launch.Binding.ConsumerExpectedKernelPeer.Kernel() {
		return ReadbackRegistration{}, w.ErrInvalid
	}
	return r.launch, nil
}
func (r fixedReadbackRegistry) Current(h w.Hash) bool { return h == r.launch.Binding.PeerBindingHash }
func init() {
	if os.Getenv("MM_ADMISSION_CHILD") != "1" {
		return
	}
	var in childInput
	inputDecoder := json.NewDecoder(os.Stdin)
	if inputDecoder.Decode(&in) != nil {
		os.Exit(2)
	}
	if in.Mode == "driver-observer" {
		b, e := os.ReadFile(fmt.Sprintf("/proc/%d/exe", in.TargetPID))
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(20)
		}
		ns, e := os.Readlink(fmt.Sprintf("/proc/%d/ns/pid", in.TargetPID))
		if e != nil {
			os.Exit(21)
		}
		fmt.Println(w.Hash(sha256.Sum256(b)).String() + " " + ns)
		os.Exit(0)
	}
	key, e := x509.ParsePKCS8PrivateKey(in.PrivateKey)
	if e != nil {
		os.Exit(3)
	}
	der := in.Binding.ConsumerCertificateDER
	if in.Mode == "server" {
		der = in.Binding.ControllerCertificateDER
	}
	own := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	if e = w.DenyFurtherExec(); e != nil {
		fmt.Fprintln(os.Stderr, "sandbox", e)
		os.Exit(4)
	}
	// Actual post-initialization exec is denied, not merely a declared flag.
	if e = exec.Command("/bin/true").Run(); !errors.Is(e, syscall.EPERM) {
		fmt.Fprintln(os.Stderr, "exec was not denied", e)
		os.Exit(5)
	}
	if in.Mode == "server" {
		reader, e := OpenReadOnlyJournal(in.Journal)
		if e != nil {
			os.Exit(6)
		}
		defer reader.Close()
		if _, e = reader.db.Exec(`DELETE FROM intents`); e == nil {
			os.Exit(7)
		}
		l, e := net.ListenUnix("unix", &net.UnixAddr{Name: w.ReadbackSocket, Net: "unix"})
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(8)
		}
		if os.Chmod(w.ReadbackSocket, 0666) != nil {
			os.Exit(9)
		}
		fmt.Println("READY_NOEXEC")
		if e = reader.ServeReadback(context.Background(), l, fixedReadbackRegistry{ReadbackRegistration{Binding: in.Binding, OwnTLS: own}}); e != nil {
			os.Exit(10)
		}
	} else {
		if os.Getuid() != 65532 || os.Getgid() != 65532 {
			os.Exit(11)
		}
		if _, e = os.ReadFile(fmt.Sprintf("/proc/%d/exe", in.Binding.ControllerExpectedKernelPeer.PID)); !errors.Is(e, os.ErrPermission) {
			fmt.Fprintln(os.Stderr, "root exe access must stay denied", e)
			os.Exit(12)
		}
		protected, e := w.LoadValidatedPeerBinding(in.Descriptor, in.Expectations)
		if e != nil {
			fmt.Fprintln(os.Stderr, "bootstrap", e)
			os.Exit(13)
		}
		client, e := w.NewReadbackClient(in.Descriptor, protected, own, time.Second)
		if e != nil {
			os.Exit(14)
		}
		if _, e = client.ObserveStartup(context.Background()); e != nil {
			fmt.Fprintln(os.Stderr, "startup", e)
			os.Exit(15)
		}
		fmt.Println("ARMED_TLS_NOEXEC_OK")
		var release string
		if inputDecoder.Decode(&release) != nil || release != "OPEN" {
			os.Exit(17)
		}
		if _, e = client.ObserveFresh(context.Background(), w.ID{40}, w.Hash{41}); e != nil {
			fmt.Fprintln(os.Stderr, "fresh", e)
			os.Exit(16)
		}
		fmt.Println("CROSS_UID_TLS_NOEXEC_OK")
		os.Exit(0)
	}
	os.Exit(0)
}
func ipcCertificate(t *testing.T, server bool) (tls.Certificate, []byte) {
	t.Helper()
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "owned launch IPC"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	if server {
		tpl.DNSNames = []string{"mm-local-controller.test"}
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	} else {
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	der, e := x509.CreateCertificate(rand.Reader, tpl, tpl, pub, key)
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, encoded
}
func TestActualCrossUIDLaunchTLS(t *testing.T) {
	if os.Getenv("MM_ADMISSION_IPC_TEST") != "1" {
		t.Skip("explicit owned isolated Linux IPC runner required")
	}
	if os.Getuid() != 0 {
		t.Fatal("driver oracle requires owned root runner, serving remains65532")
	}
	ctx := context.Background()
	dir := t.TempDir()
	j, e := CreateLocalJournal(filepath.Join(dir, "controller.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	d := &unitDriver{}
	c, e := NewLocalController(j, d, w.ID{1}, w.ID{2}, w.ID{3})
	if e != nil {
		t.Fatal(e)
	}
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	binary, e := os.ReadFile(executable)
	if e != nil {
		t.Fatal(e)
	}
	exeHash := sha256.Sum256(binary)
	spec := unitSpec()
	spec.Replicas[0].ExecutableHash = exeHash
	spec.Replicas[0].RuntimeUID = 65532
	r, e := c.Execute(ctx, Command{Version: 1, Kind: "PREPARE", OperationID: w.ID{11}, LaunchSpec: &spec})
	if e != nil {
		t.Fatal(e)
	}
	for i, kind := range []string{"ARM", "OPEN"} {
		r, e = c.Execute(ctx, Command{Version: 1, Kind: kind, OperationID: w.ID{byte(12 + i)}, ExpectedRevision: r.Revision, TransitionID: w.ID{11}})
		if e != nil {
			t.Fatal(e)
		}
	}
	if e = os.MkdirAll(filepath.Dir(w.ReadbackSocket), 0755); e != nil {
		t.Fatal(e)
	}
	server := exec.Command(executable, "-test.run=^$")
	server.Env = append(os.Environ(), "MM_ADMISSION_CHILD=1")
	server.Stderr = os.Stderr
	sin, _ := server.StdinPipe()
	sout, _ := server.StdoutPipe()
	if e = server.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = server.Process.Kill(); _ = server.Wait() }()
	client := exec.Command(executable, "-test.run=^$")
	client.Env = append(os.Environ(), "MM_ADMISSION_CHILD=1")
	client.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65532, Gid: 65532}}
	client.Stderr = os.Stderr
	cin, _ := client.StdinPipe()
	cout, _ := client.StdoutPipe()
	if e = client.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = client.Process.Kill(); _ = client.Wait() }()
	// The owned driver actually observes both binaries before issuing pins.
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/exe", server.Process.Pid))
	if e != nil || sha256.Sum256(b) != exeHash {
		t.Fatal("driver controller executable observation", e)
	}
	// The trusted driver uses a separate owned same-UID observer before key
	// delivery. Serving gains no capability or inspection process of its own.
	observer := exec.Command(executable, "-test.run=^$")
	observer.Env = append(os.Environ(), "MM_ADMISSION_CHILD=1")
	observer.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65532, Gid: 65532}}
	observer.Stderr = os.Stderr
	request, _ := json.Marshal(map[string]any{"Mode": "driver-observer", "TargetPID": client.Process.Pid})
	observer.Stdin = strings.NewReader(string(request))
	observed, e := observer.Output()
	if e != nil {
		t.Fatal("driver serving executable observation", e)
	}
	parts := strings.Fields(string(observed))
	if len(parts) != 2 || parts[0] != w.Hash(exeHash).String() {
		t.Fatal("driver observed wrong serving binary")
	}
	serverNS, e := os.Readlink(fmt.Sprintf("/proc/%d/ns/pid", server.Process.Pid))
	if e != nil {
		t.Fatal(e)
	}
	clientNS := parts[1]
	if serverNS != clientNS {
		t.Fatal("incompatible PID namespace", e)
	}
	nsHash := sha256.Sum256([]byte(serverNS))
	var namespace w.ID
	copy(namespace[:], nsHash[:16])
	s := r.Snapshot
	replica := s.Spec.Replicas[0]
	descriptor := w.DescriptorV1{ProtocolVersion: 1, Purpose: w.NativePurpose, AuthorityScope: w.LocalOwned, ScopeID: s.Identity.ScopeID, ControllerID: s.Identity.ControllerID, Binding: s.Binding, DatabaseLineageID: s.Identity.DatabaseLineageID, ReplicaID: replica.ReplicaID, ProcessLaunchID: replica.LaunchID, AdmittedImageDigest: s.Spec.ImageDigest, AdmittedExecutableHash: replica.ExecutableHash, AdmittedArgv: replica.Argv, ConfigContractHash: s.Spec.ConfigContractHash, CredentialRefsManifestHash: s.Spec.CredentialRefsManifestHash, UpstreamContractManifestHash: s.Spec.UpstreamContractManifestHash, BrokerContractManifestHash: s.Spec.BrokerContractManifestHash, ReadbackProtocol: w.Protocol}
	sc, sk := ipcCertificate(t, true)
	cc, ck := ipcCertificate(t, false)
	scert, _ := x509.ParseCertificate(sc.Certificate[0])
	ccert, _ := x509.ParseCertificate(cc.Certificate[0])
	bindingHash, _ := w.BindingHash(s.Binding)
	p := w.ReadbackPeerBindingV1{Version: 1, Transport: w.LaunchTransport, AuthorityScope: w.LocalOwned, ScopeID: s.Identity.ScopeID, EnvironmentID: s.Identity.EnvironmentID, InstallationID: s.Identity.InstallationID, ControllerID: s.Identity.ControllerID, BindingHash: bindingHash, ConsumerReplicaID: replica.ReplicaID, ConsumerProcessLaunchID: replica.LaunchID, ControllerProcessLaunchID: w.ID{80}, ControllerLaunchObservationHash: w.Hash{81}, ConsumerLaunchObservationHash: s.Observation.LaunchesHash, SocketMountObservationHash: w.Hash{82}, ControllerExpectedKernelPeer: w.NamespacedKernelPeer{PID: int32(server.Process.Pid), UID: 0, GID: 0, ObserverNamespaceID: namespace}, ConsumerExpectedKernelPeer: w.NamespacedKernelPeer{PID: int32(client.Process.Pid), UID: 65532, GID: 65532, ObserverNamespaceID: namespace}, ControllerImageDigest: s.Spec.ImageDigest, ControllerExecutableHash: exeHash, ConsumerImageDigest: s.Spec.ImageDigest, ConsumerExecutableHash: exeHash, ControllerCertificateDERHash: sha256.Sum256(scert.Raw), ControllerSPKIHash: sha256.Sum256(scert.RawSubjectPublicKeyInfo), ConsumerCertificateDERHash: sha256.Sum256(ccert.Raw), ConsumerSPKIHash: sha256.Sum256(ccert.RawSubjectPublicKeyInfo), ControllerCertificateDER: scert.Raw, ConsumerCertificateDER: ccert.Raw, ControllerServerName: "mm-local-controller.test", LaunchKeyCustodyContractHash: w.Hash{83}}
	p.PeerBindingHash, e = w.PeerBindingHash(p)
	if e != nil {
		t.Fatal(e)
	}
	s.Observation.Peers = []ReplicaPeer{{ReplicaID: replica.ReplicaID, LaunchID: replica.LaunchID, PID: int32(client.Process.Pid), UID: 65532, GID: 65532, ExecutableHash: exeHash}}
	openRaw, _ := json.Marshal(s)
	s.State = w.Armed
	s.OpenReceipt = nil
	s.IngressConfirmed = false
	raw, _ := json.Marshal(s)
	if _, e = j.db.Exec(`UPDATE head SET snapshot=? WHERE singleton=1`, raw); e != nil {
		t.Fatal(e)
	}
	public, _ := json.Marshal(p)
	if e = os.WriteFile(w.PeerBindingPath, public, 0644); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(w.PeerBindingPath)
	defer os.Remove(w.ReadbackSocket)
	if e = json.NewEncoder(sin).Encode(childInput{Mode: "server", Journal: j.path, Binding: p, PrivateKey: sk}); e != nil {
		t.Fatal(e)
	}
	_ = sin.Close()
	ready := bufio.NewScanner(sout)
	if !ready.Scan() || ready.Text() != "READY_NOEXEC" {
		t.Fatal("root readback not sandboxed/prepared", ready.Text())
	}
	expectations := w.LaunchExpectations{ControllerProcessLaunchID: p.ControllerProcessLaunchID, ControllerObservationHash: p.ControllerLaunchObservationHash, ConsumerObservationHash: p.ConsumerLaunchObservationHash, SocketMountObservationHash: p.SocketMountObservationHash, CustodyContractHash: p.LaunchKeyCustodyContractHash, ControllerNamespaceID: namespace, ConsumerNamespaceID: namespace}
	if e = json.NewEncoder(cin).Encode(childInput{Mode: "client", Descriptor: descriptor, Binding: p, Expectations: expectations, PrivateKey: ck}); e != nil {
		t.Fatal(e)
	}
	success := bufio.NewScanner(cout)
	if !success.Scan() || success.Text() != "ARMED_TLS_NOEXEC_OK" {
		t.Fatal("ARMED startup failed", success.Text())
	}
	if _, e = j.db.Exec(`UPDATE head SET snapshot=? WHERE singleton=1`, openRaw); e != nil {
		t.Fatal(e)
	}
	if e = json.NewEncoder(cin).Encode("OPEN"); e != nil {
		t.Fatal(e)
	}
	_ = cin.Close()
	if !success.Scan() || success.Text() != "CROSS_UID_TLS_NOEXEC_OK" {
		t.Fatal("cross UID client failed", success.Text())
	}
	if e = client.Wait(); e != nil {
		t.Fatal(e)
	}
	t.Logf("actual root PID%d UID0/GID0 vs serving PID%d UID65532/GID65532; shared namespace%s; both exec denied; root exe denied to serving; binary%s", server.Process.Pid, client.Process.Pid, serverNS, w.Hash(exeHash).String())
	t.Log("upstream/launch-image/mount/custody evidence remains explicit component fixture; not native or installed qualification")
}
