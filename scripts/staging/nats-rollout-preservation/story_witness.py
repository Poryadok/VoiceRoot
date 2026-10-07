"""Root-promoted reviewed Story/current4 compatibility evidence, never caller authority."""
import copy,re,hashlib,json,os,stat
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).resolve().parent))
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
from controller import Blocked
import guard

def fail():raise Blocked('story_compatibility_witness_invalid')
def fields(row,names):
    if not isinstance(row,dict) or set(row)!=set(names.split()):fail()
def digest(value,length=64):
    if not isinstance(value,str) or not re.fullmatch('[a-f0-9]{'+str(length)+'}',value):fail()

def validate(row):
    """Structural evidence checks only. Root promotion is a separate authority."""
    fields(row,'schema repository service platform content component database publisher fixture tools review_sha256')
    if (row['schema']!='voice-story-current4-actual-witness-v1' or row['repository']!='Poryadok/VoiceRoot'
        or row['service']!='story' or row['platform']!='linux/amd64'):fail()
    content=row['content'];fields(content,'child_sha256 config_sha256 binary_sha256 build_source_sha')
    for key in ('child_sha256','config_sha256','binary_sha256'):digest(content[key])
    digest(content['build_source_sha'],40)
    component=row['component'];fields(component,'source_sha manifest_sha256')
    digest(component['source_sha'],40);digest(component['manifest_sha256'])
    publisher=row['publisher'];fields(publisher,'workflow run_id job_id source_sha catalog_sha256')
    if (publisher['workflow']!='.github/workflows/ci.yml'
        or any(type(publisher[k]) is not int or not 0<publisher[k]<2**63 for k in ('run_id','job_id'))
        or component['source_sha']!=content['build_source_sha'] or publisher['source_sha']!=content['build_source_sha']):fail()
    digest(publisher['catalog_sha256'])
    database=row['database'];fields(database,'name version dirty up_sha256 catalog_before_sha256 catalog_after_sha256')
    # The schema is deliberately fixed to the reviewed current4 compatibility gate.
    import actor_root
    if (database['name']!='story_db' or type(database['version']) is not int or database['version']!=4
        or database['dirty'] is not False or database['up_sha256']!=actor_root.STORY_UP
        or database['catalog_before_sha256']!=database['catalog_after_sha256']):fail()
    digest(database['catalog_before_sha256'])
    fixture=row['fixture'];fields(fixture,'contract_sha256 input_sha256 source_sha256 result_sha256 cases positive_exit negative_exit missing_profile_code row_count cleanup_verified')
    for key in ('contract_sha256','input_sha256','source_sha256','result_sha256'):digest(fixture[key])
    if (fixture['cases']!=['health','text_writer','sql_defaults','missing_profile','wrong_count','schema_preservation','cleanup']
        or type(fixture['positive_exit']) is not int or fixture['positive_exit']!=0
        or type(fixture['negative_exit']) is not int or fixture['negative_exit']!=17
        or type(fixture['row_count']) is not int or fixture['row_count']!=1
        or fixture['missing_profile_code']!='Unauthenticated' or fixture['cleanup_verified'] is not True):fail()
    tools=row['tools'];fields(tools,'postgres caller')
    for key,prefix in (('postgres','docker.io/library/postgres@sha256:'),('caller','golang:1.26-alpine@sha256:')):
        if not isinstance(tools[key],str) or not tools[key].startswith(prefix):fail()
        digest(tools[key][len(prefix):])
    digest(row['review_sha256'])
    return copy.deepcopy(row)

LIMIT=1<<20

def directory(path):
    path=Path(path)
    for part in reversed((path,*path.parents)):
        info=part.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode&0o022:fail()

def read(path):
    path=Path(path);directory(path.parent)
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        before=os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid!=0 or before.st_nlink!=1
            or before.st_mode&0o077 or not 0<before.st_size<=LIMIT):fail()
        raw=os.read(fd,before.st_size+1);after=os.fstat(fd)
        keys=('st_dev','st_ino','st_mode','st_uid','st_gid','st_nlink','st_size','st_mtime_ns','st_ctime_ns')
        if len(raw)!=before.st_size or any(getattr(before,k)!=getattr(after,k) for k in keys):fail()
        return raw
    finally:os.close(fd)

