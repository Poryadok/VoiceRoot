import unittest
import copy
import io
import json
import os
import stat
import subprocess
import tempfile
from contextlib import redirect_stderr, redirect_stdout
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
                        "imageID": "docker.io/library/postgres@sha256:" + digest}],
                        "podIP": "10.42.0.3"}}
            raise AssertionError("unexpected fake resource " + kind)

        def run(self, args, **kwargs):
            if args[:2] == ["get", "endpointslices"]:
                return {"items": [{"ports": [{"name": None, "port": 5432, "protocol": "TCP"}],
                    "endpoints": [{"addresses": ["10.42.0.3"], "conditions": {"ready": True},
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
    def test_ledger_read_query_keeps_sql_literals_out_of_shell_quotes(self):
        class LedgerKube:
            def __init__(self):
                self.args = None

            def run(self, args, **kwargs):
                self.args = args
                return "matchmaking_db|15|f\n"

        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        kube = LedgerKube()
        self.assertEqual(runner._read_ledger(kube, item, "voice-postgres-0"), (15, False))
        shell = kube.args[-1]
        self.assertIn("chr(124)", shell)
        self.assertIn("chr(116)", shell)
        self.assertNotIn("'|'", shell)

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_kubectl_create_uses_source_document_json_stdin_and_safe_failure_context(self):
        root = Path(__file__).resolve().parents[2]
        item = runner.build_plan(root, "a" * 40)[0]
        document = runner.configmap_manifest(item)
        observed = {}

        def accepted(argv, **kwargs):
            observed.update({"argv": argv, **kwargs})
            return subprocess.CompletedProcess(argv, 0, '{"kind":"ConfigMap"}', "")

        kube = runner.Kubectl(runner=accepted)
        response = kube.run(["create", "-f", "/dev/stdin", "-o", "json"], body=document)
        self.assertEqual(response, {"kind": "ConfigMap"})
        self.assertEqual(observed["argv"], ["kubectl", "--namespace", runner.NAMESPACE,
            "create", "-f", "-", "-o", "json"])
        self.assertEqual(json.loads(observed["input"]), document)
        self.assertIs(observed["check"], False)
        self.assertTrue(observed["capture_output"])
        self.assertTrue(observed["text"])

        def rejected(argv, **kwargs):
            return subprocess.CompletedProcess(argv, 1, "",
                "Error from server (Forbidden): credential=private")

        kube = runner.Kubectl(runner=rejected)
        with self.assertRaises(runner.KubeError) as caught:
            kube.run(["create", "-f", "/dev/stdin", "-o", "json"], body=document)
        self.assertEqual(caught.exception.stage, "create.migration_configmap")
        self.assertEqual(caught.exception.status, 403)
        self.assertNotIn("credential", str(caught.exception))
        self.assertNotIn("private", str(caught.exception))

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_cli_reports_only_safe_kubernetes_stage_and_status(self):
        root = Path(__file__).resolve().parents[2]
        stderr = io.StringIO()
        with mock.patch.object(runner, "preflight",
                side_effect=runner.KubeError(403, "get.namespace")), \
             redirect_stderr(stderr):
            result = runner.main(["--source-sha", "a" * 40, "--root", str(root), "--apply"])
        self.assertEqual(result, 2)
        self.assertEqual(stderr.getvalue().strip(),
            "BLOCKED=kubernetes_api_error stage=get.namespace status=403")

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
        item["postgres_ip"] = "10.42.0.3"
        expected = runner.job_manifest(item, item["postgres_ip"])
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
    def test_controller_template_labels_are_bound_to_exact_job_identity(self):
        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        item["postgres_ip"] = "10.42.0.3"
        job = runner.job_manifest(item, item["postgres_ip"])
        job["metadata"].update({"uid": "job-uid", "resourceVersion": "8"})
        job["spec"]["selector"] = {"matchLabels": {
            "batch.kubernetes.io/controller-uid": "job-uid"}}
        labels = job["spec"]["template"]["metadata"]["labels"]
        labels.update({
            "batch.kubernetes.io/controller-uid": "job-uid",
            "controller-uid": "job-uid",
            "batch.kubernetes.io/job-name": item["job_name"],
            "job-name": item["job_name"],
        })
        self.assertTrue(runner.validate_bound_object(job, item, "job"))

        for key, bad_value in (
            ("batch.kubernetes.io/controller-uid", "other-uid"),
            ("controller-uid", "other-uid"),
            ("batch.kubernetes.io/job-name", "other-job"),
            ("job-name", "other-job"),
            ("unrecognized", "injected"),
        ):
            altered = copy.deepcopy(job)
            altered["spec"]["template"]["metadata"]["labels"][key] = bad_value
            with self.subTest(label=key), self.assertRaises(runner.Blocked):
                runner.validate_bound_object(altered, item, "job")

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_migration_failure_log_is_allowlisted_and_never_emits_private_stderr(self):
        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        command = runner.job_manifest(item, "10.42.0.3")["spec"]["template"]["spec"]["containers"][0]["command"][2]
        self.assertIn("failure_category=unknown", command)
        self.assertIn("failure_category=database_unavailable", command)
        self.assertIn("failure_category=database_authentication", command)
        self.assertIn("failure_category=migration_sql", command)
        self.assertIn("failure_category=migration_runner", command)
        self.assertIn("migration_failed category=$failure_category exit=1", command)
        self.assertNotIn("cat /run/migration/migrate.stderr", command)

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_plan_binds_only_the_three_baseline_down_readers_and_five_up_migrations(self):
        plan = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)
        expected_down = {
            "matchmaking": "000015_reconcile_player_rating_aggregates.down.sql",
            "chat": "000022_chat_create_requests.down.sql",
            "voice": "000010_space_media_admission.down.sql",
        }
        self.assertEqual(sum(len(item["new_up_migrations"]) for item in plan), 5)
        for item in plan:
            service = item["service"]
            self.assertEqual(item["baseline_down_migration"], expected_down[service])
            self.assertEqual(set(item["migration_data"]),
                set(item["new_up_migrations"]) | {expected_down[service]})
            self.assertFalse(any(name.endswith(".down.sql") for name in item["new_up_migrations"]))

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_live_job_defaulting_is_allowed_but_source_spec_drift_is_rejected(self):
        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        item["postgres_ip"] = "10.42.0.3"
        live = runner.job_manifest(item, item["postgres_ip"])
        live["metadata"].update({"uid": "uid-job", "resourceVersion": "8"})
        live["spec"].update({"parallelism": 1, "completions": 1,
            "completionMode": "NonIndexed", "suspend": False,
            "selector": {"matchLabels": {"batch.kubernetes.io/controller-uid": "uid-job"}}})
        live["spec"]["template"]["metadata"]["labels"].update({
            "batch.kubernetes.io/controller-uid": "uid-job",
            "controller-uid": "uid-job",
            "batch.kubernetes.io/job-name": item["job_name"],
            "job-name": item["job_name"],
        })
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
        live["spec"]["template"]["spec"]["initContainers"][1].update({
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
        wrong_database_target = copy.deepcopy(live)
        wrong_database_target["spec"]["template"]["spec"]["initContainers"][1]["env"][2]["value"] = "10.42.0.99"
        mutations.append(wrong_database_target)
        for changed in mutations:
            with self.subTest(spec=changed["spec"]["template"]["spec"]), \
                 self.assertRaises(runner.Blocked):
                runner.validate_bound_object(changed, item, "job")

    @unittest.skipIf(runner is None, "target-only runner not implemented")
    def test_exact_job_manifest_is_create_only_and_uses_pinned_images(self):
        item = runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)[0]
        manifest = runner.job_manifest(item, "10.42.0.3")
        pod = manifest["spec"]["template"]["spec"]
        self.assertRegex(runner.MIGRATE_IMAGE, r"@sha256:[0-9a-f]{64}$")
        self.assertRegex(runner.POSTGRES_IMAGE, r"@sha256:[0-9a-f]{64}$")
        self.assertEqual(manifest["spec"]["backoffLimit"], 0)
        self.assertEqual(pod["restartPolicy"], "Never")
        self.assertEqual(pod["containers"][0]["image"], runner.MIGRATE_IMAGE)
        self.assertEqual(pod["initContainers"][0]["image"], runner.POSTGRES_IMAGE)
        probe = pod["initContainers"][1]
        self.assertIn("current_database()", probe["command"][2])
        self.assertIn("inet_server_addr()", probe["command"][2])
        self.assertIn("host(inet_server_addr())", probe["command"][2])
        self.assertEqual(probe["env"][0]["valueFrom"]["secretKeyRef"]["key"],
            "MATCHMAKING_DATABASE_URL")
        self.assertEqual(probe["env"][1]["value"], "matchmaking_db")
        self.assertEqual(probe["env"][2]["value"], "10.42.0.3")
        self.assertEqual([entry["name"] for entry in probe["env"]],
            ["DATABASE_URL", "EXPECTED_DATABASE", "EXPECTED_POSTGRES_IP"])
        self.assertNotIn("env", pod["containers"][0])
        self.assertNotIn("secretKeyRef", json.dumps(pod["containers"][0]))
        runtime = next(volume for volume in pod["volumes"] if volume["name"] == "migration-runtime")
        self.assertEqual(runtime["emptyDir"], {"medium": "Memory", "sizeLimit": "1Mi"})
        self.assertIn("validated-database-url", pod["containers"][0]["command"][2])
        self.assertNotIn("DATABASE_URL", pod["containers"][0]["command"][2])
        self.assertFalse(pod["automountServiceAccountToken"])

        with tempfile.TemporaryDirectory() as directory:
            fake_bin = Path(directory, "bin")
            fake_bin.mkdir()
            psql = fake_bin / "psql"
            psql.write_text("#!/bin/sh\n"
                "touch \"$PSQL_CALLED\"\n"
                "while [ $# -gt 0 ]; do if [ \"$1\" = -d ]; then shift; dsn=$1; fi; shift; done\n"
                "case \"$dsn\" in postgres://voice:secret@10.42.0.3:5432/matchmaking_db?sslmode=disable) ;; *) echo secret >&2; exit 1 ;; esac\n"
                "printf '%s\\n' \"$PROBE_OUTPUT\"\n", encoding="utf-8")
            psql.chmod(0o700)
            probe_script = probe["command"][2].replace("/run/migration", directory)
            environment = {"PATH": str(fake_bin) + os.pathsep + os.environ["PATH"],
                "DATABASE_URL": "postgres://voice:secret@voice-postgres:5432/matchmaking_db?sslmode=disable",
                "EXPECTED_DATABASE": "matchmaking_db", "EXPECTED_POSTGRES_IP": "10.42.0.3",
                "PSQL_CALLED": str(Path(directory, "psql-called")),
                "PROBE_OUTPUT": "matchmaking_db|voice|10.42.0.3"}
            probe_result = subprocess.run(["/bin/sh", "-ec", probe_script], env=environment,
                capture_output=True, text=True, check=False)
            self.assertEqual(probe_result.returncode, 0, probe_result.stderr)
            self.assertTrue(Path(environment["PSQL_CALLED"]).exists())
            sealed = Path(directory, "validated-database-url")
            self.assertEqual(stat.S_IMODE(sealed.stat().st_mode), 0o600)
            self.assertEqual(sealed.read_text(),
                "postgres://voice:secret@10.42.0.3:5432/matchmaking_db?sslmode=disable")

            bad_urls = (
                "postgres://other:secret@voice-postgres:5432/matchmaking_db?sslmode=disable",
                "postgres://voice:secret@other-postgres:5432/matchmaking_db?sslmode=disable",
                "postgres://voice:secret@voice-postgres:5432/chat_db?sslmode=disable",
                "postgres://voice:secret@voice-postgres:5432/matchmaking_db?sslmode=require",
            )
            for index, bad_url in enumerate(bad_urls):
                bad_dir = Path(directory, "bad-url-" + str(index))
                bad_dir.mkdir()
                bad_script = probe["command"][2].replace("/run/migration", str(bad_dir))
                bad_env = {**environment, "DATABASE_URL": bad_url,
                    "PSQL_CALLED": str(bad_dir / "psql-called")}
                rejected = subprocess.run(["/bin/sh", "-ec", bad_script], env=bad_env,
                    capture_output=True, text=True, check=False)
                self.assertEqual(rejected.returncode, 1)
                self.assertEqual(rejected.stderr.strip(), "migration_target_url_invalid")
                self.assertFalse(Path(bad_env["PSQL_CALLED"]).exists())
                self.assertFalse((bad_dir / "validated-database-url").exists())

            for index, wrong_identity in enumerate(("chat_db|voice|10.42.0.3",
                    "matchmaking_db|other-role|10.42.0.3", "matchmaking_db|voice|10.42.0.99")):
                bad_dir = Path(directory, "bad-probe-" + str(index))
                bad_dir.mkdir()
                bad_script = probe["command"][2].replace("/run/migration", str(bad_dir))
                bad_env = {**environment, "PROBE_OUTPUT": wrong_identity,
                    "PSQL_CALLED": str(bad_dir / "psql-called")}
                rejected = subprocess.run(["/bin/sh", "-ec", bad_script], env=bad_env,
                    capture_output=True, text=True, check=False)
                self.assertEqual(rejected.returncode, 1)
                self.assertEqual(rejected.stderr.strip(), "migration_target_mismatch")
                self.assertFalse((bad_dir / "validated-database-url").exists())

            # The Secret and Service may change between init and main. Main has no
            # SecretRef/DNS input and can only launch migrate with the sealed Pod IP.
            migrate = fake_bin / "migrate"
            captured = Path(directory, "migrate-args")
            migrate.write_text("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CAPTURE_ARGS\"\n"
                "echo injected-secret-diagnostic >&2\nexit 1\n", encoding="utf-8")
            migrate.chmod(0o700)
            migration_script = pod["containers"][0]["command"][2].replace("/run/migration", directory)
            environment = {"PATH": str(fake_bin) + os.pathsep + os.environ["PATH"],
                "CAPTURE_ARGS": str(captured),
                "DATABASE_URL": "postgres://voice:changed@other-postgres:5432/other_db?sslmode=disable"}
            migrated = subprocess.run(["/bin/sh", "-ec", migration_script], env=environment,
                capture_output=True, text=True, check=False)
            self.assertEqual(migrated.returncode, 1)
            self.assertEqual(migrated.stderr.strip(), "migration_failed category=unknown exit=1")
            self.assertNotIn("secret", migrated.stderr)
            self.assertNotIn("voice-postgres", pod["containers"][0]["command"][2])
            self.assertIn("postgres://voice:secret@10.42.0.3:5432/matchmaking_db?sslmode=disable",
                captured.read_text())
            self.assertNotIn("other-postgres", captured.read_text())
            self.assertEqual(stat.S_IMODE(Path(directory, "migrate.stderr").stat().st_mode), 0o600)
        with mock.patch.object(runner, "MIGRATE_IMAGE", "migrate/migrate:v4.18.1"):
            with self.assertRaises(runner.Blocked):
                runner.build_plan(Path(__file__).resolve().parents[2], "a" * 40)
        configmap = runner.configmap_manifest(item)
        self.assertTrue(configmap["immutable"])
        configmap["metadata"].update({"uid": "cm-uid", "resourceVersion": "1"})
        self.assertTrue(runner.validate_bound_object(configmap, item, "configmap"))
        configmap["immutable"] = False
        with self.assertRaises(runner.Blocked):
            runner.validate_bound_object(configmap, item, "configmap")

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
    def test_endpoint_slice_address_and_port_must_bind_to_the_ready_pod(self):
        target = self.ReadOnlyTarget()
        original_run = target.run
        for changed in (
            {"items": [{"ports": [{"port": 5433, "protocol": "TCP"}],
                "endpoints": [{"addresses": ["10.42.0.3"], "conditions": {"ready": True},
                    "targetRef": {"kind": "Pod", "name": "voice-postgres-0", "uid": "pg-pod-uid"}}]}]},
            {"items": [{"ports": [{"port": 5432, "protocol": "TCP"}],
                "endpoints": [{"addresses": ["10.42.0.99"], "conditions": {"ready": True},
                    "targetRef": {"kind": "Pod", "name": "voice-postgres-0", "uid": "pg-pod-uid"}}]}]},
        ):
            target.run = lambda args, **kwargs: changed if args[:2] == ["get", "endpointslices"] else original_run(args, **kwargs)
            with self.subTest(changed=changed), self.assertRaises(runner.Blocked):
                runner._postgres_target_identity(target)
        target.run = original_run

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
        kube.job = runner.job_manifest(item, "10.42.0.3")
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
                        obj["spec"]["template"]["metadata"]["labels"].update({
                            "batch.kubernetes.io/controller-uid": uid,
                            "controller-uid": uid,
                            "batch.kubernetes.io/job-name": body["metadata"]["name"],
                            "job-name": body["metadata"]["name"],
                        })
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
        preflight_output = io.StringIO()
        with mock.patch.object(runner, "_read_source_identity", return_value=("a" * 40, False)), \
             mock.patch.object(runner, "Kubectl", return_value=fake), \
             redirect_stdout(preflight_output):
            preflight_result = runner.main(["--source-sha", "a" * 40, "--root", str(root), "--preflight-only"])
        self.assertEqual(preflight_result, 0)
        summary = json.loads(preflight_output.getvalue())
        self.assertEqual(summary["mode"], "preflight-only")
        self.assertEqual(summary["ledgers"], {
            service: {"database": runner.DATABASES[service],
                "version": runner.BASELINES[service], "dirty": False}
            for service in runner.SERVICES
        })
        self.assertEqual(fake.writes, [])
        self.assertNotIn("migration_data", preflight_output.getvalue())
        with redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            runner.main(["--source-sha", "a" * 40, "--root", str(root), "--preflight-only", "--apply"])
        self.assertEqual(fake.writes, [])

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
