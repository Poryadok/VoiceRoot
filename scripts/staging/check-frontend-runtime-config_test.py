import importlib.util
import json
from pathlib import Path
import unittest


path = Path(__file__).with_name("check-frontend-runtime-config.py")
spec = importlib.util.spec_from_file_location("frontend_runtime_config", path)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class RuntimeConfigSmokeTest(unittest.TestCase):
    APP = "https://developers.comrade.click"
    API = "https://voice.comrade.click"

    def test_exact_public_binding(self):
        module.check(json.dumps({"version": 1, "app_origin": self.APP,
                                 "api_base_url": self.API}), self.APP, self.API)

    def test_missing_wrong_or_invalid_fails(self):
        cases = [
            "",
            "{}",
            json.dumps({"version": 1, "app_origin": "https://developers-dev.comrade.click",
                        "api_base_url": self.API}),
            json.dumps({"version": 1, "app_origin": self.APP,
                        "api_base_url": "https://api-dev.comrade.click"}),
            json.dumps({"version": True, "app_origin": self.APP, "api_base_url": self.API}),
            f'{{"version":1,"app_origin":"{self.APP}","app_origin":"{self.APP}","api_base_url":"{self.API}"}}',
        ]
        for raw in cases:
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                module.check(raw, self.APP, self.API)


if __name__ == "__main__":
    unittest.main()
