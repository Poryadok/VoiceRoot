#!/usr/bin/env python3
"""Capture a full canonical master-push build as immutable DEV/STAGING inputs.

Read-only: this neither publishes nor deploys, and does not qualify runtime config.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "scripts/staging/nats-rollout-preservation"))
import source_authority as source
import publish_candidate as common

REGISTRY = "ghcr.io/poryadok/voiceroot"
GO = tuple("analytics bot chat file gameintegration gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice".split())
APPLICATIONS = GO + ("auth", "web", "admin", "developer-portal")
IMAGE_NAMES = frozenset(APPLICATIONS + ("nats-hub-config-renderer",))
REQUEST = ".github/ci/full-application-build-request.json"


def require(value, message):
    if not value:
        raise ValueError(message)


def required_jobs():
    jobs = {"staging-images-push (" + name + ")": "Build and push " + name + " image" for name in GO}
    jobs.update({"backend-auth": "Push auth image to GHCR", "web": "Build and push DEV web image",
                 "admin": "Build and push admin image", "developer-portal": "Build and push developer-portal image",
                 "nats-hub-config-renderer-image": "Build and push renderer image",
                 "staging-stack-lock": "Render stack lock", "ci-gate": "Verify required jobs ran"})
    return jobs


def validate_run(run, sha, run_id, attempt):
    require(re.fullmatch(r"[a-f0-9]{40}", sha) and type(run_id) is int and run_id > 0
            and type(attempt) is int and attempt > 0, "identity_shape")
    require(run.get("id") == run_id and run.get("run_attempt") == attempt and run.get("head_sha") == sha
            and run.get("head_branch") == "master" and run.get("event") == "push"
            and run.get("path", "").split("@", 1)[0] == ".github/workflows/ci.yml"
            and run.get("status") == "completed" and run.get("conclusion") == "success"
            and run.get("repository", {}).get("full_name") == "Poryadok/VoiceRoot"
            and run.get("head_repository", {}).get("full_name") == "Poryadok/VoiceRoot", "canonical_push_ci")


def validate_jobs(jobs, sha, run_id, attempt):
    require(isinstance(jobs, list), "jobs")
    names = set()
    for job in jobs:
        require(job.get("run_id") == run_id and job.get("run_attempt") == attempt and job.get("head_sha") == sha
                and job.get("status") == "completed" and job.get("name") not in names, "job_identity")
        names.add(job["name"])
    for name, step in required_jobs().items():
        rows = [job for job in jobs if job.get("name") == name]
        require(len(rows) == 1 and rows[0].get("conclusion") == "success", "required_job_" + name)
        steps = [item for item in rows[0].get("steps", []) if item.get("name") == step]
        require(len(steps) == 1 and steps[0].get("conclusion") == "success", "required_step_" + name)
    promoted = [job for job in jobs if job.get("name") == "staging-images-promote"]
    require(len(promoted) == 1 and promoted[0].get("conclusion") == "skipped", "promotion_not_empty")
    require(not any(job.get("name", "").startswith("staging-images-promote (") for job in jobs), "promotion_matrix_present")
    deployments = [job for job in jobs if job.get("name") == "deploy-staging" or job.get("name", "").startswith("deploy-staging /")]
    require(len(deployments) == 1 and deployments[0].get("conclusion") == "skipped", "automatic_deploy_not_suppressed")


def blob_oid(raw):
    return hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()


def parse_request(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "duplicate_request_key")
            result[key] = value
        return result
    value = json.loads(raw, object_pairs_hook=unique)
    require(isinstance(value, dict) and set(value) == {"schema", "cycle", "request_id"}
            and value["schema"] == "voice-full-application-build-request-v1"
            and type(value["cycle"]) is int and value["cycle"] in (1, 2)
            and isinstance(value["request_id"], str) and re.fullmatch(r"[a-f0-9]{32}", value["request_id"]), "request_schema")
    return value


def validate_request(raw, previous_raw):
    value = parse_request(raw)
    if previous_raw is None:
        require(value["cycle"] == 1, "first_request_cycle")
    else:
        previous = parse_request(previous_raw)
        require(value["cycle"] in (previous["cycle"], previous["cycle"] + 1)
                and previous["request_id"] != value["request_id"], "request_transition")
    return value


def tree_blob(headers, commit, path, deadline):
    """Read exact first-parent tree; absence differs from a transport failure."""
    data = source._get(source.API + "/git/commits/" + commit, headers, deadline)
    require(data.get("sha") == commit and re.fullmatch(r"[a-f0-9]{40}", data.get("tree", {}).get("sha", "")), "parent_commit")
    tree = data["tree"]["sha"]
    parts = path.split("/")
    for i, part in enumerate(parts):
        obj = source._get(source.API + "/git/trees/" + tree, headers, deadline)
        require(obj.get("sha") == tree and obj.get("truncated") is False and isinstance(obj.get("tree"), list), "parent_tree")
        rows = [row for row in obj["tree"] if row.get("path") == part]
        require(len(rows) <= 1, "duplicate_tree_entry")
        if not rows: return None
        row = rows[0]
        require(row.get("type") == ("blob" if i == len(parts) - 1 else "tree")
                and re.fullmatch(r"[a-f0-9]{40}", row.get("sha", "")), "parent_entry")
        tree = row["sha"]
    return tree


def validate_images(images):
    require(isinstance(images, dict) and set(images) == IMAGE_NAMES, "image_inventory")
    for name, value in images.items():
        require(isinstance(value, str) and re.fullmatch(re.escape(REGISTRY + "/" + name) + r"@sha256:[a-f0-9]{64}", value), "image_identity")


def write_receipt(output, receipt):
    """Publish complete bytes exclusively; a failed operation cannot replace a receipt."""
    data = (json.dumps(receipt, sort_keys=True, indent=2) + "\n").encode()
    fd, name = tempfile.mkstemp(prefix=".voice-application-", dir=output.parent)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.link(name, output)
    finally:
        os.unlink(name)


def capture(token, sha, run_id, attempt, output):
    require(isinstance(token, str) and token and not any(x in token for x in "\r\n"), "token")
    verifier_sha = common.command(["git", "rev-parse", "HEAD"], cwd=ROOT).decode().strip()
    require(re.fullmatch(r"[a-f0-9]{40}", verifier_sha), "verifier_identity")
    checkout = common.source_snapshot(ROOT, verifier_sha)
    headers = source._headers(token)
    deadline = time.monotonic() + 120
    run = source._get(source.API + "/actions/runs/" + str(run_id), headers, deadline)
    validate_run(run, sha, run_id, attempt)
    jobs = source._pages(source.API + "/actions/runs/" + str(run_id) + "/attempts/" + str(attempt) + "/jobs", "jobs", headers, deadline)
    validate_jobs(jobs, sha, run_id, attempt)
    commit = source._get(source.API + "/git/commits/" + sha, headers, deadline)
    parents = commit.get("parents")
    require(commit.get("sha") == sha and isinstance(parents, list) and len(parents) >= 1
            and re.fullmatch(r"[a-f0-9]{40}", parents[0].get("sha", "")), "first_parent")
    previous_oid = tree_blob(headers, parents[0]["sha"], REQUEST, deadline)
    previous = None
    if previous_oid is not None:
        blob = source._get(source.API + "/git/blobs/" + previous_oid, headers, deadline)
        require(blob.get("sha") == previous_oid and blob.get("encoding") == "base64", "previous_blob")
        previous = base64.b64decode("".join(blob.get("content", "").split()), validate=True)
        require(len(previous) <= 4096 and blob_oid(previous) == previous_oid, "previous_blob_bytes")
    with tempfile.TemporaryDirectory(prefix="voice-application-source-") as folder:
        authority = source.capture_equivalent_master_source(token, run_id, sha, Path(folder))
        require(authority.get("run_attempt") == attempt and authority.get("source_sha") == sha, "authority_binding")
        raw = (Path(folder) / REQUEST).read_bytes()
        request = validate_request(raw, previous)
        images = authority["images"]
        validate_images(images)
        # Preserve the canonical source-equivalence boundary and pin its observed
        # master, then recheck the same CI attempt and immutable tag resolution.
        check_deadline = time.monotonic() + 120
        equivalence = authority.get("source_equivalence")
        master = equivalence["current_master_sha_after"] if equivalence else sha
        repo = source._head(headers, master, check_deadline)
        if equivalence:
            source._equivalent_master_source(headers, sha, master, check_deadline)
        again, _ = source._ci(headers, run_id, sha, repo, check_deadline)
        validate_run(again, sha, run_id, attempt)
        jobs = source._pages(source.API + "/actions/runs/" + str(run_id) + "/attempts/" + str(attempt) + "/jobs", "jobs", headers, check_deadline)
        validate_jobs(jobs, sha, run_id, attempt)
        for name, reference in images.items():
            require(source._resolve_image(name, sha, check_deadline) == reference, "tag_digest_changed")
        require(common.source_snapshot(ROOT, verifier_sha) == checkout, "verifier_checkout_changed")
        receipt = {"schema": "voice-canonical-application-package-v1", "source_sha": sha,
                   "verifier_sha": verifier_sha, "run_id": run_id, "run_attempt": attempt, "request": request,
                   "request_blob_oid": blob_oid(raw), "lock_artifact_id": authority["lock_artifact_id"],
                   "lock_sha256": authority["lock_sha256"], "images": images,
                   "runtime_qualified": False, "deployment_performed": False}
        if equivalence:
            receipt["source_equivalence"] = equivalence
        write_receipt(output, receipt)
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--run-id", type=int, required=True)
    parser.add_argument("--attempt", type=int, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    capture(os.environ.get("GH_TOKEN", ""), args.source_sha, args.run_id, args.attempt, args.output)


if __name__ == "__main__": main()
