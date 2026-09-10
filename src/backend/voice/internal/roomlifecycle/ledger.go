package roomlifecycle

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
)

const receiptRetention = 24 * time.Hour

type Reservation struct {
	actorProfileID uuid.UUID
	operationID    uuid.UUID
	method         Method
	fingerprint    string
	ownerToken     string
}

func (r Reservation) ActorProfileID() string { return r.actorProfileID.String() }
func (r Reservation) OperationID() string    { return r.operationID.String() }
func (r Reservation) Method() Method         { return r.method }
func (r Reservation) Fingerprint() string    { return r.fingerprint }
func (r Reservation) OwnerToken() string     { return r.ownerToken }

type Receipt struct {
	Outcome        OperationOutcome
	VoiceSessionID string
}

type OperationOutcome string

const JoinSucceeded OperationOutcome = "JOIN_SUCCEEDED"

func validBinding(binding Binding) bool {
	return binding.actorProfileID != uuid.Nil && binding.operationID != uuid.Nil && binding.method != "" && validFingerprint(binding.fingerprint)
}

func validReservation(reservation Reservation) bool {
	return reservation.actorProfileID != uuid.Nil && reservation.operationID != uuid.Nil && reservation.method != "" && validFingerprint(reservation.fingerprint) && validOwnerToken(reservation.ownerToken)
}

func validReceipt(receipt Receipt) bool {
	if receipt.Outcome != JoinSucceeded {
		return false
	}
	sessionID, err := uuid.Parse(receipt.VoiceSessionID)
	return err == nil && sessionID != uuid.Nil && sessionID.String() == receipt.VoiceSessionID
}

func validFingerprint(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

func validOwnerToken(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) >= 16
}

func newOwnerToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func reservationFor(binding Binding, ownerToken string) Reservation {
	return Reservation{
		actorProfileID: binding.actorProfileID,
		operationID:    binding.operationID,
		method:         binding.method,
		fingerprint:    binding.fingerprint,
		ownerToken:     ownerToken,
	}
}

func sameBoundOperation(method Method, fingerprint string, binding Binding) bool {
	return method == binding.method && fingerprint == binding.fingerprint
}

func sameReservation(record ledgerRecord, reservation Reservation) bool {
	return record.actorProfileID == reservation.actorProfileID &&
		record.operationID == reservation.operationID &&
		record.method == reservation.method &&
		record.fingerprint == reservation.fingerprint &&
		record.ownerToken == reservation.ownerToken
}
