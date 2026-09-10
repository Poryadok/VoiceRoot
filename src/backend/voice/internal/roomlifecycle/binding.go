package roomlifecycle

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Method string

const JoinMethod Method = "JOIN"

type Binding struct {
	actorProfileID uuid.UUID
	operationID    uuid.UUID
	method         Method
	fingerprint    string
}

func (b Binding) ActorProfileID() string { return b.actorProfileID.String() }
func (b Binding) OperationID() string    { return b.operationID.String() }
func (b Binding) Method() Method         { return b.method }
func (b Binding) Fingerprint() string    { return b.fingerprint }

func BindJoin(decision JoinDecision, operationID string) (Binding, error) {
	operationUUID, err := parseNonNilUUID(operationID)
	if err != nil {
		return Binding{}, status.Error(codes.InvalidArgument, "invalid operation id")
	}
	if !decision.valid || decision.actorProfileID == uuid.Nil || decision.spaceID == uuid.Nil || decision.roomID == uuid.Nil {
		return Binding{}, status.Error(codes.FailedPrecondition, "successful join access decision required")
	}

	return Binding{
		actorProfileID: decision.actorProfileID,
		operationID:    operationUUID,
		method:         JoinMethod,
		fingerprint:    joinFingerprint(decision.spaceID, decision.roomID),
	}, nil
}

func joinFingerprint(spaceID, roomID uuid.UUID) string {
	var canonical bytes.Buffer
	for _, field := range []string{"roomlifecycle/v1", string(JoinMethod), spaceID.String(), roomID.String()} {
		_ = binary.Write(&canonical, binary.BigEndian, uint32(len(field)))
		_, _ = canonical.WriteString(field)
	}
	digest := sha256.Sum256(canonical.Bytes())
	return "sha256:" + hex.EncodeToString(digest[:])
}
