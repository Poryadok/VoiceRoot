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


def _image(name,reference,registry,source_sha=None):
    repository = 'poryadok/voiceroot/' + name
    top = reference.rsplit('@',1)[1]
    raw = registry.get(repository,'manifests/'+top); document = _decode(raw,top)
    if type(document.get('schemaVersion')) is not int or document['schemaVersion'] != 2: raise ValueError('image schema')
    requested_kind = 'index' if 'manifests' in document else 'manifest'
    requested_index = None
    if requested_kind == 'index':
        if document.get('mediaType') not in INDEX_TYPES | {None}: raise ValueError('index type')
        child_digest = source.platform_manifest(document)
        descriptor = _descriptor(next(item for item in document['manifests'] if item.get('digest') == child_digest and
            item.get('platform',{}).get('os') == 'linux' and item.get('platform',{}).get('architecture') == 'amd64' and
            not item.get('platform',{}).get('variant')),MANIFEST_TYPES)
        child_raw = registry.get(repository,'manifests/'+child_digest)
        if len(child_raw) != descriptor['size']: raise ValueError('manifest size differs')
        child = _decode(child_raw,child_digest); requested_index = top
        if child.get('mediaType') not in (None,descriptor['mediaType']): raise ValueError('manifest media type')
    else:
        child_digest = top; child_raw = raw; child = document
        if child.get('mediaType') not in MANIFEST_TYPES | {None}: raise ValueError('manifest media type')
    if (type(child.get('schemaVersion')) is not int or child['schemaVersion'] != 2 or
        type(child.get('layers')) is not list or 'manifests' in child): raise ValueError('manifest schema')
    for layer in child['layers']: _descriptor(layer,LAYER_TYPES)
    config_desc = _descriptor(child.get('config'),CONFIG_TYPES)
    config_raw = registry.get(repository,'blobs/'+config_desc['digest'])
    if len(config_raw) != config_desc['size']: raise ValueError('config size differs')
    config = _decode(config_raw,config_desc['digest'])
    if config.get('os') != 'linux' or config.get('architecture') != 'amd64' or config.get('variant'): raise ValueError('image platform')
    tag_digest = tag_kind = tag_index = None
    if source_sha is not None:
        labels = config.get('config',{}).get('Labels') or {}
        if (type(labels) is not dict or 'org.opencontainers.image.revision' in labels and labels['org.opencontainers.image.revision'] != source_sha
            or 'org.opencontainers.image.source' in labels and (not isinstance(labels['org.opencontainers.image.source'],str)
               or labels['org.opencontainers.image.source'].removesuffix('.git') != 'https://github.com/Poryadok/VoiceRoot')):
            raise ValueError('image source label')
        tag_raw = registry.get(repository,'manifests/'+source_sha)
        import hashlib
        tag_digest = 'sha256:'+hashlib.sha256(tag_raw).hexdigest(); tag = _decode(tag_raw,tag_digest)
        if type(tag.get('schemaVersion')) is not int or tag['schemaVersion'] != 2: raise ValueError('tag schema')
        tag_kind = 'index' if 'manifests' in tag else 'manifest'
        if tag_kind == 'index':
            if tag.get('mediaType') not in INDEX_TYPES | {None}: raise ValueError('tag index type')
            platform = source.platform_manifest(tag)
            descriptor = _descriptor(next(item for item in tag['manifests'] if item.get('digest') == platform and
                item.get('platform',{}).get('os') == 'linux' and item.get('platform',{}).get('architecture') == 'amd64' and
                not item.get('platform',{}).get('variant')),MANIFEST_TYPES)
            if platform != child_digest or descriptor['size'] != len(child_raw) or child.get('mediaType') not in (None,descriptor['mediaType']):
                raise ValueError('source tag child mismatch')
            tag_index=tag_digest
        elif tag_digest != child_digest or tag != child: raise ValueError('source tag manifest mismatch')
        if registry.get(repository,'manifests/'+source_sha) != tag_raw: raise ValueError('source tag changed')
    index_digest = tag_index if tag_index is not None else requested_index
    return child_digest, {'requested_digest':top,'requested_kind':requested_kind,'index_digest':index_digest,
        'index_source':'source_tag' if tag_index is not None else ('requested' if requested_index is not None else None),
        'platform_digest':child_digest,'config_digest':config_desc['digest'],'source_tag_digest':tag_digest,'source_tag_kind':tag_kind}


def _resolve(images, registry, source_sha):
    try:
        images = copy.deepcopy(images)
        package.validate_images(images)
        if set(update_images.SERVICES) != set(package.APPLICATIONS):
            raise ValueError('catalog mismatch')
        if source_sha is not None and (not isinstance(source_sha,str) or not re.fullmatch(r'[a-f0-9]{40}',source_sha)):
            raise ValueError('source identity')
        result = {}; provenance = {}
        for name in package.APPLICATIONS:
            child_digest,identity = _image(name,images[name],registry,source_sha)
            key='voice-'+name+'/'+update_images.SERVICES[name]
            result[key]=child_digest; provenance[key]=identity
        if len(result) != 23: raise ValueError('application catalog incomplete')
        return result,provenance
    except Exception:
        raise OCIError('qualified application platform metadata unavailable') from None


def resolve(images,registry):
    """Return image-phase keys to platform manifests, accepting an index or child."""
    return _resolve(images,registry,None)[0]


def resolve_provenance(images,registry,source_sha):
    """Also bind exact source tag and actual parent index when it exists."""
    return _resolve(images,registry,source_sha)


def validate_provenance(images,platforms,provenance):
    """Check the closed public receipt shape against the package's image map."""
    try:
        if (type(images) is not dict or type(platforms) is not dict or type(provenance) is not dict
            or set(images)!=set(platforms) or set(images)!=set(provenance)): raise ValueError()
        pattern=r'sha256:[a-f0-9]{64}'
        for key,value in provenance.items():
            if (type(value) is not dict or set(value)!={'requested_digest','requested_kind','index_digest','index_source',
                    'platform_digest','config_digest','source_tag_digest','source_tag_kind'}
                or value['requested_kind'] not in ('index','manifest') or value['source_tag_kind'] not in ('index','manifest')
                or value['requested_digest']!=images[key].rsplit('@',1)[1] or value['platform_digest']!=platforms[key]
                or any(not isinstance(value[k],str) or not re.fullmatch(pattern,value[k])
                       for k in ('requested_digest','platform_digest','config_digest','source_tag_digest'))): raise ValueError()
            if value['requested_kind']=='manifest' and value['requested_digest']!=value['platform_digest']: raise ValueError()
            if value['source_tag_kind']=='index':
                if value['index_source']!='source_tag' or value['index_digest']!=value['source_tag_digest']: raise ValueError()
            elif value['source_tag_digest']!=value['platform_digest']: raise ValueError()
            elif value['requested_kind']=='index':
                if value['index_source']!='requested' or value['index_digest']!=value['requested_digest']: raise ValueError()
            elif value['index_source'] is not None or value['index_digest'] is not None: raise ValueError()
        return True
    except Exception: raise OCIError('qualified image provenance invalid') from None
