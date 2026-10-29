"""Fixed nginx OCI content collector; the parent supplies all authority.

No cluster, node, runtime, build provenance, or approval is established here.
"""
import hashlib
import http.client
import json
import re
import ssl
import time
import urllib.parse

INDEX='sha256:0e52bd054126cdc040b1276c0af347aeb02a41b9cfe305d6094767ac30e97949'
MANIFEST='sha256:84ed8c0dd296c188bfc24ed506f46589e9285fb0b11cf3a04d91d524d3067e11'
CONFIG='sha256:88f52f3d58a505b7e1ad96ca6969478517bcccb58418e75a54223271a64da7df'
REPOSITORY='docker.io/library/nginx'
SOURCE_REF='nginx:stable-alpine'
INDEX_TYPES={'application/vnd.oci.image.index.v1+json','application/vnd.docker.distribution.manifest.list.v2+json'}
MANIFEST_TYPES={'application/vnd.oci.image.manifest.v1+json','application/vnd.docker.distribution.manifest.v2+json'}
CONFIG_TYPES={'application/vnd.oci.image.config.v1+json','application/vnd.docker.container.image.v1+json'}
LAYER_TYPES={'application/vnd.oci.image.layer.v1.tar','application/vnd.oci.image.layer.v1.tar+gzip',
             'application/vnd.oci.image.layer.v1.tar+zstd','application/vnd.oci.image.layer.nondistributable.v1.tar',
             'application/vnd.oci.image.layer.nondistributable.v1.tar+gzip','application/vnd.oci.image.layer.nondistributable.v1.tar+zstd',
             'application/vnd.docker.image.rootfs.diff.tar','application/vnd.docker.image.rootfs.diff.tar.gzip',
             'application/vnd.docker.image.rootfs.foreign.diff.tar.gzip'}
ACCEPT=', '.join(sorted(INDEX_TYPES|MANIFEST_TYPES))
LIMIT=2*1024*1024
DEADLINE=45


class ImageError(RuntimeError): pass


def _pairs(pairs):
    result={}
    for key,value in pairs:
        if key in result: raise ValueError()
        result[key]=value
    return result


def _decode(raw,expected):
    if (type(raw) is not bytes or not 0<len(raw)<=LIMIT or 'sha256:'+hashlib.sha256(raw).hexdigest()!=expected): raise ValueError()
    def invalid_constant(value): raise ValueError()
    value=json.loads(raw,object_pairs_hook=_pairs,parse_constant=invalid_constant)
    if type(value) is not dict: raise ValueError()
    return value


def _descriptor(item,types):
    if (type(item) is not dict or not isinstance(item.get('digest'),str)
        or not re.fullmatch(r'sha256:[a-f0-9]{64}',item['digest']) or type(item.get('size')) is not int
        or not 0<item['size']<=LIMIT or item.get('mediaType') not in types): raise ValueError()
    return item


