"""Emit a read-only V1 source-authority receipt for a published Matchmaking image."""

import base64
import hashlib
import io
import json
import os
import re
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
from pathlib import Path

GIS_MODULES = str(Path(__file__).resolve().parents[1] / "staging" / "nats-rollout-preservation")
if GIS_MODULES not in sys.path:
    sys.path.insert(0, GIS_MODULES)

import source_authority as authority
import gis_dev_first_install as gis

REPO = "Poryadok/VoiceRoot"
IMAGE = authority.REGISTRY + "/matchmaking"
MAX_REGISTRY_BODY = 4 << 20
MAX_BUILDX_ARTIFACT = 32 << 20
MAX_JOB_LOG = 16 << 20
SOURCE_SHA = re.compile(r"[a-f0-9]{40}\Z")
DIGEST = re.compile(r"sha256:[a-f0-9]{64}\Z")
GHCR_BLOB_REDIRECT_HOST = "pkg-containers.githubusercontent.com"
REDIRECT_TARGET_CODES = frozenset((
    "redirect_target_control_invalid", "redirect_target_scheme_invalid",
    "redirect_target_host_invalid", "redirect_target_authority_invalid",
    "redirect_target_fragment_invalid", "redirect_target_path_invalid",
    "redirect_target_digest_invalid", "redirect_target_query_invalid"))
REDIRECT_DIAGNOSTIC_CODES = REDIRECT_TARGET_CODES | {"redirect_chain_rejected"}
REDIRECT_HOST_CLASSES = frozenset(("known_cdn", "other"))
_ACTIVE_REDIRECT_DIAGNOSTIC = None
_CURRENT_PHASE = "preflight"
ACCEPT = ", ".join((
    "application/vnd.oci.image.index.v1+json",
    "application/vnd.oci.image.manifest.v1+json",
    "application/vnd.docker.distribution.manifest.list.v2+json",
    "application/vnd.docker.distribution.manifest.v2+json",
))


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        return None


_HTTPS = urllib.request.build_opener(_NoRedirect)


class ReceiptError(RuntimeError):
    """A fixed, non-sensitive failure from the read-only receipt producer."""

    def __init__(self, message="matchmaking_source_authority_rejected", *, phase=None,
            code="validation_rejected", failure_class="ReceiptError", http_status=None,
            redirect_host_class=None):
        super().__init__(message)
        fixed_phase = phase in (
            "preflight", "run-metadata", "source-capture", "source-preflight", "repository-pin",
            "source-equivalence", "source-ci", "source-tree", "source-archive", "stack-lock-artifact",
            "stack-lock-read", "stack-lock-parse", "master-recheck", "build-context-validation",
            "publisher-repository", "publisher-job-metadata", "publisher-job-log",
            "publisher-artifact-metadata", "publisher-artifact-content", "publisher-buildx-validation",
            "receipt-binding", "receipt-write", "registry-token", "registry-index",
            "registry-platform-manifest", "registry-config", "registry-config-redirect-target",
            "registry-config-redirect-content", "registry-image")
        self.phase = phase if fixed_phase or phase in tuple(
            "registry-image-" + name for name in authority.IMAGE_NAMES) else "unknown"
        self.code = code if code in (
            "validation_rejected", "http_error", "http_status", "network_error",
            "transport_rejected", "response_schema_invalid", "archive_rejected",
            "source_capture_rejected", "publisher_evidence_rejected", "receipt_write_rejected",
            "internal_error", *REDIRECT_DIAGNOSTIC_CODES) else "internal_error"
        self.failure_class = failure_class if failure_class in (
            "ReceiptError", "SourceError", "HTTPError", "HTTPResponse", "URLError",
            "TimeoutError", "OSError", "KeyError", "TypeError", "ValueError",
            "BadZipFile", "Other") else "Other"
        self.http_status = http_status if type(http_status) is int and 100 <= http_status <= 599 else None
        self.redirect_host_class = (redirect_host_class
            if self.code in REDIRECT_DIAGNOSTIC_CODES and redirect_host_class in REDIRECT_HOST_CLASSES else None)

    def safe_record(self):
        record = {"status": "REJECTED", "phase": self.phase, "code": self.code,
            "class": self.failure_class}
        if self.http_status is not None:
            record["http_status"] = self.http_status
        if self.redirect_host_class is not None:
            record["redirect_host_class"] = self.redirect_host_class
        return record