def parsed(raw):
    if not isinstance(raw,bytes) or not 0<len(raw)<=LIMIT:fail()
    def pairs(items):
        row={}
        for key,value in items:
            if key in row:fail()
            row[key]=value
        return row
    try:return json.loads(raw,object_pairs_hook=pairs)
    except (ValueError,UnicodeError,RecursionError):fail()

def selector(row):
    fields(row,'schema revision entries')
    if (row['schema']!='voice-story-witness-selector-v1' or type(row['revision']) is not int
        or not 0<row['revision']<2**63 or not isinstance(row['entries'],dict)
        or not 0<len(row['entries'])<=128):fail()
    for key,value in row['entries'].items():
        if not isinstance(key,str) or not re.fullmatch('[a-f0-9]{64}:[a-f0-9]{64}',key):fail()
        fields(value,'witness_sha256 enabled');digest(value['witness_sha256'])
        if type(value['enabled']) is not bool:fail()
    return row

def _lookup(child,config):
    """Read fresh root authority; structural fixture flags cannot enter here."""
    digest(child);digest(config)
    base=guard.ROOT/'installed'/'story-witnesses'
    raw_selector=read(base/'selector.json');selected=selector(parsed(raw_selector))
    entry=selected['entries'].get(child+':'+config)
    if entry is None or entry['enabled'] is not True:fail()
    raw=read(base/(entry['witness_sha256']+'.json'))
    if hashlib.sha256(raw).hexdigest()!=entry['witness_sha256']:fail()
    row=validate(parsed(raw));content=row['content']
    if (content['child_sha256'],content['config_sha256'])!=(child,config):fail()
    if read(base/'selector.json')!=raw_selector:fail()
    return {'schema':'voice-story-root-witness-authority-v1','registry_revision':selected['revision'],
        'witness_sha256':entry['witness_sha256'],'component_manifest_sha256':row['component']['manifest_sha256'],
        'component_source_sha':content['build_source_sha'],
        'schema_sha256':hashlib.sha256(json.dumps(row['database'],sort_keys=True,separators=(',',':')).encode()).hexdigest()}

def lookup(child,config):
    try:return _lookup(child,config)
    except (OSError,TypeError,KeyError,ValueError):fail()

def approve_content(content,captured=None):
    """Both Story preflight and fresh rollback use this exact authority."""
    import actor_root
    if not isinstance(content,tuple) or len(content)!=3:fail()
    if content in actor_root.STORY_IMAGES.values():
        if captured is not None:fail()
        return None
    current=lookup(content[0],content[1])
    if content[2]!=current['component_source_sha'] or captured is not None and captured!=current:fail()
    return current

def registry_snapshot():
    """Upgrade preserves every approved/revoked record, or vetoes corruption."""
    base=guard.ROOT/'installed'/'story-witnesses'
    try:base.lstat()
    except FileNotFoundError:return None
    directory(base);result={}
    for path in base.iterdir():
        if len(result)>=256:fail()
        if path.name!='selector.json' and not re.fullmatch(r'[a-f0-9]{64}\.json',path.name):fail()
        raw=read(path);result[path.name]=hashlib.sha256(raw).hexdigest()
        if path.name!='selector.json':
            if path.stem!=result[path.name]:fail()
            validate(parsed(raw))
    selected=selector(parsed(read(base/'selector.json')))
    for key,entry in selected['entries'].items():
        raw=read(base/(entry['witness_sha256']+'.json'));row=validate(parsed(raw))
        if key!=row['content']['child_sha256']+':'+row['content']['config_sha256']:fail()
        if result.get(entry['witness_sha256']+'.json')!=entry['witness_sha256']:fail()
    if hashlib.sha256(read(base/'selector.json')).hexdigest()!=result.get('selector.json'):fail()
    return result

