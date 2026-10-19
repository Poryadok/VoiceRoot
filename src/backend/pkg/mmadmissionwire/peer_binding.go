package mmadmissionwire

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
)

const PeerBindingPath = "/run/voice/installation/mm-readback-peers-v1.json"
const LaunchTransport = "MM_ADMISSION_UDS_MTLS_LAUNCH_V1"
const ReadbackALPN = "voice-mm-admission-readback/1"

type CertificateDER []byte

func (d *CertificateDER) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) != nil {
		return ErrInvalid
	}
	raw, e := base64.StdEncoding.DecodeString(s)
	if e != nil || len(raw) == 0 || len(raw) > 8192 || base64.StdEncoding.EncodeToString(raw) != s {
		return ErrInvalid
	}
	*d = raw
	return nil
}

// KernelPeer is observed on the connected socket, never supplied by a request.
type KernelPeer struct {
	PID int32  `json:"pid"`
	UID uint32 `json:"uid"`
	GID uint32 `json:"gid"`
}
type NamespacedKernelPeer struct {
	PID                 int32  `json:"pid"`
	UID                 uint32 `json:"uid"`
	GID                 uint32 `json:"gid"`
	ObserverNamespaceID ID     `json:"observer_namespace_id"`
}

func (p NamespacedKernelPeer) Kernel() KernelPeer { return KernelPeer{p.PID, p.UID, p.GID} }

// ReadbackPeerBindingV1 contains PUBLIC launch pins only. Executable and image
// hashes are DRIVER_OBSERVED provenance, not observations made by the consumer.
type ReadbackPeerBindingV1 struct {
	Version                         uint32               `json:"version"`
	Transport                       string               `json:"transport"`
	AuthorityScope                  Scope                `json:"authority_scope"`
	ScopeID                         ID                   `json:"scope_id"`
	EnvironmentID                   ID                   `json:"environment_id"`
	InstallationID                  ID                   `json:"installation_id"`
	ControllerID                    ID                   `json:"controller_id"`
	BindingHash                     Hash                 `json:"binding_sha256"`
	ConsumerReplicaID               ID                   `json:"consumer_replica_id"`
	ConsumerProcessLaunchID         ID                   `json:"consumer_process_launch_id"`
	ControllerProcessLaunchID       ID                   `json:"controller_process_launch_id"`
	ControllerLaunchObservationHash Hash                 `json:"controller_launch_observation_sha256"`
	ConsumerLaunchObservationHash   Hash                 `json:"consumer_launch_observation_sha256"`
	SocketMountObservationHash      Hash                 `json:"socket_mount_observation_sha256"`
	ControllerExpectedKernelPeer    NamespacedKernelPeer `json:"controller_expected_kernel_peer"`
	ConsumerExpectedKernelPeer      NamespacedKernelPeer `json:"consumer_expected_kernel_peer"`
	ControllerImageDigest           string               `json:"controller_image_digest"`
	ControllerExecutableHash        Hash                 `json:"controller_executable_sha256"`
	ConsumerImageDigest             string               `json:"consumer_image_digest"`
	ConsumerExecutableHash          Hash                 `json:"consumer_executable_sha256"`
	ControllerCertificateDERHash    Hash                 `json:"controller_certificate_der_sha256"`
	ControllerSPKIHash              Hash                 `json:"controller_spki_sha256"`
	ConsumerCertificateDERHash      Hash                 `json:"consumer_certificate_der_sha256"`
	ConsumerSPKIHash                Hash                 `json:"consumer_spki_sha256"`
	ControllerCertificateDER        CertificateDER       `json:"controller_certificate_der"`
	ConsumerCertificateDER          CertificateDER       `json:"consumer_certificate_der"`
	ControllerServerName            string               `json:"controller_server_name"`
	LaunchKeyCustodyContractHash    Hash                 `json:"launch_key_custody_contract_sha256"`
	PeerBindingHash                 Hash                 `json:"peer_binding_sha256"`
}

