#!/usr/bin/env python3
"""Generate the exact User22 owner catalog from canonical migration bodies.

Run from any directory: python scripts/generate_user_identity_catalog.py
No database, keys, generated test executables or network are needed.
"""
import hashlib
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]
MIGRATIONS = ROOT / "src/backend/migrations/user_db"
functions = {}
pattern = re.compile(
    r"CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+(?:public\.)?(\w+)\s*\((.*?)\)\s*RETURNS\s+(\w+).*?\bAS\s+\$\$(.*?)\$\$",
    re.IGNORECASE | re.DOTALL,
)
for path in sorted(MIGRATIONS.glob("*.up.sql")):
    if int(path.name[:6]) > 22:
        continue
    # Preserve the exact function bytes PostgreSQL receives from the migration.
    text = path.read_bytes().decode("utf-8")
    for match in pattern.finditer(text):
        name, arguments, return_type, body = match.groups()
        count = len(arguments.split(",")) if arguments.strip() else 0
        functions[name] = (count, return_type.lower(), hashlib.sha256(body.encode()).hexdigest())

# These already-published User19 hashes are independent compatibility anchors.
# Regeneration must not silently reinterpret an old owner guard.
USER19 = {
    "bump_sdk_eligibility_revision": "f4ec491a2668192d3e192546705ee6c6e2ab192324b833193ba5fba5876cea36",
    "reject_sdk_author_tombstone_mutation": "83dea7b72d755a134949c14ccb8ca852272ac95b797220e4f5bdc9a0194e529b",
    "user_authority_floor_guard": "a69ba01bacff05f6b7363be53f5310f33af85e83f498805a15ff243751fe4567",
    "user_authority_changed": "7e5d1cfb37caec3a57bdbab0ba763a2809aa1f331d18bb2e5587b1fe84a6b62f",
    "reject_user_inactive_mutation": "f07424675de7e20eb12cde243d108df8aba5eb439085c9f35966f5d5b5ca8ab3",
}
for name, expected in USER19.items():
    if functions[name][2] != expected:
        raise SystemExit(f"Published User19 function changed: {name}")

triggers = [
    ("profiles", "profiles_sdk_eligibility_revision", "bump_sdk_eligibility_revision", 19),
    ("sdk_author_tombstones", "sdk_author_tombstones_no_update_delete", "reject_sdk_author_tombstone_mutation", 27),
    ("sdk_author_tombstones", "sdk_author_tombstones_no_truncate", "reject_sdk_author_tombstone_mutation", 34),
    ("user_authority_revision", "user_authority_floor_change", "user_authority_floor_guard", 31),
    ("user_authority_revision", "user_authority_floor_truncate", "user_authority_floor_guard", 34),
    ("profiles", "user_authority_profiles", "user_authority_changed", 28),
    ("user_account_lifecycle", "user_authority_inactive", "user_authority_changed", 28),
    ("sdk_author_tombstones", "user_authority_tombstones", "user_authority_changed", 28),
    ("profiles", "user_authority_profiles_truncate", "user_authority_floor_guard", 34),
    ("user_account_lifecycle", "user_account_lifecycle_no_update_delete", "reject_user_inactive_mutation", 27),
    ("user_account_lifecycle", "user_account_lifecycle_no_truncate", "reject_user_inactive_mutation", 34),
    ("profiles", "user_profile_identity_immutable", "reject_user_profile_identity_rebind", 19),
    ("profiles", "identity_profiles_gate", "identity_statement_gate", 30),
    ("user_account_lifecycle", "identity_lifecycle_gate", "identity_statement_gate", 30),
    ("sdk_author_tombstones", "identity_sdk_gate", "identity_statement_gate", 30),
    ("user_authority_revision", "identity_authority_clock_gate", "identity_statement_gate", 18),
    ("identity_meta", "identity_meta_no_reset", "identity_meta_guard", 27),
    ("identity_meta", "identity_meta_no_truncate", "identity_meta_guard", 34),
    ("identity_profile_reservations", "identity_reservation_immutable", "identity_reservation_guard", 27),
    ("identity_profile_reservations", "identity_reservation_no_truncate", "identity_reservation_guard", 34),
    ("profiles", "identity_profile_registrar", "identity_profile_registrar_guard", 31),
    ("profiles", "identity_profile_lifecycle", "identity_profile_lifecycle_changed", 21),
    ("user_account_lifecycle", "identity_account_lifecycle", "identity_account_lifecycle_changed", 29),
    ("identity_associations", "identity_birth_requires_profile", "identity_birth_complete", 5),
    ("identity_associations", "identity_association_immutable", "identity_association_guard", 27),
    ("identity_associations", "identity_association_no_truncate", "identity_association_guard", 34),
    ("identity_associations", "identity_association_requires_journal", "identity_association_journal_complete", 21),
    ("identity_parent_fences", "identity_parent_no_rewind", "identity_parent_fence_guard", 27),
    ("identity_parent_fences", "identity_parent_no_truncate", "identity_parent_fence_guard", 34),
]
gated = ["identity_key_versions", "identity_profile_reservations", "identity_associations", "association_selector_aliases", "identity_journal", "identity_parent_fences", "identity_account_parent_aliases"]
for table in gated:
    triggers.append((table, "identity_mutation_gate", "identity_statement_gate", 30))
