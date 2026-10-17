"""Execute the fixed, source-bound Game Integration first install in voice-dev.

The command verifies a successful first-party image-push job and its immutable
OCI digest, checks TLS and existing PostgreSQL state, then delegates migration,
verification, paused pair creation, guarded start, readiness, and rollback to
the shared GIS prerequisite and first-install transactions.
"""
import base64
import copy
import json
import hashlib
import io
import os
import re
import secrets
import subprocess
import sys
import tarfile
import tempfile
from pathlib import Path
from datetime import datetime, timezone

NAMESPACE = "voice-dev"
BUILD_RECEIPT_SCHEMA = "voice-dev-source-build-v2"
IMAGE = "ghcr.io/poryadok/voiceroot/gameintegration"
TLS_SECRET = "voice-gameintegration-jwks-tls"
CA_SECRET = "voice-media-ca"
APP_SECRET = "voice-app-secrets"
SIGNER_SECRET = "voice-gameintegration-principal-signing"
WORKLOAD_SECRET = "voice-gameintegration-workload-credentials"
DB_NAME = "game_integration_db"
DB_USER = "gameintegration_runtime"
OPERATION_ANNOTATION = "voice.dev/gis-first-install-operation"
HASH = re.compile(r"[a-f0-9]{64}\Z")
SOURCE = re.compile(r"[a-f0-9]{40}\Z")
_STAGING_MODULES = str(Path(__file__).resolve().parents[2] / "scripts" / "staging" / "nats-rollout-preservation")
if _STAGING_MODULES not in sys.path:
    sys.path.insert(0, _STAGING_MODULES)


class Blocked(RuntimeError):
    """A fixed, non-sensitive first-install rejection."""


def _blocked(code):
    raise Blocked(code)


def _source_backend_context(root, runner):
    """Return the committed src/backend tree and its stable content identity."""
    try:
        tree = runner(["git", "-C", str(root), "rev-parse", "HEAD:src/backend"],
            check=True, capture_output=True, text=True, timeout=15).stdout.strip()
    except Exception:
        _blocked("gis_dev_build_context_unavailable")
    if not re.fullmatch(r"[a-f0-9]{40}", tree):
        _blocked("gis_dev_build_context_invalid")
    # Git tree IDs encode every path, mode, and blob under src/backend without
    # archive timestamps or host-dependent file metadata.
    return tree, hashlib.sha256(tree.encode("ascii")).hexdigest()


def _extract_source_backend_context(raw, destination):
    """Extract regular committed files only; reject links and path traversal."""
    from pathlib import PurePosixPath
    destination = Path(destination)
    seen = set()
    try:
        with tarfile.open(fileobj=io.BytesIO(raw), mode="r:") as archive:
            for member in archive:
                path = PurePosixPath(member.name)
                if (path.is_absolute() or ".." in path.parts or path.parts[:2] != ("src", "backend")
                    or not (member.isdir() or member.isfile()) or member.name in seen):
                    _blocked("gis_dev_build_context_invalid")
                seen.add(member.name)
                relative = path.parts[2:]
                if not relative:
                    continue
                target = destination.joinpath(*relative)
                if member.isdir():
                    target.mkdir(mode=0o755, parents=True, exist_ok=True)
                    continue
                target.parent.mkdir(mode=0o755, parents=True, exist_ok=True)
                stream = archive.extractfile(member)
                if stream is None:
                    _blocked("gis_dev_build_context_invalid")
                with stream, target.open("xb") as output:
                    output.write(stream.read())
                target.chmod(member.mode & 0o755)
    except Blocked:
        raise
    except Exception:
        _blocked("gis_dev_build_context_invalid")


def _valid_sha(value):
    return isinstance(value, str) and HASH.fullmatch(value) is not None


def _digest(value):
    import json
    raw = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(raw).hexdigest()


def validate_build_receipt(source_sha, image_ref, receipt, *, clean):
    """Require immutable image bytes tied to a verified successful GitHub builder."""
    keys = {"schema", "source_sha", "image_ref", "build_context_sha256", "image_config_sha256",
        "working_tree_clean", "builder_kind", "builder_run", "builder_job", "builder_sha", "builder_tree",
        "builder_artifact_id", "builder_artifact_sha256", "builder_output_index_sha256",
        "builder_provenance_sha256"}
    if (clean is not True or not isinstance(source_sha, str) or not SOURCE.fullmatch(source_sha)
        or not isinstance(image_ref, str) or not image_ref.startswith(IMAGE + "@sha256:")
        or not _valid_sha(image_ref.rsplit("@sha256:", 1)[-1])
        or not isinstance(receipt, dict) or set(receipt) != keys
        or receipt.get("schema") != BUILD_RECEIPT_SCHEMA
        or receipt.get("source_sha") != source_sha or receipt.get("image_ref") != image_ref
        or receipt.get("working_tree_clean") is not True
        or not isinstance(receipt.get("builder_sha"), str) or not SOURCE.fullmatch(receipt["builder_sha"])
        or not isinstance(receipt.get("builder_tree"), str) or not re.fullmatch(r"[a-f0-9]{40}", receipt["builder_tree"])
        or receipt.get("build_context_sha256") != hashlib.sha256(receipt["builder_tree"].encode("ascii")).hexdigest()
        or not _valid_sha(receipt.get("build_context_sha256"))
        or not _valid_sha(receipt.get("image_config_sha256"))):
        _blocked("gis_dev_source_build_receipt_invalid")
    if receipt["builder_kind"] == "github-actions":
        if (type(receipt.get("builder_run")) is not int or receipt["builder_run"] <= 0
            or type(receipt.get("builder_job")) is not int or receipt["builder_job"] <= 0
            or type(receipt.get("builder_artifact_id")) is not int or receipt["builder_artifact_id"] <= 0
            or not _valid_sha(receipt.get("builder_artifact_sha256"))
            or not _valid_sha(receipt.get("builder_output_index_sha256"))
            or receipt.get("builder_output_index_sha256") != image_ref.rsplit("@sha256:", 1)[-1]
            or not _valid_sha(receipt.get("builder_provenance_sha256"))):
            _blocked("gis_dev_source_build_receipt_invalid")
    elif receipt["builder_kind"] == "local-build":
        if receipt.get("builder_run") is not None or receipt.get("builder_job") is not None \
                or receipt["builder_sha"] != source_sha or receipt.get("builder_artifact_id") is not None \
                or receipt.get("builder_artifact_sha256") is not None \
                or receipt.get("builder_output_index_sha256") is not None \
                or receipt.get("builder_provenance_sha256") is not None:
            _blocked("gis_dev_source_build_receipt_invalid")
    else:
        _blocked("gis_dev_source_build_receipt_invalid")
    return True


def _gh_json(args, runner):
    try:
        result = runner(["gh", "api", *args], check=True, capture_output=True,
            text=True, timeout=45)
        value = json.loads(result.stdout)
    except Exception:
        _blocked("gis_dev_github_build_evidence_unavailable")
    if not isinstance(value, dict):
        _blocked("gis_dev_github_build_evidence_invalid")
    return value


def _record_member(archive, descriptors, media_type, *, annotation=None):
    matches = [row for row in descriptors if isinstance(row, dict) and row.get("mediaType") == media_type
        and (annotation is None or row.get("annotations", {}).get(annotation[0]) == annotation[1])]
    if len(matches) != 1:
        _blocked("gis_dev_build_record_invalid")
    descriptor = matches[0]
    digest = descriptor.get("digest", "")
    if not re.fullmatch(r"sha256:[a-f0-9]{64}", digest) or type(descriptor.get("size")) is not int:
        _blocked("gis_dev_build_record_invalid")
    path = "blobs/sha256/" + digest.removeprefix("sha256:")
    member = archive.getmember(path)
    if not member.isfile() or member.size != descriptor["size"] or member.size > 8 << 20:
        _blocked("gis_dev_build_record_invalid")
    raw = archive.extractfile(member).read()
    if hashlib.sha256(raw).hexdigest() != digest.removeprefix("sha256:"):
        _blocked("gis_dev_build_record_invalid")
    return raw, digest.removeprefix("sha256:")


def _timestamp(value):
    if not isinstance(value, str):
        _blocked("gis_dev_build_record_invalid")
    try:
        result = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        _blocked("gis_dev_build_record_invalid")
    if result.tzinfo is None:
        _blocked("gis_dev_build_record_invalid")
    return result.astimezone(timezone.utc)


