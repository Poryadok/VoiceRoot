"""Offline tests for the fixed, read-only Matchmaking source-authority receipt."""

import hashlib
import io
import json
import sys
import tarfile
import tempfile
import unittest
import urllib.error
from datetime import datetime, timedelta, timezone
import zipfile
from pathlib import Path
from contextlib import redirect_stderr
from io import StringIO
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import matchmaking_source_authority as receipt


class RegistryIdentityTests(unittest.TestCase):
    def test_ghcr_blob_redirect_uses_exact_signed_host_without_registry_credentials(self):
        source = "a" * 40
        body = json.dumps({"os": "linux", "architecture": "amd64",
            "config": {"Labels": {"org.opencontainers.image.revision": source}}},
            separators=(",", ":")).encode()
        config_digest = "sha256:" + hashlib.sha256(body).hexdigest()
        child = json.dumps({"schemaVersion": 2,
            "config": {"digest": config_digest, "size": len(body)}},
            separators=(",", ":")).encode()
        child_digest = "sha256:" + hashlib.sha256(child).hexdigest()
        index = json.dumps({"schemaVersion": 2,
            "mediaType": "application/vnd.oci.image.index.v1+json",
            "manifests": [{"mediaType": "application/vnd.oci.image.manifest.v1+json",
                "digest": child_digest, "size": len(child),
                "platform": {"os": "linux", "architecture": "amd64"}}]},
            separators=(",", ":")).encode()
        index_digest = "sha256:" + hashlib.sha256(index).hexdigest()
        location = "https://pkg-containers.githubusercontent.com/objects/cache/config-blob?sig=private%2Bopaque"
        redirect = urllib.error.HTTPError("https://ghcr.io/blob", 307, "redirect",
            {"Location": location}, None)
        calls = []

        class Response:
            status = 200

            def __init__(self, content, headers):
                self.content = content
                self.headers = headers
                self.stream = io.BytesIO(content)

            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return False

            def read(self, size=-1):
                return self.stream.read(size)

        def open_request(request, timeout):
            calls.append(request)
            self.assertEqual(timeout, 30)
            if len(calls) <= 3:
                self.assertEqual(request.get_header("Authorization"), "Bearer ephemeral-token")
                if len(calls) == 1:
                    return Response(index, {"Docker-Content-Digest": index_digest})
                if len(calls) == 2:
                    return Response(child, {"Docker-Content-Digest": child_digest,
                        "Content-Type": "application/vnd.oci.image.manifest.v1+json"})
                raise redirect
            self.assertEqual(request.full_url, location)
            self.assertIsNone(request.get_header("Authorization"))
            self.assertEqual(request.get_header("Accept"), "application/octet-stream")
            return Response(body, {"Content-Length": str(len(body))})

        with patch.object(receipt, "_registry_token", return_value="ephemeral-token"), \
                patch.object(receipt._HTTPS, "open", side_effect=open_request):
            result = receipt._image_identity("matchmaking", source, "actor", "secret", {})
        self.assertEqual(result["index_digest"], index_digest)
        self.assertEqual(result["platform_manifest_digest"], child_digest)
        self.assertEqual(result["config_sha256"], config_digest.removeprefix("sha256:"))
        self.assertEqual(len(calls), 4)
        self.assertNotIn("sig=private", json.dumps(result))

    def test_ghcr_blob_redirect_rejects_unsafe_locations_without_following(self):
        digest = "sha256:" + "a" * 64
        locations = (
            ("http://pkg-containers.githubusercontent.com/objects/blob?sig=x",
                "redirect_target_scheme_invalid", "known_cdn"),
            ("https://evil.example/objects/blob?sig=x",
                "redirect_target_host_invalid", "other"),
            ("https://pkg-containers.githubusercontent.com/objects/../blob?sig=x",
                "redirect_target_path_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects/%2e%2e/blob?sig=x",
                "redirect_target_path_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects/%2f..%5c/blob?sig=x",
                "redirect_target_path_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects/%252e%252e/blob?sig=x",
                "redirect_target_path_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects\\blob?sig=x",
                "redirect_target_path_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects/bad%2?sig=x",
                "redirect_target_path_invalid", "known_cdn"),
            ("https://user@pkg-containers.githubusercontent.com/objects/blob?sig=x",
                "redirect_target_authority_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com:444/objects/blob?sig=x",
                "redirect_target_authority_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects/blob?sig=x#fragment",
                "redirect_target_fragment_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects/blob?",
                "redirect_target_query_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects/blob?sig=bad%2",
                "redirect_target_query_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects/blob?sig=" + "x" * 8193,
                "redirect_target_query_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/" + "x" * 8193 + "?sig=x",
                "redirect_target_path_invalid", "known_cdn"),
            ("https://pkg-containers.githubusercontent.com/objects/blob?sig=x\r\nAuthorization: Bearer secret",
                "redirect_target_control_invalid", "known_cdn"),
        )
        for location, code, host_class in locations:
            with self.subTest(location=location), \
                    patch.object(receipt, "_CURRENT_PHASE", "registry-config"), \
                    patch.object(receipt._HTTPS, "open", side_effect=urllib.error.HTTPError(
                        "https://ghcr.io/blob", 307, "redirect", {"Location": location}, None)) as opened:
                with self.assertRaises(receipt.ReceiptError) as raised:
                    receipt._read_response("https://ghcr.io/v2/repo/blobs/" + digest,
                        {"Authorization": "Bearer private-token"}, blob_redirect_digest=digest,
                        blob_expected_size=1)
                record = raised.exception.safe_record()
                self.assertEqual(record["phase"], "registry-config-redirect-target")
                self.assertEqual(record["code"], code)
                self.assertEqual(record["redirect_host_class"], host_class)
                self.assertEqual(record["http_status"], 307)
                self.assertEqual(opened.call_count, 1)
                self.assertNotIn(location, json.dumps(record))
                self.assertNotIn("sig=x", json.dumps(record))

    def test_ghcr_redirected_blob_reads_at_most_descriptor_size_plus_one(self):
        digest = "sha256:" + "a" * 64
        url = "https://pkg-containers.githubusercontent.com/opaque/blob?sig=unchanged"

        class Response:
            status = 200
            headers = {}

            def __init__(self, content):
                self.stream = io.BytesIO(content)
                self.read_sizes = []
                self.read_bytes = 0

            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return False

            def read(self, size=-1):
                self.read_sizes.append(size)
                chunk = self.stream.read(size)
                self.read_bytes += len(chunk)
                return chunk

        for body, should_succeed in ((b"data", True), (b"dat", False),
                (b"small", False), (b"oversized", False)):
            response = Response(body)
            with self.subTest(length=len(body)), patch.object(receipt._HTTPS, "open", return_value=response):
                if should_succeed:
                    actual, _ = receipt._read_redirected_blob(url, digest, expected_size=4)
                    self.assertEqual(actual, body)
                else:
                    with self.assertRaises(receipt.ReceiptError):
                        receipt._read_redirected_blob(url, digest, expected_size=4)
                self.assertLessEqual(response.read_bytes, 5)
                self.assertTrue(all(size <= 5 for size in response.read_sizes))

    def test_ghcr_blob_redirect_chain_stops_after_one_hop(self):
        digest = "sha256:" + "a" * 64
        first = "https://pkg-containers.githubusercontent.com/ghcr1/blobs/" + digest + "?sig=first"
        second = "https://pkg-containers.githubusercontent.com/ghcr1/blobs/" + digest + "?sig=second"
        calls = []

        def open_request(request, timeout):
            calls.append(request)
            location = first if len(calls) == 1 else second
            raise urllib.error.HTTPError(request.full_url, 307, "redirect", {"Location": location}, None)

        with patch.object(receipt, "_CURRENT_PHASE", "registry-config"), \
                patch.object(receipt._HTTPS, "open", side_effect=open_request):
            with self.assertRaises(receipt.ReceiptError) as raised:
                receipt._read_response("https://ghcr.io/v2/repo/blobs/" + digest,
                    {"Authorization": "Bearer private-token"}, blob_redirect_digest=digest,
                    blob_expected_size=1)
        self.assertEqual(raised.exception.safe_record(), {"status": "REJECTED",
            "phase": "registry-config-redirect-content", "code": "redirect_chain_rejected",
            "class": "HTTPError", "http_status": 307, "redirect_host_class": "known_cdn"})
        self.assertEqual(len(calls), 2)
        self.assertIsNone(calls[1].get_header("Authorization"))
        self.assertNotIn("sig=", json.dumps(raised.exception.safe_record()))

    def test_index_child_and_config_are_bound_to_linux_amd64(self):
        config = json.dumps({"os": "linux", "architecture": "amd64",
            "config": {"Labels": {"org.opencontainers.image.revision": "a" * 40,
                "org.opencontainers.image.source": "https://github.com/Poryadok/VoiceRoot.git"}}},
            separators=(",", ":")).encode()
        config_digest = "sha256:" + hashlib.sha256(config).hexdigest()
        child = json.dumps({"schemaVersion": 2,
            "config": {"digest": config_digest, "size": len(config)}},
            separators=(",", ":")).encode()
        child_digest = "sha256:" + hashlib.sha256(child).hexdigest()
        index = json.dumps({"schemaVersion": 2,
            "mediaType": "application/vnd.oci.image.index.v1+json",
            "manifests": [{"mediaType": "application/vnd.oci.image.manifest.v1+json",
                "digest": child_digest, "size": len(child),
                "platform": {"os": "linux", "architecture": "amd64"}}]},
            separators=(",", ":")).encode()
        index_digest = "sha256:" + hashlib.sha256(index).hexdigest()
        responses = [
            (index, {"Docker-Content-Digest": index_digest}),
            (child, {"Docker-Content-Digest": child_digest,
                "Content-Type": "application/vnd.oci.image.manifest.v1+json"}),
            (config, {"Content-Type": "application/octet-stream"}),
        ]
        with patch.object(receipt, "_registry_token", return_value="ephemeral"), \
                patch.object(receipt, "_registry_get", side_effect=responses):
            actual = receipt._image_identity("matchmaking", "a" * 40, "actor", "secret", {})
        self.assertEqual(actual["image"], receipt.IMAGE + "@" + child_digest)
        self.assertEqual(actual["index_digest"], index_digest)
        self.assertEqual(actual["platform_manifest_digest"], child_digest)
        self.assertEqual(actual["config_sha256"], config_digest.removeprefix("sha256:"))

    def test_duplicate_amd64_descriptors_block_even_when_both_are_digest_valid(self):
        child = json.dumps({"schemaVersion": 2,
            "config": {"digest": "sha256:" + "b" * 64, "size": 1}}).encode()
        child_digest = "sha256:" + hashlib.sha256(child).hexdigest()
        index = json.dumps({"schemaVersion": 2,
            "mediaType": "application/vnd.oci.image.index.v1+json",
            "manifests": [
                {"mediaType": "application/vnd.oci.image.manifest.v1+json",
                    "digest": child_digest, "size": len(child),
                    "platform": {"os": "linux", "architecture": "amd64"}},
                {"mediaType": "application/vnd.oci.image.manifest.v1+json",
                    "digest": child_digest, "size": len(child),
                    "platform": {"os": "linux", "architecture": "amd64"}}]}).encode()
        index_digest = "sha256:" + hashlib.sha256(index).hexdigest()
        with patch.object(receipt, "_registry_token", return_value="ephemeral"), \
                patch.object(receipt, "_registry_get", return_value=(index,
                    {"Docker-Content-Digest": index_digest})):
            with self.assertRaises(receipt.ReceiptError):
                receipt._image_identity("matchmaking", "a" * 40, "actor", "secret", {})


