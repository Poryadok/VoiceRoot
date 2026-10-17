"""Negative contract tests for catalog environment eligibility."""
import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
CATALOG = ROOT / "scripts/ci/staging-image-catalog.json"
CHECK = ROOT / "scripts/ci/check-image-catalog-drift.sh"


class CatalogEnvironmentContractTests(unittest.TestCase):
    def setUp(self):
        self.catalog = json.loads(CATALOG.read_text(encoding="utf-8"))

    def assert_rejected(self, catalog):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "catalog.json"
            path.write_text(json.dumps(catalog), encoding="utf-8")
            env = dict(os.environ, VOICE_IMAGE_CATALOG=str(path))
            result = subprocess.run(["bash", str(CHECK)], cwd=ROOT, env=env,
                                    text=True, capture_output=True, check=False)
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("only Game Integration may declare", result.stderr)

    def test_only_gameintegration_may_be_internal_only(self):
        catalog = copy.deepcopy(self.catalog)
        next(row for row in catalog["images"] if row["name"] == "bot")["environments"] = ["dev", "staging"]
        self.assert_rejected(catalog)

    def test_gameintegration_cannot_be_promoted_to_production_catalog(self):
        catalog = copy.deepcopy(self.catalog)
        next(row for row in catalog["images"] if row["name"] == "gameintegration")["environments"] = ["dev", "staging", "prod"]
        self.assert_rejected(catalog)

    def test_gameintegration_catalog_row_cannot_be_removed(self):
        catalog = copy.deepcopy(self.catalog)
        catalog["images"] = [row for row in catalog["images"] if row["name"] != "gameintegration"]
        self.assert_rejected(catalog)


if __name__ == "__main__":
    unittest.main()