def _verify_buildx_artifact(raw, artifact_id, expected_size, image_ref, run_id, attempt,
                            builder_sha, step, runner):
    """Verify the authenticated Actions artifact's Buildx record, provenance and OCI descriptor."""
    from pathlib import PurePosixPath
    if not isinstance(raw, bytes) or not 1 <= len(raw) <= 32 << 20 or len(raw) != expected_size:
        _blocked("gis_dev_build_record_invalid")
    artifact_sha = hashlib.sha256(raw).hexdigest()
    expected_digest = image_ref.rsplit("@sha256:", 1)[-1]
    try:
        with tarfile.open(fileobj=io.BytesIO(raw), mode="r:gz") as archive:
            members = archive.getmembers()
            if len(members) > 128 or any(not m.isfile() and not m.isdir() for m in members):
                _blocked("gis_dev_build_record_invalid")
            names = [m.name for m in members]
            if (len(names) != len(set(names)) or sum(m.size for m in members) > 32 << 20 or any(
                name.startswith("/") or ".." in PurePosixPath(name).parts or "\\" in name for name in names
            )):
                _blocked("gis_dev_build_record_invalid")
            index_member = archive.getmember("index.json")
            if not index_member.isfile() or index_member.size > 64 << 10:
                _blocked("gis_dev_build_record_invalid")
            index = json.loads(archive.extractfile(index_member).read())
            if index.get("schemaVersion") != 2 or index.get("mediaType") != "application/vnd.oci.image.index.v1+json":
                _blocked("gis_dev_build_record_invalid")
            records = [row for row in index.get("manifests", [])
                if row.get("mediaType") == "application/vnd.oci.image.manifest.v1+json"]
            verified = []
            for record_descriptor in records:
                manifest_raw, _ = _record_member(archive, [record_descriptor],
                    "application/vnd.oci.image.manifest.v1+json")
                record_manifest = json.loads(manifest_raw)
                config_raw, config_sha = _record_member(archive, [record_manifest.get("config", {})],
                    "application/vnd.buildkit.historyrecord.v0")
                record = json.loads(config_raw)
                attrs = record.get("FrontendAttrs")
                if not isinstance(attrs, dict):
                    continue
                expected_builder = f"https://github.com/Poryadok/VoiceRoot/actions/runs/{run_id}/attempts/{attempt}"
                if (attrs.get("vcs:source") != "https://github.com/Poryadok/VoiceRoot"
                    or attrs.get("vcs:revision") != builder_sha
                    or attrs.get("vcs:localdir:context") != "src/backend"
                    or attrs.get("vcs:localdir:dockerfile") != "src/backend/gameintegration"
                    or attrs.get("filename") != "Dockerfile"
                    or expected_builder not in attrs.get("attest:provenance", "")):
                    continue
                exporters = record.get("Exporters")
                if (not isinstance(exporters, list) or len(exporters) != 1
                    or exporters[0].get("Type") != "image"
                    or exporters[0].get("Attrs", {}).get("push") != "true"
                    or exporters[0].get("Attrs", {}).get("name") != IMAGE + ":" + builder_sha):
                    continue
                output_rows = record.get("Result", {}).get("Results", {})
                output = output_rows.get("0") if isinstance(output_rows, dict) else None
                if (not isinstance(output, dict) or output.get("media_type") != "application/vnd.oci.image.index.v1+json"
                    or output.get("digest") != "sha256:" + expected_digest):
                    continue
                output_raw, output_sha = _record_member(archive, record_manifest.get("layers", []),
                    "application/json", annotation=("vnd.export-build.descriptor.mediatype",
                        "application/vnd.oci.image.index.v1+json"))
                if output_sha != expected_digest or len(output_raw) != output.get("size"):
                    continue
                output_index = json.loads(output_raw)
                platforms = [row for row in output_index.get("manifests", [])
                    if row.get("platform", {}).get("os") == "linux"
                    and row.get("platform", {}).get("architecture") == "amd64"]
                if len(platforms) != 1:
                    continue
                attestation, attestation_sha = _record_member(archive, record_manifest.get("layers", []),
                    "application/vnd.in-toto+json", annotation=("in-toto.io/predicate-type",
                        "https://slsa.dev/provenance/v1"))
                provenance = json.loads(attestation)
                external = provenance.get("buildDefinition", {}).get("externalParameters", {})
                request = external.get("request", {})
                root = request.get("root", {})
                args = root.get("request", {}).get("args", {})
                step_start = _timestamp(step.get("started_at"))
                step_end = _timestamp(step.get("completed_at"))
                started = _timestamp(provenance.get("runDetails", {}).get("metadata", {}).get("startedOn"))
                finished = _timestamp(provenance.get("runDetails", {}).get("metadata", {}).get("finishedOn"))
                if (provenance.get("runDetails", {}).get("builder", {}).get("id") != expected_builder
                    or args != {"vcs:localdir:context": "src/backend",
                        "vcs:localdir:dockerfile": "src/backend/gameintegration",
                        "vcs:revision": builder_sha,
                        "vcs:source": "https://github.com/Poryadok/VoiceRoot"}
                    or root.get("configSource", {}).get("path") != "Dockerfile"
                    or started < step_start or finished > step_end or finished < started):
                    continue
                verified.append((config_sha, attestation_sha))
            if len(verified) != 1:
                _blocked("gis_dev_build_record_binding_invalid")
            return {"builder_artifact_id": artifact_id,
                "builder_artifact_sha256": artifact_sha,
                "builder_output_index_sha256": expected_digest,
                "builder_provenance_sha256": verified[0][1]}
    except Blocked:
        raise
    except Exception:
        _blocked("gis_dev_build_record_invalid")


def _verify_github_build(source_root, image_ref, builder_run, builder_job, current_tree, runner):
    """Bind an OCI digest to the successful fixed master image job and its Buildx record."""
    if (type(builder_run) is not int or builder_run <= 0
        or type(builder_job) is not int or builder_job <= 0):
        _blocked("gis_dev_builder_identity_invalid")
    root = Path(source_root)
    run = _gh_json([f"repos/Poryadok/VoiceRoot/actions/runs/{builder_run}"], runner)
    if (run.get("id") != builder_run or run.get("status") != "completed"
        or run.get("conclusion") != "success" or run.get("event") != "push"
        or run.get("head_branch") != "master"
        or (run.get("head_repository") or {}).get("full_name") != "Poryadok/VoiceRoot"
        or not SOURCE.fullmatch(str(run.get("head_sha", "")))
        or type(run.get("run_attempt")) is not int or run["run_attempt"] <= 0):
        _blocked("gis_dev_builder_run_not_successful_master_push")
    workflow_id = run.get("workflow_id")
    if type(workflow_id) is not int:
        _blocked("gis_dev_builder_workflow_invalid")
    workflow = _gh_json([f"repos/Poryadok/VoiceRoot/actions/workflows/{workflow_id}"], runner)
    if workflow.get("path") != ".github/workflows/ci.yml":
        _blocked("gis_dev_builder_workflow_invalid")
    jobs = _gh_json([f"repos/Poryadok/VoiceRoot/actions/runs/{builder_run}/jobs?per_page=100"], runner)
    rows = jobs.get("jobs")
    matched = [row for row in rows if isinstance(row, dict) and row.get("id") == builder_job] if isinstance(rows, list) else []
    job = matched[0] if len(matched) == 1 else None
    if (not job or job.get("status") != "completed" or job.get("conclusion") != "success"
        or job.get("name") != "staging-images-push (gameintegration)"
        or not isinstance(job.get("steps"), list)
        or any(step.get("conclusion") != "success" for step in job["steps"])):
        _blocked("gis_dev_builder_job_not_successful_gameintegration_push")
    producer_steps = [step for step in job["steps"]
        if step.get("name") == "Build and push gameintegration image"]
    if len(producer_steps) != 1 or producer_steps[0].get("conclusion") != "success":
        _blocked("gis_dev_builder_image_step_not_successful")
    step = producer_steps[0]
    builder_sha = run["head_sha"]
    try:
        builder_tree = runner(["git", "-C", str(root), "rev-parse", builder_sha + ":src/backend"],
            check=True, capture_output=True, text=True, timeout=15).stdout.strip()
        context_list = runner(["git", "-C", str(root), "show", builder_sha + ":scripts/ci/backend-docker-context.txt"],
            check=True, capture_output=True, text=True, timeout=15).stdout.splitlines()
        workflow_source = runner(["git", "-C", str(root), "show", builder_sha + ":.github/workflows/ci.yml"],
            check=True, capture_output=True, text=True, timeout=15).stdout
    except Exception:
        _blocked("gis_dev_builder_source_unavailable")
    push_at = workflow_source.find("  staging-images-push:")
    promote_at = workflow_source.find("  staging-images-promote:", push_at + 1)
    push_job = workflow_source[push_at:promote_at] if push_at >= 0 and promote_at > push_at else ""
    if (not re.fullmatch(r"[a-f0-9]{40}", builder_tree) or builder_tree != current_tree
        or "gameintegration" not in context_list
        or 'if grep -qx "$S" scripts/ci/backend-docker-context.txt' not in workflow_source
        or 'echo "context=./src/backend"' not in workflow_source
        or "- name: Build and push ${{ matrix.service }} image" not in push_job
        or "docker/build-push-action@v6" not in push_job
        or "context: ${{ steps.dockerctx.outputs.context }}" not in push_job
        or "file: ${{ steps.dockerctx.outputs.file }}" not in push_job
        or "push: true" not in push_job):
        _blocked("gis_dev_builder_source_context_mismatch")
    try:
        artifacts = _gh_json([f"repos/Poryadok/VoiceRoot/actions/runs/{builder_run}/artifacts?per_page=100"], runner)
        rows = artifacts.get("artifacts")
        if not isinstance(rows, list) or artifacts.get("total_count") != len(rows):
            _blocked("gis_dev_builder_artifact_invalid")
    except Exception:
        _blocked("gis_dev_builder_artifact_invalid")
    candidates = [row for row in rows if isinstance(row, dict)
        and isinstance(row.get("name"), str) and row["name"].endswith(".dockerbuild")
        and row.get("expired") is False and type(row.get("id")) is int
        and row["id"] > 0 and type(row.get("size_in_bytes")) is int
        and 0 < row["size_in_bytes"] <= 32 << 20]
    verified = []
    for artifact in candidates:
        try:
            raw = runner(["gh", "api", f"repos/Poryadok/VoiceRoot/actions/artifacts/{artifact['id']}/zip"],
                check=True, capture_output=True, timeout=90).stdout
            evidence = _verify_buildx_artifact(raw, artifact["id"], artifact["size_in_bytes"],
                image_ref, builder_run, run["run_attempt"], builder_sha, step, runner)
            verified.append(evidence)
        except Blocked:
            continue
        except Exception:
            continue
    if len(verified) != 1:
        _blocked("gis_dev_builder_artifact_binding_invalid")
    return builder_sha, builder_tree, verified[0]