class ReceiptTests(unittest.TestCase):
    def test_collector_captures_historical_v1_receipt_with_unprivileged_filesystem(self):
        source = "a" * 40
        current = "b" * 40
        root = Path(__file__).resolve().parents[2]
        paths = (".github/workflows/ci.yml", "scripts/ci/backend-docker-context.txt",
            "scripts/ci/staging-image-catalog.json")
        files = {path: (root / path).read_bytes() for path in paths}
        tree_rows = []
        archive_bytes = io.BytesIO()
        with tarfile.open(fileobj=archive_bytes, mode="w:gz") as tar:
            prefix = "Poryadok-VoiceRoot-" + source[:7]
            for directory in (prefix + "/", prefix + "/.github/", prefix + "/.github/workflows/",
                prefix + "/scripts/", prefix + "/scripts/ci/"):
                member = tarfile.TarInfo(directory)
                member.type = tarfile.DIRTYPE
                member.mode = 0o755
                tar.addfile(member)
            for path, content in files.items():
                blob_sha = hashlib.sha1(b"blob " + str(len(content)).encode() + b"\0" + content).hexdigest()
                tree_rows.append({"path": path, "type": "blob", "mode": "100644",
                    "size": len(content), "sha": blob_sha})
                member = tarfile.TarInfo(prefix + "/" + path)
                member.size = len(content)
                member.mode = 0o644
                tar.addfile(member, io.BytesIO(content))
        tree_sha = "1" * 40
        now = datetime.now(timezone.utc)
        lock_bytes = ("version: 1\nregistry: " + receipt.authority.REGISTRY + "\ntag: " + source + "\nimages:\n"
            + "".join("  " + name + ": " + source + "\n"
                for name in sorted(receipt.authority.IMAGE_NAMES))).encode()
        packed_lock = io.BytesIO()
        with zipfile.ZipFile(packed_lock, "w") as archive:
            archive.writestr("stack.lock.yaml", lock_bytes)
        packed_lock_bytes = packed_lock.getvalue()
        repo = {"id": 7, "full_name": "Poryadok/VoiceRoot"}
        source_run = {"id": 11, "status": "completed", "conclusion": "success",
            "event": "push", "head_branch": "master", "path": ".github/workflows/ci.yml",
            "workflow_id": 263230665, "head_sha": source, "run_attempt": 1,
            "repository": {"full_name": "Poryadok/VoiceRoot", "id": 7},
            "head_repository": {"full_name": "Poryadok/VoiceRoot", "id": 7}}
        lock_job = {"started_at": (now - timedelta(minutes=2)).isoformat(),
            "completed_at": now.isoformat()}
        lock_artifact = {"id": 14, "name": "staging-stack-lock", "expired": False,
            "size_in_bytes": len(packed_lock_bytes),
            "workflow_run": {"id": 11, "head_sha": source, "repository_id": 7,
                "head_repository_id": 7},
            "created_at": (now - timedelta(minutes=1)).isoformat(),
            "expires_at": (now + timedelta(days=1)).isoformat()}
        image_index = "b" * 64
        image_child = "c" * 64
        image_config = "d" * 64
        build_job = {"id": 12}
        build_step = {"started_at": (now - timedelta(minutes=2)).isoformat(),
            "completed_at": now.isoformat()}
        build_artifact = {"id": 13}
        build = {"builder_output_index_sha256": image_index,
            "builder_artifact_sha256": "e" * 64, "builder_provenance_sha256": "f" * 64}

        def get(url, _headers, _deadline):
            if url == receipt.authority.API + "/actions/runs/11":
                return source_run
            if url == receipt.authority.API:
                return {"id": 7, "full_name": "Poryadok/VoiceRoot"}
            if url == receipt.authority.API + "/git/ref/heads/master":
                return {"ref": "refs/heads/master", "object": {"type": "commit", "sha": current}}
            if url == receipt.authority.API + "/git/commits/" + source:
                return {"sha": source, "tree": {"sha": tree_sha}}
            if url == receipt.authority.API + "/git/trees/" + tree_sha + "?recursive=1":
                return {"sha": tree_sha, "truncated": False, "tree": tree_rows}
            if url == receipt.authority.API + "/actions/artifacts/14":
                return lock_artifact
            raise AssertionError("unexpected fixed API request")

        def fetch(url, _headers, _limit, _deadline, target=None):
            if url.endswith("/tarball/" + source):
                raw = archive_bytes.getvalue()
            elif url.endswith("/actions/artifacts/14/zip"):
                raw = packed_lock_bytes
            else:
                raise AssertionError("unexpected fixed archive request")
            target.write(raw)
            return len(raw), {}

        def identity(name, tag, _actor, _token, _cache):
            self.assertEqual(tag, source)
            if name == "matchmaking":
                return {"image": receipt.IMAGE + "@sha256:" + image_child,
                    "index_digest": "sha256:" + image_index,
                    "platform_manifest_digest": "sha256:" + image_child,
                    "config_sha256": image_config}
            return {"image": receipt.authority.REGISTRY + "/" + name + "@sha256:" + "9" * 64,
                "index_digest": "sha256:" + "8" * 64,
                "platform_manifest_digest": "sha256:" + "9" * 64,
                "config_sha256": "7" * 64}

        equivalence = {"current_master_sha_before": current,
            "current_master_sha_after": current, "source_is_ancestor": True,
            "equivalent_inputs": {"src/backend": {"git_object_sha1": "6" * 40}}}
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "source-authority-result.json"
            with patch.object(receipt.authority, "_headers", return_value={}), \
                    patch.object(receipt.authority, "_get", side_effect=get), \
                    patch.object(receipt.authority, "_head", return_value={"id": 7}), \
                    patch.object(receipt.authority.shutil, "disk_usage", return_value=type(
                        "Usage", (), {"free": 3 * 1024 ** 3})()), \
                    patch.object(receipt.authority, "_ci", return_value=(
                        {"run_attempt": 1}, {"staging-stack-lock": lock_job})), \
                    patch.object(receipt.authority, "_equivalent_master_source", return_value=equivalence), \
                    patch.object(receipt.authority, "_pages", return_value=[lock_artifact]), \
                    patch.object(receipt.authority, "_fetch", side_effect=fetch), \
                    patch.object(receipt.authority, "_resolve_image", side_effect=AssertionError("resolver replaced")), \
                    patch.object(receipt, "_image_identity", side_effect=identity), \
                    patch.object(receipt, "_verified_buildx", return_value=(
                        build_job, build_step, build_artifact, build)):
                result = receipt.capture_receipt(11, "token", "actor", target)
            saved = json.loads(target.read_text(encoding="utf-8"))
        self.assertEqual(result["source_sha"], source)
        self.assertEqual(saved["source_equivalence"]["current_master_sha_before"], current)
        self.assertEqual(saved["source_equivalence"]["current_master_sha_after"], current)
        self.assertEqual(set(saved["images"]), set(receipt.authority.IMAGE_NAMES))
        self.assertEqual(saved["selected_images"]["matchmaking"]["index_digest"], "sha256:" + image_index)
        self.assertEqual(saved["selected_images"]["matchmaking"]["platform_manifest_digest"], "sha256:" + image_child)
        self.assertEqual(saved["selected_images"]["matchmaking"]["config_sha256"], image_config)

    def test_publisher_log_binds_exact_buildx_artifact_id_name_and_digest(self):
        # Sanitized, real-shape GitHub job-log fixture. The actual 94,931-byte
        # publisher response repeats the same artifact name three times.
        log = """\ufeff[command]/usr/bin/docker buildx history export --output /tmp/Poryadok~VoiceRoot~NGMG82.dockerbuild record --finalize
Poryadok~VoiceRoot~NGMG82.dockerbuild
Poryadok~VoiceRoot~NGMG82.dockerbuild
SHA256 hash of uploaded artifact is ed59e1d216e957da41d9c8d7a98f005b7a3c3eff324ddb9c453df022191dd805
Artifact successfully finalized (12113921386)
"""
        self.assertEqual(receipt._published_buildx_artifact(log.encode("utf-8")), {
            "id": 12113921386, "name": "Poryadok~VoiceRoot~NGMG82.dockerbuild",
            "digest": "sha256:ed59e1d216e957da41d9c8d7a98f005b7a3c3eff324ddb9c453df022191dd805"})

    def test_publisher_log_rejects_ambiguous_artifact_finalization(self):
        digest = hashlib.sha256(b"buildx test artifact").hexdigest()
        log = ("docker buildx history export Poryadok~VoiceRoot~KFDCOM.dockerbuild\n"
            f"SHA256 hash of uploaded artifact is {digest}\n"
            "Artifact successfully finalized (12110717476)\n"
            "Artifact successfully finalized (12110880999)\n").encode("utf-8")
        with self.assertRaises(receipt.ReceiptError):
            receipt._published_buildx_artifact(log)

    def test_publisher_log_rejects_archive_malformed_and_unbounded_plain_text(self):
        digest = hashlib.sha256(b"buildx test artifact").hexdigest()
        source = ("docker buildx history export Poryadok~VoiceRoot~KFDCOM.dockerbuild\n"
            f"SHA256 hash of uploaded artifact is {digest}\n"
            "Artifact successfully finalized (12110717476)\n").encode("utf-8")
        packed = io.BytesIO()
        with zipfile.ZipFile(packed, "w") as archive:
            archive.writestr("post.txt", source)
        malformed = b"\xff" + source
        truncated = b"docker buildx history export Poryadok~VoiceRoot~KFDCOM.dockerbuild\n"
        overlong_line = b"x" * (64 * 1024 + 1)
        too_many_lines = b"\n".join([b"x"] * 100_002)
        oversized = b"x" * ((16 << 20) + 1)
        for invalid in (packed.getvalue(), malformed, truncated, overlong_line, too_many_lines, oversized):
            with self.subTest(size=len(invalid)), self.assertRaises(receipt.ReceiptError):
                receipt._published_buildx_artifact(invalid)

    def test_publisher_log_rejects_distinct_names_and_duplicate_digest(self):
        digest = hashlib.sha256(b"buildx test artifact").hexdigest()
        base = ("docker buildx history export Poryadok~VoiceRoot~KFDCOM.dockerbuild\n"
            f"SHA256 hash of uploaded artifact is {digest}\n"
            "Artifact successfully finalized (12110717476)\n")
        ambiguous_name = base + "Poryadok~VoiceRoot~OTHER.dockerbuild\n"
        duplicate_digest = base + f"SHA256 hash of uploaded artifact is {digest}\n"
        duplicate_export = base + "docker buildx history export Poryadok~VoiceRoot~KFDCOM.dockerbuild\n"
        for invalid in (ambiguous_name, duplicate_digest, duplicate_export):
            with self.subTest(log=invalid[-64:]), self.assertRaises(receipt.ReceiptError):
                receipt._published_buildx_artifact(invalid.encode("utf-8"))

    def test_publisher_log_rejects_overlong_artifact_name_and_digest_tokens(self):
        digest = hashlib.sha256(b"buildx test artifact").hexdigest()
        name = "Poryadok~VoiceRoot~KFDCOM.dockerbuild"
        log = (f"docker buildx history export {name}\n"
            f"SHA256 hash of uploaded artifact is {digest}\n"
            "Artifact successfully finalized (12110717476)\n")
        for invalid in (log.replace(name, name + ".extra"),
                        log.replace(digest, digest + "a")):
            with self.subTest(log=invalid[-80:]), self.assertRaises(receipt.ReceiptError):
                receipt._published_buildx_artifact(invalid.encode("utf-8"))

    def test_receipt_preserves_complete_v1_map_and_adds_verified_matchmaking_binding(self):
        sha = "a" * 40
        index = "b" * 64
        child = "c" * 64
        config = "d" * 64
        job = {"id": 12}
        step = {"started_at": "2026-10-17T00:01:00Z",
            "completed_at": "2026-10-17T00:02:00Z"}
        artifact = {"id": 13}
        build = {"builder_output_index_sha256": index,
            "builder_artifact_sha256": "e" * 64,
            "builder_provenance_sha256": "f" * 64}
        source_run = {"id": 11, "status": "completed", "conclusion": "success",
            "event": "push", "head_branch": "master", "path": ".github/workflows/ci.yml",
            "workflow_id": 263230665, "head_sha": sha, "run_attempt": 1,
            "repository": {"full_name": "Poryadok/VoiceRoot", "id": 7},
            "head_repository": {"full_name": "Poryadok/VoiceRoot", "id": 7}}
        base = {"schema": "voice-source-authority-v1", "repository": "Poryadok/VoiceRoot",
            "source_sha": sha, "workflow_id": 263230665, "run_id": 11,
            "run_attempt": 1, "tree_sha": "1" * 40,
            "source_equivalence": {"current_master_sha_before": "9" * 40,
                "current_master_sha_after": "9" * 40, "source_is_ancestor": True,
                "equivalent_inputs": {"src/backend": "8" * 40}},
            "source_files": {"src/backend/go.mod": "2" * 64},
            "images": {"matchmaking": receipt.IMAGE + "@sha256:" + child,
                "chat": "ghcr.io/poryadok/voiceroot/chat@sha256:" + "3" * 64},
            "lock_artifact_id": 14, "lock_sha256": "4" * 64,
            "image_provenance": "independently resolved trusted CI tag", "verified": True}
        identity = {"image": base["images"]["matchmaking"],
            "index_digest": "sha256:" + index,
            "platform_manifest_digest": "sha256:" + child,
            "config_sha256": config}

        def source_capture(_token, _run_id, _sha, _directory):
            receipt.authority._resolve_image("matchmaking", sha, 0)
            receipt.authority._resolve_image("chat", sha, 0)
            return dict(base)

        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "source-authority-result.json"
            with patch.object(receipt.authority, "_headers", return_value={}), \
                    patch.object(receipt.authority, "_get", side_effect=[source_run,
                        {"id": 7, "full_name": "Poryadok/VoiceRoot"}]), \
                    patch.object(receipt.authority, "capture_equivalent_master_source", side_effect=source_capture), \
                    patch.object(receipt, "_verify_ci_build_source"), \
                    patch.object(receipt, "_verified_buildx",
                        return_value=(job, step, artifact, build)), \
                    patch.object(receipt, "_image_identity", side_effect=[identity,
                        {"image": base["images"]["chat"], "index_digest": "sha256:" + "5" * 64,
                            "platform_manifest_digest": "sha256:" + "3" * 64,
                            "config_sha256": "6" * 64}]):
                result = receipt.capture_receipt(11, "token", "actor", target)
            saved = json.loads(target.read_text(encoding="utf-8"))
        self.assertEqual(result["schema"], "voice-source-authority-v1")
        self.assertEqual(saved["source_sha"], sha)
        self.assertEqual(saved["source_equivalence"]["current_master_sha_before"], "9" * 40)
        self.assertEqual(saved["source_equivalence"]["current_master_sha_after"], "9" * 40)
        self.assertEqual(set(saved["images"]), {"matchmaking", "chat"})
        self.assertEqual(saved["selected_images"]["matchmaking"]["tag"],
            receipt.IMAGE + ":" + sha)
        self.assertEqual(saved["selected_images"]["matchmaking"]["index_digest"],
            "sha256:" + index)
        self.assertEqual(saved["selected_images"]["matchmaking"]["platform_manifest_digest"],
            "sha256:" + child)
        self.assertEqual(saved["selected_images"]["matchmaking"]["config_sha256"], config)