def _reject():
    raise ReceiptError("matchmaking_source_authority_rejected", phase=_CURRENT_PHASE)


def _redirect_host_class(url):
    try:
        return "known_cdn" if isinstance(url, str) and urllib.parse.urlsplit(url).hostname == GHCR_BLOB_REDIRECT_HOST else "other"
    except (TypeError, ValueError):
        return "other"


def _redirect_rejected(code, url, status=307, *, phase="registry-config-redirect-target"):
    raise ReceiptError(phase=phase, code=code, failure_class="HTTPError", http_status=status,
        redirect_host_class=_redirect_host_class(url))


def _read_redirected_blob(url, digest, *, limit):
    if not isinstance(url, str) or any(ord(char) < 0x21 or ord(char) == 0x7f for char in url):
        _redirect_rejected("redirect_target_control_invalid", url)
    try:
        parsed = urllib.parse.urlsplit(url)
    except ValueError:
        _redirect_rejected("redirect_target_authority_invalid", url)
    if DIGEST.fullmatch(digest) is None:
        _redirect_rejected("redirect_target_digest_invalid", url)
    if parsed.scheme != "https":
        _redirect_rejected("redirect_target_scheme_invalid", url)
    if parsed.hostname != GHCR_BLOB_REDIRECT_HOST:
        _redirect_rejected("redirect_target_host_invalid", url)
    if parsed.username is not None or parsed.password is not None:
        _redirect_rejected("redirect_target_authority_invalid", url)
    try:
        if parsed.port not in (None, 443):
            _redirect_rejected("redirect_target_authority_invalid", url)
    except ValueError:
        _redirect_rejected("redirect_target_authority_invalid", url)
    if parsed.fragment:
        _redirect_rejected("redirect_target_fragment_invalid", url)
    if not parsed.path.startswith("/ghcr1/blobs/"):
        _redirect_rejected("redirect_target_path_invalid", url)
    if parsed.path != "/ghcr1/blobs/" + digest:
        _redirect_rejected("redirect_target_digest_invalid", url)
    if not parsed.query:
        _redirect_rejected("redirect_target_query_invalid", url)
    request = urllib.request.Request(url, headers={"Accept": "application/octet-stream"})
    try:
        with _HTTPS.open(request, timeout=30) as response:
            if response.status != 200:
                status = response.status if type(response.status) is int and 100 <= response.status <= 599 else None
                raise ReceiptError(phase=_CURRENT_PHASE, code="http_status",
                    failure_class="HTTPResponse", http_status=status)
            declared = response.headers.get("Content-Length")
            if declared is not None and (not declared.isdigit() or int(declared) > limit):
                _reject()
            body = response.read(limit + 1)
            if not 0 < len(body) <= limit or response.read(1):
                _reject()
            return body, dict(response.headers)
    except ReceiptError:
        raise
    except urllib.error.HTTPError as error:
        status = error.code if type(error.code) is int and 100 <= error.code <= 599 else None
        if status in (301, 302, 303, 307, 308):
            location = error.headers.get("Location") if error.headers is not None else ""
            error.close()
            _redirect_rejected("redirect_chain_rejected", location or "", status,
                phase="registry-config-redirect-content")
        raise ReceiptError(phase=_CURRENT_PHASE, code="http_error",
            failure_class="HTTPError", http_status=status) from None
    except urllib.error.URLError:
        raise ReceiptError(phase=_CURRENT_PHASE, code="network_error",
            failure_class="URLError") from None
    except (OSError, ValueError):
        raise ReceiptError(phase=_CURRENT_PHASE, code="network_error",
            failure_class="OSError") from None