def source_build_receipt(source_root, image_ref, *, builder_run=None, builder_job=None,
                         local_build=False, runner=subprocess.run):
    """Produce source authority from clean Git state, GitHub CI evidence, and OCI bytes."""
    root = Path(source_root).resolve(strict=True)
    try:
        source_sha = runner(["git", "-C", str(root), "rev-parse", "HEAD"],
            check=True, capture_output=True, text=True, timeout=15).stdout.strip()
        tree_sha = runner(["git", "-C", str(root), "rev-parse", "HEAD^{tree}"],
            check=True, capture_output=True, text=True, timeout=15).stdout.strip()
        dirty = runner(["git", "-C", str(root), "status", "--porcelain=v1",
            "--untracked-files=all"], check=True, capture_output=True,
            text=True, timeout=15).stdout
        if dirty:
            _blocked("gis_dev_source_tree_dirty")
        if not SOURCE.fullmatch(source_sha) or not re.fullmatch(r"[a-f0-9]{40}", tree_sha):
            _blocked("gis_dev_source_identity_invalid")
        context_tree, context_sha = _source_backend_context(root, runner)
        if (not isinstance(image_ref, str) or not image_ref.startswith(IMAGE + "@sha256:")
            or not _valid_sha(image_ref.rsplit("@sha256:", 1)[-1])):
            _blocked("gis_dev_image_reference_invalid")
        if local_build:
            builder_sha, builder_tree = source_sha, context_tree
            builder_evidence = {"builder_artifact_id": None,
                "builder_artifact_sha256": None, "builder_output_index_sha256": None,
                "builder_provenance_sha256": None}
        else:
            builder_sha, builder_tree, builder_evidence = _verify_github_build(root, image_ref, builder_run,
                builder_job, context_tree, runner)
        raw = runner(["docker", "buildx", "imagetools", "inspect", "--raw", image_ref],
            check=True, capture_output=True, timeout=30).stdout
        index_digest = image_ref.rsplit("@sha256:", 1)[-1]
        if not isinstance(raw, bytes) or hashlib.sha256(raw).hexdigest() != index_digest:
            _blocked("gis_dev_image_index_digest_mismatch")
        manifest = json.loads(raw)
        manifests = manifest.get("manifests")
        if isinstance(manifests, list):
            matches = [row for row in manifests if row.get("platform", {}).get("os") == "linux"
                and row.get("platform", {}).get("architecture") == "amd64"]
            if len(matches) != 1:
                _blocked("gis_dev_image_manifest_invalid")
            child_digest = matches[0].get("digest", "")
            if not re.fullmatch(r"sha256:[a-f0-9]{64}", child_digest):
                _blocked("gis_dev_image_manifest_invalid")
            child = runner(["docker", "buildx", "imagetools", "inspect", "--raw",
                image_ref.rsplit("@", 1)[0] + "@" + child_digest],
                check=True, capture_output=True, timeout=30).stdout
            if not isinstance(child, bytes) or hashlib.sha256(child).hexdigest() != child_digest.removeprefix("sha256:"):
                _blocked("gis_dev_image_child_digest_mismatch")
            manifest = json.loads(child)
        config_digest = manifest.get("config", {}).get("digest")
        if not isinstance(config_digest, str) or not re.fullmatch(r"sha256:[a-f0-9]{64}", config_digest):
            _blocked("gis_dev_image_manifest_invalid")
        runner(["docker", "pull", image_ref], check=True, capture_output=True, timeout=180)
        inspected = runner(["docker", "image", "inspect", image_ref], check=True,
            capture_output=True, timeout=30).stdout
        rows = json.loads(inspected)
        if not isinstance(rows, list) or len(rows) != 1:
            _blocked("gis_dev_image_inspection_invalid")
        row = rows[0]
        image_config = row.get("Id", "").removeprefix("sha256:")
        labels = row.get("Config", {}).get("Labels") or {}
        if (not _valid_sha(image_config) or config_digest != "sha256:" + image_config
            or image_ref not in row.get("RepoDigests", [])):
            _blocked("gis_dev_image_source_binding_invalid")
        if local_build and (labels.get("org.opencontainers.image.revision") != source_sha
            or labels.get("io.voice.build.context.sha256") != context_sha):
            _blocked("gis_dev_local_image_source_binding_invalid")
        return source_sha, {"schema": BUILD_RECEIPT_SCHEMA, "source_sha": source_sha,
            "image_ref": image_ref, "build_context_sha256": context_sha,
            "image_config_sha256": image_config, "working_tree_clean": True,
            "builder_kind": "local-build" if local_build else "github-actions",
            "builder_run": builder_run, "builder_job": builder_job,
            "builder_sha": builder_sha, "builder_tree": builder_tree, **builder_evidence}
    except Blocked:
        raise
    except Exception:
        _blocked("gis_dev_source_image_preflight_failed")


def verify_current_source(source_root, source_sha, backend_tree, *, runner=subprocess.run):
    """Recheck the exact clean source immediately before each write boundary."""
    root = Path(source_root).resolve(strict=True)
    try:
        current_sha = runner(["git", "-C", str(root), "rev-parse", "HEAD"],
            check=True, capture_output=True, text=True, timeout=15).stdout.strip()
        dirty = runner(["git", "-C", str(root), "status", "--porcelain=v1",
            "--untracked-files=all"], check=True, capture_output=True,
            text=True, timeout=15).stdout
        current_tree, _ = _source_backend_context(root, runner)
    except Exception:
        _blocked("gis_dev_source_revalidation_failed")
    if dirty or current_sha != source_sha or current_tree != backend_tree:
        _blocked("gis_dev_source_changed_before_mutation")


def build_and_publish_image(source_root, tag, *, runner=subprocess.run):
    """Build only the fixed GIS Dockerfile from a clean commit and publish a digest."""
    root = Path(source_root).resolve(strict=True)
    if not isinstance(tag, str) or not re.fullmatch(r"dev-[a-f0-9]{12}", tag):
        _blocked("gis_dev_image_tag_invalid")
    try:
        source_sha = runner(["git", "-C", str(root), "rev-parse", "HEAD"], check=True,
            capture_output=True, text=True, timeout=15).stdout.strip()
        tree_sha = runner(["git", "-C", str(root), "rev-parse", "HEAD^{tree}"], check=True,
            capture_output=True, text=True, timeout=15).stdout.strip()
        dirty = runner(["git", "-C", str(root), "status", "--porcelain=v1",
            "--untracked-files=all"], check=True, capture_output=True, text=True,
            timeout=15).stdout
        if dirty or not SOURCE.fullmatch(source_sha) or not re.fullmatch(r"[a-f0-9]{40}", tree_sha):
            _blocked("gis_dev_source_tree_dirty")
        _, context_sha = _source_backend_context(root, runner)
        with tempfile.TemporaryDirectory(prefix="voice-dev-gis-build-") as directory:
            metadata_path = Path(directory) / "build-metadata.json"
            context_path = Path(directory) / "context"
            context_path.mkdir(mode=0o700)
            archive = runner(["git", "-C", str(root), "archive", "--format=tar", "HEAD", "src/backend"],
                check=True, capture_output=True, timeout=120).stdout
            if not isinstance(archive, bytes) or not 1 <= len(archive) <= 512 << 20:
                _blocked("gis_dev_build_context_invalid")
            _extract_source_backend_context(archive, context_path)
            command = ["docker", "buildx", "build", "--push", "--platform", "linux/amd64",
                "--file", "gameintegration/Dockerfile", "--tag", IMAGE + ":" + tag,
                "--label", "org.opencontainers.image.revision=" + source_sha,
                "--label", "io.voice.build.context.sha256=" + context_sha,
                "--metadata-file", str(metadata_path), "."]
            runner(command, cwd=context_path, check=True, capture_output=True, timeout=900)
            metadata = json.loads(metadata_path.read_text(encoding="utf-8"))
        digest = metadata.get("containerimage.digest")
        if not isinstance(digest, str) or not re.fullmatch(r"sha256:[a-f0-9]{64}", digest):
            _blocked("gis_dev_image_publish_receipt_invalid")
        image_ref = IMAGE + "@" + digest
        actual_sha, receipt = source_build_receipt(root, image_ref, local_build=True, runner=runner)
        if actual_sha != source_sha:
            _blocked("gis_dev_source_changed_during_build")
        return image_ref, receipt
    except Blocked:
        raise
    except Exception:
        _blocked("gis_dev_image_build_failed")


