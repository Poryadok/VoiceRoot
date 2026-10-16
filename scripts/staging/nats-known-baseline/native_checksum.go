package main

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/minio/highwayhash"
	"io"
	"regexp"
)

// Exact nats-server/v2 v2.12.12 filestore consumer metadata algorithm:
// SHA256(stream+"/"+consumer) -> HighwayHash64(data) -> lowercase ASCII hex.
// This offline command cannot access a credential, store path or network.
func nativeChecksum(args []string, input io.Reader, output io.Writer) error {
	name := regexp.MustCompile(`^[A-Za-z0-9_-]{1,255}$`)
	if len(args) != 2 || !name.MatchString(args[0]) || !name.MatchString(args[1]) {
		return fixedError("metadata_checksum_identity_invalid")
	}
	raw, err := io.ReadAll(io.LimitReader(input, (256<<10)+1))
	if err != nil || len(raw) == 0 || len(raw) > 256<<10 {
		return fixedError("metadata_checksum_input_invalid")
	}
	key := sha256.Sum256([]byte(args[0] + "/" + args[1]))
	hash, err := highwayhash.NewDigest64(key[:])
	if err != nil {
		return fixedError("metadata_checksum_failed")
	}
	if _, err = hash.Write(raw); err != nil {
		return fixedError("metadata_checksum_failed")
	}
	_, err = io.WriteString(output, hex.EncodeToString(hash.Sum(nil)))
	return err
}
