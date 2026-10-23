package store

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
)

// These are compiled source qualification inputs, never caller/controller flags.
const nativeCatalogRowJSONTypeProofSHA256 = "6a6ac42bf2339dda583e20fe78795b3f2f92b69b0dfada568cc918639e4eec5c"
const nativeCatalogRowJSONTypeExpectationSHA256 = "23407dc2b92488565324ee2cc5e8ecbb06531b89ed5e09538f26b38cd50b962d"

// Current-candidate session/raw digest stability has not yet been run/reviewed.
// Empty evidence deliberately denies admission; no runtime option can open it.
const nativeCatalogRowJSONStabilityProofSHA256 = ""

type nativeCatalogRowJSONQualification struct {
	TypeProofSHA256, TypeExpectationSHA256, StabilitySHA256, DefinitionSHA256 string
}

func (q nativeCatalogRowJSONQualification) validate() error {
	expected := nativeCatalogRowJSONQualification{nativeCatalogRowJSONTypeProofSHA256, nativeCatalogRowJSONTypeExpectationSHA256, nativeCatalogRowJSONStabilityProofSHA256, nativeCatalogRowJSONDefinitionSHA256}
	if q != expected {
		return ErrMemberAuthorityUnavailable
	}
	for _, s := range []string{q.TypeProofSHA256, q.TypeExpectationSHA256, q.StabilitySHA256, q.DefinitionSHA256} {
		raw, err := hex.DecodeString(s)
		if err != nil || len(raw) != 32 {
			return ErrMemberAuthorityUnavailable
		}
	}
	return nil
}

func requireNativeCatalogRowJSONAdmission() error {
	return (nativeCatalogRowJSONQualification{nativeCatalogRowJSONTypeProofSHA256, nativeCatalogRowJSONTypeExpectationSHA256, nativeCatalogRowJSONStabilityProofSHA256, nativeCatalogRowJSONDefinitionSHA256}).validate()
}

// RequireNativeCatalog51Admission is a compiled source gate, not an external
// evidence loader or a caller-selected activation switch.
func RequireNativeCatalog51Admission() error { return requireNativeCatalogRowJSONAdmission() }

//go:embed nativecatalog/catalog-rowjson-types16.sql
var nativeCatalogRowJSONTypes16 string

//go:embed nativecatalog/catalog-rowjson-types18.sql
var nativeCatalogRowJSONTypes18 string

func nativeCatalogRowJSONTypes(server int) (string, error) {
	switch server / 10000 {
	case 16:
		return nativeCatalogRowJSONTypes16, nil
	case 18:
		return nativeCatalogRowJSONTypes18, nil
	}
	return "", ErrMemberAuthorityUnavailable
}

func nativeSourceBodySHA(body string) string {
	h := sha256.Sum256([]byte(body))
	return hex.EncodeToString(h[:])
}
