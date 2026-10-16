"""Generated-output contract, runnable without Flutter or a web server."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

HERE = Path(__file__).resolve().parent


class WebAssetsTest(unittest.TestCase):
    def test_container_activates_versioning(self):
        dockerfile = (HERE.parent / 'Dockerfile').read_text()
        self.assertIn('python3 docker/version_web_assets.py build/web', dockerfile)

    def test_cache_policy_and_ci_activation(self):
        nginx = (HERE / 'nginx-default.conf').read_text()
        self.assertIn('add_header Cache-Control "no-store" always;', nginx)
        self.assertIn('try_files $uri =404;', nginx)
        self.assertIn('main\\.[0-9a-f]{64}\\.dart\\.js', nginx)
        self.assertIn('flutter_bootstrap\\.[0-9a-f]{64}\\.js', nginx)
        self.assertIn('add_header Cache-Control "public, max-age=31536000, immutable";', nginx)
        makefile = (HERE.parents[2] / 'Makefile').read_text()
        ci_target = makefile.split('\nci-script-tests:', 1)[1].split('\n\n', 1)[0]
        self.assertIn('src/frontend/docker/version_web_assets_test.py', ci_target)

    def generate(self, root, main=b'compiled app', config=None):
        root.mkdir()
        (root / 'main.dart.js').write_bytes(main)
        # Actual 188fb generated bootstrap (8071fe8d...) includes the empty
        # trailing build sentinel. Preserve it, rather than inventing one build.
        config = config or {'engineRevision': 'engine', 'builds': [
            {'compileTarget': 'dart2js', 'renderer': 'canvaskit', 'mainJsPath': 'main.dart.js'}, {}]}
        (root / 'flutter_bootstrap.js').write_text(
            '/* loader fallback main.dart.js */\n_flutter.buildConfig = ' +
            json.dumps(config) + ';\n_flutter.loader.load();\n')
        (root / 'index.html').write_text(
            '<base href="/"><script src="flutter_bootstrap.js" async></script>')
        (root / 'flutter_service_worker.js').write_text(
            "self.addEventListener('activate', () => self.registration.unregister());")
        (root / 'firebase-messaging-sw.js').write_text('/* existing messaging worker */')
        (root / 'main.dart.js.map').write_text('{"file":"main.dart.js"}')
        (root / 'main.dart.js_1.part.js').write_text('/* deferred chunk */')
        spec = importlib.util.spec_from_file_location('assets', HERE / 'version_web_assets.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_content_chain_and_preserved_relative_paths(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / 'web'
            module = self.generate(root)
            original = {p.name: p.read_bytes() for p in root.iterdir()}
            result = module.version(root)
            self.assertEqual(set(result), {'schema', 'bootstrap', 'main'})
            self.assertEqual(result['schema'], 'voice-web-assets-v1')
            for key in ('bootstrap', 'main'):
                row = result[key]
                data = (root / row['path']).read_bytes()
                self.assertEqual(hashlib.sha256(data).hexdigest(), row['sha256'])
                self.assertEqual(len(data), row['bytes'])
                self.assertIn(row['sha256'], row['path'])
                self.assertNotIn('/', row['path'])
            self.assertIn(result['bootstrap']['path'], (root / 'index.html').read_text())
            bootstrap = (root / result['bootstrap']['path']).read_text()
            self.assertIn(result['main']['path'], bootstrap)
            self.assertIn('/* loader fallback main.dart.js */', bootstrap)
            self.assertIn('"renderer":"canvaskit"', bootstrap)
            for name in original.keys() - {'index.html'}:
                self.assertEqual((root / name).read_bytes(), original[name])
            self.assertEqual(json.loads((root / 'web-asset-manifest.json').read_text()), result)

    def test_changed_main_changes_entire_reference_chain(self):
        with tempfile.TemporaryDirectory() as temp:
            a, b = Path(temp) / 'a', Path(temp) / 'b'
            module = self.generate(a, b'old')
            self.generate(b, b'new')
            first, second = module.version(a), module.version(b)
            for key in ('main', 'bootstrap'):
                self.assertNotEqual(first[key]['path'], second[key]['path'])
            self.assertNotEqual((a / 'index.html').read_bytes(), (b / 'index.html').read_bytes())

    def test_deterministic_output(self):
        with tempfile.TemporaryDirectory() as temp:
            a, b = Path(temp) / 'a', Path(temp) / 'b'
            module = self.generate(a)
            self.generate(b)
            self.assertEqual(module.version(a), module.version(b))
            self.assertEqual((a / 'index.html').read_bytes(), (b / 'index.html').read_bytes())

    def test_unexpected_generated_shapes_fail_before_writing(self):
        mutations = {
            'duplicate_config': lambda p: (p / 'flutter_bootstrap.js').write_text(
                (p / 'flutter_bootstrap.js').read_text() * 2),
            'wrong_main': lambda p: (p / 'flutter_bootstrap.js').write_text(
                (p / 'flutter_bootstrap.js').read_text().replace('"main.dart.js"', '"other.js"')),
            'duplicate_script': lambda p: (p / 'index.html').write_text(
                (p / 'index.html').read_text() * 2),
            'legacy_cache': lambda p: (p / 'flutter_service_worker.js').write_text('const RESOURCES = {};'),
            'malformed_json': lambda p: (p / 'flutter_bootstrap.js').write_text('_flutter.buildConfig = {broken};'),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temp:
                root = Path(temp) / 'web'
                module = self.generate(root)
                mutate(root)
                before = {p.name: p.read_bytes() for p in root.iterdir()}
                with self.assertRaises(ValueError):
                    module.version(root)
                self.assertEqual(before, {p.name: p.read_bytes() for p in root.iterdir()})


if __name__ == '__main__':
    unittest.main()
