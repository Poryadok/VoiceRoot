"""Bounded native Chat producer regression in own portless Linux PostgreSQL.

Requires a locally cached postgres:16-bookworm. --before pins schema25 and must
fail REPORT_ATTACHMENT_MISSING; default applies the actual migration catalog.
No shared stand, host port, external key, socket bind or fixture ACL bypass.
"""
import argparse
import pathlib
import subprocess
import time
import uuid

parser = argparse.ArgumentParser()
parser.add_argument("--before", action="store_true")
args = parser.parse_args()
root = pathlib.Path(__file__).resolve().parents[2]
identity = "voice-report-attachment-" + uuid.uuid4().hex[:12]

def docker(*arguments, data=None, check=True):
    result = subprocess.run(["rtk", "proxy", "docker", *arguments], input=data,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=120)
    if check and result.returncode:
        raise RuntimeError(result.stdout.decode(errors="replace") + result.stderr.decode(errors="replace"))
    return result

created = False
try:
    docker("image", "inspect", "postgres:16-bookworm")
    docker("run", "--detach", "--name", identity, "--label", "voice.task=BE-066",
           "--label", "voice.actor=Rudeus-320", "--network", "none", "--memory", "768m",
           "--cpus", "1", "-e", "POSTGRES_PASSWORD=disposable-local-proof",
           "postgres:16-bookworm")
    created = True
    deadline = time.monotonic() + 45
    while True:
        ready = docker("exec", identity, "pg_isready", "-U", "postgres", check=False)
        if ready.returncode == 0:
            break
        if time.monotonic() >= deadline:
            raise RuntimeError("private PostgreSQL did not become ready")
        time.sleep(.25)
    migrations = sorted((root / "src/backend/migrations/chat_db").glob("*.up.sql"))
    for migration in migrations:
        if args.before and int(migration.name[:6]) > 25:
            continue
        docker("exec", "-i", identity, "psql", "-U", "postgres", "-v", "ON_ERROR_STOP=1",
               data=migration.read_bytes())
    sql = (root / "scripts/ci/chat-report-attachments_test.sql").read_bytes()
    result = docker("exec", "-i", identity, "psql", "-U", "postgres", "-v", "ON_ERROR_STOP=1", data=sql, check=False)
    print(result.stdout.decode(errors="replace"), end="")
    print(result.stderr.decode(errors="replace"), end="")
    if result.returncode:
        raise SystemExit(result.returncode)
    if args.before:
        raise RuntimeError("schema25 unexpectedly satisfied the new producer regression")
finally:
    if created:
        cleanup = docker("rm", "--force", "--volumes", identity, check=False)
        if cleanup.returncode:
            print("OWN_CONTAINER_CLEANUP_REQUIRED:", identity)
            raise RuntimeError(cleanup.stderr.decode(errors="replace"))