def validate(index,manifest,config):
    """Authenticate only the reviewed immutable index, amd64 child and config."""
    try:
        root=_decode(index,INDEX)
        if type(root.get('schemaVersion')) is not int or root['schemaVersion']!=2 or root.get('mediaType') not in INDEX_TYPES \
           or type(root.get('manifests')) is not list: raise ValueError()
        selected=[x for x in root['manifests'] if type(x) is dict and type(x.get('platform')) is dict
                  and x['platform'].get('os')=='linux' and x['platform'].get('architecture')=='amd64'
                  and not x['platform'].get('variant')]
        if len(selected)!=1: raise ValueError()
        desc=_descriptor(selected[0],MANIFEST_TYPES)
        if desc['digest']!=MANIFEST or desc['size']!=len(manifest): raise ValueError()
        child=_decode(manifest,MANIFEST)
        if (type(child.get('schemaVersion')) is not int or child['schemaVersion']!=2
            or child.get('mediaType') not in (None,desc['mediaType']) or type(child.get('layers')) is not list): raise ValueError()
        for layer in child['layers']:
            if (type(layer) is not dict or not isinstance(layer.get('digest'),str)
                or not re.fullmatch(r'sha256:[a-f0-9]{64}',layer['digest']) or type(layer.get('size')) is not int
                or layer['size']<0 or layer.get('mediaType') not in LAYER_TYPES): raise ValueError()
        config_desc=_descriptor(child.get('config'),CONFIG_TYPES)
        if config_desc['digest']!=CONFIG or config_desc['size']!=len(config): raise ValueError()
        data=_decode(config,CONFIG)
        if data.get('os')!='linux' or data.get('architecture')!='amd64' or data.get('variant'): raise ValueError()
        return {'source_ref':SOURCE_REF,'repository':REPOSITORY,'index_sha256':INDEX[7:],
                'manifest_sha256':MANIFEST[7:],'config_sha256':CONFIG[7:],
                'platform':{'os':'linux','architecture':'amd64','variant':None},'image_ref':REPOSITORY+'@'+MANIFEST}
    except Exception:
        raise ImageError('nginx OCI content qualification failed') from None


def _redirect(location):
    try:
        parsed=urllib.parse.urlsplit(location)
        if (parsed.scheme!='https' or parsed.hostname!='production.cloudfront.docker.com' or parsed.port not in (None,443)
            or parsed.username is not None or parsed.password is not None or parsed.fragment or not parsed.path.startswith('/')): raise ValueError()
        return parsed.hostname,parsed.path+('?' + parsed.query if parsed.query else '')
    except Exception:
        raise ImageError('Docker Hub blob redirect refused') from None


def _get(host,path,headers,deadline,limit):
    remaining=deadline-time.monotonic()
    if remaining<=0: raise ValueError()
    connection=http.client.HTTPSConnection(host,timeout=min(10,remaining),context=ssl.create_default_context())
    try:
        connection.request('GET',path,headers=headers)
        response=connection.getresponse(); data=bytearray()
        if response.status in (301,302,303,307,308):
            location=response.getheader('Location')
            if not location or len(location)>8192: raise ValueError()
            return response.status,location,b''
        while True:
            remaining=deadline-time.monotonic()
            if remaining<=0: raise ValueError()
            if connection.sock is not None: connection.sock.settimeout(min(10,remaining))
            chunk=response.read(min(65536,limit+1-len(data)))
            if not chunk: break
            data.extend(chunk)
            if len(data)>limit: raise ValueError()
        return response.status,None,bytes(data)
    finally: connection.close()


def collect():
    """Fetch only immutable reviewed content from Docker Hub within one deadline."""
    try:
        deadline=time.monotonic()+DEADLINE
        status,location,raw=_get('auth.docker.io','/token?service=registry.docker.io&scope=repository%3Alibrary%2Fnginx%3Apull',{},deadline,32768)
        if status!=200 or location is not None: raise ValueError()
        auth=json.loads(raw,object_pairs_hook=_pairs); token=auth.get('token')
        if not isinstance(token,str) or not re.fullmatch(r'[A-Za-z0-9._-]{1,16384}',token): raise ValueError()
        headers={'Authorization':'Bearer '+token,'Accept':ACCEPT}
        def registry(path,redirect=False):
            status,location,body=_get('registry-1.docker.io','/v2/library/nginx/'+path,headers,deadline,LIMIT)
            if location is not None:
                if not redirect or status not in (301,302,303,307,308): raise ValueError()
                host,target=_redirect(location)
                status,location,body=_get(host,target,{},deadline,LIMIT)
            if status!=200 or location is not None: raise ValueError()
            return body
        index=registry('manifests/'+INDEX); manifest=registry('manifests/'+MANIFEST)
        config=registry('blobs/'+CONFIG,True)
        validate(index,manifest,config)
        return {'index':index,'manifest':manifest,'config':config}
    except Exception:
        raise ImageError('Docker Hub nginx metadata unavailable') from None
