import copy
import json
import tempfile
import unittest
from pathlib import Path

import a4_rollout_evidence as rollout


SOURCE = "a" * 40
OLD_SOURCE = "b" * 40
DIGEST = "c" * 64


def candidate():
    return {
        "schema_version": 1,
        "source_sha": SOURCE,
        "images": [{"service": "gateway", "source_sha": SOURCE,
                    "image_digest": "sha256:" + "d" * 64}],
        "migrations": [{"migration_id": "001_expand.sql", "source_sha": SOURCE,
                        "content_sha256": "e" * 64}],
    }


def trigger(*, should_trigger):
    return {"metric": "http_5xx_rate", "operator": ">=", "threshold": 5,
            "observed_value": 7 if should_trigger else 3, "triggered": should_trigger}


def report(manifest=None):
    manifest = candidate() if manifest is None else manifest
    images = {row["service"]: row["image_digest"] for row in manifest["images"]}
    migrations = {row["migration_id"]: row["content_sha256"] for row in manifest["migrations"]}
    return {
        "schema_version": 1,
        "source_sha": SOURCE,
        "candidate_sha256": rollout.canonical_digest(manifest),
        "staging": {
            "environment": "staging", "source_sha": SOURCE, "images": images,
            "migrations": migrations, "migration_result": "PASS",
            "migration_evidence_sha256": "f" * 64, "evidence_sha256": "1" * 64,
            "canary": {"synthetic_sample_count": 1, "observation_window_seconds": 60,
                       "rollback_condition": trigger(should_trigger=False),
                       "evidence_sha256": "2" * 64},
            "rollback_drill": {"updated_client_source_sha": SOURCE,
                               "updated_client_artifact_sha256": "3" * 64,
                               "rollback_backend_source_sha": OLD_SOURCE,
                               "trigger": trigger(should_trigger=True),
                               "client_compatible": True, "evidence_sha256": "4" * 64},
        },
        "production": {
            "environment": "production", "source_sha": SOURCE, "images": images,
            "migrations": migrations, "migration_result": "PASS",
            "migration_evidence_sha256": "5" * 64, "evidence_sha256": "6" * 64,
        },
    }


class RolloutEvidenceTests(unittest.TestCase):
    def assert_rejected(self, manifest, evidence, expected=SOURCE):
        with self.assertRaises(ValueError):
            rollout.validate(manifest, evidence, expected)

    def test_complete_same_sha_staging_production_packet_is_accepted(self):
        manifest = candidate()
        rollout.validate(manifest, report(manifest), SOURCE)

    def test_expected_sha_is_independent_and_all_stages_bind_candidate(self):
        manifest = candidate()
        self.assert_rejected(manifest, report(manifest), OLD_SOURCE)
        evidence = report(manifest)
        evidence["production"]["source_sha"] = OLD_SOURCE
        self.assert_rejected(manifest, evidence)

    def test_images_are_immutable_source_bound_and_identical_across_stages(self):
        for mutate in (
            lambda m: m["images"][0].update(image_digest="gateway:latest"),
            lambda m: m["images"][0].update(source_sha=OLD_SOURCE),
        ):
            manifest = candidate()
            mutate(manifest)
            self.assert_rejected(manifest, report(manifest))
        manifest = candidate()
        evidence = report(manifest)
        evidence["production"]["images"]["gateway"] = "sha256:" + "9" * 64
        self.assert_rejected(manifest, evidence)

    def test_migration_inventory_result_and_evidence_are_required(self):
        manifest = candidate()
        evidence = report(manifest)
        evidence["production"]["migrations"] = {}
        self.assert_rejected(manifest, evidence)
        for field, value in (("migration_result", "FAIL"),
                             ("migration_evidence_sha256", "0" * 64)):
            evidence = report(manifest)
            evidence["staging"][field] = value
            self.assert_rejected(manifest, evidence)
        manifest = candidate()
        manifest["migrations"][0]["source_sha"] = OLD_SOURCE
        self.assert_rejected(manifest, report(manifest))

    def test_canary_requires_positive_sample_window_and_evidence(self):
        manifest = candidate()
        for field, value in (("synthetic_sample_count", 0),
                             ("observation_window_seconds", 0),
                             ("evidence_sha256", "0" * 64)):
            evidence = report(manifest)
            evidence["staging"]["canary"][field] = value
            self.assert_rejected(manifest, evidence)
        evidence = report(manifest)
        del evidence["staging"]["canary"]
        self.assert_rejected(manifest, evidence)

    def test_canary_comparator_must_match_a_nontriggered_observation(self):
        manifest = candidate()
        for change in (
            {"triggered": True},
            {"observed_value": 5},
            {"operator": "=="},
        ):
            evidence = report(manifest)
            evidence["staging"]["canary"]["rollback_condition"].update(change)
            self.assert_rejected(manifest, evidence)

    def test_rollback_drill_requires_measured_trigger_and_updated_client_compatibility(self):
        manifest = candidate()
        invalid_cases = (
            ("updated_client_source_sha", OLD_SOURCE),
            ("updated_client_artifact_sha256", "0" * 64),
            ("rollback_backend_source_sha", SOURCE),
            ("client_compatible", False),
            ("evidence_sha256", "0" * 64),
        )
        for field, value in invalid_cases:
            evidence = report(manifest)
            evidence["staging"]["rollback_drill"][field] = value
            self.assert_rejected(manifest, evidence)
        for change in ({"triggered": False}, {"observed_value": 3}, {"operator": "=="}):
            evidence = report(manifest)
            evidence["staging"]["rollback_drill"]["trigger"].update(change)
            self.assert_rejected(manifest, evidence)
        for field, value in (("metric", "different_metric"),
                             ("operator", ">"),
                             ("threshold", 8)):
            evidence = report(manifest)
            evidence["staging"]["rollback_drill"]["trigger"][field] = value
            if field == "threshold":
                evidence["staging"]["rollback_drill"]["trigger"]["observed_value"] = 9
            self.assert_rejected(manifest, evidence)

    def test_unknown_fields_duplicate_json_keys_and_nonfinite_numbers_fail_closed(self):
        manifest = candidate()
        evidence = report(manifest)
        evidence["unexpected"] = True
        self.assert_rejected(manifest, evidence)
        for raw in ('{"a":1,"a":2}', '{"a":NaN}'):
            with tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / "input.json"
                path.write_text(raw, encoding="utf-8")
                with self.assertRaises((ValueError, json.JSONDecodeError)):
                    rollout.load_json(path)


if __name__ == "__main__":
    unittest.main()
