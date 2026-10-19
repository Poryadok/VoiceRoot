package mmadmissionwire

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
)

var errUncleanTLSEOF = errors.New("MM launch TLS ended without close_notify")

// crypto/tls accepts raw EOF at a record boundary for web compatibility. This
// private one-exchange transport requires an authenticated close_notify instead.
// Replacing only underlying EOF preserves the standard TLS record parser.
type requireTLSCloseNotifyConn struct{ net.Conn }

func (c requireTLSCloseNotifyConn) Read(b []byte) (int, error) {
	n, e := c.Conn.Read(b)
	if e == io.EOF {
		e = errUncleanTLSEOF
	}
	return n, e
}

func launchTLSConfig(p ReadbackPeerBindingV1, own tls.Certificate, server bool) (*tls.Config, error) {
	if p.Validate() != nil || len(own.Certificate) != 1 || own.PrivateKey == nil {
		return nil, ErrInvalid
	}
	local, remote, remoteHash, remoteSPKI := p.ConsumerCertificateDER, p.ControllerCertificateDER, p.ControllerCertificateDERHash, p.ControllerSPKIHash
	if server {
		local, remote, remoteHash, remoteSPKI = p.ControllerCertificateDER, p.ConsumerCertificateDER, p.ConsumerCertificateDERHash, p.ConsumerSPKIHash
	}
	if !bytes.Equal(own.Certificate[0], local) {
		return nil, ErrInvalid
	}
	signer, ok := own.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, ErrInvalid
	}
	pub, e := x509.MarshalPKIXPublicKey(signer.Public())
	if e != nil {
		return nil, ErrInvalid
	}
	cert, e := x509.ParseCertificate(local)
	if e != nil || !bytes.Equal(pub, cert.RawSubjectPublicKeyInfo) {
		return nil, ErrInvalid
	}
	other, e := x509.ParseCertificate(remote)
	if e != nil {
		return nil, ErrInvalid
	}
	pool := x509.NewCertPool()
	pool.AddCert(other)
	c := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{own}, NextProtos: []string{ReadbackALPN}, SessionTicketsDisabled: true}
	if server {
		c.ClientAuth = tls.RequireAndVerifyClientCert
		c.ClientCAs = pool
	} else {
		c.RootCAs = pool
		c.ServerName = p.ControllerServerName
	}
	c.VerifyConnection = func(s tls.ConnectionState) error {
		if s.Version != tls.VersionTLS13 || s.DidResume || s.NegotiatedProtocol != ReadbackALPN || len(s.PeerCertificates) != 1 || len(s.VerifiedChains) == 0 || sha256.Sum256(s.PeerCertificates[0].Raw) != remoteHash || sha256.Sum256(s.PeerCertificates[0].RawSubjectPublicKeyInfo) != remoteSPKI {
			return ErrInvalid
		}
		return nil
	}
	return c, nil
}

// AuthenticateReadbackServer checks a driver-registered tuple before TLS and
// returns only after normal certificate verification and exact launch pinning.
// The caller must recheck current registration after decoding the request.
func AuthenticateReadbackServer(ctx context.Context, raw *net.UnixConn, p ReadbackPeerBindingV1, own tls.Certificate) (*tls.Conn, error) {
	got, e := PeerCredentials(raw)
	if e != nil || got != p.ConsumerExpectedKernelPeer.Kernel() {
		return nil, ErrInvalid
	}
	cfg, e := launchTLSConfig(p, own, true)
	if e != nil {
		return nil, e
	}
	conn := tls.Server(requireTLSCloseNotifyConn{raw}, cfg)
	if e = conn.HandshakeContext(ctx); e != nil {
		return nil, ErrInvalid
	}
	return conn, nil
}
func authenticateReadbackClient(ctx context.Context, raw *net.UnixConn, p ReadbackPeerBindingV1, own tls.Certificate) (*tls.Conn, error) {
	got, e := PeerCredentials(raw)
	if e != nil || got != p.ControllerExpectedKernelPeer.Kernel() {
		return nil, ErrInvalid
	}
	cfg, e := launchTLSConfig(p, own, false)
	if e != nil {
		return nil, e
	}
	conn := tls.Client(requireTLSCloseNotifyConn{raw}, cfg)
	if e = conn.HandshakeContext(ctx); e != nil {
		return nil, ErrInvalid
	}
	return conn, nil
}
