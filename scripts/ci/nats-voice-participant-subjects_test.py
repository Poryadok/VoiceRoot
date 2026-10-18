"""Execute the actual bootstrap function against a deterministic NATS API fixture."""
import copy
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
LEGACY = "voice.call_incoming voice.call_accepted voice.call_declined voice.call_missed voice.call_ended voice.state_changed voice.screen_share_started voice.screen_share_stopped voice.call_started voice.member_joined".split()
PARTICIPANT = LEGACY + ["voice.participant_joined", "voice.participant_left"]
TARGET = PARTICIPANT + ["voice.roster_membership_changed"]
AGE = 604800000000000

FAKE = r'''
import json, os, sys
from pathlib import Path
path=Path(sys.argv[1]); args=sys.argv[2:]; state=json.loads(path.read_text())
subject=args[-2]
if subject == '$JS.API.STREAM.INFO.voice_events':
    print(json.dumps({k:v for k,v in state.items() if k != 'updates'}))
elif subject == '$JS.API.STREAM.UPDATE.voice_events':
    if os.getenv('FAIL_UPDATE'):
        print(json.dumps({'error':{'description':'injected refusal'}}));sys.exit(0)
    state['config']=json.loads(args[-1]);state['updates']+=1
    path.write_text(json.dumps(state));print(json.dumps(state))
else:
    sys.exit('unexpected API (no stream/consumer reset permitted): '+subject)
'''


class VoiceSubjectPreservation(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.bash = str(Path(os.environ.get("ProgramFiles", "C:/Program Files")) / "Git/bin/bash.exe") if os.name == "nt" else shutil.which("bash")
        if not cls.bash or not Path(cls.bash).is_file():
            if os.environ.get("REQUIRE_NATS_BOOTSTRAP_TEST") == "1":
                raise RuntimeError("required bootstrap proof has no bash")
            raise unittest.SkipTest("bash unavailable")
        if subprocess.run([cls.bash, "-c", "command -v jq"], capture_output=True).returncode:
            if os.environ.get("REQUIRE_NATS_BOOTSTRAP_TEST") == "1":
                raise RuntimeError("required bootstrap proof has no jq")
            raise unittest.SkipTest("jq unavailable")
        source = (ROOT / "docker/nats/realtime-bootstrap.sh").read_text(encoding="utf-8")
        cls.functions = source[source.index("max_age_to_cli() {"):source.index("stream_with_duplicate_window() {")]

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.state = self.directory / "state.json"
        self.fixture = self.directory / "fixture.py"
        self.fixture.write_text(FAKE, encoding="utf-8")
        self.initial = {"config": {"name": "voice_events", "subjects": LEGACY, "storage": "file", "retention": "limits", "max_age": AGE,
                                    "duplicate_window": 300000000000, "max_msgs": 731, "description": "retained-custom-setting"},
                        "state": {"messages": 17, "first_seq": 12, "last_seq": 28},
                        "consumers": {"rt_realtime1_voice": {"ack_floor": 13, "pending": 4, "identity": "original"}}, "updates": 0}

    def invoke(self, state, fail_update=False):
        self.state.write_text(json.dumps(state), encoding="utf-8")
        script = self.directory / "exercise.sh"
        script.write_text('set -eu\nnats_url=disposable\nnats() { "$PYTHON" "$FIXTURE" "$STATE" "$@"; }\n' + self.functions +
                          '\nstream_with_max_age voice_events ' + str(AGE) + ' ' + ' '.join(TARGET) + '\n', encoding="utf-8", newline="\n")
        env = dict(os.environ, PYTHON=sys.executable, FIXTURE=str(self.fixture), STATE=str(self.state))
        if fail_update:
            env["FAIL_UPDATE"] = "1"
        result = subprocess.run([self.bash, str(script)], env=env, capture_output=True, text=True)
        return result, json.loads(self.state.read_text(encoding="utf-8"))

    def test_addition_preserves_populated_stream_and_consumer_state(self):
        result, after = self.invoke(self.initial)
        self.assertEqual(result.returncode, 0, result.stderr)
        expected = copy.deepcopy(self.initial)
        expected["config"]["subjects"] = sorted(TARGET)
        expected["updates"] = 1
        self.assertEqual(after, expected)
        repeated, unchanged = self.invoke(after)
        self.assertEqual(repeated.returncode, 0, repeated.stderr)
        self.assertEqual(unchanged, after, "retry must not reset state or repeat the update")

    def test_unrelated_drift_and_update_refusal_fail_without_reset(self):
        for field, value in [("subjects", LEGACY + ["voice.rogue"]), ("subjects", LEGACY[:-1]), ("storage", "memory"), ("retention", "workqueue"), ("max_age", 1)]:
            with self.subTest(field=field, value=value):
                before = copy.deepcopy(self.initial)
                before["config"][field] = value
                result, after = self.invoke(before)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(after, before)
        result, after = self.invoke(self.initial, fail_update=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(after, self.initial)

    def test_roster_addition_preserves_complete_participant_predecessor(self):
        before = copy.deepcopy(self.initial)
        before["config"]["subjects"] = PARTICIPANT
        result, after = self.invoke(before)
        self.assertEqual(result.returncode, 0, result.stderr)
        expected = copy.deepcopy(before)
        expected["config"]["subjects"] = sorted(TARGET)
        expected["updates"] = 1
        self.assertEqual(after, expected)


if __name__ == "__main__":
    unittest.main()
