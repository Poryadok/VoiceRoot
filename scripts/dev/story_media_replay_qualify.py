#!/usr/bin/env python3
"""Exercise the DEV replay fixture in disposable Linux Docker volumes only.

This is not a Kubernetes, power-loss, backup-freshness or production proof.
No live endpoint or user secret is accepted by this program.
"""
import json
from pathlib import Path
import secrets
import subprocess
import tempfile
import time
import uuid

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "deploy/templates/story-media-replay-redis.yaml"
IMAGE = "redis@sha256:858f009f9709ce576febc734aa78b8f6d624b82571f9ddb6bda4377c833b3499"
OWNER = "WT21-Replay-qualification"


def docker(*args, data=None, check=True):
    result = subprocess.run(["docker", *args], input=data, capture_output=True, timeout=90)
    if check and result.returncode:
        raise RuntimeError("private Docker fixture failed: " + args[0])
    return result


def main():
    if docker("info", "--format", "{{.OSType}}").stdout.strip() != b"linux":
        raise RuntimeError("Linux Docker required")
    doc = json.loads(MANIFEST.read_text(encoding="utf-8"))
    cm = next(x for x in doc["items"] if x["kind"] == "ConfigMap")
    spec = next(x for x in doc["items"] if x["kind"] == "Deployment")
    if spec["spec"]["template"]["spec"]["containers"][0]["image"] != IMAGE:
        raise RuntimeError("unexpected fixture image")
    tag = uuid.uuid4().hex
    binding = str(uuid.uuid4()) + "|" + str(uuid.uuid4())
    password = secrets.token_hex(32)
    volumes = []
    containers = []

    def volume(suffix):
        name = "story-replay-qualification-" + tag + "-" + suffix
        docker("volume", "create", "--label", "voice.test.owner=" + OWNER,
               "--label", "voice.test.operation=" + tag, name)
        volumes.append(name)
        return name

    def create(*args):
        cid = docker("create", "--label", "voice.test.owner=" + OWNER,
                     "--label", "voice.test.operation=" + tag, *args).stdout.decode().strip()
        if len(cid) != 64:
            raise RuntimeError("container identity missing")
        containers.append(cid)
        return cid

    def start(cid):
        docker("start", cid)

    def admit(cid):
        for _ in range(60):
            result = docker("exec", cid, "cat", "/run/replay/nonce", check=False)
            if result.returncode == 0 and result.stdout.strip():
                break
            time.sleep(0.1)
        else:
            raise RuntimeError("admission nonce not available")
        if docker("exec", cid, "redis-cli", "PING", check=False).returncode == 0:
            raise RuntimeError("Redis listened before admission")
        docker("exec", cid, "sh", "-ec", "printf stale > /run/replay/admit")
        time.sleep(1.2)
        if docker("exec", cid, "redis-cli", "PING", check=False).returncode == 0:
            raise RuntimeError("Redis accepted a stale admission nonce")
        docker("exec", cid, "sh", "-ec", "cat /run/replay/nonce > /run/replay/admit")

    def wait_ready(cid):
        admit(cid)
        for _ in range(60):
            p = docker("exec", "-e", "REDISCLI_AUTH=" + password, cid,
                       "redis-cli", "--no-auth-warning", "PING", check=False)
            if p.returncode == 0 and p.stdout.strip() == b"PONG":
                return
            time.sleep(0.1)
        raise RuntimeError("fixture did not become ready")

    def cli(cid, *args):
        return docker("exec", "-e", "REDISCLI_AUTH=" + password, cid,
                      "redis-cli", "--no-auth-warning", "--raw", *args).stdout.strip()

    def helper(data_volume, command, data=None):
        return docker("run", "--rm", "-i", "--network", "none", "--label",
                      "voice.test.owner=" + OWNER, "--label", "voice.test.operation=" + tag,
                      "-v", data_volume + ":/data", IMAGE, "sh", "-ec", command, data=data).stdout

    def redis(data_volume, config_volume, expected=binding):
        return create("--network", "none", "--cpus", "0.5", "--memory", "512m",
                      "--user", "999:999", "--read-only", "--tmpfs", "/run/replay:rw,uid=999,gid=999",
                      "-e", "REPLAY_BINDING=" + expected, "-e", "REDIS_PASSWORD=" + password,
                      "-v", data_volume + ":/data", "-v", config_volume + ":/config:ro",
                      IMAGE, "sh", "/config/start.sh")

    try:
        data, config, restored = volume("data"), volume("config"), volume("restored")
        helper_id = create("--network", "none", "-v", data + ":/data", "-v", config + ":/config",
                           IMAGE, "sh", "-c", "sleep 300")
        start(helper_id)
        with tempfile.TemporaryDirectory() as directory:
            for name in ("redis.conf", "start.sh"):
                path = Path(directory) / name
                path.write_text(cm["data"][name], encoding="utf-8", newline="\n")
                docker("cp", str(path), helper_id + ":/config/" + name)
        docker("exec", helper_id, "sh", "-ec", "chown -R 999:999 /data; chmod 755 /config; chmod 644 /config/*")
        absent = redis(data, config)
        start(absent)
        if docker("wait", absent).stdout.strip() == b"0":
            raise RuntimeError("missing marker was accepted")
        docker("exec", "-e", "BIND=" + binding, helper_id, "sh", "-ec",
               "printf '%s' \"$BIND\" > /data/.voice-replay-binding; chown 999:999 /data/.voice-replay-binding")
        wrong = redis(data, config, str(uuid.uuid4()) + "|" + str(uuid.uuid4()))
        start(wrong)
        if docker("wait", wrong).stdout.strip() == b"0":
            raise RuntimeError("mismatched marker was accepted")
        live = redis(data, config)
        start(live)
        wait_ready(live)
        if b"NOAUTH" not in docker("exec", live, "redis-cli", "PING").stdout:
            raise RuntimeError("unauthenticated client was accepted")
        if cli(live, "SET", "test:crash", "1", "NX", "PX", "30000") != b"OK":
            raise RuntimeError("initial replay admission failed")
        docker("kill", "--signal", "KILL", live)
        start(live)
        wait_ready(live)
        if cli(live, "SET", "test:crash", "1", "NX", "PX", "30000") != b"":
            raise RuntimeError("acknowledged replay was lost on process crash")
        if int(cli(live, "PTTL", "test:crash")) <= 0:
            raise RuntimeError("replay expired before the crash check")
        if cli(live, "SET", "test:survival", "1", "NX", "PX", "300000") != b"OK":
            raise RuntimeError("sentinel admission failed")
        if cli(live, "CONFIG", "SET", "maxmemory", "1") != b"OK":
            raise RuntimeError("memory fault setup failed")
        if b"OOM" not in cli(live, "SET", "test:oom", "1", "NX", "PX", "30000"):
            raise RuntimeError("over-limit write did not fail")
        if cli(live, "GET", "test:survival") != b"1":
            raise RuntimeError("noeviction lost the sentinel")
        docker("stop", live)
        archive = helper(data, "tar -C /data -cf - .")
        helper(restored, "tar -C /data -xf -; chown -R 999:999 /data", archive)
        start(live)
        wait_ready(live)
        if cli(live, "SET", "test:after-backup", "1", "NX", "PX", "300000") != b"OK":
            raise RuntimeError("post-backup admission failed")
        docker("stop", live)
        restore = redis(restored, config)
        start(restore)
        wait_ready(restore)
        if cli(restore, "GET", "test:survival") != b"1":
            raise RuntimeError("offline backup failed to restore")
        if cli(restore, "SET", "test:after-backup", "1", "NX", "PX", "300000") != b"OK":
            raise RuntimeError("stale backup unexpectedly contained later key")
        docker("stop", restore)
        helper(data, "set -- /data/appendonlydir/*.incr.aof; [ \"$#\" -eq 1 ]; printf '*3' >> \"$1\"")
        start(live)
        admit(live)
        if docker("wait", live).stdout.strip() == b"0":
            raise RuntimeError("truncated AOF was accepted")
        print("PASS: marker missing/mismatch, authentication, crash replay, noeviction, offline restore, stale backup and truncated AOF")
    finally:
        cleanup_failed = False
        for cid in reversed(containers):
            info = docker("inspect", "--format", "{{index .Config.Labels \"voice.test.operation\"}}", cid, check=False)
            if info.returncode == 0 and info.stdout.decode().strip() == tag:
                cleanup_failed |= docker("rm", "-f", cid, check=False).returncode != 0
            else:
                cleanup_failed = True
        for name in reversed(volumes):
            info = docker("volume", "inspect", "--format", "{{index .Labels \"voice.test.operation\"}}", name, check=False)
            if info.returncode == 0 and info.stdout.decode().strip() == tag:
                cleanup_failed |= docker("volume", "rm", name, check=False).returncode != 0
            else:
                cleanup_failed = True
        if cleanup_failed:
            raise RuntimeError("private fixture cleanup incomplete; inspect operation " + tag)


if __name__ == "__main__":
    main()
