"""Verify that the trusted helper image's complete Docker COPY closure is pinned."""
from __future__ import annotations

import argparse
import glob
import hashlib
import json
import re
import shlex
import stat
from pathlib import Path


SCHEMA = 'voice-staging-native-phase-image-inputs-v1'
MANIFEST = Path('scripts/staging/nats-rollout-preservation/phase-image-source-manifest.json')
DOCKERFILE = Path('scripts/staging/nats-rollout-preservation/Dockerfile')
HEX = re.compile(r'[a-f0-9]{64}')


class ManifestError(ValueError):
    pass


def _fail(code: str) -> None:
    raise ManifestError('phase_image_manifest_' + code)


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def _copy_sources(root: Path, dockerfile: Path) -> set[str]:
    sources: set[str] = set()
    for line in dockerfile.read_text(encoding='utf-8').splitlines():
        stripped = line.strip()
        if not stripped.startswith('COPY '):
            continue
        tokens = shlex.split(stripped)
        if len(tokens) < 3 or tokens[0] != 'COPY':
            _fail('dockerfile_copy_invalid')
        if any(token.startswith('--from=') for token in tokens[1:-1]):
            continue
        for pattern in tokens[1:-1]:
            if pattern.startswith('--') or pattern.startswith('/') or '..' in Path(pattern).parts:
                _fail('dockerfile_source_invalid')
            matches = glob.glob(str(root / pattern), recursive=True)
            if not matches:
                _fail('dockerfile_source_missing')
            for item in matches:
                path = Path(item)
                try:
                    resolved = path.resolve(strict=True)
                    relative = resolved.relative_to(root).as_posix()
                except (OSError, ValueError):
                    _fail('dockerfile_source_escape')
                row = path.lstat()
                if not stat.S_ISREG(row.st_mode) or path.is_symlink():
                    _fail('dockerfile_source_not_regular')
                sources.add(relative)
    if not sources:
        _fail('dockerfile_copy_empty')
    return sources


def verify(root: Path, manifest_path: Path = MANIFEST) -> dict:
    root = root.resolve(strict=True)
    manifest_file = root / manifest_path
    dockerfile = root / DOCKERFILE
    try:
        value = json.loads(manifest_file.read_text(encoding='utf-8'))
    except (OSError, ValueError, UnicodeError):
        _fail('json_invalid')
    if not isinstance(value, dict) or set(value) != {'schema', 'image', 'baseImages', 'inputs', 'qualification'}:
        _fail('shape_invalid')
    if value.get('schema') != SCHEMA:
        _fail('schema_invalid')
    image = value.get('image')
    if (not isinstance(image, dict) or set(image) != {'localImageId', 'registryPullDigest', 'platform', 'user', 'entrypoint'}
        or (image.get('localImageId') is not None and
            (not isinstance(image.get('localImageId'), str)
             or not re.fullmatch(r'sha256:[a-f0-9]{64}', image['localImageId'])))
        or image.get('registryPullDigest') is not None
        or image.get('platform') != 'linux/amd64' or image.get('user') != '10000:10000'
        or image.get('entrypoint') != ['/usr/bin/python3', '/voice-nats-runner/phase']):
        _fail('image_identity_invalid')
    bases = value.get('baseImages')
    if (not isinstance(bases, list) or len(bases) != 2
        or any(not isinstance(row, str) or '@sha256:' not in row or not HEX.fullmatch(row.rsplit('@sha256:', 1)[1]) for row in bases)):
        _fail('base_images_invalid')
    if not isinstance(value.get('qualification'), str) or not value['qualification']:
        _fail('qualification_invalid')
    inputs = value.get('inputs')
    if (not isinstance(inputs, dict) or not inputs
        or any(not isinstance(path, str) or path.startswith('/') or '..' in Path(path).parts
            or not isinstance(digest, str) or not HEX.fullmatch(digest) for path, digest in inputs.items())):
        _fail('input_hashes_invalid')
    expected = _copy_sources(root, dockerfile) | {DOCKERFILE.as_posix()}
    if set(inputs) != expected:
        _fail('input_closure_mismatch')
    for relative in sorted(expected):
        path = root / relative
        if path.is_symlink() or not path.is_file() or _sha256(path) != inputs[relative]:
            _fail('input_hash_mismatch')
    closure = hashlib.sha256()
    for relative in sorted(expected):
        closure.update(relative.encode('utf-8') + b'\0' + inputs[relative].encode('ascii') + b'\n')
    return {'schema': 'voice-staging-native-phase-closure-receipt-v1',
        'input_count': len(expected), 'source_closure_sha256': closure.hexdigest(),
        'image_id': image['localImageId'], 'registry_pull_digest': image['registryPullDigest']}


def main(argv=None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[3])
    args = parser.parse_args(argv)
    print(json.dumps(verify(args.root), sort_keys=True, separators=(',', ':')))
    return 0


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except ManifestError:
        raise SystemExit('phase_image_manifest_invalid')
