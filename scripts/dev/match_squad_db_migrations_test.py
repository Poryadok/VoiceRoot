import unittest
import copy
import io
from contextlib import redirect_stdout
from unittest import mock
from pathlib import Path

try:
    import match_squad_db_migrations as runner
except ModuleNotFoundError:
    runner = None


class MatchSquadMigrationRunnerContracts(unittest.TestCase):
    class ReadOnlyTarget:
        """Fake only the bounded kubectl reads using their real output shapes."""
        def __init__(self):
            self.rv = 10

        def get_namespace(self):
            return {"metadata": {"name": runner.NAMESPACE, "uid": runner.NAMESPACE_UID}}

        def get(self, kind, name):
            self.rv += 1
            metadata = {"name": name, "namespace": runner.NAMESPACE,
                "uid": {"service": "pg-service-uid", "statefulset": "pg-sts-uid",
                    "pod": "pg-pod-uid"}[kind], "resourceVersion": str(self.rv)}
            if kind == "service":
                return {"metadata": metadata, "spec": {"ports": [{"port": 5432,
                    "protocol": "TCP", "targetPort": 5432}], "selector": {"app": "postgres"}}}
            if kind == "statefulset":
                return {"metadata": {**metadata, "generation": 1}, "spec": {
                    "replicas": 1, "template": {"spec": {"containers": [{"name": "postgres",
                        "image": runner.POSTGRES_IMAGE}]}}}, "status": {"observedGeneration": 1,
                            "readyReplicas": 1}}
            if kind == "pod":
                digest = "a" * 64
                return {"metadata": {**metadata, "ownerReferences": [{"kind": "StatefulSet",
                    "name": "voice-postgres", "uid": "pg-sts-uid"}], "labels": {"app": "postgres"}},
                    "spec": {"containers": [{"name": "postgres", "image": runner.POSTGRES_IMAGE}]},
                    "status": {"containerStatuses": [{"name": "postgres", "ready": True,
                        "imageID": "docker.io/library/postgres@sha256:" + digest}]}}
            raise AssertionError("unexpected fake resource " + kind)

        def run(self, args, **kwargs):
            if args[:2] == ["get", "endpointslices"]:
                return {"items": [{"endpoints": [{"conditions": {"ready": True},
                    "targetRef": {"kind": "Pod", "name": "voice-postgres-0", "uid": "pg-pod-uid"}}]}]}
            if args[:3] == ["get", "secret", "voice-app-secrets"]:
                # Match kubectl's Go-template string result. Only key names are returned.
                return "voice-app-secrets|voice-dev|db-secret-uid|12|CHAT_DATABASE_URL MATCHMAKING_DATABASE_URL VOICE_DATABASE_URL "
            raise AssertionError("unexpected fake kubectl call: " + repr(args))

    def test_target_only_runner_module_exists(self):
        self.assertIsNotNone(
            runner,
            "missing target-only DEV MatchSquad migration runner; bootstrap currently applies all databases",
        )

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_allowlist_and_planned_input_hashes_are_fixed(self):
        root = Path(__file__).resolve().parents[2]
        services = runner.plan_services()
        self.assertEqual(services, ("matchmaking", "chat", "voice"))
        plan = runner.build_plan(root, "a" * 40)
        self.assertEqual(tuple(item["service"] for item in plan), services)
        self.assertEqual(
            tuple(item["expected_version"] for item in plan), (18, 23, 11)
        )
        self.assertEqual(
            tuple(item["baseline_version"] for item in plan), (15, 22, 10)
        )
        self.assertEqual(
            [name for item in plan for name in item["new_up_migrations"]],
            [
                "000016_match_deadline_squad_recovery.up.sql",
                "000017_match_squad_lifecycle.up.sql",
                "000018_match_squad_durable_recovery.up.sql",
                "000023_match_squad_operations.up.sql",
                "000011_match_squad_operations.up.sql",
            ],
        )

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_wrong_source_namespace_and_out_of_scope_target_fail_closed(self):
        with self.assertRaises(runner.Blocked):
            runner.validate_source("b" * 40, "a" * 40, clean=True)
        with self.assertRaises(runner.Blocked):
            runner.validate_source("a" * 40, "a" * 40, clean=False)
        with self.assertRaises(runner.Blocked):
            runner.validate_namespace(
                {"metadata": {"name": "voice-staging", "uid": runner.NAMESPACE_UID}}
            )
        with self.assertRaises(runner.Blocked):
            runner.database_for_service("auth")
        with self.assertRaises(runner.Blocked):
            runner.database_for_service("gameintegration")

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_ledgers_require_expected_database_clean_baseline_or_proven_completion(self):
        matchmaking = runner.plan_services()[0]
        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        self.assertEqual(runner.validate_ledger(item, "matchmaking_db|15|f\n"), 15)
        with self.assertRaises(runner.Blocked):
            runner.validate_ledger(item, "matchmaking_db|15|t\n")
        with self.assertRaises(runner.Blocked):
            runner.validate_ledger(item, "matchmaking_db|14|f\n")
        with self.assertRaises(runner.Blocked):
            runner.parse_ledger(matchmaking, "matchmaking_db|15|f\nmatchmaking_db|16|f\n")
        with self.assertRaises(runner.Blocked):
            runner.parse_ledger(matchmaking, "chat_db|15|f\n")
        with self.assertRaises(runner.Blocked):
            runner.parse_ledger("auth", "auth_db|15|f\n")

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_foreign_create_only_collision_is_never_replaced_or_deleted(self):
        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        obj = {
            "kind": "ConfigMap",
            "metadata": {
                "name": item["configmap_name"],
                "namespace": "voice-dev",
                "uid": "foreign-uid",
                "resourceVersion": "1",
                "annotations": {runner.SOURCE_ANNOTATION: "b" * 40},
            },
            "data": item["migration_data"],
        }
        with self.assertRaises(runner.Blocked):
            runner.validate_bound_object(obj, item, "configmap")

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_interrupted_or_failed_job_is_not_replayed_and_uid_readback_is_bound(self):
        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        expected = runner.job_manifest(item)
        expected["metadata"].update({"uid": "uid-job", "resourceVersion": "8"})
        expected["spec"]["selector"] = {"matchLabels": {
            "batch.kubernetes.io/controller-uid": "uid-job"}}
        active = {**expected, "status": {"active": 1}}
        with self.assertRaises(runner.Blocked):
            runner.validate_terminal_job(active, item)
        failed = {**expected, "status": {"failed": 1}}
        with self.assertRaises(runner.Blocked):
            runner.validate_terminal_job(failed, item)
        created = {"metadata": {"uid": "uid-1", "resourceVersion": "7"}}
        changed = {"metadata": {"uid": "uid-2", "resourceVersion": "8"}}
        with self.assertRaises(runner.Blocked):
            runner.validate_create_readback(created, changed)
        advanced = {"metadata": {"uid": "uid-1", "resourceVersion": "8"}}
        self.assertTrue(runner.validate_create_readback(created, advanced))
        self.assertTrue(runner.validate_create_readback(created, created))

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_live_job_defaulting_is_allowed_but_source_spec_drift_is_rejected(self):
        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        live = runner.job_manifest(item)
        live["metadata"].update({"uid": "uid-job", "resourceVersion": "8"})
        live["spec"].update({"parallelism": 1, "completions": 1,
            "completionMode": "NonIndexed", "suspend": False,
            "selector": {"matchLabels": {"batch.kubernetes.io/controller-uid": "uid-job"}}})
        live["spec"]["template"]["spec"]["dnsPolicy"] = "ClusterFirst"
        live["spec"]["template"]["spec"].update({
            "schedulerName": "default-scheduler", "terminationGracePeriodSeconds": 30,
            "serviceAccountName": "default", "enableServiceLinks": True,
            "preemptionPolicy": "PreemptLowerPriority",
        })
        live["spec"]["template"]["spec"]["containers"][0]["imagePullPolicy"] = "IfNotPresent"
        live["spec"]["template"]["spec"]["containers"][0].update({
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File", "stdin": False,
            "stdinOnce": False, "tty": False,
        })
        live["spec"]["template"]["spec"]["initContainers"][0].update({
            "imagePullPolicy": "IfNotPresent",
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File", "stdin": False,
            "stdinOnce": False, "tty": False,
        })
        live["spec"]["template"]["spec"]["volumes"][0]["configMap"].update(
            {"defaultMode": 420, "optional": False}
        )
        self.assertTrue(runner.validate_bound_object(live, item, "job"))
        mutations = []
        sidecar = copy.deepcopy(live)
        sidecar["spec"]["template"]["spec"]["containers"].append({"name": "injected"})
        mutations.append(sidecar)
        extra_env = copy.deepcopy(live)
        extra_env["spec"]["template"]["spec"]["containers"][0]["envFrom"] = [
            {"secretRef": {"name": "foreign"}}
        ]
        mutations.append(extra_env)
        secret_volume = copy.deepcopy(live)
        secret_volume["spec"]["template"]["spec"]["volumes"].append(
            {"name": "foreign", "secret": {"secretName": "foreign"}}
        )
        mutations.append(secret_volume)
        privileged = copy.deepcopy(live)
        privileged["spec"]["template"]["spec"]["containers"][0]["securityContext"] = {
            "privileged": True
        }
        mutations.append(privileged)
        extra_args = copy.deepcopy(live)
        extra_args["spec"]["template"]["spec"]["containers"][0]["args"] = ["--all"]
        mutations.append(extra_args)
        wrong_image = copy.deepcopy(live)
        wrong_image["spec"]["template"]["spec"]["containers"][0]["image"] = "untrusted:latest"
        mutations.append(wrong_image)
        for changed in mutations:
            with self.subTest(spec=changed["spec"]["template"]["spec"]), \
                 self.assertRaises(runner.Blocked):
                runner.validate_bound_object(changed, item, "job")

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_exact_job_manifest_is_create_only_and_uses_pinned_images(self):
        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        manifest = runner.job_manifest(item)
        pod = manifest["spec"]["template"]["spec"]
        self.assertRegex(runner.MIGRATE_IMAGE, r"@sha256:[0-9a-f]{64}$")
        self.assertRegex(runner.POSTGRES_IMAGE, r"@sha256:[0-9a-f]{64}$")
        self.assertEqual(manifest["spec"]["backoffLimit"], 0)
        self.assertEqual(pod["restartPolicy"], "Never")
        self.assertEqual(pod["containers"][0]["image"], runner.MIGRATE_IMAGE)
        self.assertEqual(pod["initContainers"][0]["image"], runner.POSTGRES_IMAGE)
        self.assertEqual(
            pod["containers"][0]["env"][0]["valueFrom"]["secretKeyRef"]["key"],
            "MATCHMAKING_DATABASE_URL",
        )
        self.assertFalse(pod["automountServiceAccountToken"])
        with mock.patch.object(runner, "MIGRATE_IMAGE", "migrate/migrate:v4.18.1"):
            with self.assertRaises(runner.Blocked):
                runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_live_preflight_identity_allows_opaque_resource_version_advancement(self):
        target = self.ReadOnlyTarget()
        initial = runner._postgres_target_identity(target)
        secret = runner._credential_secret_identity(target)
        prepared = {"postgres_identity": initial, "credential_secret": secret}
        # Each API GET may return a newer opaque RV while UIDs and specs stay stable.
        runner._verify_live_targets(target, prepared)

        original_get = target.get
        def changed_service_uid(kind, name):
            value = original_get(kind, name)
            if kind == "service":
                value["metadata"]["uid"] = "replacement-service-uid"
            return value
        target.get = changed_service_uid
        with self.assertRaises(runner.Blocked):
            runner._verify_live_targets(target, prepared)

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_preflight_requires_exact_source_namespace_and_all_three_clean_baselines(self):
        root = Path(__file__).resolve().parents[2]

        class FakeKube(self.ReadOnlyTarget):
            def __init__(self, namespace):
                super().__init__()
                self.namespace = namespace
                self.writes = []

            def get_namespace(self):
                return self.namespace

            def run(self, args, **kwargs):
                self.writes.append(args)
                return super().run(args, **kwargs)

        namespace = {"metadata": {"name": runner.NAMESPACE, "uid": runner.NAMESPACE_UID}}
        kube = FakeKube(namespace)
        with mock.patch.object(runner, "_read_source_identity", return_value=("a" * 40, False)), \
             mock.patch.object(runner, "_read_ledger", side_effect=lambda _k, item, _p:
                 (item["baseline_version"], False)):
            prepared = runner.preflight(kube, root, "a" * 40)
        self.assertEqual(tuple(prepared["ledgers"]), runner.SERVICES)
        self.assertEqual(len(prepared["plan"]), 3)
        self.assertFalse(any(call[0] == "create" for call in kube.writes))

        for source_result, wrong_namespace in [
            (("b" * 40, False), namespace),
            (("a" * 40, True), namespace),
            (("a" * 40, False), {"metadata": {"name": "voice-staging", "uid": runner.NAMESPACE_UID}}),
        ]:
            rejected = FakeKube(wrong_namespace)
            with self.subTest(source=source_result, namespace=wrong_namespace), \
                 mock.patch.object(runner, "_read_source_identity", return_value=source_result):
                with self.assertRaises(runner.Blocked):
                    runner.preflight(rejected, root, "a" * 40)
            self.assertFalse(any(call[0] == "create" for call in rejected.writes))

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_bad_ledger_and_foreign_create_only_collision_stop_before_any_create(self):
        root = Path(__file__).resolve().parents[2]
        item = runner.build_plan(root, "a" * 40)[0]

        class FakeKube:
            def __init__(self, existing):
                self.existing = existing
                self.creates = []

            def get_namespace(self):
                return {"metadata": {"name": runner.NAMESPACE, "uid": runner.NAMESPACE_UID}}

            def get(self, kind, _name):
                if kind == "configmap":
                    return self.existing
                if kind == "job":
                    return self.job
                raise AssertionError("unexpected kind")

            def run(self, args, **kwargs):
                if args[0] == "create":
                    self.creates.append((args, kwargs))
                return ""

        foreign = {"kind": "ConfigMap", "metadata": {"name": item["configmap_name"],
            "namespace": runner.NAMESPACE, "uid": "other", "resourceVersion": "1",
            "annotations": {runner.SOURCE_ANNOTATION: "b" * 40}}, "data": item["migration_data"]}
        kube = FakeKube(foreign)
        kube.job = None
        root = Path(__file__).resolve().parents[2]
        prepared = {"plan": [item], "postgres_pod": "voice-postgres-0",
            "source_sha": "a" * 40, "root": str(root)}
        with mock.patch.object(runner, "_read_source_identity", return_value=("a" * 40, False)), \
             mock.patch.object(runner, "_read_ledger", return_value=(item["baseline_version"], False)):
            with self.assertRaises(runner.Blocked):
                runner.execute(kube, prepared)
        self.assertEqual(kube.creates, [])

        kube = FakeKube(None)
        kube.job = None
        with mock.patch.object(runner, "_read_source_identity", return_value=("a" * 40, False)), \
             mock.patch.object(runner, "_read_ledger", return_value=(item["baseline_version"] - 1, False)):
            with self.assertRaises(runner.Blocked):
                runner.execute(kube, prepared)
        self.assertEqual(kube.creates, [])

        kube = FakeKube(runner.configmap_manifest(item))
        kube.existing["metadata"].update({"uid": "uid-cm", "resourceVersion": "3"})
        kube.job = runner.job_manifest(item)
        kube.job["metadata"].update({"uid": "uid-job", "resourceVersion": "8"})
        kube.job["spec"]["selector"] = {"matchLabels": {
            "batch.kubernetes.io/controller-uid": "uid-job"}}
        kube.job["status"] = {"active": 1}
        with mock.patch.object(runner, "_read_source_identity", return_value=("a" * 40, False)), \
             mock.patch.object(runner, "_read_ledger", return_value=(item["baseline_version"], False)):
            with self.assertRaises(runner.Blocked):
                runner.execute(kube, prepared)
        self.assertEqual(kube.creates, [])

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_apply_entrypoint_drives_adapter_contract_for_exact_three_targets(self):
        root = Path(__file__).resolve().parents[2]

        class FakeKube(self.ReadOnlyTarget):
            def __init__(self):
                super().__init__()
                self.objects = {}
                self.versions = dict(runner.BASELINES)
                self.job_reads = {}
                self.writes = []

            def run(self, args, body=None, timeout=30):
                if args[:2] == ["get", "endpointslices"] or args[:3] == ["get", "secret", "voice-app-secrets"]:
                    return super().run(args, timeout=timeout)
                if args[0] == "exec":
                    command = args[-1]
                    database = next(name for name in runner.DATABASES.values() if "-d " + name + " " in command)
                    service = next(name for name, db in runner.DATABASES.items() if db == database)
                    return f"{database}|{self.versions[service]}|f\n"
                if args[0] == "create":
                    self.writes.append(body["kind"])
                    obj = copy.deepcopy(body)
                    obj["metadata"].update({"uid": "uid-" + str(len(self.objects) + 1),
                        "resourceVersion": str(10 + len(self.objects))})
                    if body["kind"] == "Job":
                        uid = obj["metadata"]["uid"]
                        obj["spec"]["selector"] = {"matchLabels": {
                            "batch.kubernetes.io/controller-uid": uid}}
                        obj["status"] = {"active": 1}
                    key = (body["kind"].lower(), body["metadata"]["name"])
                    self.objects[key] = obj
                    return copy.deepcopy(obj)
                raise AssertionError("unexpected adapter command: " + repr(args))

            def get(self, kind, name):
                if kind in {"service", "statefulset", "pod"}:
                    return super().get(kind, name)
                key = (kind, name)
                obj = self.objects.get(key)
                if obj is None:
                    return None
                if kind == "job":
                    reads = self.job_reads.get(name, 0)
                    self.job_reads[name] = reads + 1
                    if reads >= 1 and obj["status"].get("active"):
                        service = next(service for service in runner.SERVICES
                            if name.startswith("voice-msq-migrate-" + service + "-"))
                        self.versions[service] = runner.TARGETS[service]
                        obj["status"] = {"succeeded": 1}
                return copy.deepcopy(obj)

        fake = FakeKube()
        output = io.StringIO()
        with mock.patch.object(runner, "_read_source_identity", return_value=("a" * 40, False)), \
             mock.patch.object(runner, "Kubectl", return_value=fake), \
             redirect_stdout(output):
            result = runner.main(["--source-sha", "a" * 40, "--root", str(root), "--apply"])
        self.assertEqual(result, 0)
        self.assertIn("MATCH_SQUAD_DEV_MIGRATIONS_COMPLETE", output.getvalue())
        self.assertEqual(fake.writes, ["ConfigMap", "Job"] * 3)
        self.assertEqual(fake.versions, runner.TARGETS)


if __name__ == "__main__":
    unittest.main()