def encoded(row):return json.dumps(row,sort_keys=True,separators=(',',':')).encode()

def write_new(path,raw):
    """An interrupted write is never eligible: only a full hash readback is used."""
    directory(path.parent)
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW|os.O_NONBLOCK,0o600)
    try:
        view=memoryview(raw)
        while view:
            count=os.write(fd,view)
            if count<=0:fail()
            view=view[count:]
        os.fsync(fd)
    finally:os.close(fd)

def sync_directory(path):
    fd=os.open(path,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(fd)
    finally:os.close(fd)

def promote_locked(raw,expected_sha,unchanged,*,revoke=False):
    """Caller holds the global root lock; no bridge request can call this API."""
    digest(expected_sha)
    if hashlib.sha256(raw).hexdigest()!=expected_sha:fail()
    row=validate(parsed(raw));key=row['content']['child_sha256']+':'+row['content']['config_sha256']
    base=guard.ROOT/'installed'/'story-witnesses'
    directory(base.parent)
    try:base.mkdir(mode=0o700)
    except FileExistsError:pass
    directory(base)
    record=base/(expected_sha+'.json');selected_path=base/'selector.json'
    # Orphan complete records from an interrupted promotion are reusable, never
    # arbitrary partial bytes. Existing approved evidence is not overwritten.
    try:existing=read(record)
    except FileNotFoundError:
        write_new(record,raw);sync_directory(base);existing=read(record)
    if existing!=raw:fail()
    try:before=read(selected_path);old=selector(parsed(before))
    except FileNotFoundError:before=None;old=None
    enabled=not revoke
    entry=None if old is None else old['entries'].get(key)
    if revoke and (entry is None or entry['witness_sha256']!=expected_sha):fail()
    unchanged()
    if entry is not None and entry['witness_sha256']==expected_sha and entry['enabled']==enabled:
        if read(selected_path)!=before:fail()
        return copy.deepcopy(old)
    if entry is not None and (entry['witness_sha256']!=expected_sha or entry['enabled'] is False and not revoke):fail()
    revision=1 if old is None else old['revision']+1
    entries={} if old is None else copy.deepcopy(old['entries'])
    entries[key]={'witness_sha256':expected_sha,'enabled':enabled}
    selected=selector({'schema':'voice-story-witness-selector-v1','revision':revision,
        'entries':entries})
    import secrets
    temporary=base/('.selector-'+secrets.token_hex(16))
    try:
        write_new(temporary,encoded(selected))
        if read(temporary)!=encoded(selected):fail()
        unchanged()
        if before is None:
            if selected_path.exists() or selected_path.is_symlink():fail()
        elif read(selected_path)!=before:fail()
        os.replace(temporary,selected_path);sync_directory(base)
        if read(selected_path)!=encoded(selected):fail()
    finally:
        if temporary.exists():temporary.unlink()
    return selected

def main(args):
    """Explicit human-root admission, separate from workflow and issuer APIs."""
    import sys
    if os.geteuid()!=0 or sys.platform!='linux':fail()
    if len(args)!=3 or args[0] not in ('--promote','--revoke'):fail()
    digest(args[2]);directory(guard.ROOT)
    from root_main import operation_lock
    from bridge_root import Actions,INSTALLED,Kube
    with operation_lock():
        actions=Actions(INSTALLED/'code');actions._idle()
        kube=Kube();marker=kube.get('configmap',guard.MARKER)
        data=marker.get('data',{})
        if data.get('phase')!='active' or any(key in data for key in ('knownBaselineOperation','knownRolloutOperation')):fail()
        def unchanged():
            actions._idle()
            if kube.get('configmap',guard.MARKER)!=marker:fail()
        result=promote_locked(read(Path(args[1])),args[2],unchanged,revoke=args[0]=='--revoke')
        print('STORY_WITNESS='+('REVOKED' if args[0]=='--revoke' else 'PROMOTED'))

if __name__=='__main__':
    import sys
    main(sys.argv[1:])
