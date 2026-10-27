"""Opt-in real HTTP Auth/User regression; run only with a private fixture config.

The fixture provides already-existing account/profile rows and real Auth/User
servers. No User response, session, password change or profile is mocked here.
"""
import base64
import json
import os
from pathlib import Path
import unittest
import urllib.error
import urllib.request


class PublicPasswordChangeRealUserLoginTest(unittest.TestCase):
    def test_replacement_password_uses_existing_user_primary(self):
        config = json.loads(Path(os.environ["BE109_LOGIN_FIXTURE_CONFIG"]).read_text())
        checks = []

        def api(path, body=None, token=None, expected=200):
            headers = {"Content-Type": "application/json"}
            if token:
                headers["Authorization"] = "Bearer " + token
            request = urllib.request.Request(
                config["auth_url"] + path,
                data=json.dumps(body or {}).encode(), headers=headers, method="POST")
            try:
                with urllib.request.urlopen(request, timeout=20) as response:
                    status, raw = response.status, response.read()
            except urllib.error.HTTPError as failure:
                status, raw = failure.code, failure.read()
            checks.append({"route": path, "status": status, "body_bytes": len(raw)})
            self.assertEqual(expected, status, path)
            return json.loads(raw) if raw else None

        def user_calls():
            with urllib.request.urlopen(config["user_metrics_url"], timeout=5) as response:
                text = response.read().decode()
            return sum(float(line.rsplit(" ", 1)[1]) for line in text.splitlines()
                       if line.startswith("grpc_server_handled_total{")
                       and 'grpc_service="voice.user.v1.UserService"' in line
                       and 'grpc_method="EnsurePrimaryProfile"' in line
                       and 'grpc_code="OK"' in line)

        def claims(session):
            payload = session["access_token"].split(".")[1]
            return json.loads(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4)))

        before = user_calls()
        first = api("/api/v1/auth/login", {
            "email": config["email"], "password": config["old_password"],
            "device_info_json": '{"proof":"be109-before"}'})["session"]
        self.assertEqual(config["account_id"], first["account_id"])
        self.assertEqual(config["profile_id"], first["profile_id"])
        self.assertEqual(before + 1, user_calls(), "initial login must call the real User authority")
        old_claims = claims(first)
        self.assertEqual(first["profile_id"], old_claims["profile_id"])
        api("/api/v1/auth/validate", token=first["access_token"])

        changed = api("/api/v1/auth/password/change", {
            "current_password": config["old_password"],
            "new_password": config["new_password"]}, first["access_token"], expected=204)
        self.assertIsNone(changed, "204 must not contain a replacement session")
        self.assertEqual(0, checks[-1]["body_bytes"])
        api("/api/v1/auth/validate", token=first["access_token"], expected=401)
        api("/api/v1/auth/refresh", {"refresh_token": first["refresh_token"]}, expected=401)
        calls_after_change = user_calls()
        api("/api/v1/auth/login", {
            "email": config["email"], "password": config["old_password"]}, expected=401)
        self.assertEqual(calls_after_change, user_calls(), "wrong password must not reach User")

        fresh = api("/api/v1/auth/login", {
            "email": config["email"], "password": config["new_password"],
            "device_info_json": '{"proof":"be109-after"}'})["session"]
        self.assertEqual(config["account_id"], fresh["account_id"])
        self.assertEqual(config["profile_id"], fresh["profile_id"])
        self.assertEqual(calls_after_change + 1, user_calls(), "fresh login must read the real existing User profile")
        self.assertNotEqual(first["access_token"], fresh["access_token"])
        self.assertNotEqual(first["refresh_token"], fresh["refresh_token"])
        new_claims = claims(fresh)
        self.assertEqual(config["account_id"], new_claims["user_id"])
        self.assertEqual(config["profile_id"], new_claims["profile_id"])
        self.assertGreater(new_claims["session_epoch"], old_claims["session_epoch"])
        api("/api/v1/auth/validate", token=fresh["access_token"])

        result = {"result": "PASS", "checks": checks,
                  "account_id": config["account_id"], "profile_id": config["profile_id"],
                  "old_epoch": old_claims["session_epoch"], "new_epoch": new_claims["session_epoch"],
                  "real_user_ensure_primary_success_delta": user_calls() - before,
                  "fresh_public_login": True, "fresh_signature_validated_by_auth": True,
                  "returned_existing_profile_identity": True}
        # Private receipt lets the owner compare the actual fresh refresh hash with SQL.
        private = {"old_refresh": first["refresh_token"], "fresh_refresh": fresh["refresh_token"],
                   "old_jti": old_claims["jti"], "fresh_jti": new_claims["jti"]}
        out = Path(os.environ["BE109_LOGIN_PROOF_DIR"])
        (out / "private-session-receipt.json").write_text(json.dumps(private))
        (out / "public-login-result.json").write_text(json.dumps(result, indent=2))
        print("BE109_REAL_USER_PUBLIC_PASSWORD_LOGIN_PASS " + json.dumps(result))


if __name__ == "__main__":
    unittest.main(verbosity=2)
