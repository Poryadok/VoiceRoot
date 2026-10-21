#!/usr/bin/env python3
"""Assert production Chat's protected Realtime P3 files reach the right container."""

from __future__ import annotations

import sys
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "deploy" / "prod" / "services.yaml"
EXPECTED_CHAT_MOUNTS = {
    "realtime-p3-server": "/etc/voice/realtime-p3/tls",
    "realtime-p3-trust": "/etc/voice/realtime-p3/trust",
}
EXPECTED_CHAT_ENV_PATHS = {
    "CHAT_REALTIME_LIFECYCLE_TLS_CERT_FILE": "/etc/voice/realtime-p3/tls/tls.crt",
    "CHAT_REALTIME_LIFECYCLE_TLS_KEY_FILE": "/etc/voice/realtime-p3/tls/tls.key",
    "CHAT_REALTIME_LIFECYCLE_CLIENT_CA_FILE": "/etc/voice/realtime-p3/trust/realtime-client-ca.crt",
    "CHAT_REALTIME_PRINCIPAL_JWKS_CA_FILE": "/etc/voice/realtime-p3/trust/realtime-jwks-ca.crt",
}
EXPECTED_NATS_MOUNTS = {
    "nats-leaf-config": ("/etc/nats/leaf.conf", "leaf.conf"),
    "nats-service-creds": ("/var/run/nats/creds/chat.creds", "chat.creds"),
    "nats-hub-tls": ("/etc/nats/tls/ca.crt", "ca.crt"),
}


def fail(message: str) -> None:
    raise AssertionError(message)


def main() -> None:
    documents = list(yaml.safe_load_all(MANIFEST.read_text(encoding="utf-8")))
    deployment = next(
        (
            document
            for document in documents
            if document.get("kind") == "Deployment"
            and document.get("metadata", {}).get("name") == "voice-chat"
        ),
        None,
    )
    if deployment is None:
        fail("missing Deployment/voice-chat")

    pod_spec = deployment["spec"]["template"]["spec"]
    if [container["name"] for container in pod_spec["containers"]] != ["chat", "nats-leaf"]:
        fail("voice-chat must retain its chat and nats-leaf containers")
    containers = {container["name"]: container for container in pod_spec["containers"]}
    chat = containers.get("chat")
    nats_leaf = containers.get("nats-leaf")
    if chat is None or nats_leaf is None:
        fail("voice-chat must retain chat and nats-leaf containers")

    chat_mounts = chat.get("volumeMounts", [])
    nats_mounts = nats_leaf.get("volumeMounts", [])
    chat_env = {entry["name"]: entry.get("value") for entry in chat.get("env", [])}
    for name, expected_path in EXPECTED_CHAT_ENV_PATHS.items():
        if chat_env.get(name) != expected_path:
            fail(f"chat env {name} must consume {expected_path}")

    for volume_name, mount_path in EXPECTED_CHAT_MOUNTS.items():
        matching = [mount for mount in chat_mounts if mount.get("name") == volume_name]
        if len(matching) != 1:
            fail(f"chat must mount {volume_name} exactly once")
        mount = matching[0]
        if mount.get("mountPath") != mount_path or mount.get("readOnly") is not True:
            fail(f"chat must mount {volume_name} read-only at {mount_path}")
        if any(item.get("name") == volume_name for item in nats_mounts):
            fail(f"nats-leaf must not mount {volume_name}")

    for volume_name, (mount_path, sub_path) in EXPECTED_NATS_MOUNTS.items():
        matching = [mount for mount in nats_mounts if mount.get("name") == volume_name]
        if len(matching) != 1:
            fail(f"nats-leaf must retain {volume_name} exactly once")
        mount = matching[0]
        if (
            mount.get("mountPath") != mount_path
            or mount.get("subPath") != sub_path
            or mount.get("readOnly") is not True
        ):
            fail(f"nats-leaf mount {volume_name} must remain read-only at {mount_path}")

    volumes = {volume["name"]: volume for volume in pod_spec.get("volumes", [])}
    expected_volumes = {
        "realtime-p3-server": ("secret", "voice-chat-realtime-p3-tls"),
        "realtime-p3-trust": ("secret", "voice-p3-principal-trust"),
        "nats-leaf-config": ("configMap", "voice-nats-leaf-config"),
        "nats-service-creds": ("secret", "voice-nats-service-credentials"),
        "nats-hub-tls": ("secret", "voice-nats-hub-tls"),
    }
    for volume_name, (source_kind, source_name) in expected_volumes.items():
        source = volumes.get(volume_name, {}).get(source_kind, {})
        source_field = "name" if source_kind == "configMap" else "secretName"
        if source.get(source_field) != source_name:
            fail(f"volume {volume_name} must retain {source_kind} {source_name}")

    print("Production Chat TLS mount invariants passed.")


if __name__ == "__main__":
    try:
        main()
    except (AssertionError, OSError, yaml.YAMLError) as error:
        print(f"prod Chat TLS mount invariant failed: {error}", file=sys.stderr)
        raise SystemExit(1)
