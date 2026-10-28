#!/usr/bin/env python3
"""Validate public Admin/Portal runtime binding observed by post-deploy smoke."""

import json
import sys


def unique_object(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError("duplicate runtime config key")
        value[key] = item
    return value


def check(raw, app_origin, api_base_url):
    config = json.loads(raw, object_pairs_hook=unique_object)
    if (not isinstance(config, dict)
            or set(config) != {"version", "app_origin", "api_base_url"}
            or type(config["version"]) is not int or config["version"] != 1
            or config["app_origin"] != app_origin
            or config["api_base_url"] != api_base_url):
        raise ValueError("runtime config does not match expected public origins")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("usage: check-frontend-runtime-config.py APP_ORIGIN API_BASE_URL")
    try:
        check(sys.stdin.read(), sys.argv[1], sys.argv[2])
    except (ValueError, TypeError, json.JSONDecodeError) as error:
        raise SystemExit(f"runtime config invalid: {error}") from error
