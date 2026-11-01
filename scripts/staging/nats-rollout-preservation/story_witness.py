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
    automatic=isinstance(row,dict) and row.get('schema')=='voice-story-current4-automatic-witness-v1'
    fields(row,'schema repository service platform content component database fixture tools '+('execution' if automatic else 'publisher review_sha256'))
    if (row['schema'] not in ('voice-story-current4-actual-witness-v1','voice-story-current4-automatic-witness-v1') or row['repository']!='Poryadok/VoiceRoot'
        or row['service']!='story' or row['platform']!='linux/amd64'):fail()
    content=row['content'];fields(content,'child_sha256 config_sha256 binary_sha256 build_source_sha')
    for key in ('child_sha256','config_sha256','binary_sha256'):digest(content[key])
    if not automatic or content['build_source_sha'] is not None:digest(content['build_source_sha'],40)
    component=row['component'];fields(component,'source_sha manifest_sha256')
    if not automatic or component['source_sha'] is not None:digest(component['source_sha'],40)
    digest(component['manifest_sha256'])
    if automatic:
        if component['source_sha']!=content['build_source_sha'] or component['manifest_sha256']!=content['child_sha256']:fail()
        execution=row['execution'];fields(execution,'helper_binding_sha256 caller_sha256 fixture_sha256')
        for value in execution.values():digest(value)
    else:
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
    for key,prefix in (('postgres','docker.io/library/postgres@sha256:'),('caller','ghcr.io/poryadok/voiceroot/story@sha256:' if automatic else 'golang:1.26-alpine@sha256:')):
        if not isinstance(tools[key],str) or not tools[key].startswith(prefix):fail()
        digest(tools[key][len(prefix):])
    if not automatic:digest(row['review_sha256'])
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
    return {'schema':'voice-story-root-automatic-witness-authority-v1' if row['schema']=='voice-story-current4-automatic-witness-v1' else 'voice-story-root-witness-authority-v1','registry_revision':selected['revision'],
        'witness_sha256':entry['witness_sha256'],'component_manifest_sha256':row['component']['manifest_sha256'],
        'component_source_sha':content['build_source_sha'],
        'schema_sha256':hashlib.sha256(json.dumps(row['database'],sort_keys=True,separators=(',',':')).encode()).hexdigest()}

def lookup(child,config):
    try:return _lookup(child,config)
    except (OSError,TypeError,KeyError,ValueError):fail()

def ensure_content(child,config,prove,unchanged):
    """Locked pre-fence admission: only genuinely missing content runs a fixture."""
    digest(child);digest(config)
    path=guard.ROOT/'installed'/'story-witnesses'/'selector.json'
    try:selected=selector(parsed(read(path)))
    except FileNotFoundError:selected=None
    if selected is not None and child+':'+config in selected['entries']:
        return lookup(child,config) # revoked/corrupt/conflicting authority never reproves
    unchanged()
    row=validate(prove())
    if (row['schema']!='voice-story-current4-automatic-witness-v1'
        or (row['content']['child_sha256'],row['content']['config_sha256'])!=(child,config)):fail()
    unchanged()
    raw=encoded(row)
    promote_locked(raw,hashlib.sha256(raw).hexdigest(),unchanged)
    return lookup(child,config)

def approve_content(content,captured=None):
    """Both Story preflight and fresh rollback use this exact authority."""
    import actor_root
    if not isinstance(content,tuple) or len(content)!=3:fail()
    if content in actor_root.STORY_IMAGES.values():
        if captured is not None:fail()
        return None
    current=lookup(content[0],content[1])
    if content[2]!=current['component_source_sha']:fail()
    if captured is not None and captured!=current:
        # Fresh lookup already rejects revoked/changed evidence. Adding another
        # independently proven automatic image cannot revoke this exact record.
        comparable=copy.deepcopy(current)
        if (current['schema']!='voice-story-root-automatic-witness-authority-v1'
            or not isinstance(captured,dict) or type(captured.get('registry_revision')) is not int
            or not 0<captured['registry_revision']<=current['registry_revision']):fail()
        comparable['registry_revision']=captured['registry_revision']
        if comparable!=captured:fail()
        return copy.deepcopy(captured)
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


