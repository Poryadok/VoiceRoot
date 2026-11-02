"""Validate the explicit inputs for the owned staging-shard worker.

Path validation is not authentication: the worker must verify each pinned
asset's bytes and maintain caller custody throughout execution.
"""
import json
from pathlib import Path
import re

KUBECTL_HASH = 'cfda68cba5848bc3b6c6135ae2f20ba2c78de20059f68789c090166d6abc3e2c'
PHASE0_JDK_HASH = '987387933b64b9833846dee373b640440d3e1fd48a04804ec01a6dbf718e8ab8'
PHASE0_COMPOSE_HASH = '7af95166a730b87e172d4fc9aefea8725d3c6c7327d59149267b452114ddb7d4'
ASSET_NAMES = frozenset({
    'helper', 'jq', 'postgres_image', 'go_image', 'python_image',
    'nats_image', 'nats_box_image', 'alpine_image', 'packages',
    'wheel', 'preservation_go_cache', 'staging_go_proxy',
})
POLICY_ASSET_NAMES = ASSET_NAMES | {'kubectl'}
RUNTIME_ASSET_NAMES = ASSET_NAMES | {'phase0_jdk', 'phase0_compose'}
AGGREGATE_ASSET_NAMES = RUNTIME_ASSET_NAMES | {'kubectl'}
REQUEST_NAMES = frozenset({'root', 'output', 'commit', 'tree', 'base', 'shard', 'assets'})


def validate_runtime_images(images):
    if not isinstance(images, dict) or set(images) != {'postgres', 'docker_cli', 'python'}:
        raise ValueError('invalid runtime image inventory')
    for row in images.values():
        if (not isinstance(row, dict) or set(row) != {'archive', 'sha256', 'image_id'} or
                not isinstance(row['archive'], str) or not Path(row['archive']).is_absolute() or
                not isinstance(row['sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', row['sha256']) or
                not isinstance(row['image_id'], str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', row['image_id'])):
            raise ValueError('invalid runtime image input')
    if images['postgres']['image_id'] != 'sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b':
        raise ValueError('runtime PostgreSQL image differs')
    return images


def validate(request):
    if not isinstance(request, dict):
        raise ValueError('invalid staging request fields')
    shard = request.get('shard')
    if shard not in ('staging', 'proofs', 'runtime', 'policy'):
        raise ValueError('unsupported CI script shard')
    expected = REQUEST_NAMES | ({'runtime_images'} if shard == 'runtime' else set())
    if set(request) != expected:
        raise ValueError('invalid staging request fields')
    for name in ('commit', 'tree', 'base'):
        if not isinstance(request[name], str) or not re.fullmatch(r'[0-9a-f]{40}', request[name]):
            raise ValueError('invalid staging Git identity')
    assets = request['assets']
    if (not isinstance(assets, dict) or set(assets) not in (ASSET_NAMES, POLICY_ASSET_NAMES, RUNTIME_ASSET_NAMES) or
            (shard == 'policy' and set(assets) != POLICY_ASSET_NAMES) or
            (shard == 'runtime' and set(assets) != RUNTIME_ASSET_NAMES) or
            (shard != 'runtime' and set(assets) == RUNTIME_ASSET_NAMES)):
        raise ValueError('invalid staging asset inventory')
    if shard == 'runtime':
        validate_runtime_images(request['runtime_images'])
    for value in (request['root'], request['output'], *assets.values()):
        if not isinstance(value, str) or not value or not Path(value).is_absolute():
            raise ValueError('staging paths must be absolute')
    root = Path(request['root']).resolve(strict=True)
    output = Path(request['output']).resolve()
    if not root.is_dir() or output == root or root in output.parents or output.exists():
        raise ValueError('staging output must be new and outside the source')
    return request


def _json(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError('duplicate staging request key')
            result[key] = value
        return result
    if len(raw) > 1024 * 1024:
        raise ValueError('staging request is too large')
    return json.loads(raw, object_pairs_hook=unique)


def parse(raw):
    return validate(_json(raw))


def parse_assets(raw):
    assets = _json(raw)
    if not isinstance(assets, dict) or set(assets) not in (ASSET_NAMES, POLICY_ASSET_NAMES, RUNTIME_ASSET_NAMES, AGGREGATE_ASSET_NAMES):
        raise ValueError('invalid staging asset inventory')
    if any(not isinstance(value, str) or not value or not Path(value).is_absolute()
           for value in assets.values()):
        raise ValueError('staging asset paths must be absolute')
    return assets


def parse_runtime_images(raw):
    return validate_runtime_images(_json(raw))
