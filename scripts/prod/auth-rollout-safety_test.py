"""Execute the production app helper against fake kubectl; no cluster access."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
TAG = 'a' * 40
JOBS = ['voice-nats-' + name + '-bootstrap' for name in
        ('realtime', 'notification', 'search', 'analytics-chat')]


class AuthRolloutSafetyTest(unittest.TestCase):
    def run_helper(self, failure='', *, changed='auth', full='false', present=True,
                   replicas=3, namespace='voice-prod', invalid_policy=False):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            policy = json.loads((ROOT / 'src/backend/pkg/releasepolicy/testdata/free-invite-alpha.json').read_text())
            policy['source_sha'] = TAG
            raw = json.dumps(policy, sort_keys=True, separators=(',', ':')) + '\n'
            descriptor = {'profile': 'free-invite-alpha', 'source_sha': TAG,
                'policy_sha256': hashlib.sha256(raw.encode()).hexdigest(),
                'policy_object': {'apiVersion': 'v1', 'kind': 'ConfigMap',
                    'metadata': {'name': 'voice-fixture', 'namespace': namespace},
                    'immutable': True, 'data': {'policy.json': raw}},
                'owners': {name: {'policy_directory': '/etc/fixture-' + name,
                    'snapshot_file': '/tmp/fixture-' + name + '.json'}
                    for name in ('auth', 'user', 'notification', 'realtime')}}
            (work / 'policy.json').write_text('{}' if invalid_policy else json.dumps(descriptor))
            manifests = work / 'manifests'
            manifests.mkdir()
            for source in (ROOT / 'deploy/prod').glob('*.yaml'):
                (manifests / source.name).write_text(source.read_text().replace('namespace: voice-prod', 'namespace: ' + namespace))
            (work / 'state.json').write_text(json.dumps({'replicas': replicas, 'applies': 0}))
            (work / 'adapter.py').write_text('''import json, os, sys
from pathlib import Path
args = sys.argv[1:]
path = Path(os.environ['STATE'])
state = json.loads(path.read_text())
with open(os.environ['TRACE'], 'a') as trace: trace.write(json.dumps(args) + '\\n')
failure = os.environ['FAILURE']
if args[:2] == ['get', 'deployment']:
    if failure == 'lookup': sys.exit(21)
    if os.environ['PRESENT'] == 'false':
        sys.exit(0 if '--ignore-not-found' in args else 1)
    print(state['replicas'] if any('jsonpath=' in arg for arg in args) else '{}')
elif args[0] == 'scale':
    if failure == 'scale': sys.exit(22)
    state['replicas'] = int(next(arg.split('=')[1] for arg in args if arg.startswith('--replicas=')))
    path.write_text(json.dumps(state))
    if failure == 'scale-accepted': sys.exit(23)
elif args[0] == 'wait':
    if '--for=delete' in args and failure == 'drain': sys.exit(24)
    if any(arg == 'job/' + failure for arg in args): sys.exit(25)
elif args[0] == 'apply':
    payload = sys.stdin.read()
    state['applies'] += 1
    number = state['applies']
    if failure == 'apply' and number == 1: sys.exit(26)
    if 'name: voice-auth' in payload: state['replicas'] = 1
    path.write_text(json.dumps(state))
    if failure == 'apply-accepted' and number == 1: sys.exit(27)
    if failure == 'later-apply' and number == 2: sys.exit(28)
else:
    raise SystemExit('unexpected fake kubectl call: ' + repr(args))
''')
            wrapper = '''kubectl() { command python3 "$ADAPTER" "$@"; }
bash() {
  case "$1" in
    */scripts/staging/check-social-principal-secrets.sh)
      printf '["secret-preflight"]\\n' >> "$TRACE"
      [ "$FAILURE" != secret ];;
    *) return 99;;
  esac
}
export -f kubectl bash
command bash scripts/prod/apply-app-manifests.sh'''
            env = {**os.environ, 'VOICE_IMAGE_REGISTRY': 'fixture.invalid/voice',
                'VOICE_IMAGE_TAG': TAG, 'VOICE_K8S_NAMESPACE': namespace,
                'VOICE_RELEASE_POLICY_INPUT': str(work / 'policy.json'),
                'CHANGED_SERVICES': changed, 'NEEDS_FULL_ROLLOUT': full,
                'VOICE_IMAGE_PULL_SECRET': '', 'PROD_MANIFEST_DIR': str(manifests),
                'ADAPTER': str(work / 'adapter.py'), 'FAILURE': failure,
                'PRESENT': str(present).lower(), 'STATE': str(work / 'state.json'),
                'TRACE': str(work / 'trace')}
            result = subprocess.run(['/bin/bash', '-c', wrapper], cwd=ROOT, env=env,
                capture_output=True, text=True, timeout=20)
            events = [json.loads(line) for line in (work / 'trace').read_text().splitlines()] if (work / 'trace').exists() else []
            return result, events, json.loads((work / 'state.json').read_text())

    def assert_no_mutation(self, events):
        self.assertFalse([e for e in events if e[0] in ('scale', 'apply', 'patch')], events)

    def test_invalid_policy_fails_before_cluster_calls(self):
        result, events, state = self.run_helper(invalid_policy=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(events, [])
        self.assertEqual(state['replicas'], 3)

    def test_every_prerequisite_failure_preserves_auth(self):
        for failure in ('secret', *JOBS):
            with self.subTest(failure=failure):
                result, events, state = self.run_helper(failure)
                self.assertNotEqual(result.returncode, 0, result.stderr)
                self.assert_no_mutation(events)
                self.assertEqual(state['replicas'], 3)
                self.assertNotIn('RECOVERY REQUIRED', result.stderr)

    def test_lookup_error_fails_closed_without_mutations(self):
        result, events, state = self.run_helper('lookup')
        self.assertNotEqual(result.returncode, 0)
        self.assert_no_mutation(events)
        self.assertEqual(state['replicas'], 3)

    def test_scale_or_drain_failure_stops_apply_with_explicit_recovery(self):
        for failure, status in (('scale', 22), ('scale-accepted', 23), ('drain', 24)):
            with self.subTest(failure=failure):
                result, events, state = self.run_helper(failure, namespace='fixture-prod')
                self.assertEqual(result.returncode, status, result.stderr)
                self.assertEqual(state['applies'], 0)
                self.assertIn('RECOVERY REQUIRED', result.stderr)
                self.assertIn('fixture-prod', result.stderr)
                self.assertIn('--replicas=3', result.stderr)
                self.assertEqual(len([e for e in events if e[0] == 'scale']), 1)

    def test_failed_apply_never_claims_success_or_blindly_restores(self):
        for failure, status in (('apply', 26), ('apply-accepted', 27), ('later-apply', 28)):
            with self.subTest(failure=failure):
                result, events, state = self.run_helper(failure)
                self.assertEqual(result.returncode, status, result.stderr)
                self.assertIn('RECOVERY REQUIRED', result.stderr)
                self.assertIn('partially', result.stderr)
                self.assertNotIn('Production app manifests applied', result.stdout)
                self.assertEqual(len([e for e in events if e[0] == 'scale']), 1)

    def test_invalid_replica_readback_is_rejected_before_scaling(self):
        for replicas in ('unknown', -1):
            with self.subTest(replicas=replicas):
                result, events, _ = self.run_helper(replicas=replicas)
                self.assertNotEqual(result.returncode, 0)
                self.assert_no_mutation(events)
                self.assertIn('invalid Auth desired replica count', result.stderr)

    def test_selected_auth_or_full_rollout_drains_once_after_all_preflights(self):
        for changed, full in (('auth', 'false'), ('chat,auth', 'false'), ('chat', 'true')):
            with self.subTest(changed=changed, full=full):
                result, events, state = self.run_helper(changed=changed, full=full)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(state['replicas'], 1)
                scales = [i for i, e in enumerate(events) if e[0] == 'scale']
                drains = [i for i, e in enumerate(events) if '--for=delete' in e]
                applies = [i for i, e in enumerate(events) if e[0] == 'apply']
                self.assertEqual(len(scales), 1, events)
                self.assertEqual(len(drains), 1, events)
                self.assertEqual(events[0], ['secret-preflight'])
                for job in JOBS:
                    self.assertLess(next(i for i, e in enumerate(events) if 'job/' + job in e), scales[0])
                self.assertLess(scales[0], drains[0])
                self.assertLess(drains[0], applies[0])
                self.assertNotIn('RECOVERY REQUIRED', result.stderr)

    def test_absent_or_unselected_auth_is_not_scaled(self):
        for kwargs in ({'present': False}, {'changed': 'chat'}, {'changed': 'authentication'}):
            with self.subTest(kwargs=kwargs):
                result, events, state = self.run_helper(**kwargs)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertFalse([e for e in events if e[0] == 'scale' or '--for=delete' in e])
                self.assertGreater(state['applies'], 0)

    def test_original_zero_replicas_are_reported_without_inventing_availability(self):
        result, _, _ = self.run_helper('drain', replicas=0)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('--replicas=0', result.stderr)


if __name__ == '__main__':
    unittest.main()