def _decode_secret(secret, name):
    if not isinstance(secret, dict) or secret.get("kind") != "Secret":
        _blocked("gis_dev_secret_missing_or_invalid")
    metadata = secret.get("metadata", {})
    if (metadata.get("name") != name or metadata.get("namespace") != NAMESPACE
        or not isinstance(metadata.get("uid"), str) or not metadata["uid"]
        or not isinstance(metadata.get("resourceVersion"), str) or not metadata["resourceVersion"]):
        _blocked("gis_dev_secret_identity_invalid")
    encoded = secret.get("data")
    if not isinstance(encoded, dict):
        _blocked("gis_dev_secret_data_invalid")
    try:
        return {key: base64.b64decode(value, validate=True) for key, value in encoded.items()}
    except (ValueError, TypeError):
        _blocked("gis_dev_secret_data_invalid")


def validate_tls_leaf(cert, key, ca):
    """Validate provided TLS material against the existing public media CA."""
    if not all(isinstance(value, bytes) and value for value in (cert, key, ca)):
        _blocked("gis_dev_tls_material_missing")
    try:
        with tempfile.TemporaryDirectory(prefix="voice-dev-gis-tls-") as directory:
            root = Path(directory)
            cert_path, key_path, ca_path = root / "tls.crt", root / "tls.key", root / "ca.crt"
            cert_path.write_bytes(cert); key_path.write_bytes(key); ca_path.write_bytes(ca)
            for path in (cert_path, key_path, ca_path):
                path.chmod(0o600)
            subprocess.run(["openssl", "verify", "-CAfile", str(ca_path),
                "-verify_hostname", "voice-gameintegration", "-purpose", "sslserver", str(cert_path)],
                check=True, capture_output=True, timeout=15)
            san = subprocess.run(["openssl", "x509", "-in", str(cert_path), "-noout",
                "-ext", "subjectAltName"], check=True, capture_output=True,
                text=True, timeout=15).stdout
            names = re.findall(r"(?:^|[,\s])DNS:([^,\s]+)", san)
            if names != ["voice-gameintegration"]:
                _blocked("gis_dev_tls_san_invalid")
            cert_public = subprocess.run(["openssl", "x509", "-in", str(cert_path), "-pubkey", "-noout"],
                check=True, capture_output=True, timeout=15).stdout
            key_public = subprocess.run(["openssl", "pkey", "-in", str(key_path), "-pubout"],
                check=True, capture_output=True, timeout=15).stdout
            if not cert_public or cert_public != key_public:
                _blocked("gis_dev_tls_key_mismatch")
    except Blocked:
        raise
    except Exception:
        _blocked("gis_dev_tls_validation_failed")
    return True


def generate_secret_material():
    """Generate only GIS runtime credentials; TLS must come from the existing issuer."""
    password = secrets.token_urlsafe(32).encode()
    with tempfile.TemporaryDirectory(prefix="voice-dev-gis-signers-") as directory:
        root = Path(directory)
        keys = {}
        try:
            for kid in ("current", "next"):
                path = root / (kid + ".pem")
                subprocess.run(["openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt",
                    "rsa_keygen_bits:3072", "-out", str(path)], check=True,
                    capture_output=True, timeout=30)
                path.chmod(0o600)
                keys[kid + ".pem"] = path.read_bytes()
        except Exception:
            _blocked("gis_dev_signer_generation_failed")
    if keys["current.pem"] == keys["next.pem"]:
        _blocked("gis_dev_signer_generation_invalid")
    return {
        "GAME_INTEGRATION_DB_PASSWORD": password,
        "GAME_INTEGRATION_DATABASE_URL": (
            f"postgres://{DB_USER}:{password.decode()}@voice-postgres:5432/{DB_NAME}?sslmode=disable"
        ).encode(),
        SIGNER_SECRET: {**keys, "active-kid": b"current"},
        WORKLOAD_SECRET: {
            "GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64": base64.b64encode(secrets.token_bytes(32)),
            "GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64": base64.b64encode(secrets.token_bytes(32)),
        },
    }


def _secret_body(name, values, *, operation=None):
    metadata = {"name": name, "namespace": NAMESPACE}
    if operation is not None:
        metadata["annotations"] = {OPERATION_ANNOTATION: operation}
    return {"apiVersion": "v1", "kind": "Secret",
        "metadata": metadata, "type": "Opaque",
        "data": {key: base64.b64encode(value).decode("ascii") for key, value in values.items()}}


def _validate_signer_values(values):
    if (not isinstance(values, dict) or not {"current.pem", "next.pem", "active-kid"} <= set(values)
        or values["active-kid"] not in (b"current", b"next", "current", "next")
        or values["current.pem"] == values["next.pem"]):
        _blocked("gis_dev_signer_keys_invalid")
    try:
        with tempfile.TemporaryDirectory(prefix="voice-dev-gis-signer-check-") as directory:
            for name in ("current.pem", "next.pem"):
                path = Path(directory) / name
                path.write_bytes(values[name]); path.chmod(0o600)
                subprocess.run(["openssl", "pkey", "-in", str(path), "-check", "-noout"],
                    check=True, capture_output=True, timeout=15)
    except Exception:
        _blocked("gis_dev_signer_keys_invalid")


def _verify_existing(name, secret, required, validator):
    values = _decode_secret(secret, name)
    if not set(required) <= set(values):
        _blocked("gis_dev_existing_secret_incomplete")
    validator(values)


def build_secret_plan(kube, *, operation=None, tls_validator=validate_tls_leaf, signer_validator=_validate_signer_values,
                      material_factory=generate_secret_material):
    """Preflight existing DEV identities and prepare CAS/create-only Secret writes."""
    try:
        namespace = kube.get_namespace()
    except Exception:
        _blocked("gis_dev_namespace_unavailable")
    metadata = namespace.get("metadata", {}) if isinstance(namespace, dict) else {}
    if metadata.get("name") != NAMESPACE or not isinstance(metadata.get("uid"), str) or not metadata["uid"]:
        _blocked("gis_dev_namespace_identity_invalid")
    if operation is not None and not re.fullmatch(r"[a-f0-9]{12}", operation):
        _blocked("gis_dev_operation_invalid")
    try:
        app = kube.get_secret(APP_SECRET)
        tls = kube.get_secret(TLS_SECRET)
        ca = kube.get_secret(CA_SECRET)
        signer = kube.get_secret(SIGNER_SECRET)
        workload = kube.get_secret(WORKLOAD_SECRET)
    except Exception:
        _blocked("gis_dev_secret_read_failed")
    app_values = _decode_secret(app, APP_SECRET)
    tls_values = _decode_secret(tls, TLS_SECRET)
    ca_values = _decode_secret(ca, CA_SECRET)
    if ("POSTGRES_PASSWORD" not in app_values or not {"tls.crt", "tls.key"} <= set(tls_values)
        or "jwks-ca.crt" not in ca_values):
        _blocked("gis_dev_tls_or_database_prerequisite_missing")
    if operation is not None:
        owner = app.get("metadata", {}).get("annotations", {}).get(OPERATION_ANNOTATION)
        if owner is not None and owner != operation:
            _blocked("gis_dev_secret_operation_changed")
    # This must precede generation and all writes; supplied TLS identity is never replaced.
    # This leaf serves the outbound JWKS HTTPS client. The inbound media
    # `ca.crt` trust remains untouched; only the constrained JWKS bundle applies.
    tls_validator(tls_values["tls.crt"], tls_values["tls.key"], ca_values["jwks-ca.crt"])

    database_keys = {"GAME_INTEGRATION_DB_PASSWORD", "GAME_INTEGRATION_DATABASE_URL"}
    present = database_keys & set(app_values)
    if present and present != database_keys:
        _blocked("gis_dev_database_secret_partial")
    signer_present = signer is not None
    workload_present = workload is not None
    if signer_present != workload_present:
        _blocked("gis_dev_identity_secrets_partial")
    generated = material_factory() if not present or not (signer_present and workload_present) else {}
    if not isinstance(generated, dict):
        _blocked("gis_dev_material_invalid")

    operations = []
    if not present:
        password = generated.get("GAME_INTEGRATION_DB_PASSWORD")
        url = generated.get("GAME_INTEGRATION_DATABASE_URL")
        expected_url = (f"postgres://{DB_USER}:{password.decode()}@voice-postgres:5432/{DB_NAME}?sslmode=disable".encode()
                        if isinstance(password, bytes) else None)
        if not password or url != expected_url:
            _blocked("gis_dev_database_material_invalid")
        merged = dict(app_values)
        merged.update({key: generated[key] for key in database_keys})
        body = copy.deepcopy(app)
        if operation is not None:
            body.setdefault("metadata", {}).setdefault("annotations", {})[OPERATION_ANNOTATION] = operation
        body["data"] = {key: base64.b64encode(value).decode("ascii") for key, value in merged.items()}
        operations.append({"action": "replace", "name": APP_SECRET, "body": body,
            "expected_uid": app["metadata"]["uid"],
            "expected_resource_version": app["metadata"]["resourceVersion"]})
    else:
        if app_values["GAME_INTEGRATION_DATABASE_URL"] != (
            f"postgres://{DB_USER}:{app_values['GAME_INTEGRATION_DB_PASSWORD'].decode()}@voice-postgres:5432/{DB_NAME}?sslmode=disable".encode()
        ):
            _blocked("gis_dev_database_secret_binding_invalid")

    if signer_present:
        for secret in (signer, workload):
            owner = secret.get("metadata", {}).get("annotations", {}).get(OPERATION_ANNOTATION)
            if operation is not None and owner is not None and owner != operation:
                _blocked("gis_dev_secret_operation_changed")
        _verify_existing(SIGNER_SECRET, signer, ("current.pem", "next.pem", "active-kid"),
            signer_validator)
        _verify_existing(WORKLOAD_SECRET, workload,
            ("GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64", "GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64"),
            _validate_workload_values)
    else:
        signer_values = generated.get(SIGNER_SECRET)
        workload_values = generated.get(WORKLOAD_SECRET)
        if not isinstance(signer_values, dict) or set(signer_values) != {"current.pem", "next.pem", "active-kid"}:
            _blocked("gis_dev_signer_material_invalid")
        if signer_values["active-kid"] not in (b"current", b"next", "current", "next"):
            _blocked("gis_dev_signer_material_invalid")
        signer_validator(signer_values)
        _validate_workload_values(workload_values)
        operations.extend([
            {"action": "create", "name": SIGNER_SECRET,
                "body": _secret_body(SIGNER_SECRET, signer_values, operation=operation)},
            {"action": "create", "name": WORKLOAD_SECRET,
                "body": _secret_body(WORKLOAD_SECRET, workload_values, operation=operation)},
        ])
    return {"namespace_uid": metadata["uid"], "operation": operation, "operations": operations,
            "secret_names": [row["name"] for row in operations]}


