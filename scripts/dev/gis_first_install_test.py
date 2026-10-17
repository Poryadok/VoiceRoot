"""Offline contracts for the fixed voice-dev GIS first-install helpers."""
import base64
import copy
import hashlib
import io
import json
import subprocess
import tarfile
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import gis_dev_first_install as gis
import gis_prerequisite
import gis_first_install


def encoded(values):
    return {key: base64.b64encode(value).decode('ascii') for key, value in values.items()}


def buildx_artifact(source, run_id, attempt, *, revision=None, context='src/backend',
                    output_digest=None, started='2026-10-17T00:01:33Z',
                    finished='2026-10-17T00:02:08Z'):
    """Build a minimal authenticated Buildx history-record artifact."""
    revision = revision or source
    config = 'd' * 64
    child = json.dumps({'config': {'digest': 'sha256:' + config}}, separators=(',', ':')).encode()
    child_digest = hashlib.sha256(child).hexdigest()
    image_index = json.dumps({'schemaVersion': 2,
        'mediaType': 'application/vnd.oci.image.index.v1+json',
        'manifests': [{'mediaType': 'application/vnd.oci.image.manifest.v1+json',
            'digest': 'sha256:' + child_digest, 'size': len(child),
            'platform': {'os': 'linux', 'architecture': 'amd64'}}]},
        separators=(',', ':')).encode()
    actual_output_digest = hashlib.sha256(image_index).hexdigest()
    output_digest = output_digest or actual_output_digest
    builder = f'https://github.com/Poryadok/VoiceRoot/actions/runs/{run_id}/attempts/{attempt}'
    provenance = json.dumps({'buildDefinition': {'externalParameters': {
        'request': {'root': {'configSource': {'path': 'Dockerfile'},
            'request': {'args': {'vcs:localdir:context': context,
                'vcs:localdir:dockerfile': 'src/backend/gameintegration',
                'vcs:revision': revision,
                'vcs:source': 'https://github.com/Poryadok/VoiceRoot'}}}}}},
        'runDetails': {'builder': {'id': builder},
            'metadata': {'startedOn': started, 'finishedOn': finished}}},
        separators=(',', ':')).encode()
    blobs = {}

    def blob(value):
        digest = hashlib.sha256(value).hexdigest()
        blobs['blobs/sha256/' + digest] = value
        return {'digest': 'sha256:' + digest, 'size': len(value)}

    config_desc = blob(json.dumps({'FrontendAttrs': {
        'attest:provenance': 'type=provenance,builder-id=' + builder,
        'filename': 'Dockerfile', 'vcs:localdir:context': context,
        'vcs:localdir:dockerfile': 'src/backend/gameintegration',
        'vcs:revision': revision, 'vcs:source': 'https://github.com/Poryadok/VoiceRoot'},
        'Exporters': [{'Type': 'image', 'Attrs': {'name': gis.IMAGE + ':' + source,
            'push': 'true'}}],
        'Result': {'Results': {'0': {'media_type': 'application/vnd.oci.image.index.v1+json',
            'digest': 'sha256:' + output_digest, 'size': len(image_index)}}},
        'DefaultPlatform': 'linux/amd64'}, separators=(',', ':')).encode())
    layers = [dict(blob(image_index), mediaType='application/json',
        annotations={'vnd.export-build.descriptor.mediatype': 'application/vnd.oci.image.index.v1+json'}),
        dict(blob(provenance), mediaType='application/vnd.in-toto+json',
            annotations={'in-toto.io/predicate-type': 'https://slsa.dev/provenance/v1'})]
    record_manifest = json.dumps({'schemaVersion': 2,
        'mediaType': 'application/vnd.oci.image.manifest.v1+json',
        'config': dict(config_desc, mediaType='application/vnd.buildkit.historyrecord.v0'),
        'layers': layers}, separators=(',', ':')).encode()
    record_desc = blob(record_manifest)
    archive_index = json.dumps({'schemaVersion': 2,
        'mediaType': 'application/vnd.oci.image.index.v1+json',
        'manifests': [dict(record_desc, mediaType='application/vnd.oci.image.manifest.v1+json')]},
        separators=(',', ':')).encode()
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode='w:gz') as tar:
        for name, value in [('index.json', archive_index), *blobs.items()]:
            entry = tarfile.TarInfo(name)
            entry.size = len(value)
            entry.mode = 0o600
            tar.addfile(entry, io.BytesIO(value))
    return output.getvalue(), actual_output_digest, child, config, image_index


class FakeKube:
    def __init__(self):
        self.namespace = {'metadata': {'name': 'voice-dev', 'uid': '00000000-0000-0000-0000-000000000001'}}
        self.secrets = {
            'voice-app-secrets': {'apiVersion': 'v1', 'kind': 'Secret',
                'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-dev',
                    'uid': '00000000-0000-0000-0000-000000000002', 'resourceVersion': '12'},
                'type': 'Opaque', 'data': encoded({'POSTGRES_PASSWORD': b'admin', 'KEEP': b'existing'})},
            'voice-media-ca': {'apiVersion': 'v1', 'kind': 'Secret',
                'metadata': {'name': 'voice-media-ca', 'namespace': 'voice-dev',
                    'uid': '00000000-0000-0000-0000-000000000003', 'resourceVersion': '1'},
                'data': encoded({'ca.crt': b'inbound-ca', 'jwks-ca.crt': b'outbound-jwks-ca'})},
            'voice-gameintegration-jwks-tls': {'apiVersion': 'v1', 'kind': 'Secret',
                'metadata': {'name': 'voice-gameintegration-jwks-tls', 'namespace': 'voice-dev',
                    'uid': '00000000-0000-0000-0000-000000000004', 'resourceVersion': '2'},
                'data': encoded({'tls.crt': b'cert', 'tls.key': b'tls-key'})},
        }
        self.writes = []
        self.conflict = False

    def get_namespace(self):
        return copy.deepcopy(self.namespace)

    def get_secret(self, name):
        if name not in self.secrets:
            return None
        return copy.deepcopy(self.secrets[name])

    def create_secret(self, body):
        self.writes.append(('create', body['metadata']['name']))
        if body['metadata']['name'] in self.secrets:
            raise RuntimeError('already exists')
        self.secrets[body['metadata']['name']] = copy.deepcopy(body)

    def replace_secret(self, body, *, expected_uid, expected_resource_version):
        name = body['metadata']['name']
        live = self.secrets[name]
        if (self.conflict or live['metadata']['uid'] != expected_uid
                or live['metadata']['resourceVersion'] != expected_resource_version):
            raise RuntimeError('conflict')
        self.writes.append(('replace', name))
        body = copy.deepcopy(body)
        body['metadata']['resourceVersion'] = str(int(expected_resource_version) + 1)
        self.secrets[name] = body


def materials():
    return {
        'GAME_INTEGRATION_DB_PASSWORD': b'runtime-password',
        'GAME_INTEGRATION_DATABASE_URL': b'postgres://gameintegration_runtime:runtime-password@voice-postgres:5432/game_integration_db?sslmode=disable',
        'voice-gameintegration-principal-signing': {
            'current.pem': b'private-current', 'next.pem': b'private-next', 'active-kid': b'current'},
        'voice-gameintegration-workload-credentials': {
            'GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64': base64.b64encode(b'n'*32),
            'GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64': base64.b64encode(b'r'*32)},
    }


class KubectlOptionalSecretTests(unittest.TestCase):
    def test_installed_kubectl_not_found_contract_returns_none(self):
        calls = []

        def runner(argv, **kwargs):
            calls.append(argv)
            self.assertIn("--ignore-not-found=true", argv)
            return SimpleNamespace(returncode=0, stdout="", stderr="")

        kube = gis.Kubectl(runner=runner)
        self.assertIsNone(kube.get_secret("optional-identity"))
        self.assertEqual(len(calls), 1)

    def test_secret_denials_and_api_failures_do_not_become_absence(self):
        for stderr in (
            'Error from server (Forbidden): secrets "optional-identity" is forbidden',
            'Error from server (Unauthorized): the server requires credentials',
            'Error from server (Conflict): resource version changed',
            'Unable to connect to the server: timeout',
        ):
            def runner(argv, **kwargs):
                return SimpleNamespace(returncode=1, stdout="", stderr=stderr)

            with self.subTest(stderr=stderr.split(":", 1)[0]):
                with self.assertRaises(gis.KubeError):
                    gis.Kubectl(runner=runner).get_secret("optional-identity")

    def test_nonempty_nonobject_success_responses_do_not_become_absence(self):
        for stdout in ("null", "[]", '"secret"'):
            def runner(argv, **kwargs):
                return SimpleNamespace(returncode=0, stdout=stdout, stderr="")

            with self.subTest(stdout=stdout):
                with self.assertRaises(gis.Blocked):
                    gis.Kubectl(runner=runner).get_secret("optional-identity")


class KubectlFailureReceiptTests(unittest.TestCase):
    def test_failure_retains_only_safe_exit_and_category(self):
        sensitive = "do-not-retain-this-payload"

        def runner(_argv, **_kwargs):
            return SimpleNamespace(returncode=1, stdout="", stderr=(
                "error parsing jsonpath {.metadata.name,.metadata.uid}: " + sensitive))

        with self.assertRaises(gis.KubeError) as caught:
            gis.Kubectl(runner=runner).run(["create", "-f", "/dev/stdin", "-o",
                "jsonpath-as-json={.metadata.name,.metadata.uid}"], body={"kind": "ConfigMap"})
        self.assertEqual(caught.exception.returncode, 1)
        self.assertEqual(caught.exception.category, "invalid_output_format")
        self.assertNotIn(sensitive, str(caught.exception))


class KubectlWaitAdapterContractTests(unittest.TestCase):
    def test_exact_prerequisite_wait_decodes_completed_json_object(self):
        job = {"apiVersion": "batch/v1", "kind": "Job", "metadata": {
            "name": "voice-provision-gameintegration-runtime-role", "namespace": gis.NAMESPACE,
            "uid": "00000000-0000-0000-0000-000000000001"},
            "status": {"succeeded": 1, "conditions": [{"type": "Complete", "status": "True"}]}}
        calls = []
        wait_response = [job]

        def runner(argv, **kwargs):
            calls.append((argv, kwargs))
            return subprocess.CompletedProcess(argv, 0, json.dumps(wait_response[0]), "")

        result = gis.Kubectl(runner=runner).run([
            "wait", "--for=condition=complete", "job/voice-provision-gameintegration-runtime-role",
            "--timeout=600s", "-o", "json"], timeout=610)
        self.assertEqual(result, job)
        argv, kwargs = calls[0]
        self.assertEqual(argv, ["kubectl", "--namespace", gis.NAMESPACE,
            "wait", "--for=condition=complete", "job/voice-provision-gameintegration-runtime-role",
            "--timeout=600s", "-o", "json"])
        self.assertIsNone(kwargs["input"])
        self.assertEqual(kwargs["timeout"], 610)

    def test_wait_rejects_other_names_and_any_option_or_timeout_drift_before_runner(self):
        invalid = [
            ["wait", "--for=condition=complete", "job/other", "--timeout=600s", "-o", "json"],
            ["wait", "--for=condition=Failed", "job/voice-provision-gameintegration-runtime-role", "--timeout=600s", "-o", "json"],
            ["wait", "--for=condition=complete", "job/voice-provision-gameintegration-runtime-role", "--timeout=601s", "-o", "json"],
            ["wait", "--for=condition=complete", "job/voice-provision-gameintegration-runtime-role", "--timeout=600s", "-o", "yaml"],
            ["wait", "--for=condition=complete", "job/voice-provision-gameintegration-runtime-role", "--timeout=600s", "-o", "json", "--all-namespaces"],
        ]
        calls = []
        client = gis.Kubectl(runner=lambda argv, **kwargs: calls.append(argv))
        for args in invalid:
            with self.subTest(args=args), self.assertRaisesRegex(gis.Blocked, "request_invalid"):
                client.run(args, timeout=610)
        with self.assertRaisesRegex(gis.Blocked, "request_invalid"):
            client.run(["wait", "--for=condition=complete", "job/voice-provision-gameintegration-runtime-role",
                "--timeout=600s", "-o", "json"], timeout=30)
        with self.assertRaisesRegex(gis.Blocked, "request_invalid"):
            client.run(["wait", "--for=condition=complete", "job/voice-provision-gameintegration-runtime-role",
                "--timeout=600s", "-o", "json"], timeout=610.0)
        with self.assertRaisesRegex(gis.Blocked, "request_invalid"):
            client.run(["wait", "--for=condition=complete", "job/voice-provision-gameintegration-runtime-role",
                "--timeout=600s", "-o", "json"], body={"kind": "Job"}, timeout=610)
        self.assertEqual(calls, [])

    def test_wait_nonzero_and_non_json_results_fail_closed(self):
        for result, error in ((subprocess.CompletedProcess([], 1, "", "Forbidden"), gis.KubeError),
                (subprocess.CompletedProcess([], 0, "not-json", ""), gis.Blocked)):
            with self.subTest(error=error), self.assertRaises(error):
                gis.Kubectl(runner=lambda *_a, **_k: result).run([
                    "wait", "--for=condition=complete", "job/voice-provision-gameintegration-runtime-role",
                    "--timeout=600s", "-o", "json"], timeout=610)


