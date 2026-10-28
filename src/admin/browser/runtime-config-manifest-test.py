"""Check canonical staging templates and their real DEV render for both static apps."""

import json
from pathlib import Path
import sys

import yaml

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "dev"))
import environment  # noqa: E402

CASES = (
    ("admin", "voice-admin", "voice-admin-runtime-config", "ADMIN_INGRESS_HOST",
     "admin.comrade.click", "admin-dev.comrade.click"),
    ("developer-portal", "voice-developer-portal", "voice-developer-portal-runtime-config",
     "DEVELOPER_PORTAL_INGRESS_HOST", "developers.comrade.click", "developers-dev.comrade.click"),
)

for name, deployment, config_map, host_key, staging_host, dev_host in CASES:
    source = (ROOT / "deploy" / "staging" / f"{name}.yaml").read_text(encoding="utf-8")
    source = (source.replace(f"__{host_key}__", staging_host)
                    .replace("__GATEWAY_INGRESS_HOST__", "voice.comrade.click")
                    .replace("__IMAGE_REGISTRY__", "registry.example")
                    .replace("__IMAGE_TAG__", "0" * 40))
    stage = list(yaml.safe_load_all(source))
    dev = environment.render(stage, "pmdebook")
    for label, objects, app_host, api_host in (
        ("STAGING", stage, staging_host, "voice.comrade.click"),
        ("DEV", dev, dev_host, "api-dev.comrade.click"),
    ):
        named = {(obj["kind"], obj["metadata"]["name"]): obj for obj in objects}
        config = json.loads(named[("ConfigMap", config_map)]["data"]["voice-runtime-config.json"])
        assert config == {
            "version": 1,
            "app_origin": f"https://{app_host}",
            "api_base_url": f"https://{api_host}",
        }, (name, label, config)
        pod = named[("Deployment", deployment)]["spec"]["template"]["spec"]
        volume = next(volume for volume in pod["volumes"] if volume["name"] == "runtime-config")
        assert volume["configMap"]["name"] == config_map
        mount = next(mount for mount in pod["containers"][0]["volumeMounts"]
                     if mount["name"] == "runtime-config")
        assert mount["readOnly"] is True
        assert mount["subPath"] == "voice-runtime-config.json"
        assert mount["mountPath"] == "/usr/share/nginx/html/voice-runtime-config.json"

print("admin and portal STAGING/DEV rendered runtime config: PASS")
