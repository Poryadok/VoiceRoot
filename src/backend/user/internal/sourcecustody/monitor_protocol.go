package sourcecustody

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"math"

	"github.com/google/uuid"
)

const monitorDomain = "voice.user.source-custody.monitor.v1\x00"
const monitorPayloadSize = len(monitorDomain) + 2 + 32 + 32 + 16 + 8 + 32 + 1

type MonitorResponseKind uint8

const (
	MonitorArmed MonitorResponseKind = iota + 1
	MonitorHeartbeat
	MonitorStop
)

// MonitorResponse binds one monitor incarnation to one controller-issued
// probe and opening decision. A valid signature does not itself grant custody.
type MonitorResponse struct {
	Challenge      [32]byte
	DecisionDigest [32]byte
	Incarnation    uuid.UUID
	Sequence       uint64
	Nonce          [32]byte
	Kind           MonitorResponseKind
	Signature      []byte
}

// MonitorResponsePayload returns the fixed, domain-separated v1 wire form.
func MonitorResponsePayload(response MonitorResponse) ([]byte, error) {
	if response.Challenge == [32]byte{} || response.DecisionDigest == [32]byte{} || response.Incarnation == uuid.Nil || response.Sequence == 0 || response.Sequence > math.MaxInt64 || response.Nonce == [32]byte{} || response.Kind < MonitorArmed || response.Kind > MonitorStop {
		return nil, ErrUnavailable
	}
	var b bytes.Buffer
	b.Grow(monitorPayloadSize)
	b.WriteString(monitorDomain)
	_ = binary.Write(&b, binary.BigEndian, uint16(1))
	b.Write(response.Challenge[:])
	b.Write(response.DecisionDigest[:])
	b.Write(response.Incarnation[:])
	_ = binary.Write(&b, binary.BigEndian, response.Sequence)
	b.Write(response.Nonce[:])
	b.WriteByte(byte(response.Kind))
	return b.Bytes(), nil
}

func SignMonitorResponse(response MonitorResponse, key ed25519.PrivateKey) (MonitorResponse, error) {
	if len(key) != ed25519.PrivateKeySize || len(response.Signature) != 0 {
		return MonitorResponse{}, ErrUnavailable
	}
	payload, err := MonitorResponsePayload(response)
	if err != nil {
		return MonitorResponse{}, ErrUnavailable
	}
	response.Signature = ed25519.Sign(key, payload)
	return response, nil
}

func VerifyMonitorResponse(response MonitorResponse, key ed25519.PublicKey) error {
	if len(key) != ed25519.PublicKeySize || len(response.Signature) != ed25519.SignatureSize {
		return ErrUnavailable
	}
	payload, err := MonitorResponsePayload(response)
	if err != nil || !ed25519.Verify(key, payload, response.Signature) {
		return ErrUnavailable
	}
	return nil
}

// ParseMonitorResponse rejects extra bytes and non-v1 or noncanonical shapes
// before accepting an independently pinned monitor signature.
func ParseMonitorResponse(payload, signature []byte, key ed25519.PublicKey) (MonitorResponse, error) {
	var response MonitorResponse
	if len(payload) != monitorPayloadSize || !bytes.HasPrefix(payload, []byte(monitorDomain)) || len(signature) != ed25519.SignatureSize || len(key) != ed25519.PublicKeySize {
		return response, ErrUnavailable
	}
	offset := len(monitorDomain)
	if binary.BigEndian.Uint16(payload[offset:offset+2]) != 1 {
		return response, ErrUnavailable
	}
	offset += 2
	copy(response.Challenge[:], payload[offset:offset+32])
	offset += 32
	copy(response.DecisionDigest[:], payload[offset:offset+32])
	offset += 32
	copy(response.Incarnation[:], payload[offset:offset+16])
	offset += 16
	response.Sequence = binary.BigEndian.Uint64(payload[offset : offset+8])
	offset += 8
	copy(response.Nonce[:], payload[offset:offset+32])
	offset += 32
	response.Kind = MonitorResponseKind(payload[offset])
	response.Signature = append([]byte(nil), signature...)
	if VerifyMonitorResponse(response, key) != nil {
		return MonitorResponse{}, ErrUnavailable
	}
	return response, nil
}