def _validate_workload_values(values):
    keys = ("GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64", "GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64")
    if not isinstance(values, dict) or not set(keys) <= set(values):
        _blocked("gis_dev_workload_material_invalid")
    try:
        decoded = [base64.b64decode(values[key], validate=True) for key in keys]
    except (ValueError, TypeError):
        _blocked("gis_dev_workload_material_invalid")
    if any(len(value) != 32 for value in decoded) or decoded[0] == decoded[1]:
        _blocked("gis_dev_workload_material_invalid")


def apply_secret_plan(kube, plan):
    """Apply only preflighted DEV Secret creates/CAS updates; never retry conflicts."""
    if (not isinstance(plan, dict) or plan.get("namespace_uid") is None
        or not isinstance(plan.get("operations"), list)):
        _blocked("gis_dev_secret_plan_invalid")
    operation = plan.get("operation")
    if operation is not None and not re.fullmatch(r"[a-f0-9]{12}", operation):
        _blocked("gis_dev_secret_plan_invalid")
    try:
        namespace = kube.get_namespace()
        if namespace.get("metadata", {}).get("name") != NAMESPACE or namespace.get("metadata", {}).get("uid") != plan["namespace_uid"]:
            _blocked("gis_dev_namespace_changed")
        for operation in plan["operations"]:
            if operation.get("action") == "replace" and operation.get("name") == APP_SECRET:
                if (plan.get("operation") is not None
                    and operation.get("body", {}).get("metadata", {}).get("annotations", {}).get(OPERATION_ANNOTATION) != plan["operation"]):
                    _blocked("gis_dev_secret_plan_invalid")
                kube.replace_secret(operation["body"], expected_uid=operation["expected_uid"],
                    expected_resource_version=operation["expected_resource_version"])
            elif operation.get("action") == "create" and operation.get("name") in (SIGNER_SECRET, WORKLOAD_SECRET):
                if (plan.get("operation") is not None
                    and operation.get("body", {}).get("metadata", {}).get("annotations", {}).get(OPERATION_ANNOTATION) != plan["operation"]):
                    _blocked("gis_dev_secret_plan_invalid")
                kube.create_secret(operation["body"])
            else:
                _blocked("gis_dev_secret_plan_invalid")
    except Blocked:
        raise
    except Exception:
        _blocked("gis_dev_secret_write_conflict")
    return True


def secret_write_receipt(kube, plan, operation):
    """Capture metadata only for this operation's Secret writes."""
    result = {}
    for row in plan.get("operations", []):
        secret = kube.get_secret(row["name"])
        metadata = secret.get("metadata", {}) if isinstance(secret, dict) else {}
        if (metadata.get("annotations", {}).get(OPERATION_ANNOTATION) != operation
            or not isinstance(metadata.get("uid"), str)
            or not isinstance(metadata.get("resourceVersion"), str)):
            _blocked("gis_dev_secret_write_receipt_invalid")
        result[row["name"]] = {"uid": metadata["uid"],
            "resourceVersion": metadata["resourceVersion"]}
    return result


def validate_secret_write_receipt(kube, receipt, operation):
    if not isinstance(receipt, dict) or set(receipt) - {APP_SECRET, SIGNER_SECRET, WORKLOAD_SECRET}:
        _blocked("gis_dev_secret_write_receipt_invalid")
    for name, expected in receipt.items():
        live = kube.get_secret(name)
        metadata = live.get("metadata", {}) if isinstance(live, dict) else {}
        if (metadata.get("uid") != expected.get("uid")
            or str(metadata.get("resourceVersion")) != str(expected.get("resourceVersion"))
            or metadata.get("annotations", {}).get(OPERATION_ANNOTATION) != operation):
            _blocked("gis_dev_secret_changed")
    return True


def receipt_binding(source_sha, image_ref, build_receipt, namespace_uid):
    """Return a non-secret binding for the exact dev install inputs."""
    validate_build_receipt(source_sha, image_ref, build_receipt, clean=True)
    if not isinstance(namespace_uid, str) or not namespace_uid:
        _blocked("gis_dev_namespace_identity_invalid")
    return {"schema": "voice-dev-gis-source-binding-v1", "namespace": NAMESPACE,
        "namespace_uid": namespace_uid, "source_sha": source_sha, "image_ref": image_ref,
        "build_receipt_sha256": _digest(build_receipt)}


class KubeError(RuntimeError):
    def __init__(self, status):
        super().__init__("kubernetes_api_error")
        self.status = status


class Kubectl:
    """Small adapter that pins every namespaced request to voice-dev."""
    def __init__(self, executable="kubectl", *, runner=subprocess.run):
        self.executable = executable
        self.runner = runner

    def run(self, args, body=None, timeout=30):
        if (not isinstance(args, list) or not args or any(not isinstance(x, str) for x in args)
            or args[0] not in {"get", "create", "replace", "patch", "delete", "logs", "exec"}):
            _blocked("gis_dev_kubectl_request_invalid")
        argv = [self.executable, "--namespace", NAMESPACE, *args]
        if body is not None:
            argv = ["-" if arg == "/dev/stdin" else arg for arg in argv]
            payload = json.dumps(body, sort_keys=True, separators=(",", ":"))
        else:
            payload = None
        try:
            result = self.runner(argv, input=payload, check=False, capture_output=True,
                text=True, timeout=timeout)
        except Exception:
            _blocked("gis_dev_kubectl_unavailable")
        if result.returncode != 0:
            # Kubernetes errors can contain object contents; retain only status.
            code_match = re.search(r"\b(403|404|409|422)\b", result.stderr or "")
            status = int(code_match.group(1)) if code_match else 0
            raise KubeError(status)
        output = result.stdout
        # The prerequisite consumer relies on booleans from this one exact
        # kubectl go-template probe. Keep every other raw command output typed
        # as text; in particular, never coerce UID/resourceVersion values.
        probe = re.fullmatch(
            r'go-template=\{\{if index \.data "(POSTGRES_PASSWORD|GAME_INTEGRATION_DB_PASSWORD|GAME_INTEGRATION_DATABASE_URL)"\}\}true\{\{else\}\}false\{\{end\}\}',
            args[4] if len(args) == 5 else '')
        if (len(args) == 5 and args[:4] == ["get", "secret", APP_SECRET, "-o"]
            and probe is not None):
            if output == "true":
                return True
            if output == "false":
                return False
            _blocked("gis_dev_kubectl_probe_response_invalid")
        if "-o" in args and "json" in args:
            if "--ignore-not-found=true" in args and output == "":
                return ""
            try:
                return json.loads(output)
            except Exception:
                _blocked("gis_dev_kubectl_response_invalid")
        return output

    def get_namespace(self):
        return self.run(["get", "namespace", NAMESPACE, "-o", "json"])

    def get_secret(self, name):
        # kubectl's supported --ignore-not-found contract returns success and
        # empty output only when this named object is absent. API authorization,
        # transport, and other failures remain non-zero and fail closed.
        output = self.run(["get", "secret", name, "--ignore-not-found=true", "-o", "json"])
        if output == "":
            return None
        if not isinstance(output, dict):
            _blocked("gis_dev_kubectl_response_invalid")
        return output

    def get(self, kind, name):
        return self.run(["get", kind.lower(), name, "-o", "json"])

    def secret_meta(self, name):
        secret = self.get_secret(name)
        metadata = secret.get("metadata", {}) if isinstance(secret, dict) else {}
        return {"uid": metadata.get("uid"), "resourceVersion": metadata.get("resourceVersion")}

    def create_secret(self, body):
        return self.run(["create", "-f", "/dev/stdin", "-o", "json"], body=body)

    def replace_secret(self, body, *, expected_uid, expected_resource_version):
        current = self.get_secret(body["metadata"]["name"])
        metadata = current.get("metadata", {}) if isinstance(current, dict) else {}
        if (metadata.get("uid") != expected_uid
            or str(metadata.get("resourceVersion")) != str(expected_resource_version)):
            _blocked("gis_dev_secret_write_conflict")
        body = copy.deepcopy(body)
        body["metadata"]["uid"] = expected_uid
        body["metadata"]["resourceVersion"] = str(expected_resource_version)
        return self.run(["replace", "-f", "/dev/stdin", "-o", "json"], body=body)


