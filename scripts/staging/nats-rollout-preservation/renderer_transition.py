"""Private descriptor for a single captured hub renderer image transition."""

import copy
import hashlib
import json
import re

SCHEMA = 'voice-nats-renderer-transition-v1'
HUB = 'voice-nats-pvc-candidate'
RENDERER = 'nats-config-renderer'
MAX_BYTES = 2 << 20
FIELDS = {'schema','source_sha','hub','original_template','target_template',
          'original_template_sha256','target_template_sha256','images','storage_binding'}

class TransitionError(ValueError):
    pass

def _fail():
    raise TransitionError('renderer_transition_rejected') from None

def _text(value, pattern=r'[a-zA-Z0-9_.-]{1,253}'):
    if type(value) is not str or not re.fullmatch(pattern,value):
        _fail()
    return value

def _json(value):
    stack=[(value,0)]; nodes=0
    while stack:
        item,depth=stack.pop();nodes+=1
        if depth>64 or nodes>65536:
            _fail()
        if type(item) is dict:
            if any(type(key) is not str for key in item):
                _fail()
            stack.extend((entry,depth+1) for entry in item.values())
        elif type(item) is list:
            stack.extend((entry,depth+1) for entry in item)
        elif type(item) is str:
            if len(item)>MAX_BYTES:
                _fail()
        elif item is not None and type(item) not in (int,float,bool):
            _fail()
    try:
        raw=json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False).encode('utf-8')
    except (TypeError,ValueError,RecursionError,UnicodeError):
        _fail()
    if len(raw)>MAX_BYTES:
        _fail()
    return raw

def _hash(value):
    return hashlib.sha256(_json(value)).hexdigest()

def _storage(value):
    if type(value) is not dict or set(value)!={'pvc_name','pvc_uid','pv_uid','generation'}:
        _fail()
    for key in value:
        _text(value[key])
    _text(value['pvc_name'],r'[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?')
    _text(value['generation'],r'legacy|[gr][0-9]{8}[a-z0-9]{1,8}')
    return value

def _repository(image, immutable=False):
    pattern=r'([a-z0-9][a-zA-Z0-9._/:~-]{1,255})@sha256:[a-f0-9]{64}'
    if type(image) is not str:
        _fail()
    match=re.fullmatch(pattern,image)
    if match:
        repository=match[1]
        if ':' in repository.rsplit('/',1)[-1]:
            _fail()
        return repository
    if immutable or not re.fullmatch(r'[a-z0-9][a-zA-Z0-9._/:~-]{1,255}',image):
        _fail()
    return image.rsplit(':',1)[0] if ':' in image.rsplit('/',1)[-1] else image

def _template(value, storage):
    if type(value) is not dict or type(value.get('spec')) is not dict:
        _fail()
    _json(value)
    spec=value['spec']; init=spec.get('initContainers'); containers=spec.get('containers')
    if type(init) is not list or type(containers) is not list or not containers:
        _fail()
    names=[]
    for entry in init+containers:
        if type(entry) is not dict:
            _fail()
        names.append(_text(entry.get('name'),r'[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?'))
    if len(names)!=len(set(names)):
        _fail()
    renderers=[entry for entry in init if entry['name']==RENDERER]
    if len(renderers)!=1:
        _fail()
    volumes=spec.get('volumes')
    if type(volumes) is not list or any(type(v) is not dict for v in volumes):
        _fail()
    claims=[]
    for volume in volumes:
        if 'persistentVolumeClaim' in volume:
            pvc=volume['persistentVolumeClaim']
            if type(pvc) is not dict:
                _fail()
            claims.append(pvc.get('claimName'))
    if claims!=[storage['pvc_name']]:
        _fail()
    return renderers[0]

def _snapshot(value, storage):
    if type(value) is not dict or set(value)-{'apiVersion','kind','metadata','spec','status'} or value.get('apiVersion')!='apps/v1' or value.get('kind')!='Deployment':
        _fail()
    metadata=value.get('metadata'); spec=value.get('spec')
    if type(metadata) is not dict or metadata.get('name')!=HUB or metadata.get('namespace') not in (None,'voice-staging') or type(spec) is not dict:
        _fail()
    _text(metadata.get('uid'));_text(metadata.get('resourceVersion'),r'[1-9][0-9]{0,31}')
    if type(spec.get('replicas')) is not int or spec['replicas']<0:
        _fail()
    _json(spec);_template(spec.get('template'),storage)
    return metadata,spec

def plan(hub, old_image, target_image, source_sha, storage_binding):
    storage=_storage(storage_binding); metadata,spec=_snapshot(hub,storage)
    _text(source_sha,r'[a-f0-9]{40}')
    original=spec['template']; renderer=_template(original,storage)
    repository=_repository(old_image,True)
    if _repository(target_image,True)!=repository or _repository(renderer.get('image'))!=repository:
        _fail()
    target=copy.deepcopy(original)
    _template(target,storage)['image']=target_image
    return {'schema':SCHEMA,'source_sha':source_sha,
            'hub':{key:metadata[key] for key in ('name','uid','resourceVersion')},
            'original_template':copy.deepcopy(original),'target_template':target,
            'original_template_sha256':_hash(original),'target_template_sha256':_hash(target),
            'images':{'old':old_image,'target':target_image},'storage_binding':copy.deepcopy(storage)}

def validate(descriptor, current_hub, storage_binding, *, expected_hub, paused=True):
    if type(descriptor) is not dict or set(descriptor)!=FIELDS or descriptor['schema']!=SCHEMA or type(paused) is not bool:
        _fail()
    binding=descriptor['hub']; images=descriptor['images'];storage=_storage(storage_binding)
    if type(binding) is not dict or set(binding)!={'name','uid','resourceVersion'} or binding['name']!=HUB or type(images) is not dict or set(images)!={'old','target'} or descriptor['storage_binding']!=storage:
        _fail()
    _text(binding['uid']);_text(binding['resourceVersion'],r'[1-9][0-9]{0,31}')
    original=descriptor['original_template']
    reconstructed=plan({'apiVersion':'apps/v1','kind':'Deployment','metadata':binding,
                        'spec':{'replicas':0,'template':original}},images['old'],images['target'],descriptor['source_sha'],storage)
    if _json(reconstructed)!=_json(descriptor):
        _fail()
    expected_meta,expected_spec=_snapshot(expected_hub,storage)
    current_meta,current_spec=_snapshot(current_hub,storage)
    if expected_meta['uid']!=binding['uid'] or any(current_meta.get(key)!=expected_meta.get(key) for key in ('name','namespace','uid','resourceVersion')) or _json(current_spec)!=_json(expected_spec):
        _fail()
    if (paused and current_spec['replicas']!=0) or _json(expected_spec['template']) not in (_json(original),_json(descriptor['target_template'])):
        _fail()
    return True

def compare_outputs(old_bytes, new_bytes):
    if type(old_bytes) is not bytes or type(new_bytes) is not bytes or not old_bytes or not new_bytes or len(old_bytes)>MAX_BYTES or len(new_bytes)>MAX_BYTES or old_bytes!=new_bytes:
        _fail()
    return {'sha256':hashlib.sha256(old_bytes).hexdigest(),'bytes':len(old_bytes),'verified':True}