def _preserved_proof(row):
    """Closed target8 structure only; this function does not issue authority."""
    import story_fixture
    fields(row,'schema input baseline_sha256 target_sha256 catalog_sha256 authority_sha256 baseline_rows_sha256 target_rows_sha256 table_count created_gate_owner_incarnation writer_results final_sha256')
    if row['schema']!='voice-story-preserved-target8-proof-v1' or type(row['table_count']) is not int or row['table_count']!=24:fail()
    for key in row:
        if key.endswith('_sha256'):digest(row[key])
    if not re.fullmatch('[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}',row['created_gate_owner_incarnation']):fail()
    inputs=row['input']
    fields(inputs,'schema operation context_sha256 helper_sha256 plan_sha256 source_capture_sha256 profile_sha256 migration_plan_sha256 migration_files_sha256 archive_sha256 binding_sha256 dump_receipt_sha256 dump_plaintext_sha256 restore_attempt phase postgres runtime_user images caller_sha256')
    if (inputs['schema']!='voice-story-preserved-target8-input-v1' or inputs['phase'] not in ('local','offnode')
        or inputs['caller_sha256']!=story_fixture.PRESERVED_CALLER_SHA256
        or not re.fullmatch('[a-z_][a-z0-9_]{0,31}',inputs['runtime_user'])):fail()
    digest(inputs['operation'],12);digest(inputs['restore_attempt'],32);digest(inputs['postgres'])
    for key in inputs:
        if key.endswith('_sha256'):digest(inputs[key])
    images=inputs['images'];writers=row['writer_results']
    if (not isinstance(images,dict) or set(images) not in ({'story0'},{'story0','story1'})
        or not isinstance(writers,list) or len(writers)!=len(images)):fail()
    ids=set();previous=row['target_sha256']
    for image,result in zip(images.values(),writers):
        fields(result,'image child_sha256 config_sha256 binary_sha256 contract positive positive_exit negative negative_exit invalid_input invalid_exit before_sha256 after_sha256 allowed_new_story_id')
        for key in ('child_sha256','config_sha256','binary_sha256','before_sha256','after_sha256'):digest(result[key])
        if (not isinstance(image,str) or not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/story@sha256:[a-f0-9]{64}',image)
            or result['image']!=image or result['contract']!=story_fixture.PRESERVED_CONTRACT
            or result['before_sha256']!=previous or type(result['positive_exit']) is not int or result['positive_exit']!=0
            or type(result['negative_exit']) is not int or result['negative_exit']!=17
            or type(result['invalid_exit']) is not int or result['invalid_exit']!=17
            or result['negative']!={'status':'FAIL','phase':'wrong_expected_row_count'}
            or result['invalid_input']!={'status':'FAIL','phase':'baseline_stories_input'}):fail()
        positive=result['positive']
        fields(positive,'status contract create_grpc_code response_identity_type_verified sql_fields_verified baseline_story_count story_count new_story_id default_ttl_24h_verified archive_plus30d_verified missing_profile_code negative_count_unchanged unrelated_row_count row_retained')
        if (positive['status']!='PASS' or positive['contract']!=story_fixture.PRESERVED_CONTRACT
            or positive['create_grpc_code']!='OK' or positive['missing_profile_code']!='Unauthenticated'
            or any(positive[key] is not True for key in ('response_identity_type_verified','sql_fields_verified',
                'default_ttl_24h_verified','archive_plus30d_verified','negative_count_unchanged','row_retained'))
            or type(positive['baseline_story_count']) is not int or not 0<=positive['baseline_story_count']<2**63
            or type(positive['story_count']) is not int or positive['story_count']!=positive['baseline_story_count']+1
            or type(positive['unrelated_row_count']) is not int or not 0<=positive['unrelated_row_count']<2**63
            or positive['new_story_id']!=result['allowed_new_story_id'] or positive['new_story_id'] in ids
            or not re.fullmatch('[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}',positive['new_story_id'])):fail()
        ids.add(positive['new_story_id']);previous=result['after_sha256']
    if previous!=row['final_sha256']:fail()
    return copy.deepcopy(row)


def _preserved_record(session):
    """Rebuild exclusively from authenticated complete producer disk records.

    No paused-old live observations run here: this is also used after the owned
    NATS transition and during read-only final/cache validation.
    """
    import commands,resource_claim,staging_cohort_preservation as owner
    if type(session) is not resource_claim.Session or commands.authority_guard!=session.check:fail()
    session.check();plan=owner.load_plan(session);owner._no_pending(session)
    base=owner._base(session);preflight=owner._read(base/'preflight.json')
    if owner.digest(preflight)!=plan['preflight_sha256']:fail()
    profile=(preflight.get('actor') or {}).get('story_schema')
    if not isinstance(profile,dict) or profile.get('schema')!='voice-story-baseline4-target8-composition-v1':fail()
    approved,_,prepared=owner._source(session,plan);owner._offnode_records(session)
    cut_binding={**session.binding(),'target_sha256':owner.digest(plan)}
    if owner._state(session).get('initial_resource_fence')!=cut_binding:fail()
    return _preserved_record_data(session.operation,plan,preflight,approved,prepared,cut_binding)


