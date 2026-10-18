#!/usr/bin/env python3
"""Offline paired Notification/User principal deployment contract (PyYAML)."""
import json
from pathlib import Path
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[2]


class PresenceDeploymentContract(unittest.TestCase):
    def test_paired_presence_authority(self):
        for environment in ("staging", "prod"):
            with self.subTest(environment=environment):
                docs = list(yaml.safe_load_all((ROOT / f"deploy/{environment}/services.yaml").read_text(encoding="utf-8")))
                def resource(kind, name):
                    found = [d for d in docs if d and d["kind"] == kind and d["metadata"]["name"] == name]
                    self.assertEqual(len(found), 1)
                    return found[0]
                def container(name):
                    pod = resource("Deployment", f"voice-{name}")["spec"]["template"]["spec"]
                    return pod, next(c for c in pod["containers"] if c["name"] == name)
                userpod, user = container("user")
                notifpod, notif = container("notification")
                uenv = {v["name"]: v for v in user["env"]}
                nenv = {v["name"]: v for v in notif["env"]}
                self.assertEqual(uenv["USER_NOTIFICATION_PRINCIPAL_GRPC_LISTEN"]["value"], ":9095")
                self.assertIn("configMapKeyRef", uenv["USER_NOTIFICATION_PRINCIPAL_REPLAY_REDIS_ADDR"]["valueFrom"])
                self.assertEqual(json.loads(uenv["S2S_JWKS_URLS_JSON"]["value"])["notification"], "https://voice-notification:8443/.well-known/jwks.json")
                self.assertEqual(nenv["USER_NOTIFICATION_PRINCIPAL_GRPC_ADDR"]["value"], "voice-user:9095")
                self.assertEqual(nenv["USER_NOTIFICATION_PRINCIPAL_TLS_SERVER_NAME"]["value"], "voice-user")
                self.assertEqual(nenv["NOTIFICATION_PRINCIPAL_JWKS_LISTEN"]["value"], ":8443")
                self.assertEqual(nenv["NOTIFICATION_PRINCIPAL_ACTIVE_KID"]["valueFrom"]["secretKeyRef"], {"name": "voice-notification-principal-signing", "key": "active-kid"})
                for env, prefix, fields in ((uenv, "USER_NOTIFICATION_PRINCIPAL_", ("TLS_CERT_FILE", "TLS_KEY_FILE")), (nenv, "NOTIFICATION_PRINCIPAL_", ("SIGNING_KEYS_DIR", "JWKS_TLS_CERT_FILE", "JWKS_TLS_KEY_FILE")), (nenv, "USER_NOTIFICATION_PRINCIPAL_", ("TLS_CA_FILE",))):
                    for field in fields:
                        path = env[prefix + field]["value"]
                        pod, app = (userpod, user) if env is uenv else (notifpod, notif)
                        mounts = [m for m in app["volumeMounts"] if path == m["mountPath"] or path.startswith(m["mountPath"] + "/")]
                        self.assertEqual(len(mounts), 1, path)
                        self.assertTrue(mounts[0]["readOnly"])
                        volume = next(v for v in pod["volumes"] if v["name"] == mounts[0]["name"])
                        self.assertNotIn("optional", volume["secret"])
                signing = next(v["secret"] for v in notifpod["volumes"] if v.get("secret", {}).get("secretName") == "voice-notification-principal-signing")
                self.assertEqual({v["key"] for v in signing["items"]}, {"current.pem", "next.pem"})
                self.assertEqual(signing["defaultMode"], 0o440)
                for pod, app in ((userpod, user), (notifpod, notif)):
                    for sidecar in pod["containers"]:
                        if sidecar is not app:
                            self.assertFalse(any("principal" in m["name"] for m in sidecar.get("volumeMounts", [])))
                self.assertFalse(any(v.get("secret", {}).get("secretName") == "voice-notification-principal-signing" for v in userpod["volumes"]))
                for name, port in (("user", 9095), ("notification", 8443)):
                    self.assertIn(port, [p["port"] for p in resource("Service", f"voice-{name}")["spec"]["ports"]])
                self.assertEqual(nenv["NOTIFICATION_RECORD_PUSHES"]["value"], "false", "must override inherited recording/offline fixture mode")
                self.assertEqual(notifpod["securityContext"]["fsGroup"], 65532, "nonroot process must read group-only secret projections")
                self.assertNotIn("NOTIFICATION_DEBUG_HTTP_ENABLED", nenv)

    def test_internal_network_routes(self):
        template = "deploy/templates/network-policy-notification-user-presence.yaml"
        docs = list(yaml.safe_load_all((ROOT / template).read_text(encoding="utf-8")))
        for target, caller, port in (("user", "notification", 9095), ("notification", "user", 8443)):
            policy = next(d for d in docs if d["spec"]["podSelector"] == {"matchLabels": {"app": f"voice-{target}"}})
            rules = [r for r in policy["spec"]["ingress"] if any(p["port"] == port for p in r["ports"])]
            self.assertEqual(len(rules), 1)
            self.assertEqual(rules[0]["from"], [{"podSelector": {"matchLabels": {"app": f"voice-{caller}"}}}])
        for environment in ("staging", "prod"):
            self.assertIn(template, (ROOT / f"scripts/{environment}/apply-app-manifests.sh").read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
