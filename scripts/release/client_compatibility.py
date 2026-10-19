"""Offline A4 client compatibility plan and evidence completeness; no runtime attestation."""
import argparse
import json
from pathlib import Path
import re
import sys
from urllib.parse import urlsplit

from capability_manifest import digest, exact_keys, load_json, require_source, require_version

LABELS = ("previous", "current")
PLATFORMS = ("windows", "web")


def exact_value(value):
    if (not isinstance(value, str) or not value.strip() or len(value) > 256
            or value != value.strip() or any(c in value for c in "*<>\r\n\t")
            or value.lower() in ("latest", "current", "previous", "stable", "any", "unknown")):
        raise ValueError("explicit observed value required")


def content_hash(value):
    if not isinstance(value, str) or not re.fullmatch(r"[0-9a-f]{64}", value) or value == "0" * 64:
        raise ValueError("nonzero SHA256 required")


def validate(source):
    exact_keys(source, ("schema_version", "source_sha", "matrix_version", "regression_sampling_policy",
                        "backends", "clients", "tuples"), "compatibility manifest")
    require_version(source["schema_version"])
    require_source(source["source_sha"])
    exact_value(source["matrix_version"])
    exact_value(source["regression_sampling_policy"])
    exact_keys(source["backends"], LABELS, "backends")
    exact_keys(source["clients"], PLATFORMS, "clients")
    for platform in (None,) + PLATFORMS:
        releases = source["backends"] if platform is None else source["clients"][platform]
        exact_keys(releases, LABELS, "releases")
        for release in releases.values():
            fields = ("source_sha", "artifact_sha256") + (() if platform is None else ("version",))
            exact_keys(release, fields, "release")
            require_source(release["source_sha"])
            content_hash(release["artifact_sha256"])
            if platform is not None:
                if (not isinstance(release["version"], str)
                        or not re.fullmatch(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:[-+][0-9A-Za-z.-]+)?", release["version"])):
                    raise ValueError("client semantic version required")
        if (releases["previous"]["artifact_sha256"] == releases["current"]["artifact_sha256"]
                or (platform is not None and releases["previous"]["version"] == releases["current"]["version"])):
            raise ValueError("previous and current releases must be distinct")
    if source["backends"]["current"]["source_sha"] != source["source_sha"]:
        raise ValueError("current backend must match release source")
    tuples = source["tuples"]
    if not isinstance(tuples, list) or not 2 <= len(tuples) <= 100:
        raise ValueError("nonempty Windows and Web tuples required")
    ids, identities, platforms = set(), set(), set()
    for item in tuples:
        if not isinstance(item, dict) or item.get("platform") not in PLATFORMS:
            raise ValueError("unsupported tuple platform")
        fields = ("id", "platform", "os", "os_version", "arch")
        if item["platform"] == "web":
            fields += ("browser", "browser_version")
        exact_keys(item, fields, "tuple")
        for value in item.values():
            exact_value(value)
        if item["platform"] == "windows" and item["os"] != "Windows":
            raise ValueError("Windows tuple requires Windows OS")
        if not re.fullmatch(r"[0-9][0-9A-Za-z._ -]*", item["os_version"]):
            raise ValueError("explicit OS version/build required")
        if item["platform"] == "web" and not re.fullmatch(r"[0-9]+(?:\.[0-9]+){1,3}", item["browser_version"]):
            raise ValueError("exact numeric browser version required")
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,63}", item["id"]) or item["id"] in ids:
            raise ValueError("unique tuple ID required")
        identity = tuple((key, item[key]) for key in sorted(item) if key != "id")
        if identity in identities:
            raise ValueError("duplicate tuple")
        ids.add(item["id"])
        identities.add(identity)
        platforms.add(item["platform"])
    if platforms != set(PLATFORMS):
        raise ValueError("both Windows and Web tuples required")


def compile_suite(source):
    validate(source)
    cases = []
    for item in sorted(source["tuples"], key=lambda item: item["id"]):
        for client in LABELS:
            for backend in LABELS:
                checks = ("smoke",) if item["platform"] == "web" else ("smoke", "version", "forced_update", "update_url")
                for check in checks:
                    cases.append({"case_id": f"{item['id']}.{client}.{backend}.{check}",
                                  "tuple_id": item["id"], "check": check, "client": client, "backend": backend})
        cases.append({"case_id": f"{item['id']}.current.backend_rollback", "tuple_id": item["id"],
                      "check": "backend_rollback", "client": "current", "client_sequence": ["previous", "current"],
                      "backend_sequence": ["current", "previous"]})
    return {"schema_version": 1, "source_sha": source["source_sha"], "manifest_sha256": digest(source),
            "scope": "compatibility plan only; no runtime checks executed", "cases": cases}


def verify_results(source, report):
    suite = compile_suite(source)
    exact_keys(report, ("schema_version", "source_sha", "manifest_sha256", "observations"), "report")
    require_version(report["schema_version"])
    if report["source_sha"] != suite["source_sha"] or report["manifest_sha256"] != suite["manifest_sha256"]:
        raise ValueError("stale source or manifest")
    if not isinstance(report["observations"], list):
        raise ValueError("observations must be a list")
    required = {case["case_id"] for case in suite["cases"]}
    seen = set()
    for observation in report["observations"]:
        exact_keys(observation, ("case_id", "result", "evidence_uri", "evidence_sha256"), "observation")
        case_id = observation["case_id"]
        if not isinstance(case_id, str) or case_id not in required or case_id in seen or observation["result"] != "PASS":
            raise ValueError("each mandatory case requires exactly one PASS")
        content_hash(observation["evidence_sha256"])
        uri = observation["evidence_uri"]
        if not isinstance(uri, str) or any(c.isspace() for c in uri):
            raise ValueError("portable evidence URI required")
        parsed = urlsplit(uri)
        if (parsed.scheme not in ("https", "artifact") or not parsed.netloc or not parsed.path.strip("/")
                or parsed.username is not None or parsed.password is not None or parsed.query or parsed.fragment):
            raise ValueError("portable uncredentialed evidence URI required")
        seen.add(case_id)
    if seen != required:
        raise ValueError("missing mandatory compatibility cases")
    return {"schema_version": 1, "source_sha": suite["source_sha"], "manifest_sha256": suite["manifest_sha256"],
            "result": "PASS", "case_count": len(seen), "scope": "offline evidence completeness only"}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--expected-source-sha", required=True)
    parser.add_argument("--results", type=Path)
    args = parser.parse_args(argv)
    try:
        source = load_json(args.manifest.read_text(encoding="utf-8"))
        require_source(args.expected_source_sha)
        if not isinstance(source, dict) or source.get("source_sha") != args.expected_source_sha:
            raise ValueError("external release source mismatch")
        result = (verify_results(source, load_json(args.results.read_text(encoding="utf-8")))
                  if args.results else compile_suite(source))
        print(json.dumps(result, sort_keys=True, indent=2))
        return 0
    except (ValueError, OSError):
        print("client compatibility gate rejected input", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
