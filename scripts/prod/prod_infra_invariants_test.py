#!/usr/bin/env python3
"""Structural safety checks for the single-node production manifests."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PROD = ROOT / "deploy" / "prod"


def fail(message: str) -> None:
    raise AssertionError(message)


def load(path: Path) -> list[dict]:
    if not path.is_file():
        fail(f"missing manifest: {path.relative_to(ROOT)}")
    result = subprocess.run(
        [
            "kubectl",
            "apply",
            "--dry-run=client",
            "--validate=false",
            "-f",
            str(path),
            "-o",
            "json",
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    parsed = json.loads(result.stdout)
    return parsed.get("items", [parsed])


def named(resources: list[dict], kind: str, name: str) -> dict:
    for resource in resources:
        if resource.get("kind") == kind and resource.get("metadata", {}).get("name") == name:
            return resource
    fail(f"missing {kind}/{name}")


def assert_persistent_workload(resources: list[dict], name: str, volume: str, mount_path: str) -> None:
    workload = named(resources, "StatefulSet", name)
    claims = {
        claim.get("metadata", {}).get("name")
        for claim in workload.get("spec", {}).get("volumeClaimTemplates", [])
    }
    if volume not in claims:
        fail(f"StatefulSet/{name} must declare volumeClaimTemplate {volume}")
    containers = workload["spec"]["template"]["spec"].get("containers", [])
    mounts = [mount for container in containers for mount in container.get("volumeMounts", [])]
    if not any(mount.get("name") == volume and mount.get("mountPath") == mount_path for mount in mounts):
        fail(f"StatefulSet/{name} must mount {volume} at {mount_path}")


def main() -> None:
    manifest_paths = [
        PROD / "infra.yaml",
        PROD / "flutter-web.yaml",
        PROD / "admin.yaml",
        PROD / "developer-portal.yaml",
        PROD / "livekit-ingress.yaml",
    ]
    loaded = {path: load(path) for path in manifest_paths}
    resources = [resource for group in loaded.values() for resource in group]

    for resource in resources:
        if resource.get("kind") != "Namespace":
            namespace = resource.get("metadata", {}).get("namespace")
            if namespace != "voice-prod":
                fail(
                    f"{resource.get('kind')}/{resource.get('metadata', {}).get('name')} "
                    f"must use namespace voice-prod, got {namespace!r}"
                )

    infra = loaded[PROD / "infra.yaml"]
    assert_persistent_workload(infra, "voice-postgres", "pgdata", "/var/lib/postgresql/data")
    assert_persistent_workload(infra, "voice-redis", "redisdata", "/data")
    assert_persistent_workload(infra, "voice-nats", "jsdata", "/data")
    assert_persistent_workload(infra, "voice-clickhouse", "chdata", "/var/lib/clickhouse")

    livekit = named(infra, "Service", "voice-livekit")
    ports = {port["name"]: port for port in livekit["spec"]["ports"]}
    expected_ports = {
        "rtc-tcp": {"protocol": "TCP", "targetPort": 7881, "nodePort": 30981},
        "rtc-udp": {"protocol": "UDP", "targetPort": 7882, "nodePort": 30982},
    }
    for port_name, expected in expected_ports.items():
        actual = ports.get(port_name, {})
        for field, value in expected.items():
            if actual.get(field) != value:
                fail(f"voice-livekit port {port_name} must set {field}={value}")

    livekit_deployment = named(infra, "Deployment", "voice-livekit")
    volumes = livekit_deployment["spec"]["template"]["spec"].get("volumes", [])
    livekit_volume = next((volume for volume in volumes if volume.get("name") == "livekit-config"), {})
    if livekit_volume.get("secret", {}).get("secretName") != "voice-livekit-config":
        fail("voice-livekit must mount its rendered configuration from a Secret")
    if any(
        resource.get("kind") == "ConfigMap"
        and resource.get("metadata", {}).get("name") == "voice-livekit-config"
        for resource in infra
    ):
        fail("voice-livekit credentials must not be rendered into a ConfigMap")

    livekit_template = (PROD / "livekit-config.template.yaml").read_text(encoding="utf-8")
    if "node_ip: __LIVEKIT_NODE_IP__" not in livekit_template:
        fail("production LiveKit config must declare the explicit edge node IP placeholder")
    if "use_external_ip: false" not in livekit_template:
        fail("production LiveKit config must disable STUN public-IP discovery")

    for resource in resources:
        if resource.get("kind") != "Ingress":
            continue
        name = resource["metadata"]["name"]
        annotations = resource["metadata"].get("annotations", {})
        if annotations.get("traefik.ingress.kubernetes.io/router.entrypoints") != "websecure":
            fail(f"Ingress/{name} must use Traefik websecure")
        tls = resource.get("spec", {}).get("tls", [])
        rule_hosts = {rule.get("host") for rule in resource.get("spec", {}).get("rules", [])}
        tls_hosts = {host for entry in tls for host in entry.get("hosts", [])}
        if not tls or not rule_hosts.issubset(tls_hosts):
            fail(f"Ingress/{name} must cover every rule host with TLS")
        if any(not entry.get("secretName") for entry in tls):
            fail(f"Ingress/{name} must name its TLS secret")

    for resource in resources:
        pod_spec = resource.get("spec", {}).get("template", {}).get("spec", {})
        for container in pod_spec.get("containers", []):
            image = container.get("image", "")
            if image.endswith(":latest"):
                fail(f"{resource['kind']}/{resource['metadata']['name']} uses :latest")

    print("Production infrastructure invariants passed.")


if __name__ == "__main__":
    try:
        main()
    except (AssertionError, subprocess.CalledProcessError, json.JSONDecodeError) as error:
        print(f"prod infra invariant failed: {error}", file=sys.stderr)
        raise SystemExit(1)