def _read_response(url, headers, *, limit=MAX_REGISTRY_BODY, blob_redirect_digest=None):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != "https" or parsed.hostname != "ghcr.io" or parsed.username or parsed.password:
        _reject()
    request = urllib.request.Request(url, headers=headers)
    try:
        with _HTTPS.open(request, timeout=30) as response:
            if response.status != 200:
                status = response.status if type(response.status) is int and 100 <= response.status <= 599 else None
                raise ReceiptError(phase=_CURRENT_PHASE, code="http_status",
                    failure_class="HTTPResponse", http_status=status)
            declared = response.headers.get("Content-Length")
            if declared is not None and (not declared.isdigit() or int(declared) > limit):
                _reject()
            body = response.read(limit + 1)
            if not 0 < len(body) <= limit or response.read(1):
                _reject()
            return body, dict(response.headers)
    except ReceiptError:
        raise
    except urllib.error.HTTPError as error:
        status = error.code if type(error.code) is int and 100 <= error.code <= 599 else None
        if status == 307 and blob_redirect_digest is not None:
            location = error.headers.get("Location") if error.headers is not None else None
            error.close()
            return _read_redirected_blob(location or "", blob_redirect_digest, limit=limit)
        raise ReceiptError(phase=_CURRENT_PHASE, code="http_error",
            failure_class="HTTPError", http_status=status) from None
    except urllib.error.URLError:
        raise ReceiptError(phase=_CURRENT_PHASE, code="network_error",
            failure_class="URLError") from None
    except (OSError, ValueError):
        raise ReceiptError(phase=_CURRENT_PHASE, code="network_error",
            failure_class="OSError") from None


def _registry_token(name, username, github_token, cache):
    global _CURRENT_PHASE
    _CURRENT_PHASE = "registry-token"
    if name not in authority.IMAGE_NAMES or not username or not github_token:
        _reject()
    if name not in cache:
        basic = base64.b64encode((username + ":" + github_token).encode("utf-8")).decode("ascii")
        query = urllib.parse.urlencode({
            "service": "ghcr.io",
            "scope": "repository:poryadok/voiceroot/" + name + ":pull",
        })
        raw, _ = _read_response("https://ghcr.io/token?" + query,
            {"Authorization": "Basic " + basic, "Accept": "application/json"})
        try:
            value = json.loads(raw)
        except (UnicodeDecodeError, json.JSONDecodeError):
            _reject()
        token = value.get("token", value.get("access_token")) if isinstance(value, dict) else None
        if not isinstance(token, str) or not token or "\r" in token or "\n" in token:
            _reject()
        cache[name] = token
    return cache[name]


def _registry_get(name, reference, token, *, blob=False):
    global _CURRENT_PHASE
    _CURRENT_PHASE = "registry-config" if blob else (
        "registry-index" if SOURCE_SHA.fullmatch(reference) else "registry-platform-manifest")
    if (name not in authority.IMAGE_NAMES or not isinstance(reference, str)
        or not (SOURCE_SHA.fullmatch(reference) or DIGEST.fullmatch(reference))):
        _reject()
    endpoint = ("blobs/" if blob else "manifests/") + reference
    url = "https://ghcr.io/v2/poryadok/voiceroot/" + name + "/" + endpoint
    headers = {"Authorization": "Bearer " + token,
        "Accept": "application/octet-stream" if blob else ACCEPT}
    return _read_response(url, headers, blob_redirect_digest=reference if blob else None)