func PeerBindingHash(p ReadbackPeerBindingV1) (Hash, error) {
	if p.Version != 1 || p.Transport != LaunchTransport || !scope(p.AuthorityScope) || !text(p.ControllerServerName, 253) {
		return Hash{}, ErrInvalid
	}
	for _, id := range []ID{p.ScopeID, p.EnvironmentID, p.InstallationID, p.ControllerID, p.ConsumerReplicaID, p.ConsumerProcessLaunchID, p.ControllerProcessLaunchID, p.ControllerExpectedKernelPeer.ObserverNamespaceID, p.ConsumerExpectedKernelPeer.ObserverNamespaceID} {
		if id == (ID{}) {
			return Hash{}, ErrInvalid
		}
	}
	if p.ControllerExpectedKernelPeer.PID <= 0 || p.ConsumerExpectedKernelPeer.PID <= 0 {
		return Hash{}, ErrInvalid
	}
	for _, label := range strings.Split(p.ControllerServerName, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return Hash{}, ErrInvalid
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
				return Hash{}, ErrInvalid
			}
		}
	}
	for _, h := range []Hash{p.BindingHash, p.ControllerLaunchObservationHash, p.ConsumerLaunchObservationHash, p.SocketMountObservationHash, p.ControllerExecutableHash, p.ConsumerExecutableHash, p.LaunchKeyCustodyContractHash} {
		if h == (Hash{}) {
			return Hash{}, ErrInvalid
		}
	}
	for _, image := range []string{p.ControllerImageDigest, p.ConsumerImageDigest} {
		if len(image) != 71 || image[:7] != "sha256:" {
			return Hash{}, ErrInvalid
		}
		if _, e := ParseHash(image[7:]); e != nil {
			return Hash{}, ErrInvalid
		}
	}
	for index, pin := range []struct {
		der        []byte
		leaf, spki Hash
	}{{p.ControllerCertificateDER, p.ControllerCertificateDERHash, p.ControllerSPKIHash}, {p.ConsumerCertificateDER, p.ConsumerCertificateDERHash, p.ConsumerSPKIHash}} {
		if len(pin.der) == 0 || len(pin.der) > 8192 || sha256.Sum256(pin.der) != pin.leaf {
			return Hash{}, ErrInvalid
		}
		cert, e := x509.ParseCertificate(pin.der)
		if e != nil || sha256.Sum256(cert.RawSubjectPublicKeyInfo) != pin.spki {
			return Hash{}, ErrInvalid
		}
		usage := x509.ExtKeyUsageServerAuth
		if index == 1 {
			usage = x509.ExtKeyUsageClientAuth
		}
		if cert.PublicKeyAlgorithm != x509.Ed25519 || cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) != nil || len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != usage || len(cert.UnknownExtKeyUsage) != 0 {
			return Hash{}, ErrInvalid
		}
		if index == 0 && (len(cert.DNSNames) != 1 || cert.DNSNames[0] != p.ControllerServerName || cert.VerifyHostname(p.ControllerServerName) != nil) {
			return Hash{}, ErrInvalid
		}
	}
	e := newEncoder("voice.mm.readback-peer-binding.v1")
	e.u32(p.Version)
	e.str(p.Transport)
	e.str(string(p.AuthorityScope))
	for _, id := range []ID{p.ScopeID, p.EnvironmentID, p.InstallationID, p.ControllerID} {
		e.id(id)
	}
	e.hash(p.BindingHash)
	for _, id := range []ID{p.ConsumerReplicaID, p.ConsumerProcessLaunchID, p.ControllerProcessLaunchID} {
		e.id(id)
	}
	for _, h := range []Hash{p.ControllerLaunchObservationHash, p.ConsumerLaunchObservationHash, p.SocketMountObservationHash} {
		e.hash(h)
	}
	for _, k := range []NamespacedKernelPeer{p.ControllerExpectedKernelPeer, p.ConsumerExpectedKernelPeer} {
		e.u32(uint32(k.PID))
		e.u32(k.UID)
		e.u32(k.GID)
		e.id(k.ObserverNamespaceID)
	}
	e.str(p.ControllerImageDigest)
	e.hash(p.ControllerExecutableHash)
	e.str(p.ConsumerImageDigest)
	e.hash(p.ConsumerExecutableHash)
	for _, h := range []Hash{p.ControllerCertificateDERHash, p.ControllerSPKIHash, p.ConsumerCertificateDERHash, p.ConsumerSPKIHash} {
		e.hash(h)
	}
	for _, der := range [][]byte{p.ControllerCertificateDER, p.ConsumerCertificateDER} {
		e.u32(uint32(len(der)))
		e.Write(der)
	}
	e.str(p.ControllerServerName)
	e.hash(p.LaunchKeyCustodyContractHash)
	return e.sum()
}
func (p ReadbackPeerBindingV1) Validate() error {
	h, e := PeerBindingHash(p)
	if e != nil || h != p.PeerBindingHash {
		return ErrInvalid
	}
	return nil
}

