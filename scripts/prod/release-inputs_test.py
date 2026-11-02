"""Production input rejection and dispatch using fake helpers; no live cluster."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = 'a' * 40
IMAGE = 'fixture.invalid/voice/space-native-migrate@sha256:' + 'b' * 64
spec = importlib.util.spec_from_file_location('policy_fixture', ROOT / 'scripts/prod/policy-owner-images-only_test.py')
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)


class ReleaseInputsTest(unittest.TestCase):
    def run_entrypoint(self, mode, policy='valid', image=IMAGE, source=SOURCE, workflow=False):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            descriptor = fixture.descriptor()
            if policy == 'wrong-source':
                descriptor = fixture.descriptor('c' * 40)
            elif policy == 'wrong-namespace':
                descriptor['policy_object']['metadata']['namespace'] = 'voice-staging'
            elif policy == 'wrong-digest':
                descriptor['policy_sha256'] = 'd' * 64
            target = work / 'input.json'
            target.write_text('{' if policy == 'malformed' else json.dumps(descriptor))
            trace = work / 'trace'
            wrapper = '''
bash() {
  if [ "$1" = scripts/prod/render-and-apply-prod.sh ]; then
    printf 'input-mode:%s\\n' "$(stat -c %a "$VOICE_RELEASE_POLICY_INPUT")" >> "$TRACE"
    command bash "$@"
    return
  fi
  printf 'child:%s|%s|%s\\n' "${1##*/}" "$SPACE_NATIVE_INSTALLER_IMAGE" "$SPACE_NATIVE_INSTALLER_SOURCE_SHA" >> "$TRACE"
}
kubectl() { printf 'kubectl:%s\\n' "$*" >> "$TRACE"; }
export -f bash kubectl
'''
            command = 'command bash scripts/prod/render-and-apply-prod.sh'
            if workflow:
                # Execute the actual protected workflow block, with GitHub's
                # expression expansion simulated only for these fixture values.
                text = (ROOT / '.github/workflows/prod-deploy.yml').read_text()
                step = text.split('      - name: Apply production manifests\n', 1)[1].split('\n      - name:', 1)[0]
                command = textwrap.dedent(step.split('        run: |\n', 1)[1])
                for expression, value in {'github.repository': 'fixture/voice',
                        'steps.verify_image.outputs.deploy_tag': SOURCE,
                        'steps.tag.outputs.tag': SOURCE, 'vars.VOICE_PROD_K8S_NAMESPACE': 'voice-prod',
                        'inputs.deploy_mode': mode}.items():
                    command = command.replace('${{ ' + expression + ' }}', value)
                self.assertNotIn('${{', command)
                self.assertIn('PROD_RELEASE_POLICY_INPUT_JSON: ${{ secrets.PROD_RELEASE_POLICY_INPUT }}', step)
                self.assertIn('SPACE_NATIVE_INSTALLER_IMAGE: ${{ vars.SPACE_NATIVE_INSTALLER_IMAGE }}', step)
                self.assertIn('SPACE_NATIVE_INSTALLER_SOURCE_SHA: ${{ vars.SPACE_NATIVE_INSTALLER_SOURCE_SHA }}', step)
            env = {**os.environ, 'VOICE_IMAGE_REGISTRY': 'fixture.invalid/voice',
                'VOICE_IMAGE_TAG': SOURCE, 'VOICE_K8S_NAMESPACE': 'voice-prod',
                'DEPLOY_MODE': mode, 'CHANGED_SERVICES': 'chat',
                'VOICE_APPLY_OBSERVABILITY': 'false', 'NEEDS_FULL_ROLLOUT': 'false',
                'NEEDS_USER_SPACE_ROLLOUT': 'false', 'TRACE': str(trace),
                'SPACE_NATIVE_INSTALLER_IMAGE': image, 'SPACE_NATIVE_INSTALLER_SOURCE_SHA': source,
                'RUNNER_TEMP': str(work),
                'PROD_RELEASE_POLICY_INPUT_JSON': '' if policy == 'missing' else target.read_text(),
                'VOICE_RELEASE_POLICY_INPUT': '' if policy == 'missing' else str(target)}
            result = subprocess.run(['/bin/bash', '-c', wrapper + command], cwd=ROOT, env=env,
                                    capture_output=True, text=True, timeout=20)
            if workflow:
                self.assertFalse((work / 'voice-prod-release-policy.json').exists(), result.stderr)
            return result, trace.read_text().splitlines() if trace.exists() else []

    def test_policy_rejection_precedes_every_cluster_effect_in_all_modes(self):
        for mode in ('full', 'app-only', 'images-only'):
            for policy in ('missing', 'malformed', 'wrong-source', 'wrong-namespace', 'wrong-digest'):
                with self.subTest(mode=mode, policy=policy):
                    result, events = self.run_entrypoint(mode, policy=policy)
                    self.assertNotEqual(result.returncode, 0, result.stdout)
                    self.assertEqual(events, [], result.stderr)
                    self.assertIn('release policy', result.stderr)

    def test_installer_rejection_precedes_every_full_and_app_only_effect(self):
        for mode in ('full', 'app-only'):
            for image, source in (('', SOURCE), ('fixture.invalid/installer:latest', SOURCE),
                                  (IMAGE + '\n', SOURCE), (IMAGE, ''), (IMAGE, 'c' * 40)):
                with self.subTest(mode=mode, image=image, source=source):
                    result, events = self.run_entrypoint(mode, image=image, source=source)
                    self.assertNotEqual(result.returncode, 0, result.stdout)
                    self.assertEqual(events, [], result.stderr)
                    self.assertIn('SPACE_NATIVE_INSTALLER', result.stderr)

    def test_valid_inputs_keep_order_and_pinned_installer_in_children(self):
        expected = {
            'full': ['apply-infra.sh', 'apply-gateway-ingress.sh', 'apply-app-manifests.sh',
                     'rollout-app-tier.sh', 'apply-livekit-ingress.sh'],
            'app-only': ['apply-gateway-ingress.sh', 'apply-migrate-jobs.sh', 'apply-app-manifests.sh',
                         'rollout-subset.sh', 'apply-minio-cors.sh', 'apply-livekit-ingress.sh'],
            'images-only': ['apply-gateway-ingress.sh', 'deploy-changed.sh', 'apply-signing-env.sh',
                            'apply-minio-cors.sh', 'apply-livekit-ingress.sh']}
        for mode, steps in expected.items():
            with self.subTest(mode=mode):
                result, events = self.run_entrypoint(mode)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual([event for event in events if event.startswith('child:')],
                                 ['child:' + step + '|' + IMAGE + '|' + SOURCE for step in steps])
                if mode != 'images-only':
                    self.assertTrue(events[len(steps)].startswith('kubectl:rollout status deployment/voice-gateway'))

    def test_images_only_without_migrations_does_not_require_installer(self):
        result, events = self.run_entrypoint('images-only', image='', source='')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(events)

    def test_actual_workflow_materializes_private_policy_and_cleans_it_on_success_or_rejection(self):
        for mode in ('full', 'app-only', 'images-only'):
            for policy in ('valid', 'missing', 'malformed'):
                with self.subTest(mode=mode, policy=policy):
                    result, events = self.run_entrypoint(mode, policy=policy, workflow=True)
                    self.assertEqual(events[0], 'input-mode:600')
                    if policy == 'valid':
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertTrue(any(event.startswith('child:') for event in events))
                    else:
                        self.assertNotEqual(result.returncode, 0)
                        self.assertEqual(events, ['input-mode:600'], result.stderr)


if __name__ == '__main__':
    unittest.main()