def _image_identity(name, tag, username, github_token, token_cache):
    global _CURRENT_PHASE
    _CURRENT_PHASE = "registry-image"
    if name not in authority.IMAGE_NAMES or tag != tag.lower() or not SOURCE_SHA.fullmatch(tag):
        _reject()
    token = _registry_token(name, username, github_token, token_cache)
    raw, returned = _registry_get(name, tag, token)
    index_digest = "sha256:" + hashlib.sha256(raw).hexdigest()
    declared = next((value for key, value in returned.items()
        if key.lower() == "docker-content-digest"), None)
    if declared != index_digest:
        _reject()
    try:
        index = json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError):
        _reject()
    child_digest = index_digest
    child = index
    if isinstance(index, dict) and "manifests" in index:
        if index.get("mediaType") not in (
            "application/vnd.oci.image.index.v1+json",
            "application/vnd.docker.distribution.manifest.list.v2+json"):
            _reject()
        descriptors = index.get("manifests")
        matches = [row for row in descriptors if isinstance(row, dict)
            and row.get("platform", {}).get("os") == "linux"
            and row.get("platform", {}).get("architecture") == "amd64"
            and row.get("platform", {}).get("variant") in (None, "")]
        if len(matches) != 1 or not DIGEST.fullmatch(str(matches[0].get("digest", ""))):
            _reject()
        descriptor = matches[0]
        if descriptor.get("mediaType") not in (
            "application/vnd.oci.image.manifest.v1+json",
            "application/vnd.docker.distribution.manifest.v2+json"):
            _reject()
        child_digest = descriptor["digest"]
        child_raw, child_headers = _registry_get(name, child_digest, token)
        if ("sha256:" + hashlib.sha256(child_raw).hexdigest() != child_digest
            or child_headers.get("Content-Type", "").split(";", 1)[0] not in (
                descriptor["mediaType"], "application/octet-stream")
            or type(descriptor.get("size")) is not int or descriptor["size"] != len(child_raw)):
            _reject()
        child_declared = next((value for key, value in child_headers.items()
            if key.lower() == "docker-content-digest"), None)
        if child_declared != child_digest:
            _reject()
        try:
            child = json.loads(child_raw)
        except (UnicodeDecodeError, json.JSONDecodeError):
            _reject()
    elif name == "matchmaking":
        _reject()
    if not isinstance(child, dict) or child.get("schemaVersion") != 2:
        _reject()
    _CURRENT_PHASE = "registry-config"
    config = child.get("config")
    if not isinstance(config, dict) or not DIGEST.fullmatch(str(config.get("digest", ""))) \
            or type(config.get("size")) is not int or not 0 < config["size"] <= MAX_REGISTRY_BODY:
        _reject()
    config_raw, _ = _registry_get(name, config["digest"], token, blob=True)
    if len(config_raw) != config["size"] or "sha256:" + hashlib.sha256(config_raw).hexdigest() != config["digest"]:
        _reject()
    try:
        config_json = json.loads(config_raw)
    except (UnicodeDecodeError, json.JSONDecodeError):
        _reject()
    if config_json.get("os") != "linux" or config_json.get("architecture") != "amd64":
        _reject()
    labels = config_json.get("config", {}).get("Labels") or {}
    if labels.get("org.opencontainers.image.revision") not in (None, tag):
        _reject()
    if labels.get("org.opencontainers.image.source", "").removesuffix(".git") not in (
        "", "https://github.com/" + REPO):
        _reject()
    return {
        "image": authority.REGISTRY + "/" + name + "@" + child_digest,
        "index_digest": index_digest,
        "platform_manifest_digest": child_digest,
        "config_sha256": config["digest"].removeprefix("sha256:"),
    }