class PrerequisiteSecretProbeAdapterTests(unittest.TestCase):
    def run_preflight(self, outputs):
        calls = []

        def runner(argv, **kwargs):
            calls.append(argv)
            if len(calls) > len(outputs):
                return SimpleNamespace(returncode=0, stdout="true", stderr="")
            output, status = outputs[len(calls) - 1]
            return SimpleNamespace(returncode=status, stdout=output,
                stderr="Error from server (Forbidden)" if status else "")

        client = gis.Kubectl(runner=runner)

        class Kube:
            def run(self, args, **kwargs):
                return client.run(args, **kwargs)

            def secret_meta(self, name):
                return {"uid": "secret-uid", "resourceVersion": "12"}

            def get(self, kind, name):
                return {"metadata": {"uid": "config-uid", "resourceVersion": "13"},
                    "data": {"POSTGRES_USER": "voice"}}

        identity = {"namespace_uid": "namespace-uid", "service_uid": "service-uid",
            "service_spec_sha256": "a" * 64, "endpoints": [{"uid": "slice-uid", "pod_uids": ["pod-uid"]}]}
        plan = {"phase": "PLANNED", "inputs_sha256": "b" * 64, "namespace": "voice-dev"}
        with patch.object(gis_prerequisite, "_endpoint_identity", return_value=identity):
            result = gis_prerequisite.preflight(Kube(), plan)
        return result, calls

    def test_real_probe_command_maps_only_exact_true_false_for_consumer(self):
        identity, calls = self.run_preflight([("true", 0)] * 3)
        self.assertEqual(identity["namespace_uid"], "namespace-uid")
        self.assertEqual(len(calls), 3)
        self.assertTrue(all(call[4] == "secret" and call[5] == "voice-app-secrets"
            and call[6] == "-o" and call[7].startswith("go-template=") for call in calls))
        self.assertIsInstance(gis.Kubectl(runner=lambda *_a, **_k:
            SimpleNamespace(returncode=0, stdout="true", stderr="")).run(["get", "pods"]), str)

    def test_false_empty_malformed_and_nonzero_probe_results_fail_closed(self):
        for output, status, expected in (("false", 0, gis_prerequisite.Blocked),
                ("", 0, gis.Blocked), ("True", 0, gis.Blocked), ("true\n", 0, gis.Blocked),
                ("", 1, gis.KubeError)):
            with self.subTest(output=output, status=status), self.assertRaises(expected):
                self.run_preflight([(output, status)])


