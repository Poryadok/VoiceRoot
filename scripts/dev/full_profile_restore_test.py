#!/usr/bin/env python3
import copy
import json
import re
import unittest
from unittest.mock import Mock, patch

import full_profile_restore as restore


class RestoreSafetyTest(unittest.TestCase):
    def setUp(self):
        self.manifest = json.loads(restore.MANIFEST.read_text(encoding="utf-8"))

    def test_complete_inventory_is_supported(self):
        restore.validate_manifest(self.manifest)

    def test_database_inventory_matches_canonical_summary(self):
        canon = (restore.ROOT / "docs/DATA_STORES.md").read_text(encoding="utf-8")
        table = canon.split("## Сводная таблица", 1)[1].split("### Game Integration", 1)[0]
        databases = set(re.findall(r"\b[a-z_]+_db\b", table)) - {"federation_db"}
        self.assertEqual(set(self.manifest["postgres_databases"]), databases)

    def test_missing_database_or_authoritative_family_is_rejected(self):
        for section in ("postgres_databases", "cold_store_families"):
            for omitted in self.manifest[section]:
                with self.subTest(section=section, omitted=omitted):
                    broken = copy.deepcopy(self.manifest)
                    broken[section].remove(omitted)
                    with self.assertRaises(ValueError):
                        restore.validate_manifest(broken)

    def test_unknown_and_duplicate_inventory_is_rejected(self):
        for extra in ("federation_db", "auth_db"):
            broken = copy.deepcopy(self.manifest)
            broken["postgres_databases"].append(extra)
            with self.assertRaises(ValueError):
                restore.validate_manifest(broken)

    def test_mixed_redis_cannot_be_discarded_as_cache(self):
        broken = copy.deepcopy(self.manifest)
        broken["redis_disposition"] = "rebuild-all"
        with self.assertRaises(ValueError):
            restore.validate_manifest(broken)

    def test_restore_requires_disjoint_exact_stopped_sources(self):
        good = {"postgres": "source-pg", "redis": "source-redis"}
        restore.validate_cold_boundary(good, {"postgres": "target-pg", "redis": "target-redis"},
                                       {"postgres": False, "redis": False})
        cases = [
            (good, {"postgres": "source-pg", "redis": "target-redis"}, {"postgres": False, "redis": False}),
            (good, {"postgres": "target-pg", "redis": "target-redis"}, {"postgres": True, "redis": False}),
            (good, {"postgres": "target-pg"}, {"postgres": False, "redis": False}),
            (good, {"postgres": "target-pg", "redis": "target-redis"}, {"postgres": False}),
            (good, {"postgres": "target-pg", "redis": "source-pg"}, {"postgres": False, "redis": False}),
        ]
        for source, target, running in cases:
            with self.subTest(target=target, running=running), self.assertRaises(ValueError):
                restore.validate_cold_boundary(source, target, running)

    def test_unobserved_cleanup_is_not_reported_complete(self):
        drill = restore.Drill.__new__(restore.Drill)
        drill.containers, drill.volumes, drill.networks = ["owned-intent"], [], []
        drill.run_id, drill.result, drill.save = "run", {}, Mock()
        unavailable = Mock(returncode=1, stderr="Cannot connect to the Docker daemon", stdout="")
        with patch.object(restore.subprocess, "run", return_value=unavailable):
            self.assertFalse(drill.cleanup())
        self.assertFalse(drill.result["cleanup"]["complete"])

    def test_cleanup_never_removes_foreign_resource(self):
        drill = restore.Drill.__new__(restore.Drill)
        drill.containers, drill.volumes, drill.networks = ["foreign"], [], []
        drill.run_id, drill.result, drill.save, drill.command = "run", {}, Mock(), Mock()
        foreign = Mock(returncode=0, stdout=json.dumps([{"Config": {"Labels": {restore.LABEL: "other"}}}]))
        with patch.object(restore.subprocess, "run", return_value=foreign):
            self.assertFalse(drill.cleanup())
        drill.command.assert_not_called()


if __name__ == "__main__":
    unittest.main()