def _capture_prerequisite_sources(root, prerequisite):
    paths = {
        "scripts/dev/migrations.py",
        "docker/postgres/provision-gameintegration-runtime-role.sql",
        "docker/postgres/verify-gameintegration-runtime-role.sql",
        "deploy/templates/provision-gameintegration-runtime-role-job.yaml",
        "deploy/templates/migrate-game-integration-db-job.yaml",
        "deploy/templates/verify-game-integration-db-job.yaml",
    }
    inputs = prerequisite._load_producer(root).gameintegration_prerequisite_inputs(root)
    paths.update("src/backend/migrations/game_integration_db/" + row["name"]
                 for row in inputs["migrations"])
    return {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in paths}


def _source_pair(root, image_ref):
    try:
        import yaml
        import gis_first_install as gis
        documents = list(yaml.safe_load_all((root / "deploy/staging/services.yaml").read_text(encoding="utf-8")))
    except Exception:
        _blocked("gis_dev_source_manifest_invalid")
    wanted = {gis.SERVICE, gis.DEPLOYMENT}
    rows = [row for row in documents if isinstance(row, dict)
        and (row.get("kind"), row.get("metadata", {}).get("name")) in wanted]
    if len(rows) != 2 or {(row["kind"], row["metadata"]["name"]) for row in rows} != wanted:
        _blocked("gis_dev_source_pair_invalid")
    for row in rows:
        row.setdefault("metadata", {})["namespace"] = NAMESPACE
    deployment = next(row for row in rows if row["kind"] == "Deployment")
    containers = deployment.get("spec", {}).get("template", {}).get("spec", {}).get("containers", [])
    images = [row for row in containers if row.get("name") == "gameintegration"]
    if len(images) != 1:
        _blocked("gis_dev_source_image_invalid")
    images[0]["image"] = image_ref
    return {"schema": gis.SOURCE_SCHEMA, "objects": rows}


def _database_exists_readonly(kube, expected_identity):
    """Check the fixed target DB through a ready PostgreSQL pod without writes."""
    import gis_prerequisite
    identity = gis_prerequisite._endpoint_identity(kube, NAMESPACE)
    if identity != expected_identity:
        _blocked("gis_dev_database_identity_changed")
    response = kube.run(["get", "endpointslices", "-l",
        "kubernetes.io/service-name=voice-postgres", "-o", "json"])
    pods = []
    for row in response.get("items", []):
        for endpoint in row.get("endpoints", []):
            target = endpoint.get("targetRef", {})
            if endpoint.get("conditions", {}).get("ready") is True and target.get("kind") == "Pod":
                pods.append(target.get("name"))
    if not pods or any(not isinstance(name, str) or not re.fullmatch(r"[a-z0-9-]{1,63}", name) for name in pods):
        _blocked("gis_dev_database_endpoint_invalid")
    query = "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'game_integration_db');"
    command = ["exec", pods[0], "-c", "postgres", "--", "sh", "-c",
        'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" '
        '-d postgres -Atqc "' + query + '"']
    result = kube.run(command, timeout=30)
    if result.strip() not in {"t", "true"}:
        _blocked("gis_dev_database_missing")
    if gis_prerequisite._endpoint_identity(kube, NAMESPACE) != expected_identity:
        _blocked("gis_dev_database_identity_changed")
    return identity


def _save_plan(state_path, state):
    path = Path(state_path)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(path.parent, 0o700)
    temporary = path.with_name(path.name + ".tmp")
    raw = json.dumps(state, sort_keys=True, separators=(",", ":")).encode()
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "wb") as stream:
        stream.write(raw); stream.flush(); os.fsync(stream.fileno())
    os.replace(temporary, path)
    os.chmod(path, 0o600)


_PARTIAL_OPERATION = "737e63e3aaee"
_PARTIAL_SOURCE_SHA = "860b995dd883ade2b79996f9fcbe34302b3c7bac"
_PARTIAL_JOURNAL_SHA256 = "659d0379811afa39c22d056b0b3cf9787a3d50d5dbef2f275bc9524ddd19cac1"
_PARTIAL_CHANGED_PATHS = {
    "scripts/dev/gis_dev_first_install.py",
    "scripts/dev/gis_first_install_test.py",
    "scripts/staging/nats-rollout-preservation/gis_prerequisite.py",
    "scripts/staging/nats-rollout-preservation/gis_prerequisite_test.py",
}


def _git_text(root, args, *, runner=subprocess.run):
    try:
        return runner(["git", "-C", str(root), *args], check=True,
            capture_output=True, text=True, timeout=15).stdout.strip()
    except Exception:
        _blocked("gis_dev_partial_source_binding_invalid")


def _partial_source_binding(root, state, original_sha, *, runner=subprocess.run):
    """Bind this one recovery to an allowlisted clean operator/test source delta."""
    candidate = _git_text(root, ["rev-parse", "HEAD"], runner=runner)
    status = _git_text(root, ["status", "--porcelain=v1", "--untracked-files=all"], runner=runner)
    ancestor = _git_text(root, ["merge-base", _PARTIAL_SOURCE_SHA, candidate], runner=runner)
    if (status or not SOURCE.fullmatch(candidate) or original_sha != _PARTIAL_JOURNAL_SHA256
        or ancestor != _PARTIAL_SOURCE_SHA):
        _blocked("gis_dev_partial_source_binding_invalid")
    changed = _git_text(root, ["diff", "--name-only", "--no-renames", _PARTIAL_SOURCE_SHA, candidate], runner=runner)
    paths = set(changed.splitlines())
    if paths != _PARTIAL_CHANGED_PATHS:
        _blocked("gis_dev_partial_source_paths_invalid")
    files = {name: hashlib.sha256((Path(root) / name).read_bytes()).hexdigest()
        for name in sorted(paths)}
    backend_tree, _ = _source_backend_context(root, runner)
    receipt = state.get("source_build_receipt", {})
    if (state.get("source_sha") != _PARTIAL_SOURCE_SHA
        or receipt.get("source_sha") != _PARTIAL_SOURCE_SHA
        or receipt.get("builder_tree") != backend_tree
        or state.get("plan", {}).get("source_sha") != _PARTIAL_SOURCE_SHA):
        _blocked("gis_dev_partial_source_binding_invalid")
    return {"old_source_sha": _PARTIAL_SOURCE_SHA, "candidate_source_sha": candidate,
        "changed_files": files, "backend_tree": backend_tree,
        "image_ref": receipt.get("image_ref"),
        "build_receipt_sha256": _digest(receipt),
        "source_plan_sha256": state.get("plan", {}).get("source_plan_sha256"),
        "source_capture_sha256": _digest(state.get("plan", {}).get("source_capture")),
        "inputs_sha256": state.get("plan", {}).get("inputs_sha256")}


def _partial_state_admissible(state):
    plan = state.get("plan") if isinstance(state, dict) else None
    receipt = state.get("secret_write_receipt") if isinstance(state, dict) else None
    if (not isinstance(plan, dict) or state.get("schema") != "voice-dev-gis-first-install-state-v1"
        or state.get("operation") != _PARTIAL_OPERATION
        or state.get("source_sha") != _PARTIAL_SOURCE_SHA
        or state.get("phase") != "SECRETS_APPLIED" or state.get("secrets_applied") is not True
        or state.get("plan_phase") not in (None, "PLANNED") or plan.get("phase") != "PLANNED"
        or plan.get("prerequisite_phase") is not None or plan.get("prerequisite_receipt") is not None
        or plan.get("created") not in (None, {}) or plan.get("jobs") not in (None, {})
        or plan.get("source_sha") != _PARTIAL_SOURCE_SHA
        or not isinstance(receipt, dict) or set(receipt) != {APP_SECRET, SIGNER_SECRET, WORKLOAD_SECRET}
        or not isinstance(plan.get("inputs"), dict)
        or plan.get("inputs_sha256") != _digest(plan.get("inputs"))
        or not isinstance(plan.get("source_capture"), dict)
        or not isinstance(plan.get("source_plan_sha256"), str)
        or not HASH.fullmatch(plan["source_plan_sha256"])
        or plan.get("namespace_uid") != state.get("namespace_uid")
        or not isinstance(state.get("database_identity"), dict)):
        _blocked("gis_dev_partial_state_not_resumable")
    for expected in receipt.values():
        if (not isinstance(expected, dict) or set(expected) != {"uid", "resourceVersion"}
            or not isinstance(expected["uid"], str)
            or not re.fullmatch(r"[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}", expected["uid"])
            or not isinstance(expected["resourceVersion"], str)
            or not re.fullmatch(r"[1-9][0-9]{0,31}", expected["resourceVersion"])):
            _blocked("gis_dev_partial_state_not_resumable")
    return True


