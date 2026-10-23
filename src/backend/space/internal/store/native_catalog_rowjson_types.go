package store

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
)

// These are compiled source qualification inputs, never caller/controller flags.
const nativeCatalogRowJSONTypeProofSHA256 = "6a6ac42bf2339dda583e20fe78795b3f2f92b69b0dfada568cc918639e4eec5c"
const nativeCatalogRowJSONTypeExpectationSHA256 = "23407dc2b92488565324ee2cc5e8ecbb06531b89ed5e09538f26b38cd50b962d"

// Exact recorded rollback-only R0 proof, independently reviewed against983d3f8a.
// This admits the compiled source prerequisite, not a database or release.
const nativeCatalogRowJSONStabilityProofSHA256 = "9461a493de63eed3317f5e69d9830a542348bb1cd2d033a2c11b836d2bd0685e"

//go:embed nativecatalog/catalog-rowjson-stability-proof.json
var nativeCatalogRowJSONStabilityProof []byte

type nativeCatalogRecordedStability struct {
	Schema string `json:"schema"`
	Source struct {
		Commit, Tree, Profile string
		DefinitionSHA256      string `json:"definition_sha256"`
		PreimageSHA256        string `json:"preimage_sha256"`
		Classifier16SHA256    string `json:"classifier16_sha256"`
		Classifier18SHA256    string `json:"classifier18_sha256"`
		TypeProofSHA256       string `json:"type_proof_sha256"`
		TypeExpectationSHA256 string `json:"type_expectation_sha256"`
	} `json:"qualified_source"`
	Evidence struct {
		ManifestSHA256 string `json:"manifest_sha256"`
		ManifestJSON   string `json:"manifest_json"`
		ResultSHA256   string `json:"result_sha256"`
		ResultJSON     string `json:"result_json"`
		ReviewSHA256   string `json:"independent_review_sha256"`
		ReviewMarkdown string `json:"independent_review_markdown"`
	} `json:"evidence"`
}

// Only immutable compiled evidence is evaluated here. No live catalog/seal
// observation is cached; each existing database qualification still runs fresh.
var nativeCatalogRecordedStabilityError = validateNativeCatalogRecordedStability(nativeCatalogRowJSONStabilityProof)

func validateNativeCatalogRecordedStability(raw []byte) error {
	if nativeSourceBodySHA(string(raw)) != nativeCatalogRowJSONStabilityProofSHA256 {
		return ErrMemberAuthorityUnavailable
	}
	var p nativeCatalogRecordedStability
	if json.Unmarshal(raw, &p) != nil || p.Schema != "voice.native51.recorded-stability.v1" || p.Source.Commit != "983d3f8a6ca872598eabd31cdeb045f938c7b32f" || p.Source.Tree != "ceb27b41990435dc667784b59794508d7f57decf" || p.Source.Profile != NativeCatalogRowJSONProfile || p.Source.DefinitionSHA256 != nativeCatalogRowJSONDefinitionSHA256 || p.Source.PreimageSHA256 != nativeSourceBodySHA(nativeCatalogRowJSONPreimageSQL) || p.Source.Classifier16SHA256 != nativeSourceBodySHA(nativeCatalogRowJSONTypes16) || p.Source.Classifier18SHA256 != nativeSourceBodySHA(nativeCatalogRowJSONTypes18) || p.Source.TypeProofSHA256 != nativeCatalogRowJSONTypeProofSHA256 || p.Source.TypeExpectationSHA256 != nativeCatalogRowJSONTypeExpectationSHA256 {
		return ErrMemberAuthorityUnavailable
	}
	if p.Evidence.ManifestSHA256 != "c6f6dd4bc70058388b36e0ec739a91fe878f7f9c8ebe081886cb71079bcaba98" || nativeSourceBodySHA(p.Evidence.ManifestJSON) != p.Evidence.ManifestSHA256 || p.Evidence.ResultSHA256 != "fb536805370237350c506ca0fbe1eec7508b23961f81cbc6816c6fbd2ac76c06" || nativeSourceBodySHA(p.Evidence.ResultJSON) != p.Evidence.ResultSHA256 || p.Evidence.ReviewSHA256 != "363ab1ed1545e5fe647ef26395aa925fc710f9ed48a5cc94270ce16e0dc9cee3" || nativeSourceBodySHA(p.Evidence.ReviewMarkdown) != p.Evidence.ReviewSHA256 {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}

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
	return nativeCatalogRecordedStabilityError
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
