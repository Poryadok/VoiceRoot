#!/usr/bin/env python3
"""Offline completeness checks for A4 exact-SHA rollout evidence.

This tool validates declared evidence bindings. It never deploys, runs
migrations, observes a cluster, evaluates a live SLO, or attests supplied
observations as true.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path


SHA = re.compile(r"[0-9a-f]{40}\Z")
DIGEST = re.compile(r"[0-9a-f]{64}\Z")
SERVICE = re.compile(r"[a-z][a-z0-9-]{0,62}\Z")
MIGRATION = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,127}\Z")
OPERATORS = {">", ">=", "<", "<="}


def exact_keys(value: object, expected: set[str], name: str) -> dict:
    if not isinstance(value, dict) or set(value) != expected:
        raise ValueError(f"{name} has an invalid shape")
    return value


def nonzero_sha(value: object, name: str) -> str:
    if not isinstance(value, str) or not SHA.fullmatch(value) or value == "0" * 40:
        raise ValueError(f"{name} must be a nonzero full lowercase SHA")
    return value


def digest(value: object, name: str) -> str:
    if not isinstance(value, str) or not DIGEST.fullmatch(value) or value == "0" * 64:
        raise ValueError(f"{name} must be a nonzero lowercase SHA-256 digest")
    return value


def object_pairs(pairs: list[tuple[str, object]]) -> dict:
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON key")
        result[key] = value
    return result


def reject_constant(_value: str) -> None:
    raise ValueError("non-finite JSON number")


def load_json(path: Path) -> object:
    return json.loads(path.read_text(encoding="utf-8"), object_pairs_hook=object_pairs,
                      parse_constant=reject_constant)


def canonical_digest(value: object) -> str:
    raw = json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)
    return hashlib.sha256(raw.encode("utf-8")).hexdigest()


def candidate_inventory(candidate: object, expected_source_sha: str) -> tuple[dict, dict]:
    candidate = exact_keys(candidate, {"schema_version", "source_sha", "images", "migrations"}, "candidate")
    if type(candidate["schema_version"]) is not int or candidate["schema_version"] != 1:
        raise ValueError("candidate schema_version must be integer 1")
    source_sha = nonzero_sha(candidate["source_sha"], "candidate source_sha")
    if source_sha != expected_source_sha:
        raise ValueError("candidate source_sha does not match the independently supplied SHA")

    images = candidate["images"]
    if not isinstance(images, list) or not images:
        raise ValueError("candidate images must be a nonempty list")
    image_digests = {}
    for item in images:
        item = exact_keys(item, {"service", "source_sha", "image_digest"}, "candidate image")
        service = item["service"]
        if not isinstance(service, str) or not SERVICE.fullmatch(service) or service in image_digests:
            raise ValueError("candidate image service is invalid or duplicated")
        if nonzero_sha(item["source_sha"], "image source_sha") != source_sha:
            raise ValueError("candidate image source does not match the candidate SHA")
        image = item["image_digest"]
        if not isinstance(image, str) or not re.fullmatch(r"sha256:[0-9a-f]{64}", image):
            raise ValueError("candidate image must use an immutable sha256 digest")
        image_digests[service] = image

    migrations = candidate["migrations"]
    if not isinstance(migrations, list):
        raise ValueError("candidate migrations must be a list")
    migration_digests = {}
    for item in migrations:
        item = exact_keys(item, {"migration_id", "source_sha", "content_sha256"}, "candidate migration")
        identity = item["migration_id"]
        if not isinstance(identity, str) or not MIGRATION.fullmatch(identity) or identity in migration_digests:
            raise ValueError("candidate migration ID is invalid or duplicated")
        if nonzero_sha(item["source_sha"], "migration source_sha") != source_sha:
            raise ValueError("candidate migration source does not match the candidate SHA")
        migration_digests[identity] = digest(item["content_sha256"], "migration content_sha256")
    return image_digests, migration_digests


def require_trigger(value: object, name: str, *, should_trigger: bool) -> None:
    trigger = exact_keys(value, {"metric", "operator", "threshold", "observed_value", "triggered"}, name)
    if not isinstance(trigger["metric"], str) or not trigger["metric"].strip():
        raise ValueError(f"{name} metric is required")
    if trigger["operator"] not in OPERATORS:
        raise ValueError(f"{name} operator is unsupported")
    threshold, observed = trigger["threshold"], trigger["observed_value"]
    if (type(threshold) not in (int, float) or type(observed) not in (int, float)
            or not all(map(lambda x: float("-inf") < x < float("inf"), (threshold, observed)))):
        raise ValueError(f"{name} values must be finite numbers")
    op = trigger["operator"]
    measured = {">": observed > threshold, ">=": observed >= threshold,
                "<": observed < threshold, "<=": observed <= threshold}[op]
    if type(trigger["triggered"]) is not bool or trigger["triggered"] is not measured:
        raise ValueError(f"{name} triggered must match the declared comparator and measurement")
    if measured is not should_trigger:
        raise ValueError(f"{name} measurement does not match the required trigger state")


def validate_stage(stage: object, environment: str, source_sha: str,
                   images: dict, migrations: dict, *, canary_required: bool) -> None:
    expected = {"environment", "source_sha", "images", "migrations", "migration_result",
                "migration_evidence_sha256", "evidence_sha256"}
    if canary_required:
        expected |= {"canary", "rollback_drill"}
    stage = exact_keys(stage, expected, f"{environment} stage")
    if stage["environment"] != environment:
        raise ValueError(f"stage environment must be {environment}")
    if nonzero_sha(stage["source_sha"], f"{environment} source_sha") != source_sha:
        raise ValueError(f"{environment} source does not match the candidate SHA")
    if stage["images"] != images or stage["migrations"] != migrations:
        raise ValueError(f"{environment} image or migration inventory differs from the candidate")
    if stage["migration_result"] != "PASS":
        raise ValueError(f"{environment} migration result must be PASS")
    digest(stage["migration_evidence_sha256"], f"{environment} migration_evidence_sha256")
    digest(stage["evidence_sha256"], f"{environment} evidence_sha256")

    if canary_required:
        canary = exact_keys(stage["canary"],
                            {"synthetic_sample_count", "observation_window_seconds", "rollback_condition",
                             "evidence_sha256"}, "canary")
        for field in ("synthetic_sample_count", "observation_window_seconds"):
            if type(canary[field]) is not int or canary[field] <= 0:
                raise ValueError(f"canary {field} must be a positive integer")
        require_trigger(canary["rollback_condition"], "canary rollback_condition", should_trigger=False)
        digest(canary["evidence_sha256"], "canary evidence_sha256")

        drill = exact_keys(stage["rollback_drill"],
                           {"updated_client_source_sha", "updated_client_artifact_sha256",
                            "rollback_backend_source_sha", "trigger",
                            "client_compatible", "evidence_sha256"}, "rollback drill")
        if nonzero_sha(drill["updated_client_source_sha"], "updated client source_sha") != source_sha:
            raise ValueError("rollback drill must use the updated client from the candidate source")
        digest(drill["updated_client_artifact_sha256"], "updated_client_artifact_sha256")
        if nonzero_sha(drill["rollback_backend_source_sha"], "rollback backend source_sha") == source_sha:
            raise ValueError("rollback backend source must differ from the candidate source")
        require_trigger(drill["trigger"], "rollback trigger", should_trigger=True)
        condition = canary["rollback_condition"]
        if any(drill["trigger"][field] != condition[field]
               for field in ("metric", "operator", "threshold")):
            raise ValueError("rollback drill trigger must match the canary rollback policy")
        if type(drill["client_compatible"]) is not bool or not drill["client_compatible"]:
            raise ValueError("rollback drill must record updated-client compatibility")
        digest(drill["evidence_sha256"], "rollback drill evidence_sha256")


def validate(candidate: object, report: object, expected_source_sha: str) -> None:
    expected_source_sha = nonzero_sha(expected_source_sha, "expected source_sha")
    images, migrations = candidate_inventory(candidate, expected_source_sha)
    report = exact_keys(report, {"schema_version", "source_sha", "candidate_sha256", "staging", "production"},
                        "rollout evidence")
    if type(report["schema_version"]) is not int or report["schema_version"] != 1:
        raise ValueError("report schema_version must be integer 1")
    if nonzero_sha(report["source_sha"], "report source_sha") != expected_source_sha:
        raise ValueError("report source does not match the independently supplied SHA")
    if digest(report["candidate_sha256"], "candidate_sha256") != canonical_digest(candidate):
        raise ValueError("report candidate_sha256 does not match the candidate manifest")
    validate_stage(report["staging"], "staging", expected_source_sha, images, migrations, canary_required=True)
    validate_stage(report["production"], "production", expected_source_sha, images, migrations, canary_required=False)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("candidate", type=Path)
    parser.add_argument("evidence", type=Path)
    parser.add_argument("--expected-source-sha", required=True)
    args = parser.parse_args()
    try:
        validate(load_json(args.candidate), load_json(args.evidence), args.expected_source_sha)
    except (OSError, UnicodeError, json.JSONDecodeError, ValueError, TypeError) as error:
        print(json.dumps({"result": "INCOMPLETE", "reason": str(error)}, sort_keys=True))
        return 1
    print(json.dumps({"result": "PASS", "scope": "offline evidence completeness only",
                      "source_sha": args.expected_source_sha}, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
