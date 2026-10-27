"""Offline public-config rendering checks; no cluster or deployment authority."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[3]
spec = importlib.util.spec_from_file_location('web_dev_environment', ROOT / 'scripts/dev/environment.py')
dev = importlib.util.module_from_spec(spec)
spec.loader.exec_module(dev)


class RuntimeBindingTest(unittest.TestCase):
    def test_production_web_render_substitutes_all_public_origins_offline(self):
        script = (ROOT / 'scripts/prod/apply-app-manifests.sh').read_text(encoding='utf-8')
        block = script.split('if [ -f "${MANIFEST_DIR}/flutter-web.yaml" ]; then\n', 1)[1].split('\nfi', 1)[0]
        self.assertEqual(block.count('kubectl apply -f -'), 1)
        # Exercise the actual render pipeline with its cluster sink replaced.
        block = block.replace('kubectl apply -f -', 'cat')
        output = subprocess.check_output(['bash', '-c', 'render() { cat "$1"; };\n' + block],
            text=True, env={**os.environ, 'MANIFEST_DIR': str(ROOT / 'deploy/prod'),
                'VOICE_WEB_INGRESS_HOST': 'app.prod.example',
                'VOICE_GATEWAY_INGRESS_HOST': 'api.prod.example',
                'VOICE_LIVEKIT_INGRESS_HOST': 'rtc.prod.example'})
        objects = list(yaml.safe_load_all(output))
        config = next(row for row in objects if row['kind'] == 'ConfigMap')
        self.assertEqual(json.loads(config['data']['voice-runtime-config.json']),
            dict(version=1, app_origin='https://app.prod.example',
                 api_base_url='https://api.prod.example', livekit_url='wss://rtc.prod.example'))

    def test_canonical_and_dev_configs_are_explicit_and_mounted_readonly(self):
        raw = (ROOT / 'deploy/staging/flutter-web.yaml').read_text(encoding='utf-8')
        for key, value in {'__WEB_INGRESS_HOST__': 'app.comrade.click',
                           '__GATEWAY_INGRESS_HOST__': 'voice.comrade.click',
                           '__LIVEKIT_INGRESS_HOST__': 'livekit.comrade.click',
                           '__IMAGE_REGISTRY__': 'registry', '__IMAGE_TAG__': 'a' * 40}.items():
            raw = raw.replace(key, value)
        staging = list(yaml.safe_load_all(raw))
        development = dev.render(staging, 'fixture-node')
        for objects, origin, api, rtc in (
                (staging, 'https://app.comrade.click', 'https://voice.comrade.click', 'wss://livekit.comrade.click'),
                (development, 'https://dev.comrade.click', 'https://api-dev.comrade.click', 'wss://livekit-dev.comrade.click')):
            config = next(row for row in objects if row['kind'] == 'ConfigMap')
            self.assertEqual(json.loads(config['data']['voice-runtime-config.json']),
                             dict(version=1, app_origin=origin, api_base_url=api, livekit_url=rtc))
            deployment = next(row for row in objects if row['kind'] == 'Deployment')
            pod = deployment['spec']['template']['spec']
            mount = pod['containers'][0]['volumeMounts'][0]
            self.assertTrue(mount['readOnly'])
            self.assertEqual(mount['subPath'], 'voice-runtime-config.json')
            self.assertEqual(pod['volumes'][0]['configMap']['name'], config['metadata']['name'])
        # Changing environment does not change the canonical image reference.
        image = lambda rows: next(r for r in rows if r['kind'] == 'Deployment')['spec']['template']['spec']['containers'][0]['image']
        self.assertEqual(image(staging), image(development))

    def test_runtime_image_requires_config_and_cannot_include_endpoint_args(self):
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        builds = [step for step in workflow['jobs']['web']['steps']
                  if step.get('uses', '').startswith('docker/build-push-action@')]
        publishers = [step for step in builds if step['with']['push'] is True]
        self.assertEqual(len(publishers), 1, 'one producer must publish both aliases')
        for step in builds:
            self.assertEqual(step['with']['build-args'].splitlines(),
                ['VOICE_RUNTIME_CONFIG_REQUIRED=true', 'SOURCE_REVISION=${{ github.sha }}'])
        tags = publishers[0]['with']['tags'].splitlines()
        self.assertEqual(len(tags), 2)
        self.assertTrue(tags[0].endswith('/web:${{ github.sha }}'))
        self.assertTrue(tags[1].endswith('/dev-web:${{ github.sha }}'))


if __name__ == '__main__':
    unittest.main()
