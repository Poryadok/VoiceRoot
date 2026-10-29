"""Resolve the admitted full application package to Linux amd64 OCI manifests.

The caller must authenticate the canonical package and qualify scheduler/node
architecture before using this map. This performs registry reads only; it does
not prove that an image ran, or bind a caller-supplied package to a build.
"""
import copy
import json
import re
import canonical_application_package as package
import publish_candidate as source
import update_images


class OCIError(RuntimeError):
    pass


INDEX_TYPES = {'application/vnd.oci.image.index.v1+json',
               'application/vnd.docker.distribution.manifest.list.v2+json'}
MANIFEST_TYPES = {'application/vnd.oci.image.manifest.v1+json',
                  'application/vnd.docker.distribution.manifest.v2+json'}
CONFIG_TYPES = {'application/vnd.oci.image.config.v1+json',
                'application/vnd.docker.container.image.v1+json'}
LAYER_TYPES = {'application/vnd.oci.image.layer.v1.tar', 'application/vnd.oci.image.layer.v1.tar+gzip',
               'application/vnd.oci.image.layer.v1.tar+zstd', 'application/vnd.oci.image.layer.nondistributable.v1.tar',
               'application/vnd.oci.image.layer.nondistributable.v1.tar+gzip',
               'application/vnd.oci.image.layer.nondistributable.v1.tar+zstd',
               'application/vnd.docker.image.rootfs.diff.tar', 'application/vnd.docker.image.rootfs.diff.tar.gzip',
               'application/vnd.docker.image.rootfs.foreign.diff.tar.gzip'}


def _descriptor(item, allowed=None):
    if (not isinstance(item,dict) or not isinstance(item.get('digest'),str) or
            not re.fullmatch(r'sha256:[0-9a-f]{64}', item['digest']) or
            type(item.get('size')) is not int or item['size'] < 0 or
            not isinstance(item.get('mediaType'),str) or not item['mediaType'] or
            (allowed is not None and item['mediaType'] not in allowed)):
        raise ValueError('invalid OCI descriptor')
    return item


def _decode(raw, digest):
    source.decode_manifest(raw, digest)
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result: raise ValueError('duplicate metadata key')
            result[key] = value
        return result
    value = json.loads(raw, object_pairs_hook=unique)
    if not isinstance(value, dict): raise ValueError('metadata object required')
    return value


def resolve(images, registry):
    """Return image-phase keys to verified platform manifest digests."""
    try:
        images = copy.deepcopy(images)
        package.validate_images(images)
        if set(update_images.SERVICES) != set(package.APPLICATIONS):
            raise ValueError('catalog mismatch')
        result = {}
        for name in package.APPLICATIONS:
            reference = images[name]
            top = reference.rsplit('@', 1)[1]
            repository = 'poryadok/voiceroot/' + name
            index = _decode(registry.get(repository, 'manifests/' + top), top)
            if (type(index.get('schemaVersion')) is not int or index['schemaVersion'] != 2 or
                    index.get('mediaType') not in INDEX_TYPES | {None}): raise ValueError('index schema')
            child_digest = source.platform_manifest(index)
            descriptor = _descriptor(next(item for item in index['manifests'] if item.get('digest') == child_digest and
                item.get('platform', {}).get('os') == 'linux' and item.get('platform', {}).get('architecture') == 'amd64' and
                not item.get('platform', {}).get('variant')), MANIFEST_TYPES)
            child_raw = registry.get(repository, 'manifests/' + child_digest)
            if len(child_raw) != descriptor['size']: raise ValueError('manifest size differs')
            child = _decode(child_raw, child_digest)
            if type(child.get('schemaVersion')) is not int or child['schemaVersion'] != 2 or not isinstance(child.get('config'), dict):
                raise ValueError('platform manifest schema')
            if child.get('mediaType') not in (None, descriptor['mediaType']) or not isinstance(child.get('layers'), list):
                raise ValueError('platform manifest type or layers')
            for layer in child['layers']: _descriptor(layer, LAYER_TYPES)
            config_descriptor = _descriptor(child['config'], CONFIG_TYPES)
            config_digest = config_descriptor['digest']
            config_raw = registry.get(repository, 'blobs/' + config_digest)
            if len(config_raw) != config_descriptor['size']: raise ValueError('config size differs')
            config = _decode(config_raw, config_digest)
            if config.get('os') != 'linux' or config.get('architecture') != 'amd64' or config.get('variant'):
                raise ValueError('image config platform differs')
            result['voice-' + name + '/' + update_images.SERVICES[name]] = child_digest
        if len(result) != 23: raise ValueError('application catalog incomplete')
        return result
    except Exception:
        raise OCIError('qualified application platform metadata unavailable') from None
