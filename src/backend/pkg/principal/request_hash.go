package principal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"google.golang.org/protobuf/proto"
)

// RequestHash provides a deterministic protobuf SHA-256 binding for a signed credential.
func RequestHash(message proto.Message) (string, error) {
	if message == nil {
		return "", errors.New("protobuf request is required")
	}
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