def _verified_buildx(run_id, run, sha, index_digest, headers, deadline, repo):
    global _CURRENT_PHASE
    _CURRENT_PHASE = "publisher-job-metadata"
    attempt = run["run_attempt"]
    jobs = authority._pages(authority.API + "/actions/runs/" + str(run_id)
        + "/attempts/" + str(attempt) + "/jobs", "jobs", headers, deadline)
    rows = [job for job in jobs if job.get("name") == "staging-images-push (matchmaking)"]
    if len(rows) != 1:
        _reject()
    job = rows[0]
    steps = job.get("steps")
    producers = [step for step in steps if step.get("name") == "Build and push matchmaking image"] \
        if isinstance(steps, list) else []
    post_steps = [step for step in steps if step.get("name") == "Post Build and push matchmaking image"] \
        if isinstance(steps, list) else []
    if (job.get("run_id") != run_id or job.get("head_sha") != sha
        or job.get("run_attempt") != attempt or job.get("status") != "completed"
        or job.get("conclusion") != "success" or not isinstance(steps, list)
        or any(step.get("conclusion") != "success" for step in steps)
        or len(producers) != 1 or producers[0].get("conclusion") != "success"
        or len(post_steps) != 1 or post_steps[0].get("conclusion") != "success"):
        _reject()
    step = producers[0]
    post_step = post_steps[0]
    _CURRENT_PHASE = "publisher-job-log"
    log, _ = authority._fetch(authority.API + "/actions/jobs/" + str(job["id"]) + "/logs",
        headers, MAX_JOB_LOG, deadline)
    published = _published_buildx_artifact(log)
    _CURRENT_PHASE = "publisher-artifact-metadata"
    artifacts = authority._pages(authority.API + "/actions/runs/" + str(run_id)
        + "/artifacts", "artifacts", headers, deadline)
    candidates = [item for item in artifacts if item.get("id") == published["id"]
        and item.get("name") == published["name"] and item.get("digest") == published["digest"]
        and item.get("expired") is False
        and type(item.get("id")) is int and 0 < item["id"]
        and type(item.get("size_in_bytes")) is int
        and 0 < item["size_in_bytes"] <= MAX_BUILDX_ARTIFACT]
    verified = []
    failures = []
    for candidate in candidates:
        try:
            _CURRENT_PHASE = "publisher-artifact-content"
            artifact = authority._get(authority.API + "/actions/artifacts/"
                + str(candidate["id"]), headers, deadline)
            workflow = artifact.get("workflow_run", {})
            if (artifact.get("id") != candidate["id"] or artifact.get("name") != candidate["name"]
                or artifact.get("digest") != published["digest"]
                or artifact.get("expired") is not False or workflow.get("id") != run_id
                or workflow.get("head_branch") != "master" or workflow.get("head_sha") != sha
                or workflow.get("repository_id") != repo["id"]
                or workflow.get("head_repository_id") != repo["id"]
                or not authority.transport._timestamp(post_step.get("started_at"))
                    <= authority.transport._timestamp(artifact.get("created_at"))
                    <= authority.transport._timestamp(post_step.get("completed_at"))):
                continue
            raw, _ = authority._fetch(authority.API + "/actions/artifacts/"
                + str(candidate["id"]) + "/zip", headers, MAX_BUILDX_ARTIFACT, deadline)
            if len(raw) != candidate["size_in_bytes"]:
                continue
            evidence = gis._verify_buildx_artifact(raw, candidate["id"],
                candidate["size_in_bytes"], IMAGE + "@" + index_digest,
                run_id, attempt, sha, step, None, service="matchmaking")
            verified.append((candidate, evidence))
        except ReceiptError as error:
            failures.append(error)
        except authority.SourceError as error:
            failures.append(ReceiptError(phase=error.phase or _CURRENT_PHASE,
                code=error.code if error.code in ("validation_rejected", "transport_rejected",
                    "response_schema_invalid", "archive_rejected", "http_error", "http_status",
                    "network_error") else "source_capture_rejected",
                failure_class=error.failure_class, http_status=error.http_status))
        except Exception as error:
            failure_class = type(error).__name__ if type(error).__name__ in (
                "TimeoutError", "OSError", "KeyError", "TypeError", "ValueError", "BadZipFile") else "Other"
            failures.append(ReceiptError(phase=_CURRENT_PHASE, code="internal_error",
                failure_class=failure_class))
    if len(verified) != 1:
        if len(candidates) == 1 and len(failures) == 1:
            raise failures[0]
        raise ReceiptError(phase="publisher-buildx-validation",
            code="publisher_evidence_rejected", failure_class="ReceiptError")
    return job, step, verified[0][0], verified[0][1]


def _published_buildx_artifact(raw):
    """Bind Buildx evidence to the single artifact finalized by this publisher job."""
    try:
        if not isinstance(raw, bytes) or not 1 <= len(raw) <= MAX_JOB_LOG:
            _reject()
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            members = archive.infolist()
            if not 1 <= len(members) <= 64 or sum(row.file_size for row in members) > MAX_JOB_LOG:
                _reject()
            matches = []
            for member in members:
                mode = (member.external_attr >> 16) & 0o170000
                if (member.is_dir() or member.file_size > MAX_JOB_LOG
                    or mode not in (0, 0o100000) or member.flag_bits & 1):
                    _reject()
                with archive.open(member) as stream:
                    body = stream.read(MAX_JOB_LOG + 1)
                    if len(body) != member.file_size or len(body) > MAX_JOB_LOG or stream.read(1):
                        _reject()
                try:
                    text = body.decode("utf-8")
                except UnicodeDecodeError:
                    _reject()
                if "docker buildx history export" not in text:
                    continue
                names = re.findall(r"Poryadok~VoiceRoot~[A-Za-z0-9_-]+\.dockerbuild", text)
                identifiers = re.findall(r"Artifact successfully finalized \(([0-9]+)\)", text)
                hashes = re.findall(r"SHA256 hash of uploaded artifact is ([a-f0-9]{64})", text)
                if len(names) == len(identifiers) == len(hashes) == 1:
                    matches.append({"id": int(identifiers[0]), "name": names[0],
                        "digest": "sha256:" + hashes[0]})
                else:
                    _reject()
            if len(matches) != 1 or matches[0]["id"] <= 0:
                _reject()
            return matches[0]
    except ReceiptError:
        raise
    except Exception:
        _reject()


