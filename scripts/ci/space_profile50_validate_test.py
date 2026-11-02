import json
import unittest

import space_profile50_validate as validator

PROFILE = "NATIVE50_PROFILE_V1 result=NONE elapsed_ns=1 catalog_calls=3 catalog_total_ms=1.000 catalog_self_ms=0.500"


def event(action, test=True, output=None):
    value = {"Action": action, "Package": validator.PACKAGE}
    if test:
        value["Test"] = validator.TEST
    if output is not None:
        value["Output"] = output
    return value


class ValidateProfileEvents(unittest.TestCase):
    def check(self, events, result=0):
        validator.validate([json.dumps(e) for e in events], result)

    def test_complete_success(self):
        self.check([event("run"), event("output", output=PROFILE), event("pass"), event("pass", test=False)])

    def test_missing_duplicate_skip_or_failure_cannot_pass(self):
        profile = event("output", output=PROFILE)
        cases = [
            [],
            [event("run"), event("pass"), event("pass", test=False)],
            [event("run"), event("output", output="NATIVE50_PROFILE_V1 result=NONE elapsed_ns=1"), event("pass"), event("pass", test=False)],
            [event("run"), event("pass"), profile, event("pass", test=False)],
            [event("run"), profile, profile, event("pass"), event("pass", test=False)],
            [event("run"), event("output", output=PROFILE.replace("catalog_calls=3", "catalog_calls=0")), event("pass"), event("pass", test=False)],
            [event("run"), profile, event("fail", test=False), event("pass"), event("pass", test=False)],
            [event("run"), event("run"), profile, event("pass"), event("pass", test=False)],
            [event("run"), profile, event("skip"), event("pass", test=False)],
            [event("run"), event("output", output="NATIVE50_PROFILE_V1 result=RPC_UNAVAILABLE elapsed_ns=1"), event("fail"), event("fail", test=False)],
        ]
        for events in cases:
            with self.subTest(events=events), self.assertRaises(ValueError):
                self.check(events)

    def test_nonzero_exit_cannot_pass(self):
        events = [event("run"), event("output", output=PROFILE), event("pass"), event("pass", test=False)]
        with self.assertRaises(ValueError):
            self.check(events, 1)


if __name__ == "__main__":
    unittest.main()
