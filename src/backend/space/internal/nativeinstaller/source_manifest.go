// Code generated from the selected immutable Space SQL inventory; do not hand edit.
package nativeinstaller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const InventorySHA256 = "44e53479d6fd4fa1fc0b73adf77a0c28d07e023aeb116e0b91e28f37b0720100"

var selectedSQL = map[string]string{
	"000001_init.down.sql":                           "e2054e9782c6a5db73112aff5e803ec9fef8ab8834c55b5b53db0b4517de6789",
	"000001_init.up.sql":                             "61507408dd5f2c333b46eb8d3b99ef4dbe5e8a250b60c51b2a42b73c135db962",
	"000002_tree.down.sql":                           "8dc57b8fbe37ae11c7bfa0363fe7b85339a1bea299f726ed68b3182ba3272642",
	"000002_tree.up.sql":                             "1f47c45a0d425d8276e32bca9395eaeea12a386daea83f4718800d7c70cb3004",
	"000003_invites.down.sql":                        "0a7a1d6b8ed6270037832637ae84cadf9414b14e4517a58acee83f3d340d0728",
	"000003_invites.up.sql":                          "c2d72f3bdff0bf3929386cd3f575ccb7ca3a31639751c03c48da345573f315d3",
	"000004_moderation.down.sql":                     "bf94c4e51878fa351b5944820eca730620505dc10e98d73638b07ae8738dcbd0",
	"000004_moderation.up.sql":                       "68b5a82cea1c8e0adf743d6cec876b1a3a58ad80822776d69a6a7e89cd773fd6",
	"000005_space_subscriptions.up.sql":              "1767491a57b5674060e1fce8ba6bf15c7f85864474a5bdcdae5c760ebcdd6ddf",
	"000006_allow_guests.down.sql":                   "7ade1e42b7f4cb37c0b2016ebf036936c358f872c1e10ab41dfc032c4d46cfcc",
	"000006_allow_guests.up.sql":                     "ece3d97f97681c5f49326b9bb036a972b96a86f043db0216ea2b04ccd9c7c1d8",
	"000007_tree_pin.down.sql":                       "03066896e35833f9f75acd211b8108b8527e0049214681e551b3365ffbe2c5c5",
	"000007_tree_pin.up.sql":                         "0fa72cf13a3a0a1b003ccbcf0be67cd2ce47b72b7e88265c4901ae16e3650fea",
	"000008_ownership_journal.down.sql":              "0f1ab1f7b15fbcd026352a9505a54852227cbe571038b8cf3eb273b17a7f6869",
	"000008_ownership_journal.up.sql":                "76a0d2260be769b0104af34580765abc410f5315069243425ef9ea8f519fcd0f",
	"000009_ownership_journal_decision.down.sql":     "b400513b9697657ffe984313c68063dd1086d0b698d3539165ab7a8aed3ea545",
	"000009_ownership_journal_decision.up.sql":       "703d69df0522c53bb549593c51d952e48c198363ee549d8e6a61a23984b668d2",
	"000010_ownership_journal_commit.down.sql":       "648775917aab69365a0cf2e9f0fb49297106844f489fcedf7747d9cdcd419249",
	"000010_ownership_journal_commit.up.sql":         "37c461ee83580e30bd3735a87c6fbbb3889d62bdb4dc4fd27115f60279e44d5b",
	"000011_voice_access_epoch.down.sql":             "93e8c775f90ffe006ef33700b053995babf59a351c9f262335470963bbef3b76",
	"000011_voice_access_epoch.up.sql":               "c72108871c7b4b64f2553eb1ab4200e8588c23e892016a70279909ce16961eff",
	"000012_ownership_journal_completion.down.sql":   "b51b509564e471cfc5386eaa61de45a6483014e5cd55444ea6054555897ae1b7",
	"000012_ownership_journal_completion.up.sql":     "4a56c4707d9d28613c1f6a9c3fc6239db8cdcc1125c862640c8c472cb431de16",
	"000013_space_lifecycle.down.sql":                "b55b09b87d65d5e726627c0f666d03a92162bc335de56b18e149ea64d125e76d",
	"000013_space_lifecycle.up.sql":                  "c5c0e92896a9f7e406eda60b94b26062b80bad6bd6e827ebbe80b3968d1a1763",
	"000014_ownership_outbox_delivery.down.sql":      "4e41b09944b25c8cb4c299c1522b0b77cc51a2850a03ad6d50fb395753b57dc1",
	"000014_ownership_outbox_delivery.up.sql":        "b397fefdd45bc896795db7c84b7528c5c9fc16995d0673b7614747c5ec0eb65f",
	"000015_audit_ledger.down.sql":                   "b0ff773df106127ec3113e776cb07f50c2226aeab9874f2639c7f50d6cfa6af7",
	"000015_audit_ledger.up.sql":                     "80bb240a4cb9ea102cb9fb1c58037ebbc0ecfab451f92a1f1079f53b978ddf79",
	"000016_allow_guests_fail_closed.down.sql":       "739feb9732dddbd46e9ed1ab4be5e7dd3a7a80d47e3a702c10d6fc97261f4a3e",
	"000016_allow_guests_fail_closed.up.sql":         "b2224833e8a69710fe45ae599edafb0fd6a54a292c40221076641b9d01056abb",
	"000017_lifecycle_restore_outcome.down.sql":      "13d5aece3aafdc2be35721247fe595fd1b18ab50eb22dcaed53519959978a58f",
	"000017_lifecycle_restore_outcome.up.sql":        "873a3182f76f081a836a6d1727475727b0ddf9ddc0a60bbb5df3d9ba1be34a7e",
	"000018_ownership_consume_started.down.sql":      "2de01ef44d46c7fd7ea6145cbfdf903dea8bfc372100dbb9b0ab9daf9005bd42",
	"000018_ownership_consume_started.up.sql":        "5e8bc4e15c023b03ed63ed851672b125018ce6030abb9ee6ef6f515163d04569",
	"000019_t37_community_bootstrap.down.sql":        "143fd09e854cbd5ae06e320b2831c5abceba6819a9c75a5ccc4ac7ce103b19fe",
	"000019_t37_community_bootstrap.up.sql":          "fb4f5847be410926b803574749a2fff7a669f7d08b42806cdc37034a213abc6e",
	"000020_t37_owner_recovery.down.sql":             "c78fcea143affa444307252b8d49fdfc6b467ec59700b7292119d776748a4cc7",
	"000020_t37_owner_recovery.up.sql":               "24251b08f5b6055e14b772bbf04c2d63a8c5a8ef4a7bfb9f63dafb9568010550",
	"000021_t38_community_roster.down.sql":           "023936bd22aebcfbef9c724923da7314329d355a41dbd7738f3ed63d0638217a",
	"000021_t38_community_roster.up.sql":             "fd0ee0de9abf20f048e5cfb2cbea5abbf49b0dbd939456a94a96a73f2c622fbf",
	"000022_lifecycle_runtime.down.sql":              "f2a44b0417f6e359f33fb255ba00f1648a595b6c8860eb300385af712a47a422",
	"000022_lifecycle_runtime.up.sql":                "7ce45a057f1365f932838014eff7300bea99a9855f41395d7abce0927f44f2eb",
	"000023_lifecycle_evidence_retention.down.sql":   "90ea412c165b262110b188b374bc7bb790700847c5e5c8d3fef806694abfbfb3",
	"000023_lifecycle_evidence_retention.up.sql":     "2cd4f772eccdce8fee9ee1e372b9d2b81c0249813c024fcedb1a0bae9ee9fc64",
	"000024_authority_revision.down.sql":             "1cb6940bd3ad165bc8776ca4c36d09fb697cc2522a6a6575b2cbe8ddce8ad453",
	"000024_authority_revision.up.sql":               "b0613814e9996c6473cf6ef0b2b8ed808dba94c9d8a1c218846169040ebcd5fc",
	"000025_tree_audit_actions.down.sql":             "31969d955f293cd04210c38d3bd791596c36ccab095695402690b876b8b942dc",
	"000025_tree_audit_actions.up.sql":               "2f5990035f00a8474465580d855eef290c638f1267d39eab6418762df1914ae7",
	"000026_member_nickname_audit.down.sql":          "5773c685423d3f32ead8a78d68891f8c1abcab9f47385fc5547f666b83bcdc90",
	"000026_member_nickname_audit.up.sql":            "97c685ee4f251c3cb10871ba38dcd151765eda5027d316503590f356bc44edff",
	"000027_audit_ingestion.down.sql":                "bbe2a35d5e4a7f76e62794c43e8e801741222df2d0141ea9108c4a21fc211b0f",
	"000027_audit_ingestion.up.sql":                  "7910cf8c4a0e8e088439b26dfaa77bf4723fce970b56da6a36113961b9a5bcb6",
	"000028_chat_creation_outcomes.down.sql":         "5ed8dd288b0d5113ecaf92cce16bc2cc4bab974f2dd85e8d973321f96286bd1b",
	"000028_chat_creation_outcomes.up.sql":           "69d8ad86990189ccb524c87c03edea21c55767a8270068b3921efcb8aaf471f0",
	"000029_audit_receipt_minimization.down.sql":     "11b436bccb18e487830c6b24f4439d3a8a64c23b8737193b9bdea95fe076ca26",
	"000029_audit_receipt_minimization.up.sql":       "d8207cee6d0e6467efdc891d70f6f000c31c291b3a949c26ba316cc2b4b23937",
	"000030_entry_policy.down.sql":                   "62b31675a5821546109133ac324813ec3f88c04b5b2c4c88a9dce1850174bc3c",
	"000030_entry_policy.up.sql":                     "f2fbebd19dce8f7091e7395bb540d4124108eeec3493a75900a7aa854ee33c95",
	"000031_composite_audit_reservation.down.sql":    "e0296b7003c96975a3e3f1bb83fb700d93888c4b8c3c2a5b25eed3a78416b567",
	"000031_composite_audit_reservation.up.sql":      "fbbf4477515b65c38b718bc0db94168fcddd31a1e59a8342575fe9f715583e1a",
	"000034_realtime_lifecycle_participant.down.sql": "f36d66532dea9c6a5ea18d01c5e1eb45ed62043e59a8c197edd0953bb2764e51",
	"000034_realtime_lifecycle_participant.up.sql":   "476d3483e58db7aa9b95cb807f64ee58ffa6e7f81c62c453d825913a3134ca17",
	"000035_join_attempts.down.sql":                  "1b777a49bd5c02710cbb14d26e598684a9603fc93f354aa84be8e350b402daea",
	"000035_join_attempts.up.sql":                    "3b62c7f4a991f292c1f753a0af31360ece2fe34387b8362e3fdddd81d1d439da",
	"000036_join_operation_namespace.down.sql":       "fbe9fd0aefeeeb108d7a037ea6fbc4374a79ffc0e40424a075ee22b8b9afff5a",
	"000036_join_operation_namespace.up.sql":         "3d2a89e33a0fba4bc2de1168389a31c0d1f0933d162d0b883876cb62165ac4cd",
	"000037_join_request_groups_forward.down.sql":    "181ee8c320213a1ac3c787f07f19f8315b63ed0b18a056605b475289c6b2aeb9",
	"000037_join_request_groups_forward.up.sql":      "e48c641fe92722c676b96f4cdbbe169a973f4732a3d2329cf00b338e1525972a",
	"000039_native_membership_authority.down.sql":    "f377ecf9266c92bbbb8372a7b129ed0ddc3ffdbc9f259adf2d41f10583da53eb",
	"000039_native_membership_authority.up.sql":      "fd2ef92a1a0b70d6da18cea364d01b097905370223e9281be6735c7c1ff9f6ee",
	"000040_join_decision_basis_v2.down.sql":         "91e4c66380d873f723a9ae22f67bcbdcd8d9f9f3ba2094d0c63d0245866eacbc",
	"000040_join_decision_basis_v2.up.sql":           "80b3c417985429acada34406bd7239a2f2923b4dd323d4cc74ae13502f491f1f",
	"000041_effective_member_episodes.down.sql":      "93baf82531a62596604bbde97b02d88d41edd0283b657b4224a2b63c8c665591",
	"000041_effective_member_episodes.up.sql":        "32249964cdc3ce6da473fe187b3bdd7a412b1a2c609e0015045190eedb1a708a",
	"000042_native_bot_membership_journal.down.sql":  "3c0685eb8ff78d796ef8371cd905df8c16028ccee97f00497f16efa07a804c79",
	"000042_native_bot_membership_journal.up.sql":    "f78189ebc4a1d5cbe5ed130279e142281d0a027f61728ed0f471c5b4d183300e",
	"000043_join_public_operation_context.down.sql":  "3cc873288e0db5c538d6324d44f59c042948dc82b6dcdec24e2470742f07c403",
	"000043_join_public_operation_context.up.sql":    "971b728eeb048d7cfb31bc8e23a25547dc22a47c185b4ac842931ac9743ddf8c",
	"000044_native_membership_readiness.down.sql":    "d83b30afbe236384db7e0872658cf8fb55d2bb36106627a07126582053245b3b",
	"000044_native_membership_readiness.up.sql":      "0dab50e0f42a7787ec904bf33d61c4e87d078d1860a9d4855f959716169560af",
	"000045_join_legacy_selection_context.down.sql":  "40d5b4ef1f93be9ceab4f1216cb296c72ca7c9ced9845b4afd39cec1c9fd034c",
	"000045_join_legacy_selection_context.up.sql":    "f424f4f8afbf5a2a0aeb0530a1987ada0db6046a76aad769f736f19dce39ff62",
	"000050_account_ban_coverage.down.sql":           "c7661be49a646d7f76ef97e866ae12a1f88221344e738759b4a9f9f2cc64bcd8",
	"000050_account_ban_coverage.up.sql":             "d52240e13e70eead705db61cf462cefc860957d40f7f52bb0bf40962c73dc3f3",
	"000051_native_catalog_rowjson.down.sql":         "251e4a91303b568d7257cf880363f91898a3b9d50d736d1f5448e3bfd2679ae5",
	"000051_native_catalog_rowjson.up.sql":           "23b70cc0ad088e03ef928a70fce5177c4d7d21c89efa6c0368d0aea98814bbaf",
}

// ValidateSources rejects a changed selected release inventory before any DDL.
func ValidateSources(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", errors.New("selected SQL inventory unavailable")
	}
	seen := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		expected, ok := selectedSQL[entry.Name()]
		if !ok || !entry.Type().IsRegular() {
			return "", errors.New("unexpected SQL inventory entry")
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return "", errors.New("selected SQL unavailable")
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != expected {
			return "", errors.New("selected SQL source mismatch")
		}
		seen++
	}
	if seen != len(selectedSQL) {
		return "", errors.New("incomplete SQL inventory")
	}
	return InventorySHA256, nil
}
