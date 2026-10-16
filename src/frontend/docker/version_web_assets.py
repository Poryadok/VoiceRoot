"""Bind fresh HTML to exact Flutter bootstrap/main bytes after flutter build web."""
import hashlib
import json
from pathlib import Path
import re
import sys


def version(root: Path) -> dict:
    main = (root / 'main.dart.js').read_bytes()
    bootstrap = (root / 'flutter_bootstrap.js').read_text(encoding='utf8')
    index = (root / 'index.html').read_text(encoding='utf8')
    # Flutter 3.41's worker only unregisters obsolete caches. A future/older
    # caching worker needs its own resource-map integration, never silent reuse.
    worker = root / 'flutter_service_worker.js'
    if worker.exists() and re.search(r'\b(?:RESOURCES|CORE)\b|caches\.|["\']fetch["\']',
                                    worker.read_text(encoding='utf8')):
        raise ValueError('unsupported_caching_service_worker')
    assignments = list(re.finditer(r'_flutter\.buildConfig\s*=\s*', bootstrap))
    if len(assignments) != 1:
        raise ValueError('expected_one_build_config')
    start = assignments[0].end()
    config, size = json.JSONDecoder().raw_decode(bootstrap[start:])
    if not isinstance(config, dict) or not isinstance(config.get('builds'), list):
        raise ValueError('invalid_build_config')
    builds = config['builds']
    if (len(builds) not in (1, 2) or (len(builds) == 2 and builds[1] != {})
            or not isinstance(builds[0], dict)
            or builds[0].get('compileTarget') != 'dart2js'
            or builds[0].get('mainJsPath') != 'main.dart.js'):
        raise ValueError('unsupported_main_entrypoint')
    if not bootstrap[start + size:].lstrip().startswith(';'):
        raise ValueError('invalid_build_config_suffix')
    references = list(re.finditer(r'(<script\b[^>]*\bsrc=["\'])flutter_bootstrap\.js(["\'])', index))
    if len(references) != 1:
        raise ValueError('expected_one_bootstrap_script')
    main_sha = hashlib.sha256(main).hexdigest()
    main_name = f'main.{main_sha}.dart.js'
    builds[0]['mainJsPath'] = main_name
    bound_bootstrap = (bootstrap[:start] + json.dumps(config, separators=(',', ':'))
                       + bootstrap[start + size:]).encode('utf8')
    bootstrap_sha = hashlib.sha256(bound_bootstrap).hexdigest()
    bootstrap_name = f'flutter_bootstrap.{bootstrap_sha}.js'
    bound_index = re.sub(
        r'(<script\b[^>]*\bsrc=["\'])flutter_bootstrap\.js(["\'])',
        lambda match: match[1] + bootstrap_name + match[2], index).encode('utf8')
    manifest = {
        'schema': 'voice-web-assets-v1',
        'bootstrap': {'path': bootstrap_name, 'sha256': bootstrap_sha, 'bytes': len(bound_bootstrap)},
        'main': {'path': main_name, 'sha256': main_sha, 'bytes': len(main)},
    }
    # Validate all shapes before writing. Legacy files remain available; new
    # files stay in the same directory to preserve loader-relative resources.
    for name, data in ((main_name, main), (bootstrap_name, bound_bootstrap),
                       ('web-asset-manifest.json', (json.dumps(manifest, sort_keys=True) + '\n').encode())):
        with (root / name).open('xb') as output:
            output.write(data)
    (root / 'index.html').write_bytes(bound_index)
    return manifest


if __name__ == '__main__':
    version(Path(sys.argv[1]))