def _partial_state_compatible(state, original):
    """Ensure sidecar progress preserves the frozen authority fields."""
    if not isinstance(state, dict) or not isinstance(original, dict):
        _blocked("gis_dev_partial_sidecar_state_invalid")
    for key in ("schema", "operation", "namespace_uid", "source_sha", "source_build_receipt",
                "database_identity", "secret_write_receipt", "secrets_applied"):
        if state.get(key) != original.get(key):
            _blocked("gis_dev_partial_sidecar_state_invalid")
    plan = state.get("plan")
    baseline = original.get("plan", {})
    for key in ("operation", "namespace_uid", "source_sha", "source_plan_sha256",
                "source_build_receipt", "source_capture", "inputs", "inputs_sha256"):
        if plan.get(key) != baseline.get(key):
            _blocked("gis_dev_partial_sidecar_plan_invalid")
    if (plan.get("phase") not in ("PLANNED", "CREATING", "STARTING", "CREATED", "STARTED")
        or plan.get("prerequisite_phase") not in ("CREATING", "VERIFIED")
        or not isinstance(plan.get("created"), dict) or not isinstance(plan.get("jobs"), dict)):
        _blocked("gis_dev_partial_sidecar_plan_invalid")
    allowed_created = {"Service/voice-gameintegration", "Deployment/voice-gameintegration"}
    allowed_created.update("configmap/" + value for value in baseline.get("inputs", {}).get("configmaps", {}).values())
    allowed_created.update("job/" + value for value in baseline.get("inputs", {}).get("jobs", {}).values())
    if (set(plan["created"]) - allowed_created
        or set(plan["jobs"]) - {"provision", "migrate", "verify"}
        or (plan.get("prerequisite_phase") == "CREATING" and plan.get("prerequisite_receipt") is not None)
        or (plan.get("prerequisite_phase") == "VERIFIED" and not isinstance(plan.get("prerequisite_receipt"), dict))):
        _blocked("gis_dev_partial_sidecar_plan_invalid")
    return True


def _save_continuation(path, document, expected_sha256=None):
    """Atomically create or compare-and-replace the private continuation sidecar."""
    path = Path(path)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(path.parent, 0o700)
    if path.exists():
        previous = path.read_bytes()
        if expected_sha256 is None or hashlib.sha256(previous).hexdigest() != expected_sha256:
            _blocked("gis_dev_partial_sidecar_changed")
    elif expected_sha256 is not None:
        _blocked("gis_dev_partial_sidecar_missing")
    raw = json.dumps(document, sort_keys=True, separators=(",", ":")).encode()
    temporary = path.with_name(path.name + "." + secrets.token_hex(8) + ".tmp")
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(raw); stream.flush(); os.fsync(stream.fileno())
        if expected_sha256 is None:
            os.link(temporary, path)
            temporary.unlink()
        else:
            os.replace(temporary, path)
        os.chmod(path, 0o600)
    except Exception:
        try:
            temporary.unlink()
        except OSError:
            pass
        _blocked("gis_dev_partial_sidecar_write_failed")
    return hashlib.sha256(raw).hexdigest()


def _partial_secret_fingerprints(kube, state):
    """Validate same-operation Secret custody and return hashes only."""
    receipt = state["secret_write_receipt"]
    expected_types = {APP_SECRET: "Opaque", SIGNER_SECRET: "Opaque", WORKLOAD_SECRET: "Opaque"}
    result = {}
    for name in sorted(receipt):
        secret = kube.get_secret(name)
        metadata = secret.get("metadata", {}) if isinstance(secret, dict) else {}
        data = secret.get("data") if isinstance(secret, dict) else None
        if (not isinstance(data, dict) or not data
            or metadata.get("namespace") != NAMESPACE or metadata.get("name") != name
            or metadata.get("uid") != receipt[name].get("uid")
            or metadata.get("resourceVersion") != receipt[name].get("resourceVersion")
            or secret.get("type", "Opaque") != expected_types[name]
            or metadata.get("annotations", {}).get(OPERATION_ANNOTATION) != _PARTIAL_OPERATION):
            _blocked("gis_dev_partial_secret_binding_changed")
        fingerprint = hashlib.sha256(json.dumps(data, sort_keys=True,
            separators=(",", ":")).encode()).hexdigest()
        result[name] = {"uid": metadata["uid"], "resourceVersion": metadata["resourceVersion"],
            "type": expected_types[name], "data_sha256": fingerprint}
    return result


