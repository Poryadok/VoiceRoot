package sourcecustody

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"time"
)

// OpenAttestation is a short-lived, one-leader custodian statement. The
// evidence digests identify independently retained source/restore, writer
// deployment and Auth continuity evidence; their truth is a custodian duty.
// A signature is not a substitute for local physical, catalog or role checks.
type OpenAttestation struct {
	Challenge            [32]byte
	ControlSystem        string
	ControlDatabase      string
	ControlDatabaseOID   uint32
	Binding              Binding
	Physical             PhysicalSource
	WriterRole           string
	SourceEvidenceDigest [32]byte
	WriterManifestDigest [32]byte
	AuthEvidenceDigest   [32]byte
	GuardCutover         bool
	IssuedAtUnix         int64
	ExpiresAtUnix        int64
	Signature            []byte
}

// OpenAttestationPayload is the canonical byte sequence an independent
// custodian signs. It excludes Signature and never creates a trust decision.
func OpenAttestationPayload(a OpenAttestation) ([]byte, error) {
	if a.Challenge == [32]byte{} || !systemID.MatchString(a.ControlSystem) || !databaseName.MatchString(a.ControlDatabase) || a.ControlDatabaseOID == 0 || !a.Binding.valid() || !systemID.MatchString(a.Physical.SystemIdentifier) || a.ControlSystem == a.Physical.SystemIdentifier || !databaseName.MatchString(a.Physical.Database) || a.Physical.DatabaseOID == 0 || !databaseName.MatchString(a.WriterRole) || a.SourceEvidenceDigest == [32]byte{} || a.WriterManifestDigest == [32]byte{} || a.AuthEvidenceDigest == [32]byte{} || a.IssuedAtUnix <= 0 || a.ExpiresAtUnix <= a.IssuedAtUnix {
		return nil, ErrUnavailable
	}
	var b bytes.Buffer
	b.WriteString("voice.user.source-custody.open.v2\x00")
	writeString := func(s string) {
		_ = binary.Write(&b, binary.BigEndian, uint16(len(s)))
		b.WriteString(s)
	}
	b.Write(a.Challenge[:])
	writeString(a.ControlSystem)
	writeString(a.ControlDatabase)
	_ = binary.Write(&b, binary.BigEndian, a.ControlDatabaseOID)
	_ = binary.Write(&b, binary.BigEndian, uint64(a.Binding.Epoch))
	writeString(a.Binding.SourceID)
	b.Write(a.Binding.HistoryGeneration[:])
	b.Write(a.Binding.UserSourceGeneration[:])
	b.Write(a.Binding.KeysetDigest[:])
	writeString(a.Physical.SystemIdentifier)
	writeString(a.Physical.Database)
	_ = binary.Write(&b, binary.BigEndian, a.Physical.DatabaseOID)
	writeString(a.WriterRole)
	b.Write(a.SourceEvidenceDigest[:])
	b.Write(a.WriterManifestDigest[:])
	b.Write(a.AuthEvidenceDigest[:])
	if a.GuardCutover {
		b.WriteByte(1)
	} else {
		b.WriteByte(0)
	}
	_ = binary.Write(&b, binary.BigEndian, uint64(a.IssuedAtUnix))
	_ = binary.Write(&b, binary.BigEndian, uint64(a.ExpiresAtUnix))
	return b.Bytes(), nil
}

// SignOpenAttestation signs an already reviewed decision. It checks the shape
// of the statement, not the truth of the deployment or custody evidence.
func SignOpenAttestation(a OpenAttestation, key ed25519.PrivateKey) (OpenAttestation, error) {
	if len(key) != ed25519.PrivateKeySize || len(a.Signature) != 0 {
		return OpenAttestation{}, ErrUnavailable
	}
	payload, err := OpenAttestationPayload(a)
	if err != nil {
		return OpenAttestation{}, ErrUnavailable
	}
	a.Signature = ed25519.Sign(key, payload)
	return a, nil
}

func (c *Controller) verifyOpenAttestation(a OpenAttestation, key ed25519.PublicKey, now time.Time) error {
	if c == nil || len(key) != ed25519.PublicKeySize || len(a.Signature) != ed25519.SignatureSize || a.Challenge != c.activationChallenge || a.ControlSystem != c.controlSystem || a.ControlDatabase != c.controlDatabase || a.ControlDatabaseOID != c.controlDatabaseOID || a.Binding != c.binding || a.IssuedAtUnix > now.Unix() || a.IssuedAtUnix < now.Add(-5*time.Minute).Unix() || a.ExpiresAtUnix <= now.Unix() || a.ExpiresAtUnix > now.Add(5*time.Minute).Unix() {
		return ErrUnavailable
	}
	payload, err := OpenAttestationPayload(a)
	if err != nil || !ed25519.Verify(key, payload, a.Signature) {
		return ErrUnavailable
	}
	return nil
}
