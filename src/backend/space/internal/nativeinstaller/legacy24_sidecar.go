package nativeinstaller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
)

// These functions bind private preimage bytes to the retained source24 event.
// Filesystem ownership and durable custody belong to the controller/runner.
func writeLegacy24Origin(out io.Writer, origin *legacy24OriginObservation, receipt *Legacy24Receipt) error {
	if out == nil || !validLegacy24OriginBinding(origin, receipt) {
		return errors.New("HOLD: invalid legacy24 private origin")
	}
	raw, err := json.Marshal(origin)
	if err != nil || len(raw) > 1<<20 {
		return errors.New("HOLD: invalid legacy24 private origin")
	}
	n, err := out.Write(raw)
	if err != nil || n != len(raw) {
		return errors.New("HOLD: private origin write failed")
	}
	return nil
}

func readLegacy24Origin(in io.Reader, receipt *Legacy24Receipt) (*legacy24OriginObservation, error) {
	if in == nil {
		return nil, errors.New("HOLD: private origin unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(in, (1<<20)+1))
	if err != nil || len(raw) == 0 || len(raw) > 1<<20 {
		return nil, errors.New("HOLD: private origin unavailable")
	}
	var origin legacy24OriginObservation
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&origin) != nil {
		return nil, errors.New("HOLD: invalid legacy24 private origin")
	}
	canonical, err := json.Marshal(&origin)
	if err != nil || !bytes.Equal(raw, canonical) || !validLegacy24OriginBinding(&origin, receipt) {
		return nil, errors.New("HOLD: invalid legacy24 private origin")
	}
	return &origin, nil
}

func validLegacy24OriginBinding(origin *legacy24OriginObservation, receipt *Legacy24Receipt) bool {
	if origin == nil || receipt == nil || origin.Target == nil || receipt.Source24 == nil || origin.Derived == nil || origin.Rows == nil || origin.Spaces == nil || origin.Extras == nil || origin.Functions == nil ||
		!reflect.DeepEqual(origin.Target, receipt.Source24) || origin.Target.Version != 24 || !receipt.Clean24 || origin.CatalogSHA256 != legacy24CatalogPG16SHA256 ||
		origin.CatalogSHA256 != receipt.Catalog24SHA256 || origin.PreimageSHA256 != receipt.PreimageSHA256 || origin.AccessSHA256 != receipt.AccessSHA256 || uint64(len(origin.Spaces)) != receipt.SpaceCount {
		return false
	}
	encoded, err := json.Marshal(struct {
		Rows               map[string]legacy25TableDigest `json:"rows"`
		Derived            *legacy24Derived               `json:"derived"`
		Spaces             []string                       `json:"spaces"`
		Extras             []legacy24Extra                `json:"extras"`
		StableAccessSHA256 string                         `json:"stable_access_sha256"`
		Functions          []legacy24FunctionAccess       `json:"functions"`
	}{origin.Rows, origin.Derived, origin.Spaces, origin.Extras, origin.StableAccessSHA256, origin.Functions})
	if err != nil || len(encoded) > 1<<20 {
		return false
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]) == origin.PreimageSHA256
}
