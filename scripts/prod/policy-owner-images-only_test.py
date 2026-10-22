"""Offline actual production/selective entrypoints; no live cluster or images."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
OWNERS = ('auth', 'user', 'notification', 'realtime')
S0, S1 = 'b' * 40, 'a' * 40


def descriptor(source=S1):
    policy = json.loads((ROOT / 'src/backend/pkg/releasepolicy/testdata/free-invite-alpha.json').read_bytes())
    policy['source_sha'] = source
    raw = json.dumps(policy, sort_keys=True, separators=(',', ':')) + '\n'
    return {'profile': 'free-invite-alpha', 'source_sha': source,
        'policy_sha256': hashlib.sha256(raw.encode()).hexdigest(),
        'policy_object': {'apiVersion': 'v1', 'kind': 'ConfigMap',
            'metadata': {'name': 'voice-fixture-s0' if source == S0 else 'voice-fixture-s1', 'namespace': 'voice-prod'},
            'immutable': True, 'data': {'policy.json': raw}},
        'owners': {name: {'policy_directory': '/etc/fixture-' + name,
                          'snapshot_file': '/tmp/fixture-' + name + '.json'} for name in OWNERS}}


class SelectivePolicyTest(unittest.TestCase):
    def run_entrypoint(self, mode, selection):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            tools = work / 'bin'
            tools.mkdir()
            fixture = descriptor()
            old = descriptor(S0)
            spec = importlib.util.spec_from_file_location('fixture_policy_validator',
                ROOT / 'scripts/staging/nats-rollout-preservation/release_policy_wiring.py')
            validator = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(validator)
            validator.validate(old, S0, 'voice-prod')
            validator.validate(fixture, S1, 'voice-prod')
            (work / 'input.json').write_text(json.dumps(fixture))
            # This records commanded effects only. It does not simulate readiness
            # or claim an actual healthy deployment, provider or image binding.
            state = {'images': {name: 'fixture.invalid/voice/' + name + ':' + S0 for name in (*OWNERS, 'chat', 'search')},
                     'owner_policy_source_sha': old['source_sha'],
                     'owner_policy_object': old['policy_object']['metadata']['name'],
                     'owner_policy_sha256': old['policy_sha256']}
            initial = json.loads(json.dumps(state))
            (work / 'state.json').write_text(json.dumps(state))
            (work / 'jq').write_text('''#!/usr/bin/env python3
import json, sys
from pathlib import Path
name = sys.argv[sys.argv.index('--arg') + 2]
rows = json.loads(Path(sys.argv[-1]).read_text())['images']
print(next((row['deployment'] for row in rows if row['name'] == name), 'null'))
''')
            (work / 'jq').chmod(0o700)
            (tools / 'jq').symlink_to(work / 'jq')
            (work / 'adapter.py').write_text('''import json, os, sys
from pathlib import Path
args = sys.argv[1:]
with open(os.environ['TRACE'], 'a') as trace: trace.write('kubectl:' + json.dumps(args) + '\\n')
path = Path(os.environ['STATE'])
state = json.loads(path.read_text())
if args[:2] == ['set', 'image']:
    image = next(arg for arg in args[2:] if '=' in arg)
    name, ref = image.split('=', 1)
    state['images'][name] = ref
    path.write_text(json.dumps(state))
elif args[:2] == ['get', 'deployment']:
    print(args[2].removeprefix('voice-') if any('jsonpath=' in arg for arg in args) else '{}')
''')
            wrapper = '''kubectl() { command python3 "$ADAPTER" "$@"; }
bash() {
  printf 'child:%s\\n' "$1" >> "$TRACE"
  case "$1" in */scripts/staging/deploy-changed.sh) command bash "$@";; *) return 0;; esac
}
export -f kubectl bash
command bash scripts/prod/render-and-apply-prod.sh'''
            env = {**os.environ, 'PATH': str(tools) + os.pathsep + os.environ['PATH'],
                'VOICE_IMAGE_REGISTRY': 'fixture.invalid/voice', 'VOICE_IMAGE_TAG': S1,
                'VOICE_K8S_NAMESPACE': 'voice-prod', 'VOICE_RELEASE_POLICY_INPUT': str(work / 'input.json'),
                'DEPLOY_MODE': mode, 'CHANGED_SERVICES': selection,
                'NEEDS_FULL_ROLLOUT': 'false', 'NEEDS_USER_SPACE_ROLLOUT': 'false',
                'VOICE_APPLY_OBSERVABILITY': 'false', 'ADAPTER': str(work / 'adapter.py'),
                'STATE': str(work / 'state.json'), 'TRACE': str(work / 'trace')}
            result = subprocess.run(['/bin/bash', '-c', wrapper], cwd=ROOT, env=env,
                                    capture_output=True, text=True, timeout=20)
            events = (work / 'trace').read_text().splitlines() if (work / 'trace').exists() else []
            return result, events, initial, json.loads((work / 'state.json').read_text())

    def test_valid_S1_owner_or_mixed_selection_has_zero_effects(self):
        for selection in (*OWNERS, 'chat,user', 'notification,search', 'chat,auth,realtime'):
            with self.subTest(selection=selection):
                result, events, before, after = self.run_entrypoint('images-only', selection)
                detail = json.dumps({'selection': selection, 'events': events, 'S0': before, 'observed': after})
                self.assertNotEqual(result.returncode, 0, detail)
                self.assertFalse(events, detail)
                self.assertEqual(after, before)
                self.assertIn('images-only', result.stderr)
                self.assertIn('policy', result.stderr)

    def test_nonowners_selective_updates_use_actual_child_and_preserve_S0_owner_policy(self):
        result, events, before, after = self.run_entrypoint('images-only', 'chat,search')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(any('deploy-changed.sh' in event for event in events))
        self.assertEqual(sum(event.startswith('kubectl:["set", "image"') for event in events), 2)
        self.assertEqual(after['images']['chat'], 'fixture.invalid/voice/chat:' + S1)
        self.assertEqual(after['images']['search'], 'fixture.invalid/voice/search:' + S1)
        for owner in OWNERS:
            self.assertEqual(after['images'][owner], before['images'][owner])
        for key in ('owner_policy_source_sha', 'owner_policy_object', 'owner_policy_sha256'):
            self.assertEqual(after[key], before[key])

    def test_full_and_app_only_owner_requests_keep_template_dispatch(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode):
                result, events, _, _ = self.run_entrypoint(mode, 'user,chat')
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertTrue(any(event.endswith('/scripts/prod/apply-app-manifests.sh') for event in events))
                self.assertFalse(any(event.endswith('/scripts/staging/deploy-changed.sh') for event in events))

    def test_malformed_duplicate_unknown_and_ambiguous_selection_has_zero_effects(self):
        for selection in ('', 'user,', ',chat', 'user,user', ' user', 'User', 'user\n',
                          'chat,user,unknown', 'chat,,search', 'voice-user', 'chat, user'):
            with self.subTest(selection=selection):
                result, events, before, after = self.run_entrypoint('images-only', selection)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(events)
                self.assertEqual(after, before)

    def test_json_selection_cli_is_closed_and_cannot_bypass_owner_rejection(self):
        base = [sys.executable, str(ROOT / 'scripts/prod/production_image_refs.py'),
                '--catalog', str(ROOT / 'scripts/ci/staging-image-catalog.json'),
                '--registry', 'fixture.invalid/voice', '--tag', S1, '--mode', 'images-only',
                '--changed-services-json']
        for value in ('["user"]', '["chat","auth"]', '{"user":true}', '["chat",3]', '["chat"," user "]'):
            with self.subTest(selection=value):
                result = subprocess.run([*base, value], capture_output=True, text=True, timeout=10)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(result.stdout)
        result = subprocess.run([*base, '["chat","search"]'], capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('scope\tchat,search\n', result.stdout)


if __name__ == '__main__':
    unittest.main()