def install(source_root, image_ref, state_path, *, kube=None, apply=False, operation=None,
            builder_run=38984047252, builder_job=117252300427,
            local_build=False, local_build_receipt=None, build_receipt_factory=source_build_receipt,
            secret_plan_factory=build_secret_plan, source_verifier=verify_current_source,
            resume_partial=False):
    """Run the fixed DEV-only install through source, DB, Secret, and GIS guards."""
    if not apply:
        _blocked("gis_dev_apply_flag_required")
    import gis_first_install as gis
    import gis_prerequisite
    root = Path(source_root).resolve(strict=True)
    state_file = Path(state_path)
    original_state_raw = None
    original_state = None
    candidate_source_sha = None
    if resume_partial:
        try:
            original_state_raw = state_file.read_bytes()
            if hashlib.sha256(original_state_raw).hexdigest() != _PARTIAL_JOURNAL_SHA256:
                _blocked("gis_dev_partial_journal_changed")
            original_state = json.loads(original_state_raw)
        except Blocked:
            raise
        except Exception:
            _blocked("gis_dev_partial_journal_invalid")
        _partial_state_admissible(original_state)
        candidate_source_sha = _git_text(root, ["rev-parse", "HEAD"])
        source_build = original_state["source_build_receipt"]
        validate_build_receipt(_PARTIAL_SOURCE_SHA, image_ref, source_build, clean=True)
        if source_build.get("image_ref") != image_ref:
            _blocked("gis_dev_partial_image_changed")
        source_sha, build_receipt = _PARTIAL_SOURCE_SHA, copy.deepcopy(source_build)
        source_verifier(root, candidate_source_sha, source_build["builder_tree"])
    else:
        # A receipt passed by the CLI or a test seam is only a locator. Reproduce
        # authority from the producer and registry before it can enter the plan.
        if local_build_receipt is not None:
            local_kind = (isinstance(local_build_receipt, dict)
                and local_build_receipt.get("builder_kind") == "local-build")
            if not isinstance(local_build_receipt, dict) or local_build_receipt.get("image_ref") != image_ref:
                _blocked("gis_dev_source_build_receipt_invalid")
            source_sha, build_receipt = build_receipt_factory(root, image_ref,
                local_build=local_kind)
            if build_receipt != local_build_receipt:
                _blocked("gis_dev_source_build_receipt_changed")
        else:
            source_sha, build_receipt = build_receipt_factory(root, image_ref,
                builder_run=builder_run, builder_job=builder_job, local_build=local_build)
        validate_build_receipt(source_sha, image_ref, build_receipt, clean=True)
        source_verifier(root, source_sha, build_receipt["builder_tree"])
    operator_source_sha = candidate_source_sha or source_sha
    kube = kube or Kubectl()
    namespace = kube.get_namespace().get("metadata", {})
    if namespace.get("name") != NAMESPACE or not isinstance(namespace.get("uid"), str):
        _blocked("gis_dev_namespace_identity_invalid")

    prior = None
    continuation_path = Path(str(state_path) + ".continuation.json")
    continuation = None
    continuation_sha256 = None
    if resume_partial:
        if namespace["uid"] != original_state.get("namespace_uid"):
            _blocked("gis_dev_partial_namespace_changed")
        binding = _partial_source_binding(root, original_state, _PARTIAL_JOURNAL_SHA256)
        if continuation_path.exists():
            try:
                raw = continuation_path.read_bytes()
                continuation = json.loads(raw)
            except Exception:
                _blocked("gis_dev_partial_sidecar_invalid")
            continuation_sha256 = hashlib.sha256(raw).hexdigest()
            if (not isinstance(continuation, dict)
                or set(continuation) != {"schema", "original_journal_sha256", "binding", "secret_fingerprints", "state"}
                or continuation.get("schema") != "voice-dev-gis-partial-continuation-v1"
                or continuation.get("original_journal_sha256") != _PARTIAL_JOURNAL_SHA256
                or continuation.get("binding") != binding):
                _blocked("gis_dev_partial_sidecar_binding_changed")
            prior = continuation.get("state")
            _partial_state_compatible(prior, original_state)
            current_fingerprints = _partial_secret_fingerprints(kube, original_state)
            if continuation.get("secret_fingerprints") != current_fingerprints:
                _blocked("gis_dev_partial_secret_binding_changed")
        else:
            _partial_state_admissible(original_state)
            validate_secret_write_receipt(kube, original_state["secret_write_receipt"], _PARTIAL_OPERATION)
            fingerprints = _partial_secret_fingerprints(kube, original_state)
            continuation = {"schema": "voice-dev-gis-partial-continuation-v1",
                "original_journal_sha256": _PARTIAL_JOURNAL_SHA256,
                "binding": binding, "secret_fingerprints": fingerprints,
                "state": copy.deepcopy(original_state)}
            continuation_sha256 = _save_continuation(continuation_path, continuation)
            prior = original_state
        if (not isinstance(prior, dict) or prior.get("operation") != _PARTIAL_OPERATION
            or prior.get("namespace_uid") != namespace["uid"]):
            _blocked("gis_dev_partial_sidecar_state_invalid")
        operation = _PARTIAL_OPERATION
    elif state_file.exists():
        try:
            prior = json.loads(state_file.read_text(encoding="utf-8"))
        except Exception:
            _blocked("gis_dev_state_invalid")
        if (not isinstance(prior, dict) or prior.get("schema") != "voice-dev-gis-first-install-state-v1"
            or prior.get("namespace_uid") != namespace["uid"]
            or prior.get("source_sha") != source_sha
            or prior.get("source_build_receipt") != build_receipt
            or prior.get("plan", {}).get("source_sha") != source_sha):
            _blocked("gis_dev_state_binding_changed")
        operation = prior.get("operation")
        if not isinstance(operation, str) or not re.fullmatch(r"[a-f0-9]{12}", operation):
            _blocked("gis_dev_state_invalid")
    else:
        operation = operation or secrets.token_hex(6)
    if not re.fullmatch(r"[a-f0-9]{12}", operation):
        _blocked("gis_dev_operation_invalid")

    source_plan = _source_pair(root, image_ref)
    target = {"mode": "app-only", "tag": source_sha,
        "source_build_receipt": build_receipt, "source_hashes": {
            "deploy/staging/services.yaml": hashlib.sha256(
                (root / "deploy/staging/services.yaml").read_bytes()).hexdigest()}}
    if resume_partial:
        state = prior
        plan = state["plan"]
        code_capture = plan.get("source_capture")
        if (plan.get("operation") != operation or plan.get("namespace_uid") != namespace["uid"]
            or plan.get("inputs") != gis_prerequisite.source_inputs(root, code_capture)
            or gis.digest(source_plan) != plan.get("source_plan_sha256")):
            _blocked("gis_dev_state_source_changed")
    elif prior is None:
        plan = gis.bind_operation(source_plan, operation, target, namespace=NAMESPACE)
        plan["source_build_receipt"] = copy.deepcopy(build_receipt)
        plan["source_root"] = str(root)
        code_capture = _capture_prerequisite_sources(root, gis_prerequisite)
        plan["source_capture"] = code_capture
        plan["inputs"] = gis_prerequisite.source_inputs(root, code_capture)
        plan["inputs_sha256"] = gis.digest(plan["inputs"])
        plan["namespace_uid"] = namespace["uid"]
        state = {"schema": "voice-dev-gis-first-install-state-v1", "operation": operation,
            "namespace_uid": namespace["uid"], "source_sha": source_sha,
            "source_build_receipt": build_receipt, "plan": plan, "phase": "SOURCE_VERIFIED",
            "secrets_applied": False}
        _save_plan(state_path, state)
    else:
        state = prior
        plan = state["plan"]
        code_capture = plan.get("source_capture")
        if (plan.get("operation") != operation or plan.get("namespace_uid") != namespace["uid"]
            or plan.get("inputs") != gis_prerequisite.source_inputs(root, code_capture)
            or gis.digest(source_plan) != plan.get("source_plan_sha256")):
            _blocked("gis_dev_state_source_changed")

    def persist_state():
        nonlocal continuation_sha256
        if not resume_partial:
            _save_plan(state_path, state)
            return
        continuation["state"] = copy.deepcopy(state)
        continuation_sha256 = _save_continuation(continuation_path, continuation,
            expected_sha256=continuation_sha256)

    # This checks the existing stable TLS Secret before generating any key or
    # planning the additive application Secret updates.
    if not state.get("secrets_applied"):
        secret_plan = secret_plan_factory(kube, operation=operation)
        state["phase"] = "TLS_VERIFIED"
        persist_state()
        before = gis_prerequisite._endpoint_identity(kube, NAMESPACE)
        _database_exists_readonly(kube, before)
        state["database_identity"] = before
        state["phase"] = "DATABASE_VERIFIED"
        persist_state()
        if before["namespace_uid"] != namespace["uid"]:
            _blocked("gis_dev_namespace_identity_changed")
        source_verifier(root, operator_source_sha, build_receipt["builder_tree"])
        apply_secret_plan(kube, secret_plan)
        state["secret_write_receipt"] = secret_write_receipt(kube, secret_plan, operation)
        state["secrets_applied"] = True
        state["phase"] = "SECRETS_APPLIED"
        persist_state()
        plan["database_identity"] = copy.deepcopy(before)
        if plan.get("phase") not in ("PLANNED", "CREATING", "STARTING"):
            _blocked("gis_dev_resume_phase_invalid")
    else:
        validate_secret_write_receipt(kube, state.get("secret_write_receipt", {}), operation)
        if resume_partial:
            if continuation.get("secret_fingerprints") != _partial_secret_fingerprints(kube, original_state):
                _blocked("gis_dev_partial_secret_binding_changed")
        before = state.get("database_identity")
        if not isinstance(before, dict):
            _blocked("gis_dev_database_identity_missing")
        if _database_exists_readonly(kube, before) != before:
            _blocked("gis_dev_database_identity_changed")

    def save_plan(value):
        state["plan"] = copy.deepcopy(value)
        state["plan_phase"] = value.get("phase")
        persist_state()

    try:
        if not plan.get("prerequisite_receipt"):
            source_verifier(root, operator_source_sha, build_receipt["builder_tree"])
            receipt = gis_prerequisite.execute(kube, plan, root, code_capture, save_plan)
            plan["prerequisite_receipt"] = receipt
            save_plan(plan)
        if plan.get("phase") not in ("CREATED", "STARTING", "STARTED"):
            gis.execute(kube, plan, save_plan, start=False)
        if plan.get("phase") != "STARTED":
            gis.start_owned(kube, plan, save_plan)
        ready = gis.verify_ready(kube, plan)
        state["phase"] = "READY"
        state["ready_receipt"] = ready
        persist_state()
        return ready
    except Exception:
        if resume_partial:
            # The recovery sidecar is the durable same-operation journal. Keep
            # prerequisite ConfigMaps/Jobs and their phase/identity records for
            # exact reread; never run GIS Service/Deployment rollback here.
            persist_state()
        elif plan.get("created"):
            try:
                gis.rollback(kube, plan, save_plan)
            except Exception:
                state["phase"] = "ROLLBACK_REQUIRED"
                persist_state()
        raise


def main(argv=None):
    import argparse
    parser = argparse.ArgumentParser(description="Install the fixed GIS workload in voice-dev")
    parser.add_argument("--source-root", default=str(Path(__file__).resolve().parents[2]))
    parser.add_argument("--image-ref", help="immutable Game Integration OCI digest")
    parser.add_argument("--image-tag", help="publish a clean source build as dev-<12 hex chars>")
    parser.add_argument("--build-and-publish", action="store_true",
        help="build and publish the fixed GIS image, then print its immutable digest")
    parser.add_argument("--state", help="private, persistent operation journal path")
    parser.add_argument("--builder-run", type=int, default=38984047252,
        help="successful master-push CI run that published this image (default: recorded M12 run)")
    parser.add_argument("--builder-job", type=int, default=117252300427,
        help="successful GameIntegration image-push job in that run")
    parser.add_argument("--local-build", action="store_true",
        help="admit only an image validated against the exact clean local source context")
    parser.add_argument("--apply", action="store_true", help="run guarded DEV Secret, migration, and workload steps")
    parser.add_argument("--resume-partial-operation", action="store_true",
        help="continue only the frozen GIS operation 737e63e3aaee through its reviewed sidecar")
    args = parser.parse_args(argv)
    try:
        if args.build_and_publish:
            if not args.image_tag or args.image_ref or not args.apply or not args.state or args.resume_partial_operation:
                _blocked("gis_dev_build_arguments_invalid")
            prior = None
            if Path(args.state).exists():
                try:
                    previous_state = json.loads(Path(args.state).read_text(encoding="utf-8"))
                    previous_receipt = previous_state.get("source_build_receipt", {})
                    if (not isinstance(previous_state, dict)
                        or previous_state.get("schema") != "voice-dev-gis-first-install-state-v1"
                        or not isinstance(previous_receipt, dict)
                        or previous_receipt.get("builder_kind") != "local-build"
                        or previous_receipt.get("image_ref", "").split("@", 1)[0] != IMAGE):
                        _blocked("gis_dev_state_invalid")
                    prior = source_build_receipt(args.source_root,
                        previous_receipt["image_ref"], local_build=True)
                    if prior[1] != previous_receipt:
                        _blocked("gis_dev_state_source_build_changed")
                except Exception:
                    _blocked("gis_dev_state_invalid")
            if prior is None:
                image_ref, build_receipt = build_and_publish_image(args.source_root, args.image_tag)
            else:
                image_ref, build_receipt = prior[1]["image_ref"], prior[1]
            print("GIS_DEV_IMAGE_REF=" + image_ref)
            ready = install(args.source_root, image_ref, args.state, apply=True,
                local_build_receipt=build_receipt)
            print("GIS_DEV_FIRST_INSTALL=READY " + ready["deployment_uid"])
            return 0
        if not args.image_ref or not args.state:
            _blocked("gis_dev_install_arguments_invalid")
        ready = install(args.source_root, args.image_ref, args.state, apply=args.apply,
            builder_run=args.builder_run, builder_job=args.builder_job, local_build=args.local_build,
            resume_partial=args.resume_partial_operation)
        print("GIS_DEV_FIRST_INSTALL=READY " + ready["deployment_uid"])
        return 0
    except Blocked as error:
        print("GIS_DEV_FIRST_INSTALL=BLOCKED " + str(error))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
