"""Offline A4 release capability gate. Never probes or changes a runtime."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys
from urllib.parse import urlsplit

ALPHA_CAPABILITIES = ("phone_auth", "paid", "mobile_push", "public_discovery", "unfinished_rich_types", "federation")
INVENTORIES = ("images", "rollout", "readiness", "alerts")
SERVICES = frozenset(("gateway", "auth", "user", "chat", "messaging", "realtime", "social",
                      "file", "notification", "space", "role", "voice", "livekit", "matchmaking",
                      "moderation", "search", "analytics", "story", "bot", "subscription",
                      "gameintegration", "controlledgame", "web", "admin", "developer-portal"))


def exact_keys(value, keys, name):
    if not isinstance(value, dict) or set(value) != set(keys):
        raise ValueError(f"{name} must contain exactly: {', '.join(sorted(keys))}")


def require_version(value):
    if type(value) is not int or value != 1:
        raise ValueError("schema_version must be integer 1")


def require_source(value):
    if not isinstance(value, str) or not re.fullmatch(r"[0-9a-f]{40}", value) or value == "0" * 40:
        raise ValueError("source_sha must be a nonzero full lowercase commit SHA")


def validate(source):
    exact_keys(source, ("schema_version", "profile", "source_sha", "configuration", "capabilities", "inventories"), "manifest")
    require_version(source["schema_version"])
    require_source(source["source_sha"])
    if source["profile"] != "free-invite-alpha":
        raise ValueError("unsupported release profile")
    config = source["configuration"]
    exact_keys(config, ("environment", "checkout_mode", "development_defaults"), "configuration")
    if (config["environment"] not in ("staging", "production") or config["checkout_mode"] != "live"
            or config["development_defaults"] is not False):
        raise ValueError("release configuration must exclude development defaults and test checkout")
    exact_keys(source["capabilities"], ALPHA_CAPABILITIES, "capabilities")
    if any(value != "disabled" for value in source["capabilities"].values()):
        raise ValueError("free invite alpha requires all restricted capabilities disabled")
    exact_keys(source["inventories"], INVENTORIES, "inventories")
    for name, services in source["inventories"].items():
        if (not isinstance(services, list) or not services or any(not isinstance(s, str) for s in services)
                or len(services) != len(set(services)) or not set(services).issubset(SERVICES)):
            raise ValueError(f"{name} must contain unique canonical non-Federation service IDs")


def digest(source):
    return hashlib.sha256(json.dumps(source, sort_keys=True, separators=(",", ":"),
                                     allow_nan=False).encode("utf-8")).hexdigest()


def compile_suite(source):
    validate(source)
    return {"schema_version": 1, "source_sha": source["source_sha"], "manifest_sha256": digest(source),
            "scope": "negative qualification plan; no runtime checks executed",
            "cases": [{"case_id": f"{name}.{surface}", "capability": name, "surface": surface,
                       "expected": "unavailable_without_effects"}
                      for name in sorted(source["capabilities"]) for surface in ("ui", "rest", "ws")]}


def verify_results(source, report):
    suite = compile_suite(source)
    exact_keys(report, ("schema_version", "source_sha", "manifest_sha256", "observations"), "report")
    require_version(report["schema_version"])
    if report["source_sha"] != suite["source_sha"] or report["manifest_sha256"] != suite["manifest_sha256"]:
        raise ValueError("report source/manifest does not match the candidate")
    observations = report["observations"]
    if not isinstance(observations, list):
        raise ValueError("observations must be a list")
    required = {case["case_id"] for case in suite["cases"]}
    seen = set()
    for observation in observations:
        exact_keys(observation, ("case_id", "result", "evidence_uri"), "observation")
        case_id = observation["case_id"]
        if not isinstance(case_id, str) or case_id not in required or case_id in seen:
            raise ValueError("unknown or duplicate negative case")
        if observation["result"] != "PASS":
            raise ValueError(f"negative case not passed: {case_id}")
        uri = observation["evidence_uri"]
        if not isinstance(uri, str) or any(character.isspace() for character in uri):
            raise ValueError("evidence URI must be portable and nonempty")
        parsed = urlsplit(uri)
        if (parsed.scheme not in ("https", "artifact") or not parsed.netloc or not parsed.path.strip("/")
                or parsed.username is not None or parsed.password is not None):
            raise ValueError("evidence URI must be an https/artifact reference without credentials")
        seen.add(case_id)
    if seen != required:
        raise ValueError("missing required negative cases")
    return {"schema_version": 1, "source_sha": suite["source_sha"], "manifest_sha256": suite["manifest_sha256"],
            "result": "PASS", "scope": "offline evidence completeness only; not runtime attestation or acceptance",
            "case_count": len(seen)}


def load_json(text):
    def object_pairs(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result

    def reject_constant(_value):
        raise ValueError("non-finite JSON number")

    return json.loads(text, object_pairs_hook=object_pairs, parse_constant=reject_constant)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--expected-source-sha", required=True, help="immutable release source from protected caller")
    parser.add_argument("--results", type=Path, help="verify actual runner observations instead of emitting a suite plan")
    arguments = parser.parse_args(argv)
    try:
        source = load_json(arguments.manifest.read_text(encoding="utf-8"))
        require_source(arguments.expected_source_sha)
        if not isinstance(source, dict) or source.get("source_sha") != arguments.expected_source_sha:
            raise ValueError("candidate does not match expected release source")
        result = (verify_results(source, load_json(arguments.results.read_text(encoding="utf-8")))
                  if arguments.results else compile_suite(source))
        print(json.dumps(result, sort_keys=True, indent=2))
        return 0
    except (ValueError, OSError) as error:
        # Never print input values: a malformed operator package may contain secrets.
        print(f"capability gate rejected input ({type(error).__name__})", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
