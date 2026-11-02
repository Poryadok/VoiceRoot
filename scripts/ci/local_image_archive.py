"""Verify and canonicalize a single Docker-save image without extraction.

Caller-supplied identities and exclusive daemon access are separate obligations.
Inspection alone never authorizes loading the original archive.
"""
import hashlib
import json
import re
import tarfile


class _HashingReader:
    def __init__(self, source):
        self.source, self.digest, self.total, self.tail = source, hashlib.sha256(), 0, b''

    def read(self, size=-1):
        data = self.source.read(size)
        self.digest.update(data)
        self.total += len(data)
        self.tail = (self.tail + data)[-131072:]
        return data


def _json(data, name):
    def pairs(values):
        result = {}
        for key, value in values:
            if key in result:
                raise ValueError('duplicate JSON key')
            result[key] = value
        return result
    try:
        return json.loads(data.decode('utf-8'), object_pairs_hook=pairs,
                          parse_constant=lambda value: (_ for _ in ()).throw(ValueError('invalid JSON constant')))
    except (AttributeError, UnicodeError, ValueError, TypeError):
        raise ValueError('invalid image archive ' + name) from None


def _inspect_archive(stream, archive_sha256, image_id, image_tag):
    if (not isinstance(archive_sha256, str) or not re.fullmatch(r'[0-9a-f]{64}', archive_sha256)
            or not isinstance(image_id, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id)
            or (image_tag is not None and (not isinstance(image_tag, str) or
                not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._:/-]*:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}', image_tag)))):
        raise ValueError('invalid expected image identity')
    reader = _HashingReader(stream)
    entries, metadata, count, buffered = {}, {}, 0, 0
    try:
        with tarfile.open(fileobj=reader, mode='r|', bufsize=65536) as archive:
            for item in archive:
                count += 1
                parts = item.name.split('/')
                if (count > 10000 or item.name in entries or item.name.startswith('/') or
                        any(part in ('', '.', '..') or '\\' in part or ':' in part for part in parts)):
                    raise ValueError('unsafe or duplicate image archive entry')
                if item.isdir():
                    if item.name not in ('blobs', 'blobs/sha256') and not re.fullmatch(r'[0-9a-f]{64}', item.name):
                        raise ValueError('unsupported image archive directory')
                    entries[item.name] = None
                    continue
                if item.type not in (tarfile.REGTYPE, tarfile.AREGTYPE) or item.size < 0:
                    raise ValueError('unsupported image archive entry')
                if (item.name not in ('manifest.json', 'index.json', 'repositories', 'oci-layout') and
                        not re.fullmatch(r'(?:[0-9a-f]{64}\.json|[0-9a-f]{64}/(?:VERSION|json|layer\.tar)|blobs/sha256/[0-9a-f]{64})', item.name)):
                    raise ValueError('unrecognized image archive entry')
                value = archive.extractfile(item)
                if value is None:
                    raise ValueError('image archive entry is unavailable')
                digest = hashlib.sha256()
                keep = item.name in ('manifest.json', 'index.json', 'repositories', 'oci-layout') or item.name.endswith('.json')
                keep = keep or (item.name.startswith('blobs/sha256/') and item.size <= 1024 * 1024)
                if keep and (item.size > 4 * 1024 * 1024 or buffered + item.size > 32 * 1024 * 1024):
                    raise ValueError('image archive metadata exceeds bounds')
                chunks = []
                for block in iter(lambda: value.read(1024 * 1024), b''):
                    digest.update(block)
                    if keep:
                        chunks.append(block)
                entries[item.name] = (digest.hexdigest(), item.size)
                if item.name.startswith('blobs/sha256/') and item.name.rsplit('/', 1)[1] != digest.hexdigest():
                    raise ValueError('image blob digest differs')
                if keep:
                    metadata[item.name] = b''.join(chunks)
                    buffered += item.size
            end = archive.offset
        buffered_trailing = reader.total - end
        if buffered_trailing < 512 or buffered_trailing > len(reader.tail) or any(reader.tail[-buffered_trailing:]):
            raise ValueError('invalid image archive terminator')
        trailing = buffered_trailing
        while True:
            block = reader.read(1024 * 1024)
            if not block:
                break
            trailing += len(block)
            if any(block):
                raise ValueError('nonzero trailing image archive data')
        if trailing < 1024:
            raise ValueError('incomplete image archive terminator')
    except (tarfile.TarError, OSError, EOFError, TypeError):
        raise ValueError('invalid image archive stream') from None
    if reader.digest.hexdigest() != archive_sha256:
        raise ValueError('image archive digest differs')
    manifest = _json(metadata.get('manifest.json'), 'manifest')
    if not isinstance(manifest, list) or len(manifest) != 1 or not isinstance(manifest[0], dict):
        raise ValueError('image archive must contain exactly one image')
    image = manifest[0]
    config_name, layers = image.get('Config'), image.get('Layers')
    if (not set(image) <= {'Config', 'RepoTags', 'Layers', 'LayerSources'} or
            not isinstance(config_name, str) or config_name not in entries or config_name not in metadata or
            'RepoTags' not in image or image['RepoTags'] != (None if image_tag is None else [image_tag]) or not isinstance(layers, list) or
            any(not isinstance(name, str) or name not in entries or entries[name] is None for name in layers) or
            len(set(layers)) != len(layers)):
        raise ValueError('image archive manifest differs')
    if entries[config_name] is None or 'sha256:' + entries[config_name][0] != image_id:
        raise ValueError('image configuration identity differs')
    config = _json(metadata[config_name], 'configuration')
    if (not isinstance(config, dict) or config.get('os') != 'linux' or
            not isinstance(config.get('architecture'), str) or not config['architecture'] or
            not isinstance(config.get('rootfs'), dict) or config['rootfs'].get('type') != 'layers' or
            config['rootfs'].get('diff_ids') != ['sha256:' + entries[name][0] for name in layers]):
        raise ValueError('image layer identities differ')
    if 'repositories' in metadata:
        if image_tag is None:
            raise ValueError('untagged image has repository tags')
        repositories = _json(metadata['repositories'], 'repositories')
        repository, tag = image_tag.rsplit(':', 1)
        if (not layers or not isinstance(repositories, dict) or set(repositories) != {repository} or
                not isinstance(repositories[repository], dict) or set(repositories[repository]) != {tag}):
            raise ValueError('image repository tags differ')
        top = layers[-1].split('/')[-1] if layers[-1].startswith('blobs/sha256/') else layers[-1].split('/')[0]
        if repositories[repository][tag] != top:
            raise ValueError('image repository layer differs')
    selected = None
    if 'index.json' in metadata:
        index = _json(metadata['index.json'], 'index')
        manifests = index.get('manifests') if isinstance(index, dict) else None
        if (not isinstance(index, dict) or set(index) != {'schemaVersion', 'mediaType', 'manifests'} or index.get('schemaVersion') != 2 or
                index.get('mediaType') != 'application/vnd.oci.image.index.v1+json' or
                not isinstance(manifests, list) or len(manifests) != 1 or not isinstance(manifests[0], dict)):
            raise ValueError('image index must contain exactly one manifest')
        descriptor = manifests[0]
        annotations = descriptor.get('annotations')
        if image_tag is None:
            valid_reference = set(descriptor) == {'mediaType', 'digest', 'size'}
        else:
            repository, tag = image_tag.rsplit(':', 1)
            first = repository.split('/', 1)[0]
            if '.' not in first and ':' not in first and first != 'localhost':
                repository = 'docker.io/' + (('library/' if '/' not in repository else '') + repository)
            valid_reference = (set(descriptor) == {'mediaType', 'digest', 'size', 'annotations'} and
                isinstance(annotations, dict) and set(annotations) == {'io.containerd.image.name', 'org.opencontainers.image.ref.name'} and
                annotations['io.containerd.image.name'] == repository + ':' + tag and
                annotations['org.opencontainers.image.ref.name'] == tag)
        if (not valid_reference or descriptor.get('mediaType') != 'application/vnd.oci.image.manifest.v1+json'):
            raise ValueError('image index tag differs')
        manifest_digest = descriptor.get('digest')
        if not isinstance(manifest_digest, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', manifest_digest):
            raise ValueError('invalid image index digest')
        path = 'blobs/sha256/' + manifest_digest.removeprefix('sha256:')
        if (path not in metadata or entries.get(path) is None or 'sha256:' + entries[path][0] != manifest_digest or
                type(descriptor.get('size')) is not int or descriptor['size'] != entries[path][1]):
            raise ValueError('image index manifest differs')
        oci = _json(metadata[path], 'OCI manifest')
        if (not isinstance(oci, dict) or set(oci) != {'schemaVersion', 'mediaType', 'config', 'layers'} or oci.get('schemaVersion') != 2 or
                oci.get('mediaType') != 'application/vnd.oci.image.manifest.v1+json' or
                not isinstance(oci.get('config'), dict) or set(oci['config']) != {'mediaType', 'digest', 'size'} or
                oci['config'].get('mediaType') != 'application/vnd.oci.image.config.v1+json' or
                oci['config'].get('digest') != image_id or type(oci['config'].get('size')) is not int or
                oci['config'].get('size') != entries[config_name][1] or
                config_name != 'blobs/sha256/' + image_id.removeprefix('sha256:') or
                not isinstance(oci.get('layers'), list) or
                [entry.get('digest') if isinstance(entry, dict) else None for entry in oci['layers']] !=
                ['sha256:' + entries[name][0] for name in layers]):
            raise ValueError('OCI manifest identity differs')
        for layer, name in zip(oci['layers'], layers):
            if (set(layer) != {'mediaType', 'digest', 'size'} or
                    layer.get('mediaType') != 'application/vnd.oci.image.layer.v1.tar' or
                    type(layer.get('size')) is not int or layer['size'] != entries[name][1] or
                    name != 'blobs/sha256/' + entries[name][0]):
                raise ValueError('OCI layer descriptor differs')
        if 'LayerSources' in image and image['LayerSources'] != {entry['digest']: entry for entry in oci['layers']}:
            raise ValueError('legacy layer sources differ from OCI descriptors')
        if _json(metadata.get('oci-layout'), 'layout') != {'imageLayoutVersion': '1.0.0'}:
            raise ValueError('OCI layout differs')
        selected = {'manifest.json', 'index.json', 'oci-layout', config_name, path, *layers}
        if 'repositories' in metadata:
            selected.add('repositories')
    elif 'oci-layout' in metadata or image_tag is None:
        raise ValueError('untagged image requires an OCI index')
    return ({'archive_sha256': archive_sha256, 'image_id': image_id, 'tag': image_tag,
             'platform': config['os'] + '/' + config['architecture']}, selected, entries)


def verify_archive(stream, archive_sha256, image_id, image_tag):
    """Inspect metadata only; do not load the original archive."""
    return _inspect_archive(stream, archive_sha256, image_id, image_tag)[0]


class _HashingWriter:
    def __init__(self, output):
        self.output, self.digest = output, hashlib.sha256()

    def write(self, data):
        written = self.output.write(data)
        if written != len(data):
            raise OSError('short canonical image archive write')
        self.digest.update(data)
        return written


def sanitize_archive(stream, output, archive_sha256, image_id, image_tag):
    """Emit verified OCI entries using USTAR; consume output only after success."""
    if stream is output:
        raise ValueError('image source and output must differ')
    try:
        if output.seek(0, 2) != 0:
            raise ValueError('canonical image output must be empty')
        output.seek(0)
        stream.seek(0)
        summary, selected, entries = _inspect_archive(stream, archive_sha256, image_id, image_tag)
        if selected is None:
            raise ValueError('canonicalization requires the supported OCI image format')
        stream.seek(0)
        reader, writer = _HashingReader(stream), _HashingWriter(output)
        copied = set()
        with tarfile.open(fileobj=reader, mode='r|', bufsize=65536) as source, \
                tarfile.open(fileobj=writer, mode='w|', format=tarfile.USTAR_FORMAT) as target:
            for item in source:
                if item.name not in selected:
                    continue
                if item.name in copied or not item.isfile() or entries[item.name] is None or item.size != entries[item.name][1]:
                    raise ValueError('canonical image entry differs')
                content = source.extractfile(item)
                if content is None:
                    raise ValueError('canonical image entry is unavailable')
                canonical = tarfile.TarInfo(item.name)
                canonical.size, canonical.mode = item.size, 0o644
                target.addfile(canonical, content)
                copied.add(item.name)
        while reader.read(1024 * 1024):
            pass
        if copied != selected or reader.digest.hexdigest() != archive_sha256:
            raise ValueError('image archive changed during canonicalization')
        output.flush()
        return dict(summary, source_archive_sha256=archive_sha256, archive_sha256=writer.digest.hexdigest())
    except (OSError, EOFError, TypeError, tarfile.TarError):
        raise ValueError('image canonicalization failed') from None