def _preserved_record_data(operation,plan,preflight,approved,prepared,cut_binding):
    """Pure protected-disk reconstruction after live or historical outer checks.

    This does not promote a record or issue current execution authority. Historical
    callers authenticate the completed parent without constructing an old Session.
    """
    import staging_cohort_preservation as owner
    digest(operation,12)
    base=guard.ROOT/'installed'/'cohort-preservation'/operation
    profile=(preflight.get('actor') or {}).get('story_schema')
    if (not isinstance(profile,dict) or profile.get('schema')!='voice-story-baseline4-target8-composition-v1'
        or owner.digest(preflight)!=plan['preflight_sha256']):fail()
    archive=owner._read(base/'archive-record.json');metadata=owner._read(base/'metadata.json')['source']
    migration=metadata['migrations']['story_db'];dump=owner._read(base/'effects'/'dump-story_db.json')
    if (migration['from']!=4 or migration['to']!=8 or migration['engine']!='golang-migrate'
        or owner.digest(migration['files'])!=profile['candidate']['migration_files_sha256']
        or dump.get('status')!='DONE' or dump.get('binding')!=cut_binding
        or owner.digest(dump['receipt'])!=archive['binding']['dump_receipts']['story_db']):fail()
    selected=profile['retained_image'] if prepared['request']['gis_first_install'] is True else approved['images']['story']
    images={'story'+str(index):image for index,image in enumerate(dict.fromkeys((selected,profile['retained_image'])))}
    users=[row['name'] for row in metadata['roles']['roles'] if row.get('login') is True
        and hashlib.sha256(row['name'].encode()).hexdigest()==profile['baseline']['postgres_user_sha256']]
    if len(users)!=1:fail()
    shared={'schema':'voice-story-preserved-target8-input-v1','operation':operation,
        'context_sha256':owner.digest(plan['context']),'helper_sha256':plan['helper_sha256'],'plan_sha256':owner.digest(plan),
        'source_capture_sha256':plan['source_capture_sha256'],'profile_sha256':owner.digest(profile),
        'migration_plan_sha256':profile['binding']['migration_plan_sha256'],'migration_files_sha256':owner.digest(migration['files']),
        'archive_sha256':owner.digest(archive),'binding_sha256':owner.digest(archive['binding']),
        'dump_receipt_sha256':owner.digest(dump['receipt']),'dump_plaintext_sha256':dump['plaintext_sha256'],
        'runtime_user':users[0],'images':images,'caller_sha256':__import__('story_fixture').PRESERVED_CALLER_SHA256}
    proofs={}
    for phase in ('local','offnode'):
        restore=owner._read(base/(phase+'-restore.json'));proof=_preserved_proof(restore['proof']['story_target8'])
        resources=owner._read(base/'restore'/restore['nonce']/'resources.json')
        postgres=[row['cid'] for row in resources['resources'].values() if row['kind']=='postgres']
        if len(postgres)!=1 or proof['input']!={**shared,'restore_attempt':restore['nonce'],'phase':phase,'postgres':postgres[0]}:fail()
        # Both true restore runs must actually contain every required app/probe
        # resource and verified cleanup, rather than copied fixture success JSON.
        kinds=[row['kind'] for row in resources['resources'].values()]
        if kinds.count('story')!=len(images) or kinds.count('story-probe')!=3*len(images):fail()
        proofs[phase]=proof
    for field in ('baseline_sha256','baseline_rows_sha256','catalog_sha256','authority_sha256'):
        if proofs['local'][field]!=proofs['offnode'][field]:fail()
    return {'schema':'voice-story-preserved-target8-witness-v1','operation':operation,
        'profile':profile,'plan_sha256':owner.digest(plan),'archive_sha256':owner.digest(archive),**proofs}


def _preserved_key(row):
    return row['operation']+':'+hashlib.sha256(encoded(row['profile'])).hexdigest()+':'+row['archive_sha256']


def _preserved_selector(row):
    fields(row,'schema revision entries')
    if (row['schema']!='voice-story-preserved-target8-selector-v1' or type(row['revision']) is not int
        or not 0<row['revision']<2**63 or type(row['entries']) is not dict or not 0<len(row['entries'])<=128):fail()
    for key,entry in row['entries'].items():
        if not re.fullmatch('[a-f0-9]{12}:[a-f0-9]{64}:[a-f0-9]{64}',key):fail()
        fields(entry,'witness_sha256 enabled revision');digest(entry['witness_sha256'])
        if type(entry['enabled']) is not bool or type(entry['revision']) is not int or not 0<entry['revision']<=row['revision']:fail()
    return row


def _preserved_authority(selected,key):
    entry=selected['entries'].get(key)
    if entry is None or entry['enabled'] is not True:fail()
    return {'schema':'voice-story-preserved-target8-authority-v1','key':key,
        'witness_sha256':entry['witness_sha256'],'registry_revision':entry['revision']}