class DiagnosticTests(unittest.TestCase):
    def test_source_phase_is_closed_over_fixed_allowlist(self):
        error = receipt.authority.SourceError(phase="https://private.example/token?secret=hidden")
        self.assertEqual(error.phase, "unknown")
        self.assertNotIn("private.example", json.dumps({"phase": error.phase}))

    def test_fixed_source_rejection_uses_enclosing_phase_after_normalization(self):
        with self.assertRaises(receipt.authority.SourceError) as raised:
            receipt.authority._fail()
        diagnostic = receipt.authority._source_diagnostic(raised.exception, "source-tree")
        self.assertEqual(diagnostic.phase, "source-tree")
        hostile = receipt.authority.SourceError(phase="private-response-url")
        self.assertEqual(receipt.authority._source_diagnostic(hostile, "source-tree").phase, "source-tree")

    def test_registry_http_diagnostic_is_allowlisted_and_does_not_expose_response_data(self):
        secret = "private-token-and-response-body"
        error = urllib.error.HTTPError("https://ghcr.io/token?secret=" + secret,
            403, secret, {"Authorization": secret}, None)
        with patch.object(receipt, "_CURRENT_PHASE", "registry-token"), \
                patch.object(receipt._HTTPS, "open", side_effect=error):
            with self.assertRaises(receipt.ReceiptError) as raised:
                receipt._read_response("https://ghcr.io/token?service=ghcr.io", {}, limit=100)
        record = raised.exception.safe_record()
        self.assertEqual(record, {"status": "REJECTED", "phase": "registry-token",
            "code": "http_error", "class": "HTTPError", "http_status": 403})
        self.assertNotIn(secret, json.dumps(record))
        self.assertNotIn(secret, str(raised.exception))

    def test_cli_rejection_emits_only_safe_diagnostic_fields(self):
        secret = "private-output-must-never-appear"
        error = receipt.ReceiptError(secret, phase="registry-token", code="http_error",
            failure_class="HTTPError", http_status=403)
        stderr = StringIO()
        with patch.object(receipt, "capture_receipt", side_effect=error), redirect_stderr(stderr):
            self.assertEqual(receipt.main(["11", "unused.json"]), 1)
        self.assertEqual(json.loads(stderr.getvalue()), error.safe_record())
        self.assertNotIn(secret, stderr.getvalue())

    def test_source_capture_diagnostic_preserves_only_safe_phase_and_class(self):
        sha = "a" * 40
        run = {"id": 11, "status": "completed", "conclusion": "success",
            "event": "push", "head_branch": "master", "path": ".github/workflows/ci.yml",
            "workflow_id": 263230665, "head_sha": sha, "run_attempt": 1,
            "repository": {"full_name": "Poryadok/VoiceRoot", "id": 7},
            "head_repository": {"full_name": "Poryadok/VoiceRoot", "id": 7}}
        source_error = receipt.authority.SourceError("private API response body",
            phase="source-archive", code="archive_rejected", failure_class="BadZipFile")
        with tempfile.TemporaryDirectory() as directory, \
                patch.object(receipt.authority, "_headers", return_value={}), \
                patch.object(receipt.authority, "_get", return_value=run), \
                patch.object(receipt.authority, "capture_equivalent_master_source", side_effect=source_error):
            target = Path(directory) / "receipt.json"
            with self.assertRaises(receipt.ReceiptError) as raised:
                receipt.capture_receipt(11, "token", "actor", target)
            self.assertFalse(target.exists())
        self.assertEqual(raised.exception.safe_record(), {"status": "REJECTED",
            "phase": "source-archive", "code": "archive_rejected", "class": "BadZipFile"})
        self.assertNotIn("private API response body", json.dumps(raised.exception.safe_record()))

    def test_registry_status_survives_source_capture_wrapper_without_secret_data(self):
        sha = "a" * 40
        run = {"id": 11, "status": "completed", "conclusion": "success",
            "event": "push", "head_branch": "master", "path": ".github/workflows/ci.yml",
            "workflow_id": 263230665, "head_sha": sha, "run_attempt": 1,
            "repository": {"full_name": "Poryadok/VoiceRoot", "id": 7},
            "head_repository": {"full_name": "Poryadok/VoiceRoot", "id": 7}}
        secret = "private-registry-response"
        registry_error = receipt.ReceiptError(secret, phase="registry-config-redirect-content",
            code="redirect_chain_rejected", failure_class="HTTPError", http_status=307,
            redirect_host_class="known_cdn")

        def source_capture(*args):
            try:
                receipt.authority._resolve_image("matchmaking", sha, 0)
            except Exception as error:
                raise receipt.authority._source_diagnostic(error, "registry-image-matchmaking") from None

        with patch.object(receipt.authority, "_headers", return_value={}), \
                patch.object(receipt.authority, "_get", return_value=run), \
                patch.object(receipt.authority, "capture_equivalent_master_source", side_effect=source_capture), \
                patch.object(receipt, "_image_identity", side_effect=registry_error), \
                tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "receipt.json"
            with self.assertRaises(receipt.ReceiptError) as raised:
                receipt.capture_receipt(11, "token", "actor", target)
            self.assertFalse(target.exists())
        self.assertEqual(raised.exception.safe_record(), {"status": "REJECTED",
            "phase": "registry-config-redirect-content", "code": "redirect_chain_rejected",
            "class": "HTTPError", "http_status": 307, "redirect_host_class": "known_cdn"})
        self.assertNotIn(secret, json.dumps(raised.exception.safe_record()))

    def test_buildx_and_registry_index_mismatch_blocks_before_receipt_write(self):
        sha = "a" * 40
        index = "b" * 64
        source_run = {"id": 11, "status": "completed", "conclusion": "success",
            "event": "push", "head_branch": "master", "path": ".github/workflows/ci.yml",
            "workflow_id": 263230665, "head_sha": sha, "run_attempt": 1,
            "repository": {"full_name": "Poryadok/VoiceRoot", "id": 7},
            "head_repository": {"full_name": "Poryadok/VoiceRoot", "id": 7}}
        base = {"schema": "voice-source-authority-v1", "source_sha": sha,
            "run_id": 11, "run_attempt": 1, "images": {"matchmaking": "image"},
            "verified": True}
        identity = {"image": "image", "index_digest": "sha256:" + index,
            "platform_manifest_digest": "sha256:" + "c" * 64, "config_sha256": "d" * 64}

        def source_capture(_token, _run_id, _sha, _directory):
            receipt.authority._resolve_image("matchmaking", sha, 0)
            return dict(base)

        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "source-authority-result.json"
            with patch.object(receipt.authority, "_headers", return_value={}), \
                    patch.object(receipt.authority, "_get", side_effect=[source_run,
                        {"id": 7}]), \
                    patch.object(receipt.authority, "capture_equivalent_master_source", side_effect=source_capture), \
                    patch.object(receipt, "_verify_ci_build_source"), \
                    patch.object(receipt, "_verified_buildx", return_value=({}, {}, {},
                        {"builder_output_index_sha256": "e" * 64})), \
                    patch.object(receipt, "_image_identity", return_value=identity):
                with self.assertRaises(receipt.ReceiptError):
                    receipt.capture_receipt(11, "token", "actor", target)
            self.assertFalse(target.exists())


if __name__ == "__main__":
    unittest.main()
