import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OVERLAY = ROOT / "deploy/dev/match-squad-trust-overlay"


def source(name):
    return (OVERLAY / name).read_text(encoding="utf-8")


class MatchSquadRuntimeOverlayTests(unittest.TestCase):
    def test_every_patch_is_explicitly_dev_scoped(self):
        for path in OVERLAY.glob("*.patch.yaml"):
            with self.subTest(patch=path.name):
                document = source(path.name)
                self.assertIn("namespace: voice-dev", document)
                self.assertNotIn("voice-staging", document)
                self.assertNotIn("__GATEWAY_INGRESS_HOST__", document)

    def test_service_ports_and_deployment_listeners_are_paired(self):
        expected = {
            "voice-matchmaking": ("matchmaking", [(8443, "principal-jwks"),
                                                     (9092, "principal")]),
            "voice-chat": ("chat", [(9092, "match-principal")]),
            "voice-voice": ("voice", [(9094, "match-provider"),
                                        (9095, "match-member")]),
        }
        for service, (container_name, ports) in expected.items():
            with self.subTest(service=service):
                deployment = source(f"{service}.runtime.patch.yaml")
                self.assertIn(f"name: {container_name}", deployment)
                service_patch = source(f"{service}.service.patch.yaml")
                self.assertIn(f"name: {service}", service_patch)
                for port, port_name in ports:
                    with self.subTest(port=port):
                        self.assertLessEqual(len(port_name), 15)
                        self.assertIn(f"name: {port_name}\n              containerPort: {port}", deployment)
                        self.assertIn(
                            f"name: {port_name}\n      port: {port}\n      targetPort: {port_name}",
                            service_patch)

    def test_dev_runtime_contract_uses_dedicated_trust_and_existing_credentials(self):
        matchmaking = source("voice-matchmaking.runtime.patch.yaml")
        for value in (
            "MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR",
            "MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR",
            "voice-chat:9092",
            "voice-voice:9094",
            "MATCHMAKING_MATCH_SQUAD_CHAT_CA_FILE",
            "MATCHMAKING_MATCH_SQUAD_VOICE_CA_FILE",
            "voice-matchmaking-principal-signing",
            "voice-matchmaking-principal-jwks-tls",
            "voice-matchmaking-client-tls",
        ):
            self.assertIn(value, matchmaking)
        self.assertIn(
            'name: S2S_JWKS_URLS_JSON\n              value: \'{"gateway":"https://api-dev.comrade.click/.well-known/voice-principal-jwks.json"}\'',
            matchmaking,
        )
        chat = source("voice-chat.runtime.patch.yaml")
        for value in (
            "CHAT_MATCH_SQUAD_PRINCIPAL_GRPC_LISTEN",
            ":9092",
            "CHAT_MATCH_SQUAD_PRINCIPAL_CLIENT_CA_FILE",
            "https://voice-matchmaking.voice-dev.svc:8443/internal/v1/principal/jwks.json",
            "voice-matchmaking-principal-jwks-ca",
            "voice-match-squad-replay",
            "voice-chat-match-squad-server-tls",
        ):
            self.assertIn(value, chat)
        voice = source("voice-voice.runtime.patch.yaml")
        for value in (
            "VOICE_MATCH_SQUAD_PRINCIPAL_GRPC_LISTEN",
            ":9094",
            "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_GRPC_LISTEN",
            ":9095",
            "VOICE_MATCH_SQUAD_PRINCIPAL_CLIENT_CA_FILE",
            "https://voice-matchmaking.voice-dev.svc:8443/internal/v1/principal/jwks.json",
            "https://api-dev.comrade.click/.well-known/voice-principal-jwks.json",
            "voice-matchmaking-principal-jwks-ca",
            "voice-match-squad-replay",
            "voice-match-squad-member-replay",
            "voice-voice-match-squad-server-tls",
            "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_CLIENT_CA_FILE",
            "/etc/voice/media/ca/ca.crt",
        ):
            self.assertIn(value, voice)
        gateway = source("voice-gateway.patch.yaml")
        for value in (
            "GATEWAY_MATCH_SQUAD_MEMBER_TLS_CA_FILE",
            "/etc/voice/match-squad/ca/ca.crt",
            "GATEWAY_MATCH_SQUAD_MEMBER_CLIENT_CERT_FILE",
            "/etc/voice/media/client/tls.crt",
            "GATEWAY_MATCH_SQUAD_MEMBER_CLIENT_KEY_FILE",
            "/etc/voice/media/client/tls.key",
        ):
            self.assertIn(value, gateway)

        for name in (
            "voice-matchmaking.runtime.patch.yaml",
            "voice-chat.runtime.patch.yaml",
            "voice-voice.runtime.patch.yaml",
            "voice-gateway.patch.yaml",
        ):
            document = source(name)
            self.assertIn("name: voice-match-squad-ca", document)
            self.assertIn("mountPath: /etc/voice/match-squad/ca", document)
            self.assertIn("readOnly: true", document)

    def test_dev_complete_match_transport_is_gateway_only_and_purpose_bound(self):
        matchmaking = source("voice-matchmaking.runtime.patch.yaml")
        gateway = source("voice-gateway.patch.yaml")
        service = source("voice-matchmaking.service.patch.yaml")

        for value in (
            "MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN",
            "MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE",
            "MATCHMAKING_SPACE_PRINCIPAL_TLS_KEY_FILE",
            "MATCHMAKING_SPACE_PRINCIPAL_CLIENT_CA_FILE",
            "MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_ADDR",
            "MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD",
            'name: MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD\n              value: ""',
            'name: S2S_JWKS_URLS_JSON',
            '{"gateway":"https://api-dev.comrade.click/.well-known/voice-principal-jwks.json"}',
            "voice-matchmaking-complete-server-tls",
            "voice-matchmaking-complete-client-ca",
            "/etc/voice/match-squad/complete-client-ca/ca.crt",
        ):
            self.assertIn(value, matchmaking)
        self.assertNotIn('"space":', matchmaking)
        self.assertIn(
            "name: matchmaking-complete-client-ca\n          configMap:\n            name: voice-matchmaking-complete-client-ca",
            matchmaking,
        )
        self.assertIn("name: principal\n      port: 9092\n      targetPort: principal", service)
        self.assertIn("name: principal\n              containerPort: 9092", matchmaking)

        for value in (
            "GATEWAY_MATCHMAKING_COMPLETE_GRPC_ADDR",
            "voice-matchmaking:9092",
            "GATEWAY_MATCHMAKING_COMPLETE_TLS_CA_FILE",
            "GATEWAY_MATCHMAKING_COMPLETE_TLS_SERVER_NAME",
            "voice-matchmaking.voice-dev.svc",
            "GATEWAY_MATCHMAKING_COMPLETE_CLIENT_CERT_FILE",
            "GATEWAY_MATCHMAKING_COMPLETE_CLIENT_KEY_FILE",
            "voice-gateway-matchmaking-complete-client-tls",
            "/etc/voice/match-squad/complete-client/tls.crt",
            "/etc/voice/match-squad/complete-client/tls.key",
        ):
            self.assertIn(value, gateway)
        self.assertIn("name: voice-match-squad-ca", gateway)
        self.assertNotIn("voice-matchmaking-principal-signing", gateway)
        self.assertIn("name: voice-gateway-matchmaking-complete-client-tls", gateway)
        self.assertIn("name: voice-matchmaking-complete-client-ca", matchmaking)


if __name__ == "__main__":
    unittest.main()