class PartialOperationContinuationTests(unittest.TestCase):
    def fixture(self):
        plan = {"phase": "PLANNED", "source_sha": gis._PARTIAL_SOURCE_SHA,
            "namespace_uid": "00000000-0000-0000-0000-000000000001",
            "inputs": {}, "inputs_sha256": gis._digest({}), "source_capture": {},
            "source_plan_sha256": "a" * 64}
        receipt = {name: {"uid": "00000000-0000-0000-0000-00000000000" + str(index),
            "resourceVersion": str(index)} for index, name in enumerate(
                (gis.APP_SECRET, gis.SIGNER_SECRET, gis.WORKLOAD_SECRET), 2)}
        return {"schema": "voice-dev-gis-first-install-state-v1",
            "operation": gis._PARTIAL_OPERATION, "source_sha": gis._PARTIAL_SOURCE_SHA,
            "phase": "SECRETS_APPLIED", "secrets_applied": True,
            "namespace_uid": plan["namespace_uid"], "database_identity": {},
            "secret_write_receipt": receipt, "plan": plan}

    def test_only_exact_frozen_pre_job_state_is_eligible(self):
        state = self.fixture()
        self.assertTrue(gis._partial_state_admissible(state))
        self.assertTrue(gis._partial_state_compatible(copy.deepcopy(state), state))
        for mutate in (
            lambda row: row.update(operation="other"),
            lambda row: row.update(phase="READY"),
            lambda row: row["plan"].update(phase="CREATING"),
            lambda row: row["plan"].update(created={"job/x": {}}),
            lambda row: row["plan"].update(prerequisite_receipt={}),
            lambda row: row["secret_write_receipt"].pop(gis.APP_SECRET),
        ):
            changed = copy.deepcopy(state)
            mutate(changed)
            with self.assertRaises(gis.Blocked):
                gis._partial_state_admissible(changed)

    def test_c2_sidecar_state_accepts_bounded_create_diagnostic_only(self):
        state = self.fixture()
        plan = state["plan"]
        plan["inputs"] = {"configmaps": {"provision": "voice-gameintegration-runtime-role-sql",
            "migrations": "voice-gameintegration-db-migrations",
            "verify": "voice-gameintegration-db-verifier-sql"},
            "jobs": {"provision": "voice-provision-gameintegration-runtime-role",
                "migrate": "voice-migrate-gameintegration-db",
                "verify": "voice-verify-gameintegration-db"}}
        plan.update(prerequisite_phase="CREATING", created={}, jobs={})
        self.assertTrue(gis._partial_state_compatible(copy.deepcopy(state), state))
        plan["create_failures"] = {"configmap/voice-gameintegration-runtime-role-sql": {
            "returncode": 1, "category": "invalid_output_format"}}
        self.assertTrue(gis._partial_state_compatible(copy.deepcopy(state), state))
        plan["create_failures"]["job/unreviewed"] = {
            "returncode": 1, "category": "invalid_output_format"}
        with self.assertRaises(gis.Blocked):
            gis._partial_state_compatible(copy.deepcopy(state), state)

    def c5_checkpoint_fixture(self):
        state = self.fixture()
        state["namespace_uid"] = "15950fc8-0392-4053-9156-e95e78b596f9"
        state["database_identity"] = {"namespace_uid": state["namespace_uid"],
            "service_uid": "17d50ed9-8b27-462d-91e9-ba8fbaa785aa",
            "service_spec_sha256": "783b9fca489787d04e8bd1e04fa0aa518e2d215b40a638784139443e9b1e4c41",
            "endpoints": [{"uid": "e748b3e9-2e67-4719-8e1f-f59da9b3e558",
                "pod_uids": ["fd904063-443e-46a7-94cd-8426e16fc040"]}]}
        state["secret_write_receipt"] = {
            gis.APP_SECRET: {"uid": "34cfb82d-ffd2-4df4-9826-9d97139687d1", "resourceVersion": "3378571"},
            gis.SIGNER_SECRET: {"uid": "a3318d27-b784-4bdd-8cb3-409012b5bab8", "resourceVersion": "3378572"},
            gis.WORKLOAD_SECRET: {"uid": "8964a6cd-b489-4478-8f06-5f54ca0585a6", "resourceVersion": "3378573"}}
        source_sha = gis._PARTIAL_SOURCE_SHA
        image = "ghcr.io/poryadok/voiceroot/gameintegration@sha256:482eb267824cffc4cebcae958e4ec7126ffdeaaea632f40c26bb1c6baf71ea96"
        state["source_build_receipt"] = {"source_sha": source_sha, "image_ref": image,
            "builder_tree": "e55c42d9c4d4a5d87e6211406a4b8d4cb2454a76"}
        plan = state["plan"]
        plan.update({"operation": state["operation"], "namespace_uid": state["namespace_uid"],
            "source_sha": source_sha, "source_build_receipt": copy.deepcopy(state["source_build_receipt"]),
            "source_plan_sha256": "16180026a6b244940b93523b058b94ab46efcb665c49e6369a57f262ef5f392e",
            "source_capture": {}, "inputs": {"configmaps": {
                "provision": "voice-gameintegration-runtime-role-sql",
                "migrations": "voice-gameintegration-db-migrations",
                "verify": "voice-gameintegration-db-verifier-sql"},
                "jobs": {"provision": "voice-provision-gameintegration-runtime-role",
                    "migrate": "voice-migrate-gameintegration-db", "verify": "voice-verify-gameintegration-db"},
                "job_images": {"provision": {"wait-postgres": "postgres", "provision": "postgres"},
                    "migrate": {"wait-postgres": "postgres", "migrate": "migrate"},
                    "verify": {"wait-postgres": "postgres", "verify": "postgres"}}},
            "inputs_sha256": "ed223e8ce91e3d44d6ce0e0c860529a76b7977d7f7f4baddb4d80fce318ba64b",
            "prerequisite_phase": "CREATING", "created": {
                "configmap/voice-gameintegration-runtime-role-sql": {
                    "uid": "0a064b6f-2aff-4189-9aa1-1f0c6d3edc90",
                    "spec_sha256": "426cea57713d205f295d8bf95cd49d3d5632c35c2d933dbe6d78b1934603a6dd"},
                "job/voice-provision-gameintegration-runtime-role": {
                    "uid": "e4d29319-b765-4d19-9e89-05dce1dbc862",
                    "spec_sha256": "5f95a29493a4eb0b2580899d48c0afc54740a3cd792453897eff876de9ec8ad4"}},
            "jobs": {}})
        state["plan_phase"] = "PLANNED"
        previous_binding = {"old_source_sha": source_sha,
            "candidate_source_sha": gis._PARTIAL_C5_CANDIDATE_SHA,
            "changed_files": {
                "scripts/dev/gis_dev_first_install.py": "8b253467164561b0e08a9d3fe5def79406a22217f26573cd567a8e4a1daa46d5",
                "scripts/dev/gis_first_install_test.py": "74e2aec39f2fa2b53b7b957140fb89be7216321eaf6706ae6a89c040adb5ff2f",
                "scripts/staging/nats-rollout-preservation/gis_prerequisite.py": "f64496c12a62608a1e8b53c9fae2c41e1d3c92dafdbb9a1301627750dbd8acfd",
                "scripts/staging/nats-rollout-preservation/gis_prerequisite_test.py": "9e1f22b6b267ac3a66cd889622e550c673975c7e86f351e07b8bc183258c710c"},
            "backend_tree": "e55c42d9c4d4a5d87e6211406a4b8d4cb2454a76",
            "build_receipt_sha256": "d90b8b89ae894159c4bc60126dad791f34d4a61feab78dd866c7d2d5d17d711d",
            "image_ref": image,
            "source_plan_sha256": plan["source_plan_sha256"],
            "source_capture_sha256": gis._digest({}), "inputs_sha256": plan["inputs_sha256"]}
        fingerprints = {"secret_metadata_fingerprint_map_sha256": "a7c44b1d7cac81ad580fcc37cda2605087c818ab70668f07a3799efdbec43920"}
        previous_v2 = {"schema": "voice-dev-gis-partial-continuation-v2",
            "original_journal_sha256": gis._PARTIAL_JOURNAL_SHA256,
            "previous_sidecar_sha256": gis._PARTIAL_V1_SIDECAR_SHA256,
            "binding": previous_binding, "authorization_sha256": gis._PARTIAL_C5_AUTH_SHA256,
            "secret_fingerprints_sha256": gis._digest(fingerprints), "state": copy.deepcopy(state)}
        target_binding = copy.deepcopy(previous_binding)
        target_binding.update(candidate_source_sha="26e713b31fc6efd7b43920942c8f64becc440986",
            changed_files={
                "scripts/dev/gis_dev_first_install.py": "f5625fdd432a53b9e4e3f2c3308fb14b05ac3eb6ad188f47f3ed958024554987",
                "scripts/dev/gis_first_install_test.py": "13d64f07ebff3a181bf744afc5be2994385b35508ffb462d0e573f736d5b51dd",
                "scripts/staging/nats-rollout-preservation/gis_prerequisite.py": "1d2bb02a4c97ff4b027099b768b16eddb770570202a9d2c75896bedc7d7d5c70",
                "scripts/staging/nats-rollout-preservation/gis_prerequisite_test.py": "f56b69be0d32e5bc8e3ab22f52f054f772b082233370ade02989a270d51237da"})
        target_binding["source_capture_sha256"] = gis._digest({})
        checkpoint = {"created_records": copy.deepcopy(plan["created"]),
            "database_identity": copy.deepcopy(state["database_identity"]),
            "database_owner": "voice", "database_catalog": "false|true",
            "operation": gis._PARTIAL_OPERATION,
            "secret_fingerprints_sha256": previous_v2["secret_fingerprints_sha256"],
            "namespace_uid": state["namespace_uid"]}
        auth = {"schema": "voice-dev-gis-recovery-transition-authorization-v1",
            "operation": gis._PARTIAL_OPERATION, "namespace_uid": state["namespace_uid"],
            "original_journal_sha256": gis._PARTIAL_JOURNAL_SHA256,
            "immutable_v1_sha256": gis._PARTIAL_V1_SIDECAR_SHA256,
            "previous_v2_sha256": gis._PARTIAL_C5_V2_SHA256,
            "previous_v2_binding_sha256": gis._digest(previous_binding),
            "previous_v2_state_sha256": gis._digest(state),
            "previous_authorization_sha256": gis._PARTIAL_C5_AUTH_SHA256,
            "previous_candidate_source_sha": gis._PARTIAL_C5_CANDIDATE_SHA,
            "previous_changed_files": copy.deepcopy(previous_binding["changed_files"]),
            "previous_secret_fingerprints_sha256": previous_v2["secret_fingerprints_sha256"],
            "target_source_sha": target_binding["candidate_source_sha"],
            "target_changed_files": copy.deepcopy(target_binding["changed_files"]),
            "protected_binding": {key: target_binding[key] for key in (
                "old_source_sha", "backend_tree", "build_receipt_sha256", "image_ref",
                "source_plan_sha256", "source_capture_sha256", "inputs_sha256")},
            "partial_checkpoint": checkpoint}
        auth_raw = (json.dumps(auth, sort_keys=True, separators=(",", ":")) + "\n").encode()
        expected_sha = hashlib.sha256(auth_raw).hexdigest()
        proof = {"name": "voice-provision-gameintegration-runtime-role",
            "uid": "e4d29319-b765-4d19-9e89-05dce1dbc862",
            "spec_sha256": "5f95a29493a4eb0b2580899d48c0afc54740a3cd792453897eff876de9ec8ad4",
            "pod_uid": "00000000-0000-0000-0000-000000000123",
            "image_ids": {"provision": "docker-pullable://postgres@sha256:" + "e"*64,
                "wait-postgres": "docker-pullable://postgres@sha256:" + "e"*64},
            "complete": True, "succeeded": 1, "completion_time": "2026-10-17T00:00:00Z"}
        state_v3 = copy.deepcopy(state)
        state_v3["plan"]["jobs"]["provision"] = proof
        current = {"schema": "voice-dev-gis-partial-continuation-v3",
            "original_journal_sha256": gis._PARTIAL_JOURNAL_SHA256,
            "immutable_v1_sha256": gis._PARTIAL_V1_SIDECAR_SHA256,
            "previous_v2_sha256": gis._PARTIAL_C5_V2_SHA256,
            "previous_v2_binding_sha256": gis._digest(previous_binding),
            "previous_v2_state_sha256": gis._digest(state),
            "previous_authorization_sha256": gis._PARTIAL_C5_AUTH_SHA256,
            "previous_candidate_source_sha": gis._PARTIAL_C5_CANDIDATE_SHA,
            "previous_changed_files": copy.deepcopy(previous_binding["changed_files"]),
            "source_transition_authorization_sha256": expected_sha,
            "binding": target_binding,
            "secret_fingerprints_sha256": previous_v2["secret_fingerprints_sha256"],
            "state": state_v3}
        return state, previous_v2, target_binding, fingerprints, auth, expected_sha, current

    def test_c5_checkpoint_transition_requires_exact_dispatcher_pins_and_preserves_job_proof(self):
        state, previous_v2, binding, fingerprints, auth, auth_sha, current = self.c5_checkpoint_fixture()
        receipt_raw = (json.dumps(auth, sort_keys=True, separators=(",", ":")) + "\n").encode()
        self.assertEqual(hashlib.sha256(receipt_raw).hexdigest(), auth_sha)
        with tempfile.TemporaryDirectory() as directory:
            receipt_path = Path(directory) / "authorization.json"
            receipt_path.write_bytes(receipt_raw)
            receipt_path.chmod(0o600)
            parsed_auth, verified_raw_sha = gis._read_transition_authorization(receipt_path, auth_sha)
            self.assertEqual(parsed_auth, auth)
            self.assertEqual(verified_raw_sha, auth_sha)
            with self.assertRaises(gis.Blocked):
                gis._read_transition_authorization(receipt_path, "0" * 64)
            receipt_path.write_bytes(receipt_raw + b" ")
            receipt_path.chmod(0o600)
            with self.assertRaises(gis.Blocked):
                gis._read_transition_authorization(receipt_path, auth_sha)
            reformatted = json.dumps(auth, indent=2).encode()
            receipt_path.write_bytes(reformatted)
            receipt_path.chmod(0o600)
            reformatted_auth, reformatted_sha = gis._read_transition_authorization(
                receipt_path, hashlib.sha256(reformatted).hexdigest())
            self.assertEqual(reformatted_auth, auth)
        with patch.object(gis, "_git_text", return_value=binding["candidate_source_sha"]):
            self.assertTrue(gis._partial_v3_admissible(current, previous_v2,
                gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                binding, fingerprints, state, parsed_auth, verified_raw_sha,
                verified_raw_sha, Path(".")))
            resumed = copy.deepcopy(current)
            resumed["state"]["plan"]["phase"] = "CREATING"
            resumed["state"]["plan_phase"] = "CREATING"
            self.assertTrue(gis._partial_v3_admissible(resumed, previous_v2,
                gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                binding, fingerprints, state, parsed_auth, verified_raw_sha,
                verified_raw_sha, Path(".")))
            for bad_v2_sha, bad_old_auth, bad_auth, bad_current, bad_binding in (
                ("0"*64, gis._PARTIAL_C5_AUTH_SHA256, auth, current, binding),
                (gis._PARTIAL_C5_V2_SHA256, "0"*64, auth, current, binding),
                (gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                    {**auth, "target_source_sha": "b"*40}, current, binding),
                (gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256, auth,
                    {**current, "previous_v2_sha256": "0"*64}, binding),
                (gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256, auth,
                    current, {**binding, "candidate_source_sha": "b"*40}),
            ):
                with self.assertRaises(gis.Blocked):
                    gis._partial_v3_admissible(bad_current, previous_v2,
                        bad_v2_sha, bad_old_auth, bad_binding, fingerprints, state,
                        bad_auth, verified_raw_sha, verified_raw_sha, Path("."))
            with self.assertRaises(gis.Blocked):
                gis._partial_v3_admissible(current, previous_v2,
                    gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                    binding, fingerprints, state, parsed_auth, "0" * 64,
                    verified_raw_sha, Path("."))
            invalid = copy.deepcopy(current)
            invalid["state"]["plan"]["jobs"]["provision"]["uid"] = "foreign"
            with self.assertRaises(gis.Blocked):
                gis._partial_v3_admissible(invalid, previous_v2,
                    gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                    binding, fingerprints, state, parsed_auth, verified_raw_sha,
                    verified_raw_sha, Path("."))
            invalid = copy.deepcopy(current)
            invalid["state"]["plan"]["created"][
                "configmap/voice-gameintegration-runtime-role-sql"]["uid"] = "foreign"
            with self.assertRaises(gis.Blocked):
                gis._partial_v3_admissible(invalid, previous_v2,
                    gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                    binding, fingerprints, state, parsed_auth, verified_raw_sha,
                    verified_raw_sha, Path("."))
            changed_v2 = copy.deepcopy(previous_v2)
            changed_v2["binding"]["changed_files"][
                "scripts/dev/gis_dev_first_install.py"] = "0" * 64
            with self.assertRaises(gis.Blocked):
                gis._partial_v3_admissible(current, changed_v2,
                    gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                    binding, fingerprints, state, parsed_auth, verified_raw_sha,
                    verified_raw_sha, Path("."))

    def test_c5_to_v3_transition_separates_frozen_c5_hashes_from_c8_target_hashes(self):
        state, previous_v2, binding, fingerprints, auth, auth_sha, _ = self.c5_checkpoint_fixture()
        auth_raw = (json.dumps(auth, sort_keys=True, separators=(",", ":")) + "\n").encode()
        with tempfile.TemporaryDirectory() as directory:
            auth_path = Path(directory) / "authorization.json"
            auth_path.write_bytes(auth_raw)
            auth_path.chmod(0o600)
            parsed_auth, verified_raw_sha = gis._read_transition_authorization(auth_path, auth_sha)
        plan = previous_v2["state"]["plan"]
        import gis_prerequisite
        configmap = {"apiVersion": "v1", "kind": "ConfigMap", "metadata": {
            "name": "voice-gameintegration-runtime-role-sql"}, "data": {"sql": "metadata-only"}}
        job = {"apiVersion": "batch/v1", "kind": "Job", "metadata": {
            "name": "voice-provision-gameintegration-runtime-role"}, "spec": {"template": {}}}
        expected_records = plan["created"]
        live_objects = [
            {**copy.deepcopy(configmap), "metadata": {**configmap["metadata"],
                "namespace": gis.NAMESPACE, "uid": expected_records[
                    "configmap/voice-gameintegration-runtime-role-sql"]["uid"],
                "labels": {gis_prerequisite.OP_LABEL: gis._PARTIAL_OPERATION}}},
            {**copy.deepcopy(job), "metadata": {**job["metadata"],
                "namespace": gis.NAMESPACE, "uid": expected_records[
                    "job/voice-provision-gameintegration-runtime-role"]["uid"],
                "labels": {gis_prerequisite.OP_LABEL: gis._PARTIAL_OPERATION}}},
        ]
        kube = Mock()
        kube.run.side_effect = [
            {"items": live_objects}, "", ""]
        expected_identity = copy.deepcopy(state["database_identity"])
        proof = {"name": "voice-provision-gameintegration-runtime-role",
            "uid": expected_records["job/voice-provision-gameintegration-runtime-role"]["uid"],
            "spec_sha256": expected_records["job/voice-provision-gameintegration-runtime-role"]["spec_sha256"],
            "pod_uid": "00000000-0000-0000-0000-000000000123",
            "image_ids": {"provision": "docker-pullable://postgres@sha256:" + "e"*64,
                "wait-postgres": "docker-pullable://postgres@sha256:" + "e"*64},
            "complete": True, "succeeded": 1,
            "completion_time": "2026-10-17T00:00:00Z"}
        with patch.object(gis, "_git_text", return_value=binding["candidate_source_sha"]), \
             patch.object(gis_prerequisite, "_documents", return_value=({"provision": configmap}, {"provision": job})), \
             patch.object(gis_prerequisite, "_spec_digest", side_effect=[
                 expected_records["configmap/voice-gameintegration-runtime-role-sql"]["spec_sha256"],
                 expected_records["configmap/voice-gameintegration-runtime-role-sql"]["spec_sha256"],
                 expected_records["job/voice-provision-gameintegration-runtime-role"]["spec_sha256"]]), \
             patch.object(gis_prerequisite, "_job_live_matches", return_value=True), \
             patch.object(gis_prerequisite, "_verify_job", return_value=(proof, {})), \
             patch.object(gis, "_database_exists_readonly", return_value=expected_identity):
            transitioned = gis._partial_c5_checkpoint_to_v3(previous_v2,
                gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                binding, fingerprints, state, parsed_auth, verified_raw_sha,
                verified_raw_sha, kube, Path("."), {})
            self.assertEqual(kube.run.call_count, 3)
            kube.run.reset_mock()
            with self.assertRaises(gis.Blocked):
                gis._partial_c5_checkpoint_to_v3(previous_v2,
                    gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                    binding, fingerprints, state, parsed_auth, "0" * 64,
                    auth_sha, kube, Path("."), {})
            kube.run.assert_not_called()
            for old_v2, target, target_auth, target_auth_sha in (
                (self._mutated(previous_v2, lambda row: row["binding"]["changed_files"].update(
                    {"scripts/dev/gis_dev_first_install.py": "0" * 64})), binding, auth, auth_sha),
                (previous_v2, self._mutated(binding, lambda row: row["changed_files"].update(
                    {"scripts/dev/gis_dev_first_install.py": "0" * 64})), auth, auth_sha),
                (previous_v2, binding,
                    {**auth, "target_source_sha": "0" * 40}, None),
            ):
                kube.run.reset_mock()
                if target_auth_sha is None:
                    target_auth_sha = auth_sha
                with self.assertRaises(gis.Blocked):
                    gis._partial_c5_checkpoint_to_v3(old_v2,
                        gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                        target, fingerprints, state, target_auth, verified_raw_sha, target_auth_sha,
                        kube, Path("."), {})
                kube.run.assert_not_called()

        self.assertEqual(transitioned["schema"], "voice-dev-gis-partial-continuation-v3")
        self.assertEqual(transitioned["binding"], binding)
        self.assertEqual(transitioned["previous_changed_files"], gis._PARTIAL_C5_CHANGED_FILE_SHA256)
        self.assertEqual(transitioned["state"]["plan"]["jobs"]["provision"], proof)
        with patch.object(gis, "_git_text", return_value=binding["candidate_source_sha"]):
            self.assertTrue(gis._partial_v3_admissible(transitioned, previous_v2,
                gis._PARTIAL_C5_V2_SHA256, gis._PARTIAL_C5_AUTH_SHA256,
                binding, fingerprints, previous_v2["state"], parsed_auth,
                verified_raw_sha, verified_raw_sha, Path(".")))
        self.assertEqual(kube.run.call_count, 0)

    @staticmethod
    def _mutated(value, mutation):
        result = copy.deepcopy(value)
        mutation(result)
        return result

    def test_frozen_c2_sidecar_transitions_to_separate_v2_current_receipt(self):
        state = self.fixture()
        state["plan_phase"] = "PLANNED"
        state["plan"].update(inputs={"configmaps": {"provision": "voice-gameintegration-runtime-role-sql",
            "migrations": "voice-gameintegration-db-migrations",
            "verify": "voice-gameintegration-db-verifier-sql"},
            "jobs": {"provision": "voice-provision-gameintegration-runtime-role",
                "migrate": "voice-migrate-gameintegration-db", "verify": "voice-verify-gameintegration-db"}},
            inputs_sha256=gis._digest({"configmaps": {"provision": "voice-gameintegration-runtime-role-sql",
                "migrations": "voice-gameintegration-db-migrations", "verify": "voice-gameintegration-db-verifier-sql"},
                "jobs": {"provision": "voice-provision-gameintegration-runtime-role",
                    "migrate": "voice-migrate-gameintegration-db", "verify": "voice-verify-gameintegration-db"}}),
            prerequisite_phase="CREATING", created={}, jobs={})
        c2_binding = {"old_source_sha": gis._PARTIAL_SOURCE_SHA,
            "candidate_source_sha": "db8a65b35d38406490e3c8c991866ac22619b021",
            "changed_files": {
                "scripts/dev/gis_dev_first_install.py": "533298f00704d464a6c914d73df828300787204ae4d35c9535a3e818883a727f",
                "scripts/dev/gis_first_install_test.py": "ddbfe12f578c26c3c19dc74b2e1f3c0cd3e88a3abe88c6a3dff8dada76248aaf",
                "scripts/staging/nats-rollout-preservation/gis_prerequisite.py": "375f5cd6ea7363193766edccec05bf3a42f4a73eb4fc6f0e8693bc6b1485dc64",
                "scripts/staging/nats-rollout-preservation/gis_prerequisite_test.py": "e4f7ac510e31135092884fd22b32ed37251c3d05718ad8c4b0aa2e3486599ca2"},
            "backend_tree": "e55c42d9c4d4a5d87e6211406a4b8d4cb2454a76",
            "build_receipt_sha256": "d90b8b89ae894159c4bc60126dad791f34d4a61feab78dd866c7d2d5d17d711d",
            "image_ref": "ghcr.io/poryadok/voiceroot/gameintegration@sha256:482eb267824cffc4cebcae958e4ec7126ffdeaaea632f40c26bb1c6baf71ea96",
            "source_plan_sha256": "16180026a6b244940b93523b058b94ab46efcb665c49e6369a57f262ef5f392e",
            "source_capture_sha256": "f88fab1c0849a42e18399ef1dd87f01983a20bd5536be6885da996618ae14954",
            "inputs_sha256": "ed223e8ce91e3d44d6ce0e0c860529a76b7977d7f7f4baddb4d80fce318ba64b"}
        legacy = {"schema": "voice-dev-gis-partial-continuation-v1",
            "original_journal_sha256": gis._PARTIAL_JOURNAL_SHA256,
            "binding": c2_binding, "secret_fingerprints": {"app": "a" * 64},
            "state": copy.deepcopy(state)}
        current_binding = copy.deepcopy(c2_binding)
        current_binding.update(candidate_source_sha="f7e02d235b48e8e0b08e91208891e8d788dfeea6",
            changed_files={name: "b" * 64 for name in gis._PARTIAL_CHANGED_PATHS})
        fingerprints = copy.deepcopy(legacy["secret_fingerprints"])
        sidecar_sha = "46994d2e66cb2376b1ebafb79510ef0b46ddb94ecc376bcc16fc606c4c28720a"
        self.assertEqual(gis._PARTIAL_V1_SIDECAR_SHA256, sidecar_sha)
        authorization = {"schema": "voice-dev-gis-recovery-authorization-v1",
            "operation": gis._PARTIAL_OPERATION, "namespace_uid": state["namespace_uid"],
            "original_journal_sha256": gis._PARTIAL_JOURNAL_SHA256,
            "previous_sidecar_sha256": sidecar_sha,
            "target_source_sha": current_binding["candidate_source_sha"],
            "target_changed_files": copy.deepcopy(current_binding["changed_files"]),
            "protected_binding": {key: current_binding[key] for key in ("old_source_sha", "backend_tree",
                "build_receipt_sha256", "image_ref", "source_plan_sha256", "source_capture_sha256", "inputs_sha256")}}
        authorization_sha = gis._digest(authorization)

        current = gis._partial_v1_transition(legacy, sidecar_sha, current_binding,
            fingerprints, state, authorization, authorization_sha)
        self.assertEqual(current["schema"], "voice-dev-gis-partial-continuation-v2")
        self.assertEqual(current["previous_sidecar_sha256"], sidecar_sha)
        self.assertEqual(current["binding"], current_binding)
        self.assertNotIn("secret_fingerprints", current)
        self.assertEqual(current["secret_fingerprints_sha256"], gis._digest(fingerprints))
        self.assertEqual(legacy["binding"], c2_binding)
        self.assertTrue(gis._partial_v2_admissible(current, legacy, sidecar_sha,
            current_binding, fingerprints, state, authorization, authorization_sha))

        resumed = copy.deepcopy(current)
        resumed["state"]["plan"]["phase"] = "CREATING"
        resumed["state"]["plan_phase"] = "CREATING"
        self.assertTrue(gis._partial_v2_admissible(resumed, legacy, sidecar_sha,
            current_binding, fingerprints, state, authorization, authorization_sha))

        for bad_legacy, bad_hash, bad_binding, bad_fingerprints in (
            (legacy, "0" * 64, current_binding, fingerprints),
            ({**legacy, "binding": {**c2_binding, "candidate_source_sha": "1" * 40}}, sidecar_sha, current_binding, fingerprints),
            ({**legacy, "binding": {**c2_binding, "changed_files": {**c2_binding["changed_files"],
                "scripts/dev/gis_first_install_test.py": "0" * 64}}}, sidecar_sha, current_binding, fingerprints),
            (legacy, sidecar_sha, {**current_binding,
                "candidate_source_sha": "db8a65b35d38406490e3c8c991866ac22619b021"}, fingerprints),
            (legacy, sidecar_sha, {**current_binding, "changed_files": {"scripts/unknown.py": "b" * 64}}, fingerprints),
            (legacy, sidecar_sha, {**current_binding, "old_source_sha": "3" * 40}, fingerprints),
            (legacy, sidecar_sha, {**current_binding, "backend_tree": "4" * 40}, fingerprints),
            (legacy, sidecar_sha, {**current_binding, "build_receipt_sha256": "5" * 64}, fingerprints),
            (legacy, sidecar_sha, {**current_binding, "image_ref": "image@sha256:" + "3" * 64}, fingerprints),
            (legacy, sidecar_sha, {**current_binding, "source_plan_sha256": "6" * 64}, fingerprints),
            (legacy, sidecar_sha, {**current_binding, "source_capture_sha256": "7" * 64}, fingerprints),
            (legacy, sidecar_sha, {**current_binding, "inputs_sha256": "8" * 64}, fingerprints),
            (legacy, sidecar_sha, current_binding, {"app": "d" * 64}),
        ):
            with self.assertRaises(gis.Blocked):
                gis._partial_v1_transition(bad_legacy, bad_hash, bad_binding,
                    bad_fingerprints, state, authorization, authorization_sha)

        for bad_authorization, bad_sha in (
            (authorization, "0" * 64),
            ({**authorization, "target_source_sha": "1" * 40}, authorization_sha),
            ({**authorization, "target_changed_files": {**authorization["target_changed_files"],
                "scripts/dev/gis_first_install_test.py": "c" * 64}}, authorization_sha),
            ({**authorization, "operation": "000000000000"}, authorization_sha),
            ({**authorization, "namespace_uid": "9" * 36}, authorization_sha),
            ({**authorization, "original_journal_sha256": "0" * 64}, authorization_sha),
            ({**authorization, "previous_sidecar_sha256": "0" * 64}, authorization_sha),
            ({**authorization, "protected_binding": {**authorization["protected_binding"],
                "image_ref": "image@sha256:" + "3" * 64}}, authorization_sha),
            ({**authorization, "extra": True}, authorization_sha),
        ):
            with self.assertRaises(gis.Blocked):
                gis._partial_v1_transition(legacy, sidecar_sha, current_binding,
                    fingerprints, state, bad_authorization, bad_sha)

        for mutate in (
            lambda row: row.update(operation="other"),
            lambda row: row.update(namespace_uid="9" * 36),
            lambda row: row.update(phase="READY"),
            lambda row: row["plan"].update(prerequisite_receipt={"unexpected": True}),
            lambda row: row["plan"].update(created={"job/unexpected": {}}),
        ):
            invalid_legacy = copy.deepcopy(legacy)
            mutate(invalid_legacy["state"])
            with self.assertRaises(gis.Blocked):
                gis._partial_v1_transition(invalid_legacy, sidecar_sha,
                    current_binding, fingerprints, state, authorization, authorization_sha)

        for mutate in (
            lambda row: row["source_transition"].update(previous_sidecar_sha256="0" * 64),
            lambda row: row["binding"].update(candidate_source_sha="4" * 40),
            lambda row: row.update(extra="unexpected"),
        ):
            invalid = copy.deepcopy(current)
            mutate(invalid)
            with self.assertRaises(gis.Blocked):
                gis._partial_v2_admissible(invalid, legacy, sidecar_sha,
                    current_binding, fingerprints, state, authorization, authorization_sha)

        with tempfile.TemporaryDirectory() as folder:
            old_path = Path(folder) / "continuation.json"
            new_path = Path(folder) / "continuation-current.json"
            legacy_raw = json.dumps(legacy, sort_keys=True, separators=(",", ":")).encode()
            fixture_sha = gis.hashlib.sha256(legacy_raw).hexdigest()
            with patch.object(gis, "_PARTIAL_V1_SIDECAR_SHA256", fixture_sha):
                old_path.write_bytes(legacy_raw)
                fixture_authorization = {**authorization, "previous_sidecar_sha256": fixture_sha}
                saved = gis._partial_v1_transition(legacy, fixture_sha, current_binding,
                    fingerprints, state, fixture_authorization, gis._digest(fixture_authorization))
                gis._save_continuation(new_path, saved)
            self.assertEqual(old_path.read_bytes(), legacy_raw)
            self.assertTrue(new_path.exists())
            self.assertEqual(gis._read_private_continuation(new_path),
                json.dumps(saved, sort_keys=True, separators=(",", ":")).encode())
            if gis.os.name != "nt":
                self.assertEqual(gis.stat.S_IMODE(new_path.stat().st_mode), 0o600)

    def test_recovery_authorization_json_rejects_duplicate_or_nonobject_roots(self):
        for raw in (b'{"schema":"a","schema":"b"}', b'[]', b'null'):
            with self.assertRaises((ValueError, TypeError)):
                gis._strict_json_object(raw)
        self.assertEqual(gis._strict_json_object(b'{"schema":"ok"}'), {"schema": "ok"})

    def test_continuation_sidecar_requires_exact_prior_hash_for_atomic_replace(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "continuation.json"
            first = {"schema": "voice-dev-gis-partial-continuation-v1", "state": {"phase": "SECRETS_APPLIED"}}
            expected = gis._save_continuation(path, first)
            self.assertEqual(path.read_bytes(), json.dumps(first, sort_keys=True,
                separators=(",", ":")).encode())
            second = copy.deepcopy(first)
            second["state"]["phase"] = "PREREQUISITE_RESUME_REQUIRED"
            updated = gis._save_continuation(path, second, expected_sha256=expected)
            self.assertNotEqual(updated, expected)
            with self.assertRaises(gis.Blocked):
                gis._save_continuation(path, first, expected_sha256=expected)

    def test_partial_recovery_lock_rejects_second_writer(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / 'state.json'
            with gis._partial_operation_lock(path):
                with self.assertRaisesRegex(gis.Blocked, 'gis_dev_partial_operation_busy'):
                    with gis._partial_operation_lock(path):
                        self.fail('second writer entered the same operation')

    def test_source_compatibility_requires_clean_descendant_and_exact_changed_paths(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)

            def git(*args):
                return gis.subprocess.run(["git", "-C", str(root), *args], check=True,
                    capture_output=True, text=True).stdout.strip()

            git("init")
            git("config", "user.email", "gis-test@example.invalid")
            git("config", "user.name", "GIS test")
            for name in gis._PARTIAL_CHANGED_PATHS:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("old\n", encoding="utf-8")
            backend = root / "src/backend/gameintegration"
            backend.mkdir(parents=True)
            (backend / "sentinel.go").write_text("package gameintegration\n", encoding="utf-8")
            git("add", ".")
            git("commit", "-m", "baseline")
            old_sha = git("rev-parse", "HEAD")
            tree = git("rev-parse", "HEAD:src/backend")
            for name in gis._PARTIAL_CHANGED_PATHS:
                (root / name).write_text("reviewed change\n", encoding="utf-8")
            git("add", ".")
            git("commit", "-m", "operator recovery")
            journal_sha = gis._PARTIAL_JOURNAL_SHA256
            state = {"source_sha": old_sha,
                "source_build_receipt": {"source_sha": old_sha, "image_ref": "image@sha256:" + "a"*64,
                    "builder_tree": tree},
                "plan": {"source_sha": old_sha, "source_plan_sha256": "b"*64,
                    "source_capture": {}, "inputs_sha256": "c"*64}}
            with patch.object(gis, "_PARTIAL_SOURCE_SHA", old_sha):
                binding = gis._partial_source_binding(root, state, journal_sha)
            self.assertEqual(set(binding["changed_files"]), gis._PARTIAL_CHANGED_PATHS)
            self.assertEqual(binding["candidate_source_sha"], git("rev-parse", "HEAD"))
            self.assertEqual(binding["backend_tree"], tree)


class DevWorkloadProjectionTests(unittest.TestCase):
    @staticmethod
    def add_default(obj, path, value):
        node = obj
        for part in path[:-1]:
            if part.isdigit():
                node = node[int(part)]
            else:
                node = node.setdefault(part, {})
        node.setdefault(path[-1], list(value) if isinstance(value, tuple) else value)

    def fixture(self):
        image = gis.IMAGE + "@sha256:" + "4" * 64
        source = gis._source_pair(Path(__file__).resolve().parents[2], image)
        intended = {row["kind"]: row for row in source["objects"]}
        for row in intended.values():
            row.setdefault("metadata", {}).setdefault("annotations", {})[gis_first_install.MARKER] = gis._PARTIAL_OPERATION
        service = copy.deepcopy(intended["Service"])
        service["metadata"].update(uid="c28155fe-017f-4d68-8b86-15a9b13dbbed", resourceVersion="3382156",
            creationTimestamp="2026-10-17T12:44:38Z")
        for path, value in gis._GIS_SERVICE_DEFAULTS.items():
            self.add_default(service, path, value)
        service["spec"].update(clusterIP="10.43.202.38", clusterIPs=["10.43.202.38"])
        pin = {"name": "voice-gameintegration", "uid": service["metadata"]["uid"],
            "resourceVersion": service["metadata"]["resourceVersion"], "operation": gis._PARTIAL_OPERATION,
            "raw_spec_sha256": hashlib.sha256(json.dumps(service["spec"], sort_keys=True,
                separators=(",", ":")).encode()).hexdigest(), "cluster_ip": "10.43.202.38",
            "metadata_sha256": gis._digest(service["metadata"])}
        deployment = copy.deepcopy(intended["Deployment"])
        deployment["metadata"].update(uid="10000000-0000-4000-8000-000000000001", resourceVersion="3382200",
            generation=1, creationTimestamp="2026-10-17T00:00:01Z",
            managedFields=[
                {"apiVersion": "apps/v1", "fieldsType": "FieldsV1", "fieldsV1": {"f:spec": {}},
                    "manager": "kubectl-create", "operation": "Update", "time": "2026-10-17T00:00:01Z"},
                {"apiVersion": "apps/v1", "fieldsType": "FieldsV1", "fieldsV1": {"f:spec": {"f:replicas": {}}},
                    "manager": "kubectl-patch", "operation": "Update", "time": "2026-10-17T00:00:02Z"},
                {"apiVersion": "apps/v1", "fieldsType": "FieldsV1", "fieldsV1": {"f:status": {}},
                    "manager": "k3s", "operation": "Update", "subresource": "status",
                    "time": "2026-10-17T00:00:03Z"}])
        for path, value in gis._GIS_DEPLOYMENT_DEFAULTS.items():
            self.add_default(deployment, path, value)
        # The real K3s API response inserts this only on the live object; keep
        # the producer-shaped fixture independent of the allowlist itself.
        deployment["spec"]["template"]["spec"]["containers"][0]["resources"] = {}
        # K3s' deployment controller adds this annotation after create; it is
        # accepted only as a positive decimal revision on the owned Deployment.
        deployment["metadata"]["annotations"]["deployment.kubernetes.io/revision"] = "1"
        return intended, service, deployment, pin

    @staticmethod
    def set_path(row, path, value):
        node = row
        for part in path[:-1]:
            node = node[int(part)] if part.isdigit() else node[part]
        node[path[-1]] = value

    def test_projects_only_pinned_service_and_exact_deployment_defaults(self):
        intended, service, deployment, pin = self.fixture()
        container = intended["Deployment"]["spec"]["template"]["spec"]["containers"][0]
        self.assertNotIn("resources", container)
        self.assertEqual(deployment["spec"]["template"]["spec"]["containers"][0]["resources"], {})
        projected_service = gis._gis_project_response(service, intended["Service"], service_pin=pin)
        projected_deployment = gis._gis_project_response(deployment, intended["Deployment"])
        self.assertEqual(gis_first_install.semantic(projected_service),
            gis_first_install.semantic(intended["Service"]))
        self.assertEqual(gis_first_install.semantic(projected_deployment),
            gis_first_install.semantic(intended["Deployment"]))
        # The allocated address stays bound in the raw live pin; it is omitted
        # only from the projected semantic object to compare against authored spec.
        self.assertNotIn("clusterIP", projected_service["spec"])
        self.assertNotIn("resources", projected_deployment["spec"]["template"]["spec"]["containers"][0])

    def test_empty_resources_is_only_projected_at_exact_omitted_main_container_path(self):
        intended, _, deployment, _ = self.fixture()
        authored_empty = copy.deepcopy(intended["Deployment"])
        authored_empty["spec"]["template"]["spec"]["containers"][0]["resources"] = {}
        missing_authored_field = copy.deepcopy(deployment)
        del missing_authored_field["spec"]["template"]["spec"]["containers"][0]["resources"]
        with self.assertRaises(gis.Blocked):
            gis._gis_project_response(missing_authored_field, authored_empty)

        for mutate in (
            lambda row: row["spec"]["template"]["spec"]["containers"][0].update(resources={"limits": {"cpu": "1"}}),
            lambda row: row["spec"]["template"]["spec"]["containers"][0].update(resources={"unexpected": "value"}),
            lambda row: row["spec"]["template"]["spec"]["containers"][0]["readinessProbe"].update(resources={}),
            lambda row: row["spec"]["template"]["spec"].update(initContainers=[{"name": "unexpected", "image": "example.invalid/image", "resources": {}}]),
        ):
            changed = copy.deepcopy(deployment)
            mutate(changed)
            with self.assertRaises(gis.Blocked):
                gis._gis_project_response(changed, intended["Deployment"])

        authored_exact = copy.deepcopy(intended["Deployment"])
        authored_exact["spec"]["template"]["spec"]["containers"][0]["resources"] = {}
        exact_live = copy.deepcopy(deployment)
        projected = gis._gis_project_response(exact_live, authored_exact)
        self.assertEqual(projected["spec"]["template"]["spec"]["containers"][0]["resources"], {})

    def test_service_identity_and_unknown_or_wrong_defaults_fail_closed(self):
        intended, service, deployment, pin = self.fixture()
        for mutate in (
            lambda row: row["metadata"].update(uid="20000000-0000-4000-8000-000000000002"),
            lambda row: row["metadata"].update(resourceVersion="0"),
            lambda row: row["spec"].update(clusterIP="10.43.0.99"),
            lambda row: row["spec"]["ports"][0].update(targetPort=9090),
            lambda row: row["spec"]["ports"][0].update(nodePort=32000),
            lambda row: row["spec"].update(externalName="unexpected.example"),
            lambda row: row["metadata"].update(finalizers=["example.invalid/finalizer"]),
            lambda row: row["metadata"].update(ownerReferences=[{"apiVersion": "v1", "kind": "Service", "name": "other", "uid": "00000000-0000-0000-0000-000000000001"}]),
            lambda row: row["metadata"].update(deletionTimestamp="2026-10-17T00:00:00Z"),
            lambda row: row["metadata"].update(deletionGracePeriodSeconds=30),
            lambda row: row["metadata"].update(generation=True),
            lambda row: row["metadata"].update(creationTimestamp="not-a-time"),
            lambda row: row["metadata"].update(labels={"unexpected": "label"}),
            lambda row: row["metadata"]["annotations"].update(unexpected="annotation"),
            lambda row: row["metadata"].update(unexpectedMetadata="value"),
        ):
            changed = copy.deepcopy(service)
            mutate(changed)
            with self.assertRaises(gis.Blocked):
                gis._gis_project_response(changed, intended["Service"], service_pin=pin)
        for path, default in gis._GIS_SERVICE_DEFAULTS.items():
            changed = copy.deepcopy(service)
            bad = list(default) + ["unexpected"] if isinstance(default, tuple) else "unexpected"
            self.set_path(changed, path, bad)
            with self.assertRaises(gis.Blocked):
                gis._gis_project_response(changed, intended["Service"], service_pin=pin)
        for path, default in gis._GIS_DEPLOYMENT_DEFAULTS.items():
            changed = copy.deepcopy(deployment)
            bad = list(default) + ["unexpected"] if isinstance(default, tuple) else (default + 1 if type(default) is int else "unexpected")
            self.set_path(changed, path, bad)
            with self.assertRaises(gis.Blocked):
                gis._gis_project_response(changed, intended["Deployment"])
        for mutate in (
            lambda row: row["spec"]["template"]["spec"]["containers"][0].update(securityContext={}),
            lambda row: row["metadata"].update(finalizers=["example.invalid/finalizer"]),
            lambda row: row["metadata"].update(ownerReferences=[{"apiVersion": "apps/v1", "kind": "Deployment", "name": "other", "uid": "00000000-0000-0000-0000-000000000001"}]),
            lambda row: row["metadata"].update(deletionTimestamp="2026-10-17T00:00:00Z"),
            lambda row: row["metadata"].update(deletionGracePeriodSeconds=30),
            lambda row: row["metadata"]["annotations"].update({"deployment.kubernetes.io/revision": "0"}),
            lambda row: row["metadata"]["annotations"].update({"deployment.kubernetes.io/revision": "1" * 11}),
            lambda row: row["metadata"]["annotations"].update({"unexpected": "annotation"}),
            lambda row: row["metadata"].update(generation=True),
            lambda row: row["metadata"].update(managedFields=[{"manager": "unreviewed", "operation": "Update"}]),
        ):
            changed = copy.deepcopy(deployment)
            mutate(changed)
            with self.assertRaises(gis.Blocked):
                gis._gis_project_response(changed, intended["Deployment"])

    def test_service_metadata_pin_rejects_changes_after_checkpoint(self):
        intended, service, _, pin = self.fixture()
        changed = copy.deepcopy(service)
        changed["metadata"]["creationTimestamp"] = "2026-10-17T12:44:39Z"
        with self.assertRaises(gis.Blocked):
            gis._gis_project_response(changed, intended["Service"], service_pin=pin)

    def test_projection_adapter_scopes_transforms_to_the_fixed_pair(self):
        intended, service, deployment, pin = self.fixture()

        class Kube:
            def __init__(self):
                other_service = {"kind": "Service", "metadata": {"name": "other"}, "spec": {}}
                self.rows = [other_service, service, deployment]

            def run(self, args, body=None, timeout=30):
                if args == ["get", "services,deployments", "-o", "json"]:
                    return {"apiVersion": "v1", "kind": "List", "metadata": {},
                        "items": copy.deepcopy(self.rows)}
                if args == ["create", "-f", "/dev/stdin", "-o", "json"]:
                    return copy.deepcopy(self.rows[2])
                return {"kind": "Deployment", "metadata": {"name": "other"}}

            def get(self, kind, name):
                return {"kind": kind.title(), "metadata": {"name": name}}

        adapter = gis.GisDevWorkloadProjection(Kube(),
            {"operation": gis._PARTIAL_OPERATION, "objects": list(intended.values())}, pin)
        rows = adapter.run(["get", "services,deployments", "-o", "json"])["items"]
        self.assertEqual(rows[0]["spec"], {})
        self.assertEqual(gis_first_install.semantic(rows[1]), gis_first_install.semantic(intended["Service"]))
        self.assertEqual(gis_first_install.semantic(rows[2]), gis_first_install.semantic(intended["Deployment"]))
        created = adapter.run(["create", "-f", "/dev/stdin", "-o", "json"],
            body=intended["Deployment"])
        self.assertEqual(gis_first_install.semantic(created), gis_first_install.semantic(intended["Deployment"]))


class PartialV4AuthorizationTests(unittest.TestCase):
    def fixture(self):
        changed_files = {name: hashlib.sha256(name.encode()).hexdigest()
            for name in sorted(gis._PARTIAL_CHANGED_PATHS)}
        binding = {"old_source_sha": gis._PARTIAL_SOURCE_SHA,
            "candidate_source_sha": "a" * 40, "changed_files": changed_files,
            "backend_tree": "b" * 40, "image_ref": "image@sha256:" + "c" * 64,
            "build_receipt_sha256": "d" * 64, "source_plan_sha256": "e" * 64,
            "source_capture_sha256": "f" * 64, "inputs_sha256": "1" * 64}
        v3_binding = {"candidate_source_sha": gis._PARTIAL_C10_CANDIDATE_SHA,
            "changed_files": gis._PARTIAL_C10_CHANGED_FILE_SHA256}
        v3_state = {"phase": "PLANNED"}
        v3 = {"schema": "voice-dev-gis-partial-continuation-v3", "binding": v3_binding,
            "state": v3_state, "secret_fingerprints_sha256": "2" * 64}
        current_v2 = {"binding": {"candidate_source_sha": gis._PARTIAL_C5_CANDIDATE_SHA},
            "state": {"phase": "PLANNED"}}
        protected = {key: binding[key] for key in ("old_source_sha", "backend_tree",
            "build_receipt_sha256", "image_ref", "source_plan_sha256", "source_capture_sha256", "inputs_sha256")}
        checkpoint = {"operation": gis._PARTIAL_OPERATION, "service": {"uid": "svc"}}
        auth = {"schema": "voice-dev-gis-c10-service-transition-authorization-v1",
            "operation": gis._PARTIAL_OPERATION, "namespace_uid": "ns",
            "original_journal_sha256": gis._PARTIAL_JOURNAL_SHA256,
            "immutable_v1_sha256": gis._PARTIAL_V1_SIDECAR_SHA256,
            "previous_v2_sha256": gis._PARTIAL_C5_V2_SHA256,
            "previous_v2_binding_sha256": gis._digest(current_v2["binding"]),
            "previous_v2_state_sha256": gis._digest(current_v2["state"]),
            "previous_c5_authorization_sha256": gis._PARTIAL_C5_AUTH_SHA256,
            "previous_v3_sha256": "3" * 64,
            "previous_v3_binding_sha256": gis._digest(v3_binding),
            "previous_v3_state_sha256": gis._digest(v3_state),
            "previous_c10_authorization_sha256": gis._PARTIAL_C10_AUTH_SHA256,
            "previous_candidate_source_sha": gis._PARTIAL_C10_CANDIDATE_SHA,
            "previous_changed_files": gis._PARTIAL_C10_CHANGED_FILE_SHA256,
            "previous_secret_fingerprints_sha256": v3["secret_fingerprints_sha256"],
            "target_source_sha": binding["candidate_source_sha"],
            "target_changed_files": changed_files,
            "protected_binding": protected, "partial_checkpoint": checkpoint}
        return binding, v3, current_v2, protected, checkpoint, auth

    def test_v3_to_v4_authorization_requires_exact_source_and_checkpoint_pins(self):
        binding, v3, current_v2, _, checkpoint, auth = self.fixture()
        with patch.object(gis, "_PARTIAL_C10_V3_SHA256", "3" * 64), \
             patch.object(gis, "_PARTIAL_C10_V3_BINDING_SHA256", gis._digest(v3["binding"])), \
             patch.object(gis, "_PARTIAL_C10_V3_STATE_SHA256", gis._digest(v3["state"])), \
             patch.object(gis, "_git_text", return_value=binding["candidate_source_sha"]):
            self.assertTrue(gis._partial_v3_to_v4_authorized(v3, "3" * 64,
                gis._PARTIAL_C10_AUTH_SHA256, binding, {}, {"namespace_uid": "ns"},
                auth, "4" * 64, "4" * 64, checkpoint, current_v2, Path(".")))
            for field, value in (("target_source_sha", "5" * 40),
                    ("namespace_uid", "other"), ("previous_v3_state_sha256", "6" * 64)):
                changed = copy.deepcopy(auth)
                changed[field] = value
                with self.subTest(field=field), self.assertRaises(gis.Blocked):
                    gis._partial_v3_to_v4_authorized(v3, "3" * 64,
                        gis._PARTIAL_C10_AUTH_SHA256, binding, {}, {"namespace_uid": "ns"},
                        changed, "4" * 64, "4" * 64, checkpoint, current_v2, Path("."))


class PartialV5DeploymentAdoptionTests(unittest.TestCase):
    def fixture(self):
        binding = {"old_source_sha": gis._PARTIAL_SOURCE_SHA,
            "candidate_source_sha": "a" * 40,
            "changed_files": {name: hashlib.sha256(name.encode()).hexdigest()
                for name in sorted(gis._PARTIAL_CHANGED_PATHS)},
            "backend_tree": "b" * 40, "build_receipt_sha256": "c" * 64,
            "image_ref": "image@sha256:" + "d" * 64,
            "source_plan_sha256": "e" * 64, "source_capture_sha256": "f" * 64,
            "inputs_sha256": "1" * 64}
        protected = {key: binding[key] for key in ("old_source_sha", "backend_tree",
            "build_receipt_sha256", "image_ref", "source_plan_sha256",
            "source_capture_sha256", "inputs_sha256")}
        service = {"uid": "20000000-0000-4000-8000-000000000001",
            "resourceVersion": "3382156"}
        deployment = {"name": "voice-gameintegration", "exists": True,
            "uid": "fc9bb9e1-b6c8-46a6-ae8b-cde5f3bd28f2", "resourceVersion": "3383482",
            "generation": 1, "operation": gis._PARTIAL_OPERATION,
            "raw_spec_sha256": "2" * 64, "metadata_sha256": "3" * 64,
            "image_ref": "image@sha256:" + "d" * 64, "replicas": 0,
            "semantic_sha256": "4" * 64, "desired_semantic_sha256": "4" * 64}
        fingerprints_sha = gis._digest({})
        checkpoint = {"operation": gis._PARTIAL_OPERATION, "namespace_uid": "ns",
            "created_records": {}, "prerequisite_receipt_sha256": "5" * 64,
            "configmaps": [], "jobs": [], "secret_fingerprints_sha256": fingerprints_sha,
            "database": {"owner": "voice", "migration_state": {"version": 31, "dirty": False}},
            "service": service, "deployment": deployment, "deployment_pods": []}
        v4_state = {"phase": "SECRETS_APPLIED", "plan_phase": None,
            "plan": {"phase": "PLANNED", "created": {}}}
        record = {"uid": deployment["uid"], "resourceVersion": deployment["resourceVersion"],
            "spec_sha256": deployment["semantic_sha256"]}
        v5_state = copy.deepcopy(v4_state)
        v5_state["plan"]["created"]["Deployment/voice-gameintegration"] = record
        current_v2 = {"previous_v2_sha256": "7" * 64,
            "previous_v2_binding_sha256": "8" * 64, "previous_v2_state_sha256": "9" * 64,
            "previous_c5_authorization_sha256": "a" * 64}
        v3 = {"previous_v3_sha256": "b" * 64, "previous_v3_binding_sha256": "c" * 64,
            "previous_v3_state_sha256": "d" * 64, "previous_c10_authorization_sha256": "e" * 64}
        v4 = {**current_v2, **v3, "secret_fingerprints_sha256": fingerprints_sha,
            "binding": {**protected, "candidate_source_sha": "f" * 40,
            "changed_files": {name: "1" * 64 for name in sorted(gis._PARTIAL_CHANGED_PATHS)}},
            "state": v4_state}
        v4_sha = "2" * 64
        authorization = {"schema": "voice-dev-gis-deployment-adoption-authorization-v1",
            "operation": gis._PARTIAL_OPERATION, "namespace_uid": "ns",
            "original_journal_sha256": gis._PARTIAL_JOURNAL_SHA256,
            "immutable_v1_sha256": gis._PARTIAL_V1_SIDECAR_SHA256,
            **current_v2, **v3,
            "previous_v4_sha256": v4_sha,
            "previous_v4_binding_sha256": gis._digest(v4["binding"]),
            "previous_v4_state_sha256": gis._digest(v4_state),
            "previous_c14_authorization_sha256": "f" * 64,
            "previous_candidate_source_sha": v4["binding"]["candidate_source_sha"],
            "previous_changed_files": v4["binding"]["changed_files"],
            "previous_secret_fingerprints_sha256": fingerprints_sha,
            "target_source_sha": binding["candidate_source_sha"],
            "target_changed_files": binding["changed_files"],
            "protected_binding": protected, "partial_checkpoint": checkpoint,
            "transition_id": "C14-v4-to-v5-paused-deployment-adoption"}
        v5 = {"schema": "voice-dev-gis-partial-continuation-v5",
            "original_journal_sha256": gis._PARTIAL_JOURNAL_SHA256,
            "previous_v4_sha256": v4_sha,
            "previous_v4_state_sha256": gis._digest(v4_state),
            "previous_v4_binding_sha256": gis._digest(v4["binding"]),
            "previous_c14_authorization_sha256": "f" * 64,
            "deployment_transition_authorization_sha256": "0" * 64,
            "binding": binding, "protected_binding": protected,
            "secret_fingerprints_sha256": fingerprints_sha,
            "checkpoint": checkpoint, "state": v5_state}
        return v5, v4, binding, checkpoint, authorization, v4_sha, deployment

    def test_v5_reconciles_only_the_pinned_paused_uid(self):
        v5, v4, binding, checkpoint, authorization, v4_sha, _ = self.fixture()
        with (patch.object(gis, "_partial_live_c10_checkpoint", return_value=checkpoint),
              patch.object(gis, "_git_text", return_value=binding["candidate_source_sha"]),
              patch.object(gis, "_partial_state_compatible", return_value=True)):
            self.assertTrue(gis._partial_v5_admissible(v5, "3" * 64, v4, v4_sha,
                binding, {}, {"namespace_uid": "ns"}, authorization, "f" * 64,
                "0" * 64, "0" * 64, object(), Path(".")))
            drifted = copy.deepcopy(checkpoint)
            drifted["deployment"]["uid"] = "40000000-0000-4000-8000-000000000004"
            with patch.object(gis, "_partial_live_c10_checkpoint", return_value=drifted):
                with self.assertRaises(gis.Blocked):
                    gis._partial_v5_admissible(v5, "3" * 64, v4, v4_sha,
                        binding, {}, {"namespace_uid": "ns"}, authorization, "f" * 64,
                        "0" * 64, "0" * 64, object(), Path("."))

    def _recover(self, v5, v4, binding, checkpoint, authorization, v4_sha):
        saved = []
        def save(plan):
            saved.append(copy.deepcopy(plan))
            v5["state"]["plan"] = copy.deepcopy(plan)
            v5["state"]["plan_phase"] = plan.get("phase")
        with (patch.object(gis, "_partial_live_c10_checkpoint", return_value=checkpoint),
              patch.object(gis, "_git_text", return_value=binding["candidate_source_sha"]),
              patch.object(gis, "_partial_state_compatible", return_value=True)):
            changed = gis._partial_v5_recover_unpatched_start(v5, "3" * 64,
                v4, v4_sha, binding, {}, {"namespace_uid": "ns"}, authorization,
                "f" * 64, "0" * 64, "0" * 64, object(), Path("."), save)
        return changed, saved

    def test_starting_intent_with_exact_paused_deployment_recovers_to_created_once(self):
        v5, v4, binding, checkpoint, authorization, v4_sha, _ = self.fixture()
        v5["state"]["plan"]["phase"] = "STARTING"
        v5["state"]["plan_phase"] = "STARTING"
        changed, saved = self._recover(v5, v4, binding, checkpoint, authorization, v4_sha)
        self.assertTrue(changed)
        self.assertEqual(len(saved), 1)
        self.assertEqual(saved[0]["phase"], "CREATED")
        self.assertEqual(v5["state"]["plan_phase"], "CREATED")

    def test_starting_intent_with_exactly_started_deployment_is_left_for_reconciliation(self):
        v5, v4, binding, checkpoint, authorization, v4_sha, _ = self.fixture()
        v5["state"]["plan"]["phase"] = "STARTING"
        v5["state"]["plan_phase"] = "STARTING"
        started = copy.deepcopy(checkpoint)
        started["deployment"]["replicas"] = 1
        changed, saved = self._recover(v5, v4, binding, started, authorization, v4_sha)
        self.assertFalse(changed)
        self.assertEqual(saved, [])
        self.assertEqual(v5["state"]["plan"]["phase"], "STARTING")

    def test_starting_paused_deployment_drift_blocks_recovery(self):
        cases = (("deployment", "resourceVersion", "3383483"),
            ("deployment", "generation", 2), ("deployment", "raw_spec_sha256", "6" * 64),
            ("deployment", "metadata_sha256", "7" * 64),
            ("deployment", "uid", "40000000-0000-4000-8000-000000000004"),
            ("deployment", "operation", "a" * 12),
            ("deployment", "image_ref", "other@sha256:" + "8" * 64),
            ("deployment", "semantic_sha256", "9" * 64),
            ("checkpoint", "namespace_uid", "foreign-namespace"),
            ("checkpoint", "operation", "a" * 12),
            ("checkpoint", "deployment_pods", [{"uid": "foreign-pod"}]))
        for section, key, value in cases:
            with self.subTest(section=section, key=key):
                v5, v4, binding, checkpoint, authorization, v4_sha, _ = self.fixture()
                v5["state"]["plan"]["phase"] = "STARTING"
                v5["state"]["plan_phase"] = "STARTING"
                drifted = copy.deepcopy(checkpoint)
                target = drifted["deployment"] if section == "deployment" else drifted
                target[key] = value
                with self.assertRaises(gis.Blocked):
                    self._recover(v5, v4, binding, drifted, authorization, v4_sha)


class PausedDeploymentStartRetryTests(unittest.TestCase):
    def test_ambiguous_start_response_reconciles_exact_uid_without_second_patch(self):
        service = {"apiVersion": "v1", "kind": "Service",
            "metadata": {"name": "voice-gameintegration", "namespace": "voice-dev",
                "labels": {"app": "voice-gameintegration"}},
            "spec": {"selector": {"app": "voice-gameintegration"},
                "ports": [{"name": "http", "port": 8080, "targetPort": "http"}]}}
        deployment = {"apiVersion": "apps/v1", "kind": "Deployment",
            "metadata": {"name": "voice-gameintegration", "namespace": "voice-dev",
                "labels": {"app": "voice-gameintegration"}},
            "spec": {"replicas": 1, "selector": {"matchLabels": {"app": "voice-gameintegration"}},
                "template": {"metadata": {"labels": {"app": "voice-gameintegration"}},
                    "spec": {"containers": [{"name": "gameintegration",
                        "image": "image@sha256:" + "d" * 64}]}}}}
        plan = gis_first_install.build_plan(service=service, deployment=deployment,
            operation="737e63e3aaee", namespace="voice-dev")
        live = {}
        for index, identity in enumerate(gis_first_install.IDENTITIES, start=1):
            intended = next(obj for obj in plan["objects"]
                if gis_first_install._identity(obj) == identity)
            row = copy.deepcopy(intended)
            row["metadata"].update(uid=f"10000000-0000-4000-8000-{index:012d}",
                resourceVersion="1")
            live[identity] = row
            plan["created"][f"{identity[0]}/{identity[1]}"] = {
                "uid": row["metadata"]["uid"], "resourceVersion": "1",
                "spec_sha256": gis_first_install.digest(gis_first_install.semantic(row))}
        plan["phase"] = "CREATED"
        class Kube:
            starts = 0
            def get(self, kind, name):
                identity = next(row for row in live if row[0].lower() == kind.lower() and row[1] == name)
                return copy.deepcopy(live[identity])
            def start(self, current, *, replicas, expected_uid, expected_resource_version):
                self.starts += 1
                self.assert_guard = (current["metadata"]["uid"], expected_uid,
                    current["metadata"]["resourceVersion"], expected_resource_version)
                live[gis_first_install.DEPLOYMENT]["spec"]["replicas"] = replicas
                live[gis_first_install.DEPLOYMENT]["metadata"]["resourceVersion"] = "2"
                raise RuntimeError("response lost after guarded patch")
        kube = Kube()
        saved = []
        with patch.object(gis_first_install, "validate_prerequisite_receipt"):
            with self.assertRaises(RuntimeError):
                gis_first_install.start_owned(kube, plan, lambda row: saved.append(copy.deepcopy(row)))
            self.assertEqual(plan["phase"], "STARTING")
            self.assertEqual(live[gis_first_install.DEPLOYMENT]["spec"]["replicas"], 1)
            receipt = gis_first_install.start_owned(kube, plan,
                lambda row: saved.append(copy.deepcopy(row)))
        self.assertEqual(kube.starts, 1)
        self.assertEqual(kube.assert_guard, (plan["created"]["Deployment/voice-gameintegration"]["uid"],
            plan["created"]["Deployment/voice-gameintegration"]["uid"], "1", "1"))
        self.assertEqual(plan["phase"], "STARTED")
        self.assertEqual(receipt["objects"]["Deployment/voice-gameintegration"]["uid"],
            plan["created"]["Deployment/voice-gameintegration"]["uid"])


class SourceAuthorityTests(unittest.TestCase):
    def test_build_receipt_binds_clean_source_and_immutable_image(self):
        source = 'a' * 40
        image = 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'b' * 64
        receipt = {'schema': gis.BUILD_RECEIPT_SCHEMA, 'source_sha': source,
            'image_ref': image, 'build_context_sha256': gis.hashlib.sha256(('e' * 40).encode()).hexdigest(), 'image_config_sha256': 'd' * 64,
            'working_tree_clean': True, 'builder_kind': 'github-actions',
            'builder_run': 11, 'builder_job': 12,
            'builder_sha': source, 'builder_tree': 'e' * 40,
            'builder_artifact_id': 13, 'builder_artifact_sha256': 'f' * 64,
            'builder_output_index_sha256': 'b' * 64, 'builder_provenance_sha256': '1' * 64}
        self.assertTrue(gis.validate_build_receipt(source, image, receipt, clean=True))
        with self.assertRaises(gis.Blocked):
            gis.validate_build_receipt(source, image, receipt, clean=False)
        with self.assertRaises(gis.Blocked):
            gis.validate_build_receipt(source, image, dict(receipt, working_tree_clean=False), clean=True)
        with self.assertRaises(gis.Blocked):
            gis.validate_build_receipt(source, image.replace('b'*64, 'e'*64), receipt, clean=True)

    def test_source_build_receipt_is_produced_from_clean_source_and_inspected_oci(self):
        source = 'a' * 40
        tree = 'c' * 40
        config = 'd' * 64
        manifest_raw = json.dumps({'config': {'digest': 'sha256:' + config}}).encode()
        image_digest = gis.hashlib.sha256(manifest_raw).hexdigest()
        image = gis.IMAGE + '@sha256:' + image_digest
        calls = []
        artifact_raw, image_digest, child_raw, _, image_index = buildx_artifact(source, 11, 1)
        image = gis.IMAGE + '@sha256:' + image_digest
        child_digest = gis.hashlib.sha256(child_raw).hexdigest()
        api_run = {'id': 11, 'status': 'completed', 'conclusion': 'success',
            'event': 'push', 'head_branch': 'master', 'head_sha': source, 'workflow_id': 9,
            'head_repository': {'full_name': 'Poryadok/VoiceRoot'}, 'run_attempt': 1}
        api_workflow = {'path': '.github/workflows/ci.yml'}
        api_jobs = {'jobs': [{'id': 12, 'status': 'completed', 'conclusion': 'success',
            'name': 'staging-images-push (gameintegration)',
            'steps': [{'name': 'Set up job', 'conclusion': 'success'},
                {'name': 'Build and push gameintegration image', 'conclusion': 'success',
                    'started_at': '2026-10-17T00:01:32Z', 'completed_at': '2026-10-17T00:02:22Z'}]}]}
        api_artifacts = {'total_count': 1, 'artifacts': [{'id': 13, 'name': 'VoiceRoot.dockerbuild',
            'expired': False, 'size_in_bytes': len(artifact_raw)}]}
        workflow_source = ('if grep -qx "$S" scripts/ci/backend-docker-context.txt\n'
            'echo "context=./src/backend"\n  staging-images-push:\n'
            '- name: Build and push ${{ matrix.service }} image\n'
            'docker/build-push-action@v6\ncontext: ${{ steps.dockerctx.outputs.context }}\n'
            'file: ${{ steps.dockerctx.outputs.file }}\npush: true\n'
            '  staging-images-promote:\n')

        def runner(argv, **_kwargs):
            calls.append(argv)
            if argv[0] == 'git' and 'rev-parse' in argv:
                return SimpleNamespace(returncode=0, stdout=source if argv[-1] == 'HEAD' else tree, stderr='')
            if argv[0] == 'git' and 'status' in argv:
                return SimpleNamespace(returncode=0, stdout='', stderr='')
            if argv[0] == 'git' and 'show' in argv:
                content = 'gameintegration\n' if argv[-1].endswith('backend-docker-context.txt') else workflow_source
                return SimpleNamespace(returncode=0, stdout=content, stderr='')
            if argv[:2] == ['gh', 'api']:
                endpoint = argv[2]
                if endpoint.endswith('/artifacts/13/zip'):
                    return SimpleNamespace(returncode=0, stdout=artifact_raw, stderr=b'')
                value = (api_run if endpoint.endswith('/runs/11') else
                    api_workflow if endpoint.endswith('/workflows/9') else
                    api_artifacts if endpoint.endswith('/artifacts?per_page=100') else api_jobs)
                return SimpleNamespace(returncode=0, stdout=json.dumps(value), stderr='')
            if argv[:5] == ['docker', 'buildx', 'imagetools', 'inspect', '--raw']:
                raw = child_raw if argv[-1].endswith('@sha256:' + child_digest) else image_index
                return SimpleNamespace(returncode=0, stdout=raw, stderr=b'')
            if argv[:2] == ['docker', 'pull']:
                return SimpleNamespace(returncode=0, stdout=b'', stderr=b'')
            if argv[:3] == ['docker', 'image', 'inspect']:
                return SimpleNamespace(returncode=0, stdout=json.dumps([{
                    'Id': 'sha256:' + config,
                    'RepoDigests': [image],
                    'Config': {'Labels': {
                        'org.opencontainers.image.revision': source,
                        'io.voice.build.context.sha256': gis.hashlib.sha256(tree.encode()).hexdigest(),
                    }},
                }]).encode(), stderr=b'')
            raise AssertionError(argv)

        actual_sha, receipt = gis.source_build_receipt(Path('.'), image,
            builder_run=11, builder_job=12, runner=runner)
        self.assertEqual(actual_sha, source)
        self.assertEqual(receipt['image_config_sha256'], config)
        self.assertEqual(receipt['builder_tree'], tree)
        self.assertEqual(receipt['builder_artifact_id'], 13)
        self.assertEqual(receipt['builder_artifact_sha256'], gis.hashlib.sha256(artifact_raw).hexdigest())
        self.assertEqual(receipt['builder_output_index_sha256'], image_digest)
        self.assertTrue(gis._valid_sha(receipt['builder_provenance_sha256']))
        self.assertIn('docker', calls[-1][0])

        # The producer workflow template is matrix-expanded by GitHub at run
        # time. Keep both sides bound: the API step must name gameintegration,
        # and source must contain the reviewed matrix template.
        self.assertEqual(api_jobs['jobs'][0]['steps'][1]['name'],
            'Build and push gameintegration image')
        workflow_source = workflow_source.replace(
            '- name: Build and push ${{ matrix.service }} image',
            '- name: Build and push ${{ matrix.other }} image')
        with self.assertRaises(gis.Blocked) as blocked:
            gis.source_build_receipt(Path('.'), image,
                builder_run=11, builder_job=12, runner=runner)
        self.assertEqual(str(blocked.exception), 'gis_dev_builder_source_context_mismatch')

    def test_buildx_record_rejects_wrong_run_revision_context_and_output_digest(self):
        source = 'a' * 40
        image = gis.IMAGE + '@sha256:' + 'b' * 64
        step = {'started_at': '2026-10-17T00:01:32Z', 'completed_at': '2026-10-17T00:02:22Z'}
        for run_id, revision, context, digest in [
            (99, source, 'src/backend', None),
            (11, 'c' * 40, 'src/backend', None),
            (11, source, 'untrusted/context', None),
            (11, source, 'src/backend', 'f' * 64),
        ]:
            raw, actual_digest, _child, _config, _image_index = buildx_artifact(source, run_id, 1,
                revision=revision, context=context, output_digest=digest)
            target = gis.IMAGE + '@sha256:' + (digest or actual_digest)
            with self.assertRaises(gis.Blocked):
                gis._verify_buildx_artifact(raw, 13, len(raw), target, 11, 1,
                    source, step, None)

    def test_dirty_source_blocks_before_image_inspection(self):
        source = 'a' * 40
        tree = 'c' * 40
        image = gis.IMAGE + '@sha256:' + 'b' * 64
        calls = []

        def runner(argv, **_kwargs):
            calls.append(argv)
            output = source if argv[-1] == 'HEAD' else tree
            if 'status' in argv:
                output = ' M scripts/dev/gis_dev_first_install.py'
            return SimpleNamespace(returncode=0, stdout=output, stderr='')

        with self.assertRaises(gis.Blocked):
            gis.source_build_receipt(Path('.'), image, builder_run=11, builder_job=12, runner=runner)
        self.assertFalse(any(argv[0] == 'docker' for argv in calls))

    def test_tls_validator_requires_explicit_exact_dns_san_and_matching_key(self):
        outputs = [
            SimpleNamespace(stdout=b''),
            SimpleNamespace(stdout='X509v3 Subject Alternative Name:\n    DNS:voice-gameintegration\n'),
            SimpleNamespace(stdout=b'public-key\n'),
            SimpleNamespace(stdout=b'public-key\n'),
        ]
        calls = []

        def run(argv, **_kwargs):
            calls.append(argv)
            return outputs.pop(0)

        with patch.object(gis.subprocess, 'run', side_effect=run):
            self.assertTrue(gis.validate_tls_leaf(b'certificate', b'private-key', b'ca'))
        self.assertEqual(calls[1][-2:], ['-ext', 'subjectAltName'])

        with patch.object(gis.subprocess, 'run', side_effect=[
            SimpleNamespace(stdout=b''),
            SimpleNamespace(stdout='X509v3 Subject Alternative Name:\n    DNS:other-host\n'),
        ]) as runner:
            with self.assertRaises(gis.Blocked):
                gis.validate_tls_leaf(b'certificate', b'private-key', b'ca')
        self.assertEqual(runner.call_count, 2)

    def test_secret_material_producer_emits_exact_signer_contract(self):
        outputs = []

        def keygen(argv, **_kwargs):
            path = Path(argv[argv.index("-out") + 1])
            value = b"generated-current" if path.name == "current.pem" else b"generated-next"
            path.write_bytes(value)
            outputs.append(path.name)
            return type("Result", (), {"stdout": b"", "stderr": b"", "returncode": 0})()

        with patch.object(gis.subprocess, "run", side_effect=keygen):
            material = gis.generate_secret_material()
        signer = material[gis.SIGNER_SECRET]
        self.assertEqual(set(signer), {"current.pem", "next.pem", "active-kid"})
        self.assertEqual(signer["active-kid"], b"current")
        self.assertEqual(outputs, ["current.pem", "next.pem"])
        self.assertEqual(set(material[gis.WORKLOAD_SECRET]), {
            "GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64", "GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64"})
        plan = gis.build_secret_plan(FakeKube(), tls_validator=lambda *_: None,
            signer_validator=lambda _values: None, material_factory=lambda: material)
        signer_operation = next(row for row in plan['operations'] if row['name'] == gis.SIGNER_SECRET)
        self.assertEqual(signer_operation['body']['data']['active-kid'], base64.b64encode(b'current').decode())


class SecretPreparationTests(unittest.TestCase):
    def test_tls_failure_precedes_material_generation_and_any_write(self):
        kube = FakeKube()
        events = []
        def reject(_cert, _key, _ca):
            events.append('tls')
            raise gis.Blocked('gis_dev_tls_invalid')
        with self.assertRaises(gis.Blocked):
            gis.build_secret_plan(kube, tls_validator=reject,
                material_factory=lambda: events.append('generate') or materials())
        self.assertEqual(events, ['tls'])
        self.assertEqual(kube.writes, [])

    def test_valid_tls_plan_preserves_app_keys_and_never_writes_until_commit(self):
        kube = FakeKube()
        calls = []
        plan = gis.build_secret_plan(kube,
            tls_validator=lambda cert, key, ca: calls.append((cert, key, ca)),
            signer_validator=lambda _values: None,
            material_factory=materials)
        self.assertEqual(calls, [(b'cert', b'tls-key', b'outbound-jwks-ca')])
        self.assertEqual(kube.writes, [])
        app = next(row['body'] for row in plan['operations'] if row['name'] == 'voice-app-secrets')
        self.assertEqual(app['data']['KEEP'], encoded({'KEEP': b'existing'})['KEEP'])
        self.assertIn('POSTGRES_PASSWORD', app['data'])
        gis.apply_secret_plan(kube, plan)
        self.assertEqual(kube.writes[0], ('replace', 'voice-app-secrets'))
        self.assertEqual(kube.secrets['voice-app-secrets']['data']['KEEP'], encoded({'KEEP': b'existing'})['KEEP'])

    def test_missing_jwks_ca_key_blocks_without_material_generation_or_write(self):
        kube = FakeKube()
        kube.secrets['voice-media-ca']['data'] = encoded({'ca.crt': b'inbound-ca'})
        events = []
        with self.assertRaises(gis.Blocked):
            gis.build_secret_plan(kube, tls_validator=lambda *_: events.append('tls'),
                material_factory=lambda: events.append('generate') or materials())
        self.assertEqual(events, [])
        self.assertEqual(kube.writes, [])

    def test_partial_secret_identity_and_namespace_mismatch_fail_before_write(self):
        kube = FakeKube()
        kube.secrets['voice-gameintegration-principal-signing'] = {
            'metadata': {'name': 'voice-gameintegration-principal-signing', 'namespace': 'voice-dev',
                'uid': '00000000-0000-0000-0000-000000000005', 'resourceVersion': '1'},
            'data': encoded({'current.pem': b'existing'})}
        with self.assertRaises(gis.Blocked):
            gis.build_secret_plan(kube, tls_validator=lambda *_: None,
                signer_validator=lambda _values: None, material_factory=materials)
        self.assertEqual(kube.writes, [])

        wrong = FakeKube()
        wrong.namespace['metadata']['name'] = 'voice-staging'
        with self.assertRaises(gis.Blocked):
            gis.build_secret_plan(wrong, tls_validator=lambda *_: None, material_factory=materials)
        self.assertEqual(wrong.writes, [])

    def test_resource_version_conflict_does_not_retry_or_overwrite(self):
        kube = FakeKube()
        plan = gis.build_secret_plan(kube, tls_validator=lambda *_: None,
            signer_validator=lambda _values: None, material_factory=materials)
        kube.conflict = True
        with self.assertRaises(gis.Blocked):
            gis.apply_secret_plan(kube, plan)

    def test_secret_mutations_carry_same_operation_marker(self):
        kube = FakeKube()
        plan = gis.build_secret_plan(kube, operation='123456abcdef',
            tls_validator=lambda *_: None, signer_validator=lambda _values: None,
            material_factory=materials)
        self.assertEqual({row['body']['metadata']['annotations'][gis.OPERATION_ANNOTATION]
            for row in plan['operations']}, {'123456abcdef'})


class ReadOnlyDatabasePreflightTests(unittest.TestCase):
    def test_database_presence_query_is_read_only_and_uses_no_credential_argument(self):
        identity = {'namespace_uid': 'n', 'service_uid': 's',
            'service_spec_sha256': 'a' * 64, 'endpoints': [{'uid': 'e', 'pod_uids': ['p']}]}

        class Kube:
            def __init__(self): self.calls = []
            def run(self, args, timeout=30):
                self.calls.append(args)
                if args[:2] == ['get', 'endpointslices']:
                    return {'items': [{'endpoints': [{'conditions': {'ready': True},
                        'targetRef': {'kind': 'Pod', 'name': 'voice-postgres-0', 'uid': 'p'}}]}]}
                if args[:2] == ['get', 'configmap']:
                    return {'data': {'POSTGRES_USER': 'voice'}}
                if '-d postgres' in args[-1]: return 'voice\n'
                return 'false|false\n'

        kube = Kube()
        with patch('gis_prerequisite._endpoint_identity', return_value=identity):
            self.assertEqual(gis._database_exists_readonly(kube, identity), identity)
        queries = [call for call in kube.calls if call[:2] == ['exec', 'voice-postgres-0']]
        self.assertEqual(len(queries), 2)
        self.assertIn('pg_database', queries[0][-1])
        self.assertIn('schema_migrations', queries[1][-1])
        query_text = ' '.join(' '.join(call) for call in queries)
        self.assertNotIn('POSTGRES_PASSWORD', query_text)
        self.assertNotIn('PGPASSWORD=', query_text)

    def test_wrong_owner_migration_ledger_and_runtime_role_block_initial_install(self):
        identity = {'namespace_uid': 'n', 'service_uid': 's',
            'service_spec_sha256': 'a' * 64, 'endpoints': [{'uid': 'e', 'pod_uids': ['p']}]}

        class Kube:
            def __init__(self, owner='voice', catalog='false|false'):
                self.owner, self.catalog = owner, catalog
            def run(self, args, timeout=30):
                if args[:2] == ['get', 'endpointslices']:
                    return {'items': [{'endpoints': [{'conditions': {'ready': True},
                        'targetRef': {'kind': 'Pod', 'name': 'voice-postgres-0', 'uid': 'p'}}]}]}
                if args[:2] == ['get', 'configmap']:
                    return {'data': {'POSTGRES_USER': 'voice'}}
                if '-d postgres' in args[-1]: return self.owner + '\n'
                return self.catalog + '\n'

        with patch('gis_prerequisite._endpoint_identity', return_value=identity):
            for kube in (Kube(owner='other'), Kube(catalog='true|false'), Kube(catalog='false|true')):
                with self.subTest(owner=kube.owner, catalog=kube.catalog):
                    with self.assertRaises(gis.Blocked):
                        gis._database_exists_readonly(kube, identity)
            self.assertEqual(gis._database_exists_readonly(Kube(catalog='true|true'),
                identity, require_initial_absence=False), identity)

    def test_missing_database_fails_closed_before_any_write(self):
        identity = {'namespace_uid': 'n', 'service_uid': 's',
            'service_spec_sha256': 'a' * 64, 'endpoints': [{'uid': 'e', 'pod_uids': ['p']}]}

        class Kube:
            writes = []
            def run(self, args, timeout=30):
                if args[:2] == ['get', 'endpointslices']:
                    return {'items': [{'endpoints': [{'conditions': {'ready': True},
                        'targetRef': {'kind': 'Pod', 'name': 'voice-postgres-0', 'uid': 'p'}}]}]}
                return 'f\n'

        with patch('gis_prerequisite._endpoint_identity', return_value=identity):
            with self.assertRaises(gis.Blocked):
                gis._database_exists_readonly(Kube(), identity)


class InstallerInvocationTests(unittest.TestCase):
    def test_callable_flow_orders_database_check_before_secrets_and_runs_shared_owner(self):
        import gis_first_install as shared_gis
        import gis_prerequisite

        image = gis.IMAGE + '@sha256:' + 'b' * 64
        source = 'a' * 40
        build = {'schema': gis.BUILD_RECEIPT_SCHEMA, 'source_sha': source,
            'image_ref': image, 'build_context_sha256': gis.hashlib.sha256(('e' * 40).encode()).hexdigest(),
            'image_config_sha256': 'd' * 64, 'working_tree_clean': True,
            'builder_kind': 'github-actions', 'builder_run': 11, 'builder_job': 12,
            'builder_sha': source, 'builder_tree': 'e' * 40,
            'builder_artifact_id': 13, 'builder_artifact_sha256': 'f' * 64,
            'builder_output_index_sha256': 'b' * 64, 'builder_provenance_sha256': '1' * 64}
        events = []
        operation = '123456abcdef'
        namespace_uid = '00000000-0000-0000-0000-000000000001'
        database_identity = {'namespace_uid': namespace_uid,
            'service_uid': '00000000-0000-0000-0000-000000000002',
            'service_spec_sha256': 'e' * 64,
            'endpoints': [{'uid': '00000000-0000-0000-0000-000000000003',
                'pod_uids': ['00000000-0000-0000-0000-000000000004']}]}
        inputs = {'schema': 'voice-gis-prerequisite-v1',
            'configmaps': {'provision': 'voice-provision-runtime-role-sql',
                'migrations': 'voice-gameintegration-migrations',
                'verify': 'voice-gameintegration-db-verifier-sql'},
            'jobs': {'provision': 'voice-provision-gameintegration-runtime-role',
                'migrate': 'voice-migrate-gameintegration-db',
                'verify': 'voice-verify-gameintegration-db'},
            'job_images': {role: {'gameintegration': logical} for role, logical in
                (('provision', 'postgres'), ('migrate', 'migrate'), ('verify', 'postgres'))},
            'images': {'postgres': 'postgres@sha256:' + 'c' * 64,
                'migrate': 'migrate@sha256:' + 'd' * 64}}
        service = {'apiVersion': 'v1', 'kind': 'Service',
            'metadata': {'name': 'voice-gameintegration', 'namespace': 'voice-dev'}, 'spec': {}}
        deployment = {'apiVersion': 'apps/v1', 'kind': 'Deployment',
            'metadata': {'name': 'voice-gameintegration', 'namespace': 'voice-dev'},
            'spec': {'replicas': 1, 'template': {'spec': {'containers': [
                {'name': 'gameintegration', 'image': image}]}}}}
        source_plan = {'schema': shared_gis.SOURCE_SCHEMA, 'objects': [service, deployment]}
        shared_prerequisite_execute = gis_prerequisite.execute

        class Kube:
            def get_namespace(self):
                events.append('namespace')
                return {'metadata': {'name': 'voice-dev', 'uid': namespace_uid}}

        def save(value):
            events.append('journal:' + str(value.get('phase')))

        def prerequisite(kube, plan, root, capture, save_plan):
            self.assertEqual(plan['source_build_receipt'], build)
            self.assertTrue(shared_gis.validate_dev_source_build_receipt(
                plan['source_build_receipt'], source, image))
            plan['secret_metadata'] = {'uid': '00000000-0000-0000-0000-000000000005',
                'resourceVersion': '5'}
            plan['config_metadata'] = {'uid': '00000000-0000-0000-0000-000000000006',
                'resourceVersion': '6'}
            config_docs = {role: object() for role in gis_prerequisite.JOB_ROLES}
            job_docs = {role: object() for role in gis_prerequisite.JOB_ROLES}
            configmaps = {role: {'name': name,
                'uid': '00000000-0000-0000-0000-000000000010', 'spec_sha256': '3' * 64}
                for role, name in (('provision', inputs['configmaps']['provision']),
                    ('migrate', inputs['configmaps']['migrations']),
                    ('verify', inputs['configmaps']['verify']))}
            jobs = {}
            for role, name in inputs['jobs'].items():
                jobs[role] = {'name': name,
                    'uid': '00000000-0000-0000-0000-000000000011', 'spec_sha256': '4' * 64,
                    'pod_uid': '00000000-0000-0000-0000-000000000012',
                    'image_ids': {container: inputs['images'][logical]
                        for container, logical in inputs['job_images'][role].items()},
                    'complete': True, 'succeeded': 1, 'completion_time': '2026-10-17T00:00:00Z'}
            created_records = iter([
                (kind, row) for role in gis_prerequisite.JOB_ROLES
                for kind, row in (('configmap', configmaps[role]), ('job', jobs[role]))])

            def create_once(_kube, kind, _obj, active_plan, _saved, _save):
                expected_kind, row = next(created_records)
                self.assertEqual(kind, expected_kind)
                record = {'uid': row['uid'], 'spec_sha256': row['spec_sha256']}
                active_plan['created'][f"{kind}/{row['name']}"] = record
                return None, record

            with (
                patch.object(gis_prerequisite, '_documents', return_value=(config_docs, job_docs)),
                patch.object(gis_prerequisite, 'preflight', return_value=database_identity),
                patch.object(gis_prerequisite, '_create_once', side_effect=create_once),
                patch.object(gis_prerequisite, '_verify_job', side_effect=[
                    (jobs[role], gis_prerequisite.OUTPUT) for role in gis_prerequisite.JOB_ROLES]),
                patch.object(gis_prerequisite, 'source_inputs', return_value=inputs),
            ):
                receipt = shared_prerequisite_execute(kube, plan, root, capture, save_plan)
            self.assertTrue(shared_gis.validate_prerequisite_receipt(receipt, plan))
            events.append('prerequisite')
            plan['prerequisite_receipt'] = receipt
            save_plan(plan)
            return receipt

        def create_pair(kube, plan, save_plan, *, start):
            self.assertFalse(start)
            events.append('paused-pair')
            plan['phase'] = 'CREATED'
            save_plan(plan)
            return {}

        def start(kube, plan, save_plan):
            events.append('guarded-start')
            plan['phase'] = 'STARTED'
            plan['receipt'] = {'schema': 'voice-gis-first-install-resource-v1'}
            save_plan(plan)
            return plan['receipt']

        ready = {'schema': 'voice-gis-first-install-ready-v1',
            'deployment_uid': 'deployment-uid'}
        with tempfile.TemporaryDirectory() as directory:
            services = Path(directory) / 'deploy' / 'staging' / 'services.yaml'
            services.parent.mkdir(parents=True)
            services.write_text('apiVersion: v1\nkind: Service\n', encoding='utf-8')
            state_path = Path(directory) / 'state.json'
            with (
                patch.object(gis, '_source_pair', return_value=source_plan),
                patch.object(gis, '_capture_prerequisite_sources', return_value={'source': 'hash'}),
                patch.object(gis_prerequisite, 'source_inputs', return_value=inputs),
                patch.object(gis_prerequisite, '_endpoint_identity', return_value=database_identity),
                patch.object(gis, '_database_exists_readonly', side_effect=lambda *_args, **_kwargs:
                    events.append('database-readonly')),
                patch.object(gis, 'apply_secret_plan', side_effect=lambda *_args:
                    events.append('secret-write')),
                patch.object(gis, 'secret_write_receipt', return_value={}),
                patch.object(gis_prerequisite, 'execute', side_effect=prerequisite),
                patch.object(shared_gis, 'execute', side_effect=create_pair),
                patch.object(shared_gis, 'start_owned', side_effect=start),
                patch.object(shared_gis, 'verify_ready', side_effect=lambda *_args:
                    events.append('readiness') or ready),
            ):
                result = gis.install(Path(directory), image, state_path, kube=Kube(),
                    apply=True, operation=operation,
                    build_receipt_factory=lambda *_args, **_kwargs: (source, build),
                    source_verifier=lambda *_args: None,
                    secret_plan_factory=lambda *_args, **_kwargs:
                        events.append('tls') or {'operations': [], 'namespace_uid': namespace_uid, 'operation': operation})

            self.assertEqual(result, ready)
            ordered = ['tls', 'database-readonly', 'secret-write', 'prerequisite',
                'paused-pair', 'guarded-start', 'readiness']
            positions = [events.index(item) for item in ordered]
            self.assertEqual(positions, sorted(positions))
            saved = json.loads(state_path.read_text(encoding='utf-8'))
            self.assertEqual(saved['phase'], 'READY')
            self.assertNotIn('runtime-password', state_path.read_text(encoding='utf-8'))

    def test_forged_build_receipt_blocks_before_namespace_or_secret_access(self):
        image = gis.IMAGE + '@sha256:' + 'b' * 64
        source = 'a' * 40
        tree = 'e' * 40
        forged = {'schema': gis.BUILD_RECEIPT_SCHEMA, 'source_sha': source,
            'image_ref': image, 'build_context_sha256': 'f' * 64,
            'image_config_sha256': 'd' * 64, 'working_tree_clean': True,
            'builder_kind': 'github-actions', 'builder_run': 11, 'builder_job': 12,
            'builder_sha': source, 'builder_tree': tree,
            'builder_artifact_id': 13, 'builder_artifact_sha256': 'f' * 64,
            'builder_output_index_sha256': 'b' * 64, 'builder_provenance_sha256': '1' * 64}
        calls = []

        class Kube:
            def get_namespace(self):
                calls.append('namespace')
                return {'metadata': {'name': 'voice-dev', 'uid': 'namespace-uid'}}

        with tempfile.TemporaryDirectory() as directory:
            with patch.object(gis, 'verify_current_source', return_value=None):
                with self.assertRaises(gis.Blocked):
                    gis.install(Path(directory), image, Path(directory) / 'state.json',
                        kube=Kube(), apply=True,
                        build_receipt_factory=lambda *_args, **_kwargs: (source, forged))
        self.assertEqual(calls, [])


if __name__ == '__main__':
    unittest.main()
