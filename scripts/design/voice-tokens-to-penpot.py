#!/usr/bin/env python3
"""Generate Penpot Tokens Studio import JSON from design/tokens/voice.tokens.json.

Canonical source: design/tokens/voice.tokens.json (git).
Penpot is a one-way mirror for design work; Flutter/runtime still reads voice.tokens.json.

Usage:
  python3 scripts/design/voice-tokens-to-penpot.py > /tmp/voice-penpot-tokens.json
  # Penpot: Tokens panel → Tools → Import → select JSON
"""
from __future__ import annotations

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
VOICE_TOKENS = ROOT / "design" / "tokens" / "voice.tokens.json"


def color_token(name: str, value: str) -> dict:
    return {
        "$type": "color",
        "$value": value.upper(),
        "$description": "",
    }


def spacing_token(value: int) -> dict:
    return {"$type": "spacing", "$value": str(value), "$description": ""}


def radius_token(value: int) -> dict:
    return {"$type": "borderRadius", "$value": str(value), "$description": ""}


def build(data: dict) -> dict:
    layout: dict = {}
    for key, val in data["space"].items():
        layout[f"space.{key}"] = spacing_token(val)
    for key, val in data["radius"].items():
        layout[f"radius.{key}"] = radius_token(val)

    accent: dict = {}
    for i, hex_val in enumerate(data["profileAccent"]["defaults"]):
        accent[f"profileAccent.{i}"] = color_token(f"profileAccent.{i}", hex_val)

    theme_sets: dict[str, dict] = {}
    theme_names = {
        "light": "Theme/Light",
        "dark": "Theme/Dark",
        "highContrast": "Theme/HighContrast",
    }
    penpot_themes = []
    for mode, set_name in theme_names.items():
        colors = {}
        for token_name, hex_val in data["themes"][mode].items():
            colors[token_name] = color_token(token_name, hex_val)
        theme_sets[set_name] = colors
        penpot_themes.append(
            {
                "id": f"voice-mode-{mode}",
                "name": {"light": "Light", "dark": "Dark", "highContrast": "HighContrast"}[
                    mode
                ],
                "group": "Mode",
                "selectedTokenSets": {set_name: "enabled"},
            }
        )

    return {
        "Foundation/Layout": layout,
        "Foundation/Accent": accent,
        **theme_sets,
        "$themes": penpot_themes,
        "$metadata": {
            "tokenSetOrder": [
                "Foundation/Layout",
                "Foundation/Accent",
                "Theme/Light",
                "Theme/Dark",
                "Theme/HighContrast",
            ],
            "activeThemes": ["voice-mode-light"],
            "activeSets": [
                "Foundation/Layout",
                "Foundation/Accent",
                "Theme/Light",
            ],
        },
    }


def main() -> int:
    path = VOICE_TOKENS
    if len(sys.argv) > 1:
        path = Path(sys.argv[1])
    with path.open(encoding="utf-8") as f:
        data = json.load(f)
    json.dump(build(data), sys.stdout, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
