#!/usr/bin/env python3
"""Disposable synthetic cold-store restore; never accepts existing environments."""
import argparse
import base64
import hashlib
import json
import re
import subprocess
import time
import uuid
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "deploy/recovery/durable-stores.v1.json"
DATABASES = frozenset(("auth user social chat messaging realtime space role voice file "
                       "notification search matchmaking moderation subscription bot "
                       "game_integration story").split())
FAMILIES = frozenset(("postgres", "redis", "object", "jetstream", "clickhouse"))
# Registry digests, resolved before creation and used by immutable local image ID.
IMAGES = {
    "postgres": "postgres@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873",
    "redis": "redis@sha256:3811787313eba226a2ef38658c6ccb91cd5e110edc89c37767de373120a0e5a0",
    "object": "minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e",
    "jetstream": "nats@sha256:b270f5e2428354c0335612694d7dd2fb588148e567a5757fdff325ef9c9332e6",
    "clickhouse": "clickhouse/clickhouse-server@sha256:b002e56ed5c16e224c312527f6fcba7e77216fec5d7a88a7828f59efc614feb5",
    "mc": "minio/mc@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727",
    "nats": "natsio/nats-box@sha256:abdc9f9f0120bb8adfbf674eb037d1551db55356eb198b7bd4ffed377f6950a6",
    "archive": "alpine@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce",
}
MOUNTS = {"postgres": "/var/lib/postgresql", "redis": "/data", "object": "/data",
          "jetstream": "/data", "clickhouse": "/var/lib/clickhouse"}
LABEL = "voice.recovery.drill"


def validate_manifest(manifest):
    if manifest.get("schema") != "voice-durable-stores-v1":
        raise ValueError("unsupported store inventory")
    databases = manifest.get("postgres_databases", [])
    families = manifest.get("cold_store_families", [])
    if len(databases) != len(DATABASES) or set(databases) != {x + "_db" for x in DATABASES}:
        raise ValueError("incomplete/unknown/duplicate database inventory")
    if len(families) != len(FAMILIES) or set(families) != FAMILIES:
        raise ValueError("incomplete/unknown/duplicate store family inventory")
    if manifest.get("redis_disposition") != "retain-mixed-authority-store":
        raise ValueError("mixed Redis includes retained authority; never discard it as cache")
    targets = manifest.get("provisional_targets", {})
    if targets != {"rpo_seconds": 86400, "rto_seconds": 14400, "accepted": False}:
        raise ValueError("provisional24h/4h are measurements, not accepted operational targets")
    return manifest


def validate_cold_boundary(sources, targets, running):
    if not sources or set(sources) != set(targets) or set(sources) != set(running):
        raise ValueError("cold snapshot requires exact source/target/engine coverage")
    if (len(set(sources.values())) != len(sources)
            or len(set(targets.values())) != len(targets)
            or set(sources.values()) & set(targets.values())):
        raise ValueError("source and target volumes must be unique and disjoint")
    if any(value is not False for value in running.values()):
        raise ValueError("all source engines must be observed stopped")


