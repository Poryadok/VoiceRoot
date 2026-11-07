package sourcecustody

import (
	"context"
	"encoding/hex"

	"github.com/google/uuid"
	"voice/backend/user/internal/identityassociation"
)

// AdoptionConfig supplies the separately reviewed physical, lineage, complete
// keyset and backup/restore/custody evidence pins. The digest references retained
// evidence; parsing it cannot establish the asserted operational facts.
type AdoptionConfig struct {
	Checked        CheckedSourceConfig
	ReceiptID      uuid.UUID
	EvidenceDigest [32]byte
}

func LoadAdoptionConfig(lookup func(string) (string, bool)) (AdoptionConfig, error) {
	var cfg AdoptionConfig
	checked, err := LoadCheckedSourceConfig(lookup)
	if err != nil {
		return cfg, ErrUnavailable
	}
	id, ok := lookup("CUSTODY_ADOPTION_RECEIPT_ID")
	parsed, err := uuid.Parse(id)
	if !ok || err != nil || parsed == uuid.Nil || parsed.String() != id {
		return cfg, ErrUnavailable
	}
	evidence, ok := lookup("CUSTODY_ADOPTION_EVIDENCE_DIGEST")
	digest, err := hex.DecodeString(evidence)
	if !ok || err != nil || len(digest) != 32 || hex.EncodeToString(digest) != evidence {
		return cfg, ErrUnavailable
	}
	cfg.Checked, cfg.ReceiptID = checked, parsed
	copy(cfg.EvidenceDigest[:], digest)
	if cfg.EvidenceDigest == [32]byte{} {
		return AdoptionConfig{}, ErrUnavailable
	}
	return cfg, nil
}

// RunAdoption accepts verified TLS endpoints and performs one closed-only
// adoption. It never resumes an interrupted attempt, activates guards or opens
// capture. Errors require review/recovery, even after an unknown commit reply.
func RunAdoption(ctx context.Context, cfg AdoptionConfig) error {
	if ctx == nil || ctx.Err() != nil || forbiddenPGEnvironment() || !validControlURL(cfg.Checked.Recovery.ControlURL) || !validControlURL(cfg.Checked.Recovery.SourceAdminURL) || !cfg.Checked.Binding.valid() || cfg.ReceiptID == uuid.Nil || cfg.EvidenceDigest == [32]byte{} {
		return ErrUnavailable
	}
	keys, err := identityassociation.LoadProductionKeyset(cfg.Checked.KeyManifest, cfg.Checked.KeysDir)
	if err != nil {
		return ErrUnavailable
	}
	digest, err := keys.CustodyDigest()
	if err != nil || digest != cfg.Checked.Binding.KeysetDigest || keys.Snapshot().Revision != 1 {
		return ErrUnavailable
	}
	control, err := openPinnedPostgres(ctx, cfg.Checked.Recovery.ControlURL)
	if err != nil {
		return ErrUnavailable
	}
	defer control.Close()
	admin, err := openPinnedPostgres(ctx, cfg.Checked.Recovery.SourceAdminURL)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Close()
	return AdoptPristineExistingSource(ctx, control, admin, cfg.Checked.Recovery.Request, cfg.Checked.Recovery.Physical, cfg.Checked.Binding, keys, cfg.ReceiptID, cfg.EvidenceDigest)
}
