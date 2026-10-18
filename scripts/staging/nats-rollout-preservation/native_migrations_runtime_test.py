import copy
import hashlib
import unittest
from unittest.mock import patch

import migrations
import native_migrations_runtime as native
from controller import Blocked


OP = "a" * 12
UID_VALUES = "11111111-1111-4111-8111-111111111111 22222222-2222-4222-8222-222222222222 33333333-3333-4333-8333-333333333333".split()


class NotFound(Exception):
    pass


def plan():
    text = "CREATE TABLE example (id bigint);\n"
    return [{"database": "social", "image": migrations.IMAGE,
             "secret_key": "SOCIAL_DATABASE_URL",
             "files": {"000001_example.up.sql": text},
             "hashes": {"000001_example.up.sql": hashlib.sha256(text.encode()).hexdigest()}}]


class FakeAPI:
    def __init__(self, *, create_unknown=False, preexisting=None, pod_profile=None):
        self.objects = {} if preexisting is None else copy.deepcopy(preexisting)
        self.calls = []
        self.create_unknown = create_unknown
        self.pod_profile = pod_profile
        self.uids = iter(UID_VALUES)
        self.pods = {}

    def get_resource(self, kind, namespace, name):
        self.calls.append(("get", kind, name))
        value = self.objects.get((kind, namespace, name))
        if value is None:
            raise NotFound("missing")
        return copy.deepcopy(value)

    def create_resource(self, kind, namespace, manifest):
        self.calls.append(("create", kind, manifest["metadata"]["name"]))
        value = copy.deepcopy(manifest)
        value.setdefault("metadata", {})["uid"] = next(self.uids)
        self.objects[(kind, namespace, manifest["metadata"]["name"])] = value
        if self.create_unknown:
            raise TimeoutError("create response lost")
        return copy.deepcopy(value)

    def wait_job(self, namespace, name, uid, timeout_seconds):
        self.calls.append(("wait", name, uid, timeout_seconds))
        value = copy.deepcopy(self.objects[("Job", namespace, name)])
        value["status"] = {"succeeded": 1}
        self.objects[("Job", namespace, name)] = value
        return value

    def get_job_pod(self, namespace, name, uid):
        self.calls.append(("pod", name, uid))
        if (namespace, name, uid) in self.pods:
            return copy.deepcopy(self.pods[(namespace, name, uid)])
        job = self.objects[("Job", namespace, name)]
        pod = {"metadata": {"name": name + "-pod", "uid": next(self.uids)},
               "spec": copy.deepcopy(job["spec"]["template"]["spec"])}
        if self.pod_profile is not None:
            pod["spec"] = self.pod_profile
        self.pods[(namespace, name, uid)] = copy.deepcopy(pod)
        return pod