// LaunchExpectations come from the protected driver's launch observations. They
// cannot be populated from an Observe response or from the public pin file alone.
type LaunchExpectations struct {
	ControllerProcessLaunchID                                                                           ID
	ControllerObservationHash, ConsumerObservationHash, SocketMountObservationHash, CustodyContractHash Hash
	ControllerNamespaceID, ConsumerNamespaceID                                                          ID
}
type ValidatedPeerBinding struct {
	record     ReadbackPeerBindingV1
	descriptor DescriptorV1
}

func LoadValidatedPeerBinding(d DescriptorV1, expected LaunchExpectations) (ValidatedPeerBinding, error) {
	var p ReadbackPeerBindingV1
	b, e := os.ReadFile(PeerBindingPath)
	if e != nil || Decode(b, &p) != nil {
		return ValidatedPeerBinding{}, ErrInvalid
	}
	return validatePeerBinding(d, p, expected)
}
func validatePeerBinding(d DescriptorV1, p ReadbackPeerBindingV1, x LaunchExpectations) (ValidatedPeerBinding, error) {
	h, e := BindingHash(d.Binding)
	if e != nil || d.Validate() != nil || p.Validate() != nil || p.AuthorityScope != d.AuthorityScope || p.ScopeID != d.ScopeID || p.EnvironmentID != d.Binding.EnvironmentID || p.InstallationID != d.Binding.InstallationID || p.ControllerID != d.ControllerID || p.BindingHash != h || p.ConsumerReplicaID != d.ReplicaID || p.ConsumerProcessLaunchID != d.ProcessLaunchID || p.ConsumerImageDigest != d.AdmittedImageDigest || p.ConsumerExecutableHash != d.AdmittedExecutableHash || p.ControllerProcessLaunchID != x.ControllerProcessLaunchID || p.ControllerLaunchObservationHash != x.ControllerObservationHash || p.ConsumerLaunchObservationHash != x.ConsumerObservationHash || p.SocketMountObservationHash != x.SocketMountObservationHash || p.LaunchKeyCustodyContractHash != x.CustodyContractHash || p.ControllerExpectedKernelPeer.ObserverNamespaceID != x.ControllerNamespaceID || p.ConsumerExpectedKernelPeer.ObserverNamespaceID != x.ConsumerNamespaceID {
		return ValidatedPeerBinding{}, ErrInvalid
	}
	p.ControllerCertificateDER = append([]byte(nil), p.ControllerCertificateDER...)
	p.ConsumerCertificateDER = append([]byte(nil), p.ConsumerCertificateDER...)
	return ValidatedPeerBinding{record: p, descriptor: copyDescriptor(d)}, nil
}
