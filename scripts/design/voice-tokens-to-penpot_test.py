#!/usr/bin/env python3
"""Tests for voice-tokens-to-penpot.py export."""
from __future__ import annotations

import json
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "design" / "voice-tokens-to-penpot.py"
CANON = ROOT / "design" / "tokens" / "voice.tokens.json"


class PenpotTokenExportTest(unittest.TestCase):
    def test_export_includes_all_canonical_keys(self) -> None:
        with CANON.open(encoding="utf-8") as f:
            canon = json.load(f)

        proc = subprocess.run(
            [sys.executable, str(SCRIPT)],
            check=True,
            capture_output=True,
            text=True,
        )
        export = json.loads(proc.stdout)

        layout = export["Foundation/Layout"]
        for key in canon["space"]:
            self.assertIn(f"space.{key}", layout)
        for key in canon["radius"]:
            self.assertIn(f"radius.{key}", layout)
        for key in canon.get("layout", {}):
            self.assertIn(f"layout.{key}", layout)
        for key in canon.get("stroke", {}):
            self.assertIn(f"stroke.{key}", layout)
        for key, style in canon.get("type", {}).items():
            self.assertIn(f"type.{key}.size", layout)
            self.assertIn(f"type.{key}.weight", layout)
            self.assertIn(f"type.{key}.lineHeight", layout)
            if "letterSpacing" in style:
                self.assertIn(f"type.{key}.letterSpacing", layout)

        accent = export["Foundation/Accent"]
        self.assertEqual(len(accent), len(canon["profileAccent"]["defaults"]))
        for i, hex_val in enumerate(canon["profileAccent"]["defaults"]):
            name = f"profileAccent.{i}"
            self.assertEqual(accent[name]["$value"], hex_val.upper())

        for mode, set_name in [
            ("light", "Theme/Light"),
            ("dark", "Theme/Dark"),
            ("highContrast", "Theme/HighContrast"),
        ]:
            theme_set = export[set_name]
            for token_name, hex_val in canon["themes"][mode].items():
                self.assertEqual(theme_set[token_name]["$value"], hex_val.upper())

        self.assertEqual(len(export["$themes"]), 3)
        self.assertIn("Foundation/Layout", export["$metadata"]["tokenSetOrder"])


if __name__ == "__main__":
    unittest.main()