def promote_preserved_target(session,*,revoke=False):
    """Automatic protected producer only; no new request/ENV/CLI admission."""
    import staging_cohort_preservation as owner
    owner._guard(session)
    row=_preserved_record(session);raw=encoded(row);sha=hashlib.sha256(raw).hexdigest();key=_preserved_key(row)
    base=guard.ROOT/'installed'/'story-preserved-witnesses';directory(base.parent)
    try:base.mkdir(mode=0o700)
    except FileExistsError:pass
    directory(base);path=base/'selector.json'
    try:before=read(path);old=_preserved_selector(parsed(before))
    except FileNotFoundError:before=None;old=None
    entry=None if old is None else old['entries'].get(key)
    if entry is not None:
        if entry['witness_sha256']!=sha or entry['enabled'] is False and not revoke:fail()
        if not revoke:return lookup_preserved_target(session)
    elif revoke:fail()
    record=base/(sha+'.json')
    try:existing=read(record)
    except FileNotFoundError:write_new(record,raw);sync_directory(base);existing=read(record)
    if existing!=raw or _preserved_record(session)!=row:fail()
    revision=1 if old is None else old['revision']+1
    entries={} if old is None else copy.deepcopy(old['entries'])
    entries[key]={'witness_sha256':sha,'enabled':not revoke,'revision':revision}
    selected=_preserved_selector({'schema':'voice-story-preserved-target8-selector-v1','revision':revision,'entries':entries})
    import secrets
    temporary=base/('.selector-'+secrets.token_hex(16))
    try:
        write_new(temporary,encoded(selected))
        if read(temporary)!=encoded(selected) or _preserved_record(session)!=row:fail()
        if before is None:
            if path.exists() or path.is_symlink():fail()
        elif read(path)!=before:fail()
        os.replace(temporary,path);sync_directory(base)
        if read(path)!=encoded(selected):fail()
    finally:
        if temporary.exists():temporary.unlink()
    if revoke:return None
    return lookup_preserved_target(session)


def lookup_preserved_target(session):
    try:
        return _lookup_preserved_record(lambda:_preserved_record(session))
    except (OSError,TypeError,KeyError,ValueError,AttributeError):fail()


def _lookup_preserved_record(reconstruct):
    row=reconstruct();key=_preserved_key(row);base=guard.ROOT/'installed'/'story-preserved-witnesses'
    before=read(base/'selector.json');selected=_preserved_selector(parsed(before));authority=_preserved_authority(selected,key)
    raw=read(base/(authority['witness_sha256']+'.json'))
    if (hashlib.sha256(raw).hexdigest()!=authority['witness_sha256'] or parsed(raw)!=row
        or read(base/'selector.json')!=before or reconstruct()!=row):fail()
    return authority


def _historical_preserved_target(operation,plan,preflight,approved,prepared,binding,rollout):
    """Read-only subordinate of the complete protected historical parent validator."""
    import staging_cohort_preservation as owner
    try:
        cut_binding={**binding,'target_sha256':owner.digest(plan)}
        record=lambda:_preserved_record_data(operation,plan,preflight,approved,prepared,cut_binding)
        result=_lookup_preserved_record(record);profile=record()['profile'];target=rollout['target']
        qualified=owner._read(guard.ROOT/'installed'/'cohort-preservation'/operation/'qualified.json')
        if (profile['binding']['migration_plan_sha256']!=owner.digest(rollout['migrations'])
            or target.get('cohort_preservation_sha256')!=owner.digest(qualified)
            or target.get('images',{}).get('voice-story/story',profile['retained_image'])!=record()['local']['input']['images']['story0']):fail()
        return result
    except (OSError,TypeError,KeyError,ValueError,AttributeError):fail()


def load_preserved_target(session,state,profile):
    """Fresh profile/backup-scoped authority for final issuance/effect/cache reads."""
    import staging_cohort_preservation as owner
    try:
        row=owner._require_session(session,{'prepare','apply-ready'})
        authority=lookup_preserved_target(session);record=_preserved_record(session)
        qualified=owner._read(owner._base(session)/'qualified.json');target=state['target']
        if (profile!=record['profile'] or profile['binding']['migration_plan_sha256']!=owner.digest(state['migrations'])
            or qualified.get('story_target8')!=authority or target.get('cohort_preservation_sha256')!=owner.digest(qualified)
            or row.get('target_sha256')!=owner.digest(target)
            or target.get('images',{}).get('voice-story/story',profile['retained_image'])!=record['local']['input']['images']['story0']):fail()
        return authority
    except (OSError,TypeError,KeyError,ValueError,AttributeError):fail()

if __name__=='__main__':
    import sys
    main(sys.argv[1:])