class NativeMigrationRuntimeTests(unittest.TestCase):
    def test_request_is_exact_canonical_projection_and_fixed_profile(self):
        request = native.build_request(plan(), "app-only", OP)
        self.assertEqual(request["namespace"], "voice-staging")
        self.assertEqual(request["service_account"], "voice-nats-staging-runner")
        self.assertEqual([row["kind"] for row in request["resources"]], ["ConfigMap", "Job"])
        job = request["resources"][1]
        pod = job["spec"]["template"]["spec"]
        self.assertFalse(pod["automountServiceAccountToken"])
        self.assertEqual(pod["securityContext"], {"runAsNonRoot": True, "runAsUser": 65532, "runAsGroup": 65532})
        self.assertEqual(job["spec"]["backoffLimit"], 0)
        self.assertEqual(job["spec"]["activeDeadlineSeconds"], 300)

    def test_images_only_keeps_the_canonical_empty_plan(self):
        request = native.build_request([], "images-only", OP)
        self.assertEqual(request["resources"], [])
        self.assertEqual(request["plan"], [])

    def test_arbitrary_manifest_or_service_account_is_rejected(self):
        request = native.build_request(plan(), "app-only", OP)
        request["service_account"] = "attacker"
        with self.assertRaises(Blocked):
            native.apply(FakeAPI(), request, journal_event=lambda _: None, revalidate=lambda: None)
        request = native.build_request(plan(), "app-only", OP)
        request["resources"][1]["spec"]["template"]["spec"]["containers"][0]["image"] = "latest"
        with self.assertRaises(Blocked):
            native.apply(FakeAPI(), request, journal_event=lambda _: None, revalidate=lambda: None)

    def test_journal_precedes_create_and_receipt_contains_job_pod_identity(self):
        api = FakeAPI()
        events = []
        receipt = native.apply(api, native.build_request(plan(), "app-only", OP),
                               journal_event=events.append, revalidate=lambda: None)
        self.assertEqual([row[0] for row in api.calls], ["get", "create", "get", "create", "wait", "pod"])
        self.assertEqual(events[0]["kind"], "native_migration_resource_intent")
        self.assertEqual(events[1]["kind"], "native_migration_resource_issued")
        self.assertEqual(receipt["jobs"][0]["pod_uid"], "33333333-3333-4333-8333-333333333333")
        self.assertEqual(receipt["jobs"][0]["profile_sha256"], native._digest(native.build_request(plan(), "app-only", OP)["resources"][1]["spec"]["template"]["spec"]))

    def test_create_timeout_reconciles_once_and_never_reposts(self):
        api = FakeAPI(create_unknown=True)
        events = []
        native.apply(api, native.build_request(plan(), "app-only", OP),
                     journal_event=events.append, revalidate=lambda: None)
        self.assertEqual(sum(call[0] == "create" for call in api.calls), 2)
        self.assertEqual(sum(call[0] == "get" for call in api.calls), 4)

    def test_resume_uses_journal_and_does_not_recreate_completed_resources(self):
        api = FakeAPI()
        events = []
        request = native.build_request(plan(), "app-only", OP)
        native.apply(api, request, journal_event=events.append, revalidate=lambda: None)
        calls_before = len(api.calls)
        native.apply(api, request, journal_event=events.append, revalidate=lambda: None,
                     journal_events=events)
        self.assertEqual(sum(call[0] == "create" for call in api.calls[calls_before:]), 0)

    def test_legacy_authority_secret_is_a_fixed_input_and_not_read_by_adapter(self):
        text = "CREATE TABLE example (id bigint);\n"
        row = {"database": "bot", "image": migrations.IMAGE,
               "secret_key": "BOT_DATABASE_URL",
               "files": {"000001_example.up.sql": text},
               "hashes": {"000001_example.up.sql": hashlib.sha256(text.encode()).hexdigest()}}
        secret = {"apiVersion": "v1", "kind": "Secret", "type": "Opaque", "immutable": True,
                  "metadata": {"name": "voice-rollout-" + OP + "-database-urls", "namespace": "voice-staging",
                               "labels": {"voice-rollout-operation": OP}},
                  "data": {"BOT_DATABASE_URL": "cG9zdGdyZXM6Ly92b2ljZQ=="}}
        request = native.build_request([row], "app-only", OP, authority_secret=secret)
        self.assertEqual(request["resources"][0]["kind"], "Secret")
        self.assertEqual(request["resources"][0]["metadata"]["name"], "voice-rollout-" + OP + "-database-urls")

    def test_preexisting_resource_without_intent_and_pod_drift_fail_closed(self):
        request = native.build_request(plan(), "app-only", OP)
        resource = request["resources"][0]
        existing = copy.deepcopy(resource)
        existing["metadata"]["uid"] = "11111111-1111-4111-8111-111111111111"
        with self.assertRaises(Blocked):
            native.apply(FakeAPI(preexisting={("ConfigMap", "voice-staging", resource["metadata"]["name"]): existing}), request,
                         journal_event=lambda _: None, revalidate=lambda: None)
        with self.assertRaises(Blocked):
            native.apply(FakeAPI(pod_profile={}), request, journal_event=lambda _: None, revalidate=lambda: None)


if __name__ == "__main__":
    unittest.main()