def _verify_ci_build_source(source_root):
    root = Path(source_root)
    try:
        contexts = (root / "scripts/ci/backend-docker-context.txt").read_text(encoding="utf-8").splitlines()
        workflow = (root / ".github/workflows/ci.yml").read_text(encoding="utf-8")
    except (OSError, UnicodeError):
        _reject()
    push_at = workflow.find("  staging-images-push:")
    promote_at = workflow.find("  staging-images-promote:", push_at + 1)
    push_job = workflow[push_at:promote_at] if push_at >= 0 and promote_at > push_at else ""
    if ("matchmaking" not in contexts
        or 'if grep -qx "$S" scripts/ci/backend-docker-context.txt' not in workflow
        or 'echo "context=./src/backend"' not in workflow
        or "- name: Build and push ${{ matrix.service }} image" not in push_job
        or "docker/build-push-action@v6" not in push_job
        or "context: ${{ steps.dockerctx.outputs.context }}" not in push_job
        or "file: ${{ steps.dockerctx.outputs.file }}" not in push_job
        or "push: true" not in push_job):
        _reject()


def capture_receipt(run_id, github_token, github_actor, output_path):
    global _CURRENT_PHASE, _ACTIVE_REDIRECT_DIAGNOSTIC
    _ACTIVE_REDIRECT_DIAGNOSTIC = None
    _CURRENT_PHASE = "preflight"
    if type(run_id) is not int or run_id <= 0 or not github_token or not github_actor:
        _reject()
    # Run the canonical V1 source capture unchanged, substituting only its image
    # resolver so private GHCR reads use this job's packages:read token.
    username = github_actor
    headers = authority._headers(github_token)
    deadline = time.monotonic() + 60
    _CURRENT_PHASE = "run-metadata"
    run = authority._get(authority.API + "/actions/runs/" + str(run_id), headers, deadline)
    source_sha = run.get("head_sha") if isinstance(run, dict) else None
    if (run.get("id") != run_id or run.get("status") != "completed"
        or run.get("conclusion") != "success" or run.get("event") != "push"
        or run.get("head_branch") != "master" or run.get("path", "").split("@", 1)[0] != ".github/workflows/ci.yml"
        or run.get("workflow_id") != authority.WORKFLOW
        or (run.get("repository") or {}).get("full_name") != REPO
        or (run.get("head_repository") or {}).get("full_name") != REPO
        or (run.get("repository") or {}).get("id") != (run.get("head_repository") or {}).get("id")
        or not isinstance(source_sha, str) or not SOURCE_SHA.fullmatch(source_sha)
        or type(run.get("run_attempt")) is not int or run["run_attempt"] <= 0):
        _reject()
    registry_tokens = {}
    identities = {}
    original_resolver = authority._resolve_image

    def authenticated_resolver(name, tag, deadline):
        global _CURRENT_PHASE, _ACTIVE_REDIRECT_DIAGNOSTIC
        if name not in authority.IMAGE_NAMES:
            _reject()
        _CURRENT_PHASE = "registry-image"
        try:
            identity = _image_identity(name, tag, username, github_token, registry_tokens)
        except ReceiptError as error:
            if error.code in REDIRECT_DIAGNOSTIC_CODES:
                _ACTIVE_REDIRECT_DIAGNOSTIC = error.safe_record()
            raise authority.SourceError(phase=error.phase or _CURRENT_PHASE,
                code=error.code, failure_class=error.failure_class,
                http_status=error.http_status) from None
        identities[name] = identity
        return identity["image"]

    authority._resolve_image = authenticated_resolver
    try:
        with tempfile.TemporaryDirectory(prefix="voice-matchmaking-authority-") as directory:
            Path(directory).chmod(0o700)
            _CURRENT_PHASE = "source-capture"
            base = authority.capture_equivalent_master_source(github_token, run_id,
                source_sha, Path(directory))
            _CURRENT_PHASE = "build-context-validation"
            _verify_ci_build_source(directory)
            deadline = time.monotonic() + 120
            _CURRENT_PHASE = "publisher-repository"
            repo = authority._get(authority.API, headers, deadline)
            _CURRENT_PHASE = "publisher-job-metadata"
            job, step, artifact, build = _verified_buildx(run_id, run,
                base["source_sha"], identities["matchmaking"]["index_digest"],
                headers, deadline, repo)
            _CURRENT_PHASE = "receipt-binding"
            selected = identities.get("matchmaking")
            if (not isinstance(selected, dict)
                or selected["index_digest"] != "sha256:" + build["builder_output_index_sha256"]
                or base["images"].get("matchmaking") != selected["image"]):
                _reject()
            base["selected_images"] = {"matchmaking": {
                "tag": IMAGE + ":" + base["source_sha"],
                "index_digest": selected["index_digest"],
                "platform_manifest_digest": selected["platform_manifest_digest"],
                "config_sha256": selected["config_sha256"],
                "builder_job_id": job["id"],
                "builder_artifact_id": artifact["id"],
                "builder_artifact_sha256": build["builder_artifact_sha256"],
                "builder_provenance_sha256": build["builder_provenance_sha256"],
                "build_step_started_at": step["started_at"],
                "build_step_completed_at": step["completed_at"],
            }}
            if base.get("schema") != "voice-source-authority-v1" or base.get("verified") is not True:
                _reject()
            encoded = json.dumps(base, sort_keys=True, separators=(",", ":")).encode("utf-8") + b"\n"
            _CURRENT_PHASE = "receipt-write"
            target = Path(output_path)
            target.write_bytes(encoded)
            return base
    except ReceiptError:
        raise
    except authority.SourceError as error:
        if _ACTIVE_REDIRECT_DIAGNOSTIC is not None:
            diagnostic = _ACTIVE_REDIRECT_DIAGNOSTIC
            raise ReceiptError(phase=diagnostic["phase"], code=diagnostic["code"],
                failure_class="HTTPError", http_status=diagnostic.get("http_status"),
                redirect_host_class=diagnostic.get("redirect_host_class")) from None
        raise ReceiptError(phase=error.phase or _CURRENT_PHASE,
            code=error.code if error.code in ("validation_rejected", "transport_rejected",
                "response_schema_invalid", "archive_rejected", "http_error", "http_status",
                "network_error") else "source_capture_rejected",
            failure_class=error.failure_class, http_status=error.http_status) from None
    except Exception as error:
        failure_class = type(error).__name__ if type(error).__name__ in (
            "TimeoutError", "OSError", "KeyError", "TypeError", "ValueError", "BadZipFile") else "Other"
        code = "response_schema_invalid" if failure_class in ("KeyError", "TypeError", "ValueError") else "internal_error"
        raise ReceiptError(phase=_CURRENT_PHASE, code=code, failure_class=failure_class) from None
    finally:
        authority._resolve_image = original_resolver
        _ACTIVE_REDIRECT_DIAGNOSTIC = None


def main(argv=None):
    argv = sys.argv[1:] if argv is None else argv
    if len(argv) != 2 or not argv[0].isdigit():
        print("usage: matchmaking_source_authority.py RUN_ID OUTPUT.json", file=sys.stderr)
        return 2
    token = os.environ.get("GITHUB_TOKEN", "")
    actor = os.environ.get("GITHUB_ACTOR", "")
    try:
        receipt = capture_receipt(int(argv[0]), token, actor, argv[1])
        print(json.dumps({"status": "PASS", "run_id": receipt["run_id"],
            "source_sha": receipt["source_sha"], "image": "matchmaking"}, sort_keys=True))
        return 0
    except ReceiptError as error:
        print(json.dumps(error.safe_record(), sort_keys=True), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
