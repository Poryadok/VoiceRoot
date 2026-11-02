"""Offline source contract tests; synthetic inputs never attest a deployment."""
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[1]
OWNERS = ('auth', 'user', 'notification', 'realtime')


def helper():
    spec = importlib.util.spec_from_file_location('release_wiring', ROOT / 'scripts/staging/nats-rollout-preservation/release_policy_wiring.py')
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def fixture(namespace='voice-staging'):
    policy = json.loads((ROOT / 'src/backend/pkg/releasepolicy/testdata/free-invite-alpha.json').read_text())
    raw = json.dumps(policy, sort_keys=True, separators=(',', ':')) + '\n'
    return {'profile': 'free-invite-alpha', 'source_sha': policy['source_sha'],
            'policy_sha256': hashlib.sha256(raw.encode()).hexdigest(),
            'policy_object': {'apiVersion': 'v1', 'kind': 'ConfigMap',
                              'metadata': {'name': 'voice-fixture-policy', 'namespace': namespace},
                              'immutable': True, 'data': {'policy.json': raw}},
            'owners': {name: {'policy_directory': '/etc/fixture-' + name,
                              'snapshot_file': '/tmp/fixture-' + name + '.json'} for name in OWNERS}}


class WiringTest(unittest.TestCase):
    def test_outer_production_validation_precedes_dispatch(self):
        bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else '/bin/bash'
        wrapper = '''python3() { case "$1" in *release_policy_wiring.py) printf "policy\\n" >> "$TRACE";; esac; command python3 "$@"; }
bash() { printf "child:%s\\n" "$1" >> "$TRACE"; return 17; }
kubectl() { printf "cluster\\n" >> "$TRACE"; return 17; }
export -f python3 bash kubectl
command bash scripts/prod/render-and-apply-prod.sh'''
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)
            descriptor = path / 'input.json'
            trace = path / 'trace'
            env = {**os.environ, 'TRACE': str(trace), 'VOICE_IMAGE_REGISTRY': 'fixture.invalid/voice',
                   'VOICE_IMAGE_TAG': 'a' * 40, 'DEPLOY_MODE': 'full'}
            for state in ('missing', 'malformed', 'valid'):
                with self.subTest(input=state):
                    if trace.exists(): trace.unlink()
                    env.pop('VOICE_RELEASE_POLICY_INPUT', None)
                    if state != 'missing':
                        descriptor.write_text('{' if state == 'malformed' else json.dumps(fixture('voice-prod')))
                        env['VOICE_RELEASE_POLICY_INPUT'] = str(descriptor)
                    result = subprocess.run([bash, '-c', wrapper], cwd=ROOT, env=env,
                                            capture_output=True, timeout=20)
                    lines = trace.read_text().splitlines() if trace.exists() else []
                    self.assertNotEqual(result.returncode, 0)  # First fake child deliberately stops dispatch.
                    if state == 'valid':
                        self.assertEqual(lines[0], 'policy')
                        self.assertTrue(lines[1].startswith('child:'))
                        self.assertEqual(result.returncode, 17)
                    else:
                        self.assertFalse(any(line.startswith(('child:', 'cluster')) for line in lines))
                        self.assertIn(b'release policy', result.stderr.lower())

    def test_source_input_rejects_broken_auth_before_consumer_observation(self):
        mod = helper()
        text = (ROOT / 'deploy/staging/services.yaml').read_text(encoding='utf-8-sig')
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'deploy/staging/services.yaml'
            path.parent.mkdir(parents=True)
            path.write_text(text.replace('name: VOICE_RELEASE_PROFILE', 'name: OMITTED_PROFILE', 1))
            with self.assertRaisesRegex(ValueError, 'release_policy_template_environment'):
                mod.source_input(directory, {'tag': 'a' * 40, 'release_policy': fixture()})

    def test_snapshot_restart_assumption_is_bound_to_direct_image_entrypoints(self):
        for owner in OWNERS:
            dockerfile = (ROOT / 'src/backend' / owner / 'Dockerfile').read_text(encoding='utf-8-sig')
            stages = dockerfile.split('\nFROM ')
            self.assertGreater(len(stages), 1)
            lines = [line for line in stages[-1].splitlines() if line.startswith('ENTRYPOINT ')]
            expected = 'ENTRYPOINT ["java", "-jar", "/auth.jar"]' if owner == 'auth' else 'ENTRYPOINT ["/service"]'
            self.assertEqual(lines, [expected])

    def test_passive_template_preflight_rejects_broken_owner_before_effects(self):
        mod = helper()
        original = (ROOT / 'deploy/prod/services.yaml').read_text(encoding='utf-8-sig')
        value = fixture('voice-prod')
        mutations = [original.replace('name: VOICE_RELEASE_PROFILE', 'name: OMITTED_PROFILE', 1),
                     original.replace('          env:\n', '          securityContext: {readOnlyRootFilesystem: true}\n          env:\n', 1),
                     original.replace('          volumeMounts:\n', '          volumeMounts:\n            - {name: persistent, mountPath: /tmp}\n', 1),
                     original.replace('__RELEASE_SNAPSHOT_AUTH__', '__UNRESOLVED_SNAPSHOT__', 1)]
        for text in mutations:
            with self.assertRaises(ValueError):
                mod.render(text, value, 'a' * 40, 'voice-prod')

    def test_production_missing_input_precedes_every_mock_cluster_effect(self):
        bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else '/bin/bash'
        with tempfile.TemporaryDirectory() as directory:
            trace = Path(directory) / 'trace'
            env = {**os.environ, 'TRACE': str(trace), 'VOICE_IMAGE_REGISTRY': 'fixture.invalid/voice',
                   'VOICE_IMAGE_TAG': 'a' * 40, 'NEEDS_FULL_ROLLOUT': 'true'}
            env.pop('VOICE_RELEASE_POLICY_INPUT', None)
            result = subprocess.run([bash, '-c', 'kubectl() { printf "called\\n" >> "$TRACE"; return 17; }; export -f kubectl; bash scripts/prod/apply-app-manifests.sh'],
                                    cwd=ROOT, env=env, capture_output=True, timeout=20)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(trace.exists(), 'missing policy reached a cluster command')
            self.assertIn(b'release policy', result.stderr.lower())

    def test_captured_staging_both_renderers_accept_bound_input(self):
        folder = ROOT / 'scripts/staging/nats-rollout-preservation'
        sys.path.insert(0, str(folder))
        try:
            import compiler
            import source_plan
            p = {'generation': 'r20261022a', 'dataPVC': 'voice-nats-jsdata-d20261022a',
                 'registry': 'fixture.invalid/voice', 'tag': 'a' * 40,
                 's3_signing_endpoint': 'https://fixture.invalid', 'release_policy': fixture()}
            decode = lambda raw: yaml.safe_load(raw)
            docs = compiler.rendered_source(ROOT, p, decode)
            self.assertEqual(set(helper().check_documents(docs, fixture(), 'a' * 40, 'voice-staging')), set(OWNERS))
            builder = source_plan._Source(ROOT, p, decode, None)
            builder.replacements = {'__IMAGE_REGISTRY__': p['registry'], '__IMAGE_TAG__': p['tag'],
                                    '__S3_SIGNING_ENDPOINT__': p['s3_signing_endpoint'], '__GATEWAY_INGRESS_HOST__': 'fixture.invalid'}
            docs = builder.documents('deploy/staging/services.yaml', 'principal-secrets')
            self.assertEqual(set(helper().check_documents(docs, fixture(), 'a' * 40, 'voice-staging')), set(OWNERS))
            calls = []
            bad = {**p, 'release_policy': {**fixture(), 'profile': 'development'}}
            with self.assertRaises(ValueError):
                compiler.rendered_source(ROOT, bad, lambda raw: calls.append(raw))
            self.assertFalse(calls, 'invalid policy must precede decoder subprocess')
        finally:
            sys.path.pop(0)

    def test_all_eight_canonical_containers_declare_required_wiring(self):
        for env in ('prod', 'staging'):
            docs = yaml.safe_load_all((ROOT / 'deploy' / env / 'services.yaml').read_text(encoding='utf-8-sig'))
            for doc in docs:
                if not doc or doc.get('kind') != 'Deployment':
                    continue
                for container in doc['spec']['template']['spec']['containers']:
                    if container['name'] not in OWNERS:
                        continue
                    with self.subTest(environment=env, owner=container['name']):
                        names = {e['name'] for e in container.get('env', [])}
                        self.assertTrue({'VOICE_RELEASE_PROFILE', 'VOICE_RELEASE_POLICY_FILE',
                                         'VOICE_RELEASE_POLICY_SHA256', 'VOICE_RELEASE_SNAPSHOT_FILE'} <= names)

    def test_strict_input_negatives(self):
        mod = helper()
        original = fixture()
        mutations = [lambda x: x.pop('profile'), lambda x: x.update(profile='development'),
                     lambda x: x.update(source_sha='b' * 40), lambda x: x.update(policy_sha256='0' * 64),
                     lambda x: x['policy_object'].update(immutable=False),
                     lambda x: x['policy_object']['metadata'].update(name='__UNRESOLVED__'),
                     lambda x: x['owners'].pop('auth'),
                     lambda x: x['owners']['auth'].update(snapshot_file='/persist/old.json'),
                     lambda x: x['owners']['auth'].update(snapshot_file='/tmp/subdir/effective.json'),
                     lambda x: x['owners']['auth'].update(policy_directory='/tmp'),
                     lambda x: x.update(extra=True)]
        for mutate in mutations:
            value = copy.deepcopy(original)
            mutate(value)
            with self.assertRaises(ValueError):
                mod.validate(value, 'a' * 40, 'voice-staging')
        with self.assertRaises(ValueError):
            mod.validate(original, 'b' * 40, 'voice-staging')

    def test_render_all_owners_and_preserve_unrelated_configuration(self):
        mod = helper()
        for env in ('prod', 'staging'):
            namespace = 'voice-' + env
            value = fixture(namespace)
            text = (ROOT / 'deploy' / env / 'services.yaml').read_text(encoding='utf-8-sig')
            result = mod.render(text, value, 'a' * 40, namespace)
            docs = list(yaml.safe_load_all(result))
            seen = mod.check_documents(docs, value, 'a' * 40, namespace)
            self.assertEqual(set(seen), set(OWNERS))
            # A policy render may replace only its explicitly enumerated tokens.
            # Semantic preservation checks are stronger than replacing shared literal values.
            before = list(yaml.safe_load_all(text))
            for old, new in zip(before, docs):
                if not old or old.get('kind') != 'Deployment':
                    self.assertEqual(old, new)
                    continue
                for a, b in zip(old['spec']['template']['spec']['containers'], new['spec']['template']['spec']['containers']):
                    self.assertEqual(a.get('image'), b.get('image'))
                    self.assertEqual([e for e in a.get('env', []) if not e['name'].startswith('VOICE_RELEASE_')],
                                     [e for e in b.get('env', []) if not e['name'].startswith('VOICE_RELEASE_')])
                    self.assertEqual([m for m in a.get('volumeMounts', []) if m['name'] != 'release-policy'],
                                     [m for m in b.get('volumeMounts', []) if m['name'] != 'release-policy'])
            for change in ('readonly-root', 'persistent-tmp', 'missing-auth-env', 'writable-policy', 'supervisor'):
                with self.subTest(environment=env, invalid=change):
                    broken = copy.deepcopy(docs)
                    bad = next(d for d in broken if d and d.get('kind') == 'Deployment' and d['metadata']['name'] == 'voice-auth')
                    container = bad['spec']['template']['spec']['containers'][0]
                    if change == 'readonly-root':
                        container['securityContext'] = {'readOnlyRootFilesystem': True}
                    if change == 'persistent-tmp':
                        container['volumeMounts'].append({'name': 'persistent', 'mountPath': '/tmp'})
                    if change == 'missing-auth-env':
                        container['env'] = [e for e in container['env'] if e['name'] != 'VOICE_RELEASE_PROFILE']
                    if change == 'writable-policy':
                        container['volumeMounts'][0]['readOnly'] = False
                    if change == 'supervisor':
                        container['command'] = ['/bin/sh', '-c', 'while true; do service; done']
                    with self.assertRaises(ValueError):
                        mod.check_documents(broken, value, 'a' * 40, namespace)


if __name__ == '__main__':
    unittest.main()