class Drill:
    def __init__(self, output, manifest, source_sha):
        self.output = output
        self.manifest = validate_manifest(manifest)
        if not re.fullmatch(r"[0-9a-f]{40}", source_sha):
            raise ValueError("full source commit required")
        self.run_id = uuid.uuid4().hex
        self.prefix = "voice-restore-" + self.run_id
        self.containers = []
        self.volumes = []
        self.networks = []
        self.images = {}
        self.result = {"schema": "voice-synthetic-storage-restore-v1", "run_id": self.run_id,
                       "source_commit": source_sha, "application_recovery_pass": False,
                       "accepted_rpo_rto": False, "status": "INCOMPLETE", "resources": {},
                       "source_files": {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
                                        for path in (Path(__file__), MANIFEST)}}
        output.mkdir(parents=True, exist_ok=False)
        self.save()

    def save(self):
        self.result["resources"] = {"containers": self.containers, "volumes": self.volumes,
                                    "networks": self.networks}
        temporary = self.output / "journal.tmp"
        temporary.write_text(json.dumps(self.result, indent=2) + "\n", encoding="utf-8")
        temporary.replace(self.output / "recovery-evidence.json")

    @staticmethod
    def command(*args, timeout=90):
        result = subprocess.run(["docker", *args], capture_output=True, text=True,
                                encoding="utf-8", timeout=timeout, check=False)
        if result.returncode:
            # Only disposable synthetic material is used; bounded errors stay actionable.
            raise RuntimeError("docker " + args[0] + ": " + (result.stderr + result.stdout)[-1500:])
        return result.stdout.strip()

    def name(self, suffix):
        return self.prefix + "-" + suffix

    def volume(self, suffix):
        name = self.name(suffix)
        # Register intent first: an interrupted command remains reconcilable.
        self.volumes.append(name)
        self.save()
        self.command("volume", "create", "--label", LABEL + "=" + self.run_id, name)
        return name

    def run(self, suffix, image, args, options=(), detached=False, network=None):
        name = self.name(suffix)
        self.containers.append(name)
        self.save()
        base = ["run", "--name", name, "--label", LABEL + "=" + self.run_id,
                "--network", network or "none", "--pull=never"]
        base += ["-d"] if detached else ["--rm"]
        return self.command(*base, *options, self.images[image], *args)

    def exec(self, name, *args):
        return self.command("exec", name, *args)

    def wait(self, name, args):
        deadline = time.monotonic() + 60
        while True:
            try:
                return self.exec(name, *args)
            except RuntimeError:
                if time.monotonic() >= deadline:
                    raise
                time.sleep(0.25)

    def start(self, side, family, volume, network):
        options = ["--mount", f"type=volume,src={volume},dst={MOUNTS[family]}"]
        args = []
        if family == "postgres":
            options += ["-e", "POSTGRES_HOST_AUTH_METHOD=trust"]
        elif family == "redis":
            args = ["redis-server", "--appendonly", "no", "--save", "60", "1"]
        elif family == "object":
            options += ["-e", "MINIO_ROOT_USER=synthetic", "-e", "MINIO_ROOT_PASSWORD=synthetic-drill-only"]
            args = ["server", "/data"]
        elif family == "jetstream":
            args = ["-js", "-sd", "/data", "-n", "synthetic-recovery"]
        elif family == "clickhouse":
            options += ["-e", "CLICKHOUSE_SKIP_USER_SETUP=1"]
        self.run(side + "-" + family, family, args, options, True, network)
        return self.name(side + "-" + family)

    def psql(self, container, database, sql):
        return self.exec(container, "psql", "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1",
                         "-U", "postgres", "-d", database, "-c", sql)

    def mc(self, side, network, script):
        endpoint = f"http://synthetic:synthetic-drill-only@{self.name(side + '-object')}:9000"
        return self.run("mc-" + uuid.uuid4().hex[:8], "mc", ["-c", script],
                        ["-e", "MC_HOST_drill=" + endpoint, "--entrypoint", "/bin/sh"], network=network)

    def nats(self, side, network, *args):
        return self.run("nats-" + uuid.uuid4().hex[:8], "nats",
                        ["nats", "--server", "nats://" + self.name(side + "-jetstream") + ":4222", *args],
                        network=network)

    def cold_hash(self, family, source, backup, snapshot=False):
        mounts = ["--mount", f"type=volume,src={source},dst=/source,readonly",
                  "--mount", f"type=volume,src={backup},dst=/backup"]
        script = (f"tar -C /source -cpf /backup/{family}.tar . && sha256sum /backup/{family}.tar"
                  if snapshot else "tar -C /source -cpf - . | sha256sum")
        return self.run("hash-" + uuid.uuid4().hex[:8], "archive", ["sh", "-ec", script], mounts).split()[0]

    def execute(self):
        for key, reference in IMAGES.items():
            self.images[key] = self.command("image", "inspect", reference, "--format", "{{.Id}}")
        self.result["images"] = self.images
        network = self.name("isolated")
        self.networks.append(network)
        self.save()
        self.command("network", "create", "--internal", "--label", LABEL + "=" + self.run_id, network)
        sources = {family: self.volume("source-" + family) for family in sorted(FAMILIES)}
        source_containers = {family: self.start("source", family, volume, network)
                             for family, volume in sources.items()}
        pg = source_containers["postgres"]
        self.wait(pg, ["pg_isready", "-U", "postgres", "-h", "127.0.0.1"])
        account, profile, file_id = [str(uuid.uuid4()) for _ in range(3)]
        payload = "synthetic-attachment-" + self.run_id
        digest = hashlib.sha256(payload.encode()).hexdigest()
        self.result["synthetic"] = {"account_id": account, "profile_id": profile,
                                    "file_id": file_id, "object_sha256": digest}
        for database in self.manifest["postgres_databases"]:
            self.psql(pg, "postgres", 'CREATE DATABASE "' + database + '"')
            self.psql(pg, database, "CREATE TABLE recovery_canary (account_id uuid, profile_id uuid, "
                      "file_id uuid, object_sha256 text, committed_at timestamptz DEFAULT clock_timestamp()); "
                      f"INSERT INTO recovery_canary(account_id,profile_id,file_id,object_sha256) "
                      f"VALUES ('{account}','{profile}','{file_id}','{digest}')")
        redis = source_containers["redis"]
        self.wait(redis, ["redis-cli", "PING"])
        self.exec(redis, "redis-cli", "SET", "synthetic:authority:receipt", file_id)
        self.exec(redis, "redis-cli", "SET", "synthetic:cache:presence", "stale-projection")
        self.exec(redis, "redis-cli", "SAVE")
        ch = source_containers["clickhouse"]
        self.wait(ch, ["clickhouse-client", "--query", "SELECT 1"])
        self.exec(ch, "clickhouse-client", "--multiquery", "--query",
                  "CREATE DATABASE voice; CREATE TABLE voice.recovery_canary (profile_id String, "
                  "object_sha256 String) ENGINE=MergeTree ORDER BY profile_id; "
                  f"INSERT INTO voice.recovery_canary VALUES ('{profile}','{digest}');")
        # Sidecars have no host ports and only the private internal network.
        self.mc("source", network, f"mc ready drill && mc mb drill/synthetic && "
                f"printf %s {payload} | mc pipe drill/synthetic/{file_id}")
        self.nats("source", network, "stream", "add", "SYNTHETIC", "--subjects", "synthetic.>",
                  "--storage", "file", "--defaults")
        self.nats("source", network, "consumer", "add", "SYNTHETIC", "PENDING", "--pull", "--defaults")
        self.nats("source", network, "pub", "synthetic.attachment", file_id)
        canary_time = float(self.psql(pg, "auth_db", "SELECT extract(epoch FROM committed_at) FROM recovery_canary"))
        # failure injection starts RTO; all source engines settle before capture.
        failure_time, start = time.time(), time.monotonic()
        self.command("stop", "--time", "30", *source_containers.values(), timeout=180)
        running = {key: json.loads(self.command("inspect", value))[0]["State"]["Running"]
                   for key, value in source_containers.items()}
        targets = {family: self.volume("target-" + family) for family in sorted(FAMILIES)}
        validate_cold_boundary(sources, targets, running)
        backup = self.volume("backup")
        hashes = {family: self.cold_hash(family, source, backup, True) for family, source in sources.items()}
        for family, target in targets.items():
            self.run("restore-" + family, "archive", ["sh", "-ec",
                     f"test -z \"$(ls -A /target)\"; tar -C /target -xpf /backup/{family}.tar"],
                     ["--mount", f"type=volume,src={backup},dst=/backup,readonly",
                      "--mount", f"type=volume,src={target},dst=/target"])
        target_containers = {family: self.start("target", family, volume, network)
                             for family, volume in targets.items()}
        target_pg = target_containers["postgres"]
        self.wait(target_pg, ["pg_isready", "-U", "postgres", "-h", "127.0.0.1"])
        expected = f"{account}|{profile}|{file_id}|{digest}"
        for database in self.manifest["postgres_databases"]:
            actual = self.psql(target_pg, database,
                               "SELECT account_id,profile_id,file_id,object_sha256 FROM recovery_canary")
            if actual != expected:
                raise RuntimeError("restored database cross-store canary mismatch: " + database)
        target_redis = target_containers["redis"]
        self.wait(target_redis, ["redis-cli", "PING"])
        if self.exec(target_redis, "redis-cli", "GET", "synthetic:authority:receipt") != file_id:
            raise RuntimeError("retained Redis authority receipt lost")
        # Rebuild exactly one disposable projection, never FLUSHDB or blanket Redis discard.
        self.exec(target_redis, "redis-cli", "DEL", "synthetic:cache:presence")
        self.exec(target_redis, "redis-cli", "SET", "synthetic:cache:presence", profile)
        if self.exec(target_redis, "redis-cli", "GET", "synthetic:cache:presence") != profile:
            raise RuntimeError("target-only projection rebuild failed")
        if self.mc("target", network, f"mc ready drill >/dev/null && mc cat drill/synthetic/{file_id}") != payload:
            raise RuntimeError("restored object bytes mismatch")
        target_ch = target_containers["clickhouse"]
        self.wait(target_ch, ["clickhouse-client", "--query", "SELECT 1"])
        if self.exec(target_ch, "clickhouse-client", "--query",
                     "SELECT profile_id,object_sha256 FROM voice.recovery_canary FORMAT TSV") != profile + "\t" + digest:
            raise RuntimeError("restored ClickHouse canary mismatch")
        message = json.loads(self.nats("target", network, "stream", "get", "SYNTHETIC", "1", "--json"))
        if base64.b64decode(message["data"]).decode() != file_id:
            raise RuntimeError("restored JetStream exact message mismatch")
        consumer = json.loads(self.nats("target", network, "consumer", "info", "SYNTHETIC", "PENDING", "--json"))
        if consumer["num_pending"] != 1 or consumer["ack_floor"]["stream_seq"] != 0:
            raise RuntimeError("restored JetStream pending consumer evidence mismatch")
        for family, source in sources.items():
            if self.cold_hash(family, source, backup) != hashes[family]:
                raise RuntimeError("source changed during restore: " + family)
        rpo, rto = failure_time - canary_time, time.monotonic() - start
        if rpo < 0 or rto < 0:
            raise RuntimeError("invalid recovery clock measurement")
        self.result.update(status="SYNTHETIC_STORAGE_PASS", source_unchanged=True, archive_sha256=hashes,
                           database_count=len(self.manifest["postgres_databases"]),
                           rpo_seconds=rpo, rto_seconds=rto,
                           within_provisional_targets=rpo <= 86400 and rto <= 14400)
        # Export the actual source-generated backup before deleting the owned volume.
        self.run("export", "archive", ["sleep", "600"],
                 ["--mount", f"type=volume,src={backup},dst=/backup,readonly"], True)
        self.command("cp", self.name("export") + ":/backup", str(self.output / "archives"), timeout=180)
        for family, expected_hash in hashes.items():
            with (self.output / "archives" / (family + ".tar")).open("rb") as archive:
                if hashlib.file_digest(archive, "sha256").hexdigest() != expected_hash:
                    raise RuntimeError("exported archive hash mismatch: " + family)
        self.save()

    def cleanup(self):
        errors = []
        for kind, resources in (("container", reversed(self.containers)),
                                ("volume", reversed(self.volumes)), ("network", reversed(self.networks))):
            for name in resources:
                result = subprocess.run(["docker", kind, "inspect", name], capture_output=True,
                                        text=True, encoding="utf-8", timeout=30, check=False)
                if result.returncode:
                    # A missing --rm helper is settled; an unavailable daemon is unknown.
                    absent = ((kind == "container" and f"No such container: {name}" in result.stderr)
                              or (kind == "volume" and name in result.stderr and "no such volume" in result.stderr)
                              or (kind == "network" and name in result.stderr and "not found" in result.stderr))
                    if not absent:
                        errors.append("resource state unobserved: " + name)
                    continue
                observed = json.loads(result.stdout)[0]
                labels = observed.get("Config", {}).get("Labels", {}) if kind == "container" else observed.get("Labels", {})
                if (labels or {}).get(LABEL) != self.run_id:
                    errors.append("ownership mismatch: " + name)
                    continue
                try:
                    self.command(kind, "rm", *(["-f"] if kind == "container" else []), name)
                except (RuntimeError, subprocess.SubprocessError) as exc:
                    errors.append(str(exc))
        self.result["cleanup"] = {"complete": not errors, "errors": errors}
        self.save()
        return not errors

    def diagnostics(self):
        for name in self.containers:
            if any(name == self.name(side + "-" + family)
                   for side in ("source", "target") for family in FAMILIES):
                result = subprocess.run(["docker", "logs", "--tail", "40", name], capture_output=True,
                                        text=True, encoding="utf-8", timeout=30, check=False)
                (self.output / (name + ".log")).write_text(result.stdout + result.stderr, encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--synthetic-drill", action="store_true", required=True)
    parser.add_argument("--artifact-dir", type=Path, required=True)
    parser.add_argument("--source-sha", required=True)
    args = parser.parse_args()
    drill = Drill(args.artifact_dir, json.loads(MANIFEST.read_text(encoding="utf-8")), args.source_sha)
    try:
        drill.execute()
    except (RuntimeError, ValueError, OSError, subprocess.SubprocessError) as exc:
        drill.result.update(status="FAIL", error=str(exc))
        drill.save()
        drill.diagnostics()
        raise
    finally:
        if not drill.cleanup():
            raise RuntimeError("owned resources remain; see recovery-evidence.json")
    print(json.dumps({key: drill.result[key] for key in ("status", "application_recovery_pass",
                     "rpo_seconds", "rto_seconds", "source_unchanged", "cleanup")}))


if __name__ == "__main__":
    main()
