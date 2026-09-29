#!/usr/bin/env python3
"""Name an issued staging NATS Secret set for an immutable rotation generation.

The issuer's original List and its signing seeds stay in the protected backup.
This tool writes only a new Kubernetes List and never prints credential bytes.
"""

import base64
import binascii
import json
import os
from pathlib import Path
import re
import stat
import sys


GENERATION = re.compile(r"r[0-9]{8}[a-z0-9]{0,8}\Z")
SERVICE_NAMES = "analytics auth bot chat file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice".split()
SECRET_KEYS = {
    "voice-nats-operator": {"operator.jwt", "account.jwt", "system-account.jwt", "account.public", "system-account.public"},
    "voice-nats-hub-tls": {"tls.crt", "tls.key", "ca.crt"},
    "voice-nats-bootstrap-credentials": {"bootstrap.creds"},
    "voice-nats-service-credentials": {f"{name}.creds" for name in SERVICE_NAMES},
}


def validated_list(source: Path) -> dict:
    if not source.is_file() or source.stat().st_size > 1024 * 1024:
        raise ValueError("invalid source file")
    bundle = json.loads(source.read_text(encoding="utf-8"))
    if set(bundle) != {"apiVersion", "kind", "items"} or bundle["apiVersion"] != "v1" or bundle["kind"] != "List":
        raise ValueError("invalid List")
    items = bundle["items"]
    if not isinstance(items, list) or len(items) != 4:
        raise ValueError("invalid Secret count")
    seen = set()
    for item in items:
        if not isinstance(item, dict) or set(item) != {"apiVersion", "kind", "metadata", "type", "data"}:
            raise ValueError("invalid Secret fields")
        meta = item["metadata"]
        if not isinstance(meta, dict) or set(meta) != {"name", "namespace"}:
            raise ValueError("invalid Secret metadata")
        name = meta["name"]
        if name not in SECRET_KEYS or name in seen or meta["namespace"] != "voice-staging":
            raise ValueError("invalid Secret identity")
        if item["apiVersion"] != "v1" or item["kind"] != "Secret" or item["type"] != "Opaque":
            raise ValueError("invalid Secret type")
        data = item["data"]
        if not isinstance(data, dict) or set(data) != SECRET_KEYS[name]:
            raise ValueError("invalid Secret keys")
        for encoded in data.values():
            if not isinstance(encoded, str) or not encoded:
                raise ValueError("invalid Secret data")
            try:
                decoded = base64.b64decode(encoded, validate=True)
            except (binascii.Error, ValueError) as exc:
                raise ValueError("invalid Secret data") from exc
            if not decoded or base64.b64encode(decoded).decode("ascii") != encoded:
                raise ValueError("invalid Secret data")
        seen.add(name)
    if seen != set(SECRET_KEYS):
        raise ValueError("incomplete Secret set")
    return bundle


def main() -> int:
    if len(sys.argv) != 4 or GENERATION.fullmatch(sys.argv[1]) is None:
        print("NATS rotation bundle packaging failed", file=sys.stderr)
        return 2
    generation, source_name, destination_name = sys.argv[1:]
    destination = Path(destination_name)
    try:
        if destination.is_symlink() or destination.exists():
            raise ValueError("destination exists")
        if os.name == "posix":
            parent = destination.parent.stat()
            if parent.st_uid != os.geteuid() or stat.S_IMODE(parent.st_mode) != 0o700:
                raise ValueError("destination parent is not protected")
        bundle = validated_list(Path(source_name))
        for item in bundle["items"]:
            item["metadata"]["name"] += "-" + generation
            item["immutable"] = True
        contents = (json.dumps(bundle, indent=2, sort_keys=True) + "\n").encode("utf-8")
        previous_umask = os.umask(0o077)
        try:
            descriptor = os.open(destination, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
            try:
                with os.fdopen(descriptor, "wb") as output:
                    output.write(contents)
                    output.flush()
                    os.fsync(output.fileno())
            except BaseException:
                destination.unlink(missing_ok=True)
                raise
        finally:
            os.umask(previous_umask)
    except (OSError, ValueError, TypeError, KeyError, json.JSONDecodeError):
        print("NATS rotation bundle packaging failed; no values printed", file=sys.stderr)
        return 1
    print("NATS rotation bundle packaged")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
