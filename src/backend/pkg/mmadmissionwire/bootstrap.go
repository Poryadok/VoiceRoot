package mmadmissionwire

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"time"
)

const NativeBootstrapFD = 3
const NativeBootstrapProfile = "MM_NATIVE_BOOTSTRAP_PRIVATE_FD_V1"
const NativeBootstrapTimeout = 2 * time.Second

// Private expectations are supplied by the independent launch driver. The
// public peer file and Observe response are never used to populate this record.
type privateLaunchExpectations struct {
	ControllerProcessLaunchID  ID   `json:"controller_process_launch_id"`
	ControllerObservationHash  Hash `json:"controller_launch_observation_sha256"`
	ConsumerObservationHash    Hash `json:"consumer_launch_observation_sha256"`
	SocketMountObservationHash Hash `json:"socket_mount_observation_sha256"`
	CustodyContractHash        Hash `json:"launch_key_custody_contract_sha256"`
	ControllerNamespaceID      ID   `json:"controller_namespace_id"`
	ConsumerNamespaceID        ID   `json:"consumer_namespace_id"`
}
type nativeBootstrapRecord struct {
	Version             uint32                    `json:"version"`
	Profile             string                    `json:"profile"`
	DescriptorBytesHash Hash                      `json:"descriptor_bytes_sha256"`
	Recipient           KernelPeer                `json:"recipient"`
	Expected            privateLaunchExpectations `json:"independent_launch_expectations"`
	Certificate         CertificateDER            `json:"own_client_certificate_der"`
	PrivateKey          CertificateDER            `json:"own_client_private_key_pkcs8"`
}

func privateExpected(x LaunchExpectations) privateLaunchExpectations {
	return privateLaunchExpectations{x.ControllerProcessLaunchID, x.ControllerObservationHash, x.ConsumerObservationHash, x.SocketMountObservationHash, x.CustodyContractHash, x.ControllerNamespaceID, x.ConsumerNamespaceID}
}
func (x privateLaunchExpectations) public() LaunchExpectations {
	return LaunchExpectations{x.ControllerProcessLaunchID, x.ControllerObservationHash, x.ConsumerObservationHash, x.SocketMountObservationHash, x.CustodyContractHash, x.ControllerNamespaceID, x.ConsumerNamespaceID}
}
func (x privateLaunchExpectations) valid() bool {
	return x.ControllerProcessLaunchID != (ID{}) && x.ControllerNamespaceID != (ID{}) && x.ConsumerNamespaceID != (ID{}) && x.ControllerObservationHash != (Hash{}) && x.ConsumerObservationHash != (Hash{}) && x.SocketMountObservationHash != (Hash{}) && x.CustodyContractHash != (Hash{})
}
func ownClientTLS(der, pkcs8 []byte) (tls.Certificate, error) {
	cert, e := x509.ParseCertificate(der)
	if e != nil || cert.PublicKeyAlgorithm != x509.Ed25519 || len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth || len(cert.UnknownExtKeyUsage) != 0 || cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) != nil {
		return tls.Certificate{}, ErrInvalid
	}
	key, e := x509.ParsePKCS8PrivateKey(pkcs8)
	if e != nil {
		return tls.Certificate{}, ErrInvalid
	}
	own, ok := key.(ed25519.PrivateKey)
	if !ok || !bytes.Equal(cert.PublicKey.(ed25519.PublicKey), own.Public().(ed25519.PublicKey)) {
		return tls.Certificate{}, ErrInvalid
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	if _, e = cert.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); e != nil {
		return tls.Certificate{}, ErrInvalid
	}
	return tls.Certificate{Certificate: [][]byte{append([]byte(nil), der...)}, PrivateKey: append(ed25519.PrivateKey(nil), own...)}, nil
}
func encodeNativeBootstrap(descriptor []byte, recipient KernelPeer, expected LaunchExpectations, own tls.Certificate) ([]byte, error) {
	var d DescriptorV1
	if Decode(descriptor, &d) != nil || recipient.PID <= 0 || recipient.UID == 0 || !privateExpected(expected).valid() || len(own.Certificate) != 1 {
		return nil, ErrInvalid
	}
	key, e := x509.MarshalPKCS8PrivateKey(own.PrivateKey)
	if e != nil {
		return nil, ErrInvalid
	}
	defer clear(key)
	if _, e = ownClientTLS(own.Certificate[0], key); e != nil {
		return nil, ErrInvalid
	}
	record := nativeBootstrapRecord{1, NativeBootstrapProfile, sha256.Sum256(descriptor), recipient, privateExpected(expected), own.Certificate[0], key}
	raw, e := json.Marshal(record)
	if e != nil || len(raw) > MaxFrame {
		return nil, ErrInvalid
	}
	return raw, nil
}
func decodeNativeBootstrap(raw, descriptor []byte, recipient KernelPeer) (LaunchExpectations, tls.Certificate, error) {
	var d DescriptorV1
	var record nativeBootstrapRecord
	defer func() { clear(record.PrivateKey) }()
	if Decode(descriptor, &d) != nil || Decode(raw, &record) != nil {
		return LaunchExpectations{}, tls.Certificate{}, ErrInvalid
	}
	if record.Version != 1 || record.Profile != NativeBootstrapProfile || record.DescriptorBytesHash != sha256.Sum256(descriptor) || record.Recipient != recipient || recipient.PID <= 0 || recipient.UID == 0 || !record.Expected.valid() {
		return LaunchExpectations{}, tls.Certificate{}, ErrInvalid
	}
	own, e := ownClientTLS(record.Certificate, record.PrivateKey)
	if e != nil {
		return LaunchExpectations{}, tls.Certificate{}, ErrInvalid
	}
	return record.Expected.public(), own, nil
}

// WriteNativeLaunchBootstrap is a serialization/delivery primitive for the
// trusted launch driver, not an authority constructor. Inputs must already be
// independently observed under that driver's actual launch custody.
func WriteNativeLaunchBootstrap(ctx context.Context, conn *net.UnixConn, descriptor []byte, recipient KernelPeer, expected LaunchExpectations, own tls.Certificate) error {
	if ctx == nil || ctx.Err() != nil || conn == nil {
		return ErrInvalid
	}
	raw, e := encodeNativeBootstrap(descriptor, recipient, expected, own)
	if e != nil {
		return ErrInvalid
	}
	defer clear(raw)
	deadline := time.Now().Add(NativeBootstrapTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if conn.SetWriteDeadline(deadline) != nil {
		return ErrInvalid
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetWriteDeadline(time.Now()) })
	defer stop()
	if WriteFrame(conn, json.RawMessage(raw)) != nil || conn.CloseWrite() != nil {
		return ErrInvalid
	}
	return nil
}