for table in ["identity_key_versions", "association_selector_aliases", "identity_journal", "identity_account_parent_aliases"]:
    triggers.append((table, "identity_evidence_immutable", "identity_append_only_guard", 27))
    triggers.append((table, "identity_evidence_no_truncate", "identity_append_only_guard", 34))

tables = ["profiles", "user_account_lifecycle", "sdk_author_tombstones", "user_authority_revision", "identity_meta"] + gated
tables = list(dict.fromkeys(tables))
required_functions = set(USER19) | {row[2] for row in triggers} | {"identity_has_preparation", "identity_append_event", "identity_append_capture_batch"}
lines = [
    "// Code generated by scripts/generate_user_identity_catalog.py; DO NOT EDIT.",
    "package userauthoritycatalog", "", 'import "voice/backend/pkg/authoritysource"', "",
    "// Catalog returns a fresh immutable-definition value for the supported User schema.",
    "func Catalog() authoritysource.Catalog {", "return authoritysource.Catalog{Version: 22,",
    "Tables: []string{" + ",".join('"' + table + '"' for table in tables) + "},",
    "Triggers: []authoritysource.CatalogTrigger{",
]
for table, name, function, trigger_type in triggers:
    lines.append(f'{{Table:"{table}",Name:"{name}",Function:"{function}",Type:{trigger_type}}},')
lines += ["}, Functions: []authoritysource.CatalogFunction{"]
for name in sorted(required_functions):
    arguments, return_type, digest = functions[name]
    lines.append(f'{{Name:"{name}",Arguments:{arguments},ReturnType:"{return_type}",BodySHA256:"{digest}"}},')
lines += ["}}", "}", ""]
destination = ROOT / "src/backend/user/internal/userauthoritycatalog/catalog.go"
destination.parent.mkdir(parents=True, exist_ok=True)
destination.write_text("\n".join(lines), encoding="utf-8", newline="\n")
(ROOT / "src/backend/user/internal/store/authority_source_catalog.go").write_text(
    '// Code generated by scripts/generate_user_identity_catalog.py; DO NOT EDIT.\n'
    'package store\n\nimport "voice/backend/user/internal/userauthoritycatalog"\n\n'
    'var userAuthorityCatalog = userauthoritycatalog.Catalog()\n', encoding="utf-8", newline="\n"
)
print(f"Generated User22 catalog: {len(tables)} tables, {len(triggers)} triggers, {len(required_functions)} functions; User19 hashes unchanged")
