#!/usr/bin/env python3
"""Validate the emitted stack-lock format and enumerate the production refs.

The format intentionally accepts only the plain scalars/mappings emitted by
render-stack-lock.sh, not YAML aliases, tags, flow collections or duplicate keys.
Validation finishes before any refs are printed or registry lookup may start.
"""
import argparse
import json
from pathlib import Path
import re
import sys


IMMUTABLE_TAG = re.compile(r"(?:[0-9a-f]{7,40}|v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)\Z")
REGISTRY = re.compile(r"[a-z0-9][a-z0-9.:-]*(?:/[a-z0-9][a-z0-9._-]*)+\Z")


def production_names(catalog):
    rows = json.loads(Path(catalog).read_text(encoding="utf-8"))["images"]
    names = [row["name"] for row in rows
             if "prod" in row.get("environments", ["dev", "staging", "prod"])]
    names.append("nats-hub-config-renderer")
    if len(names) != len(set(names)) or not names:
        raise ValueError("invalid production image catalog")
    return names


def read_lock(path):
    roots, images = {}, {}
    in_images = False
    for number, line in enumerate(Path(path).read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if line == "images:":
            if in_images:
                raise ValueError("duplicate images mapping")
            in_images = True
            continue
        root = re.fullmatch(r"(version|registry|tag): ([A-Za-z0-9_./:-]+)", line)
        image = re.fullmatch(r"  ([a-z][a-z0-9-]*): ([A-Za-z0-9_.-]+)", line)
        if root and not in_images:
            key, value = root.groups()
            if key in roots:
                raise ValueError(f"duplicate lock field: {key}")
            roots[key] = value
        elif image and in_images:
            key, value = image.groups()
            if key in images:
                raise ValueError(f"duplicate lock image: {key}")
            images[key] = value
        else:
            raise ValueError(f"invalid stack lock syntax at line {number}")
    if roots.keys() != {"version", "registry", "tag"} or roots["version"] != "1" or not in_images:
        raise ValueError("stack lock requires version: 1, registry, tag and images")
    return roots, images


def image_plan(catalog, registry, tag, lock=None):
    names = production_names(catalog)
    if not REGISTRY.fullmatch(registry):
        raise ValueError("invalid production image registry")
    if lock:
        roots, images = read_lock(lock)
        if roots["registry"] != registry:
            raise ValueError("stack lock registry differs from apply registry")
        if tag and roots["tag"] != tag:
            raise ValueError("stack lock tag differs from apply tag")
        tag = roots["tag"]
        if images.keys() != set(names):
            missing, extra = set(names) - images.keys(), images.keys() - set(names)
            raise ValueError(f"invalid production image set; missing={sorted(missing)} extra={sorted(extra)}")
        if any(value != tag for value in images.values()):
            raise ValueError("all production image tags must equal the applied lock tag")
    if len(tag) > 128 or not IMMUTABLE_TAG.fullmatch(tag):
        raise ValueError("production requires an immutable git SHA or semver tag")
    return tag, [(name, f"{registry}/{name}:{tag}") for name in names]


def changed_scope(catalog, mode, value, json_value=None):
    if json_value is not None:
        if value:
            raise ValueError("supply changed services as CSV or JSON, not both")
        names = json.loads(json_value)
        if not isinstance(names, list) or any(not isinstance(name, str) for name in names):
            raise ValueError("changed_services must be an array of image names")
    else:
        names = value.split(",") if value else []
    eligible = set(production_names(catalog)) - {"nats-hub-config-renderer"}
    if any(name not in eligible for name in names) or len(names) != len(set(names)):
        raise ValueError("changed services contain duplicate, unknown or nonproduction image names")
    if mode == "images-only" and not names:
        raise ValueError("images-only requires nonempty changed services")
    if mode == "images-only" and set(names) & {"auth", "user", "notification", "realtime"}:
        raise ValueError("images-only cannot update release-policy owners; use a reviewed full/app-only template rollout")
    return ",".join(names)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--catalog", required=True)
    parser.add_argument("--registry", required=True)
    parser.add_argument("--tag", default="")
    parser.add_argument("--lock")
    parser.add_argument("--mode", choices=("full", "app-only", "images-only"), default="full")
    parser.add_argument("--changed-services", default="")
    parser.add_argument("--changed-services-json")
    args = parser.parse_args()
    try:
        tag, refs = image_plan(args.catalog, args.registry, args.tag, args.lock)
        scope = changed_scope(args.catalog, args.mode, args.changed_services, args.changed_services_json)
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"ERROR: production image plan: {error}", file=sys.stderr)
        return 1
    print(f"tag\t{tag}")
    print(f"scope\t{scope}")
    for name, ref in refs:
        print(f"{name}\t{ref}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
