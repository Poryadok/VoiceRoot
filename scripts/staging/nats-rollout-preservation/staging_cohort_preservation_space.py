"""Fixed native Space rehearsal on the current protected isolated PG clone.

Publication provenance is obtained by the captured STAGING reader. No caller
image, database URL, path, command, sidecar or claimed migration result enters
this API. The clone's original evidence never authorizes a live migration.
"""
import copy
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat

from controller import Blocked
import staging_cohort_preservation as owner
from staging_cohort_preservation_runtime import DOCKER, _resources


STAGES=('source24','trusted25','trusted44','trusted45','trusted50','trusted51','initialized','upgrade-complete')


def capture_publication(session,request):
    plan=owner.load_plan(session);approved,execution,_=owner._source(session,plan)
    row=session.check()
    if row.get('phase')!='prepare' or row.get('target_sha256') is not None:
        raise Blocked('cohort_space_publication_phase_invalid')
    preflight=owner._read(owner._base(session)/'preflight.json');owner._preflight_shape(preflight,session)
    if owner.digest(preflight)!=plan['preflight_sha256']:raise Blocked('cohort_preflight_changed')
    selected='space' in preflight['services']
    proof=None
    if selected:
        reader=owner.shared_module(session,'space_native_tool_identity_staging')
        proof=reader.authenticated_staging_publication(request['token'],'Poryadok',approved,
            owner.guard.ROOT/'installed'/'sources'/session.operation)
    if owner._source(session,plan)[:2]!=(approved,execution) or session.check()!=row:
        raise Blocked('cohort_space_publication_changed')
    return {'schema':'voice-staging-cohort-space-tool-v1','operation':session.operation,
        'source_capture_sha256':plan['source_capture_sha256'],'approved_sha256':plan['approved_sha256'],
        'selected':selected,'authenticated_publication':proof}


def load_tool(session):
    plan=owner.load_plan(session);owner._phase_continuity(session,plan)
    record=owner._read(owner._base(session)/'space-tool.json')
    if (set(record)!={'schema','operation','source_capture_sha256','approved_sha256','selected','authenticated_publication'}
        or record['schema']!='voice-staging-cohort-space-tool-v1' or record['operation']!=session.operation
        or record['source_capture_sha256']!=plan['source_capture_sha256']
        or record['approved_sha256']!=plan['approved_sha256'] or owner.digest(record)!=plan['space_tool_sha256']
        or record['selected'] is not True):raise Blocked('cohort_space_tool_required')
    proof=record['authenticated_publication'];publication=proof['publication']
    if (set(proof)!={'artifact_id','artifact_sha256','publication'} or type(proof['artifact_id']) is not int
        or proof['artifact_id']<=0 or not re.fullmatch('[a-f0-9]{64}',proof['artifact_sha256'])
        or publication['source_sha']!=session.context['commit']
        or publication['runtime_qualified'] is not False or publication['deployment_performed'] is not False
        or publication['immutable_image']!='ghcr.io/poryadok/voiceroot/space-native-installer@'+publication['platform_digest']):
        raise Blocked('cohort_space_tool_changed')
    return copy.deepcopy(publication)


def _available_image(raw,tool):
    try:rows=owner.private._json(raw)
    except Exception:raise Blocked('cohort_space_image_changed') from None
    if (not isinstance(rows,list) or len(rows)!=1 or rows[0].get('Id')!=tool['config_digest']
        or rows[0].get('Os')!='linux' or rows[0].get('Architecture')!='amd64'
        or tool['immutable_image'] not in rows[0].get('RepoDigests',[])
        or rows[0].get('Config',{}).get('User')!='65532:65532'
        or rows[0]['Config'].get('Entrypoint')!=['/usr/local/bin/native-migrate']
        or rows[0]['Config'].get('Labels',{}).get('org.opencontainers.image.revision')!=tool['source_sha']):
        raise Blocked('cohort_space_image_changed')
    return {'config_digest':rows[0]['Id'],'image':tool['immutable_image'],
        'platform':{'os':'linux','architecture':'amd64'},'source_sha':tool['source_sha']}


def prepare_tool(session,token):
    """Fetch only the authenticated immutable Tool before any writer is fenced."""
    plan=owner._guard(session);owner._no_pending(session);tool=load_tool(session)
    if not isinstance(token,str) or not 1<=len(token)<=16384 or any(ord(c)<33 or ord(c)>126 for c in token):
        raise Blocked('cohort_space_registry_credential_invalid')
    base=owner._base(session);path=base/'space-tool-available.json'
    record={'schema':'voice-staging-space-tool-available-v1','operation':session.operation,
        'plan_sha256':owner.digest(plan),'tool_sha256':owner.digest(tool),'status':'PENDING','commands':[]}
    owner._write(path,record)
    auth=base/'tool-pull-auth';auth.mkdir(mode=0o700);owner.private._private_directory(auth)
    descriptor=os.open(auth,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    identity=os.fstat(descriptor)
    config=owner.private._bytes({'auths':{'ghcr.io':{'auth':base64.b64encode(('Poryadok:'+token).encode()).decode()}}})
    try:
        owner.private._private_write(auth/'config.json',config)
        for label,argv,limit,timeout in (
            ('pull',DOCKER+['--config',str(auth),'pull','--platform','linux/amd64',tool['immutable_image']],1<<20,600),
            ('inspect',DOCKER+['image','inspect',tool['immutable_image']],2<<20,30)):
            owner._guard(session);owner._no_pending(session)
            if (load_tool(session)!=tool or owner._read(path)!=record
                or _private_generated(auth/'config.json',uid=0,limit=32768)!=config):
                raise Blocked('cohort_space_tool_availability_changed')
            effect_id='space-tool-'+label
            intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':effect_id,
                'binding':session.binding(),'argv':argv,'body_sha256':hashlib.sha256(b'').hexdigest(),'status':'PENDING'}
            effect=base/'effects'/(effect_id+'.json');owner.private._private_directory(effect.parent);owner._write(effect,intent)
            raw=owner._send(session,effect_id,argv,b'',limit=limit,timeout=timeout)
            if (owner._read(effect)!=intent or load_tool(session)!=tool or owner._read(path)!=record
                or _private_generated(auth/'config.json',uid=0,limit=32768)!=config):
                raise Blocked('cohort_space_tool_availability_changed')
            observation=_available_image(raw,tool) if label=='inspect' else None
            if label=='inspect':owner.private._private_write(base/'space-tool-image.json',raw)
            done={**intent,'status':'DONE','response_sha256':hashlib.sha256(raw).hexdigest()}
            owner.save(effect,done);record['commands'].append({'effect_id':effect_id,'sha256':owner.digest(done)})
            owner.save(path,record)
    finally:
        try:
            current=os.stat(auth,follow_symlinks=False)
            if (current.st_dev,current.st_ino)!=(identity.st_dev,identity.st_ino):
                raise Blocked('cohort_space_auth_directory_changed')
            try:os.unlink('config.json',dir_fd=descriptor)
            except FileNotFoundError:pass
            os.rmdir(auth)
        finally:os.close(descriptor)
    owner._guard(session)
    if load_tool(session)!=tool or owner._read(path)!=record:raise Blocked('cohort_space_tool_availability_changed')
    record.update(status='COMPLETE',observation=observation,credential_cleanup='verified')
    owner.save(path,record)
    return load_available_tool(session)


def _available_record(operation,plan,tool):
    base=owner.guard.ROOT/'installed'/'cohort-preservation'/operation
    record=owner._read(base/'space-tool-available.json');state=owner._read(base/'checkpoint.json')
    fields={'schema','operation','plan_sha256','tool_sha256','status','commands','observation','credential_cleanup'}
    if (set(record)!=fields or record['schema']!='voice-staging-space-tool-available-v1'
        or record['operation']!=operation or record['plan_sha256']!=owner.digest(plan)
        or record['tool_sha256']!=owner.digest(tool) or record['status']!='COMPLETE'
        or record['credential_cleanup']!='verified' or (base/'tool-pull-auth').exists()
        or not isinstance(record['commands'],list) or len(record['commands'])!=2):
        raise Blocked('cohort_space_tool_availability_changed')
    raw=_private_generated(base/'space-tool-image.json',uid=0,limit=2<<20)
    if _available_image(raw,tool)!=record['observation']:raise Blocked('cohort_space_image_changed')
    for ref,(label,argv) in zip(record['commands'],(
        ('pull',DOCKER+['--config',str(base/'tool-pull-auth'),'pull','--platform','linux/amd64',tool['immutable_image']]),
        ('inspect',DOCKER+['image','inspect',tool['immutable_image']]))):
        ident='space-tool-'+label
        if set(ref)!={'effect_id','sha256'} or ref['effect_id']!=ident:raise Blocked('cohort_space_tool_availability_changed')
        effect=owner._read(base/'effects'/(ident+'.json'))
        if (effect.get('status')!='DONE' or effect.get('operation')!=operation or effect.get('effect_id')!=ident
            or effect.get('argv')!=argv or effect.get('binding')!=state['initial_resource_fence']
            or effect.get('body_sha256')!=hashlib.sha256(b'').hexdigest() or owner.digest(effect)!=ref['sha256']
            or not re.fullmatch('[a-f0-9]{64}',str(effect.get('response_sha256')))
            or label=='inspect' and effect['response_sha256']!=hashlib.sha256(raw).hexdigest()):
            raise Blocked('cohort_space_tool_availability_changed')
    return copy.deepcopy(record)


def load_available_tool(session):
    plan=owner.load_plan(session);owner._phase_continuity(session,plan);owner._no_pending(session)
    tool=load_tool(session);record=_available_record(session.operation,plan,tool)
    if load_tool(session)!=tool or owner.load_plan(session)!=plan:raise Blocked('cohort_space_tool_availability_changed')
    return record


def _private_generated(path,*,uid,limit):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        row=os.fstat(fd)
        if (not stat.S_ISREG(row.st_mode) or row.st_uid!=uid or row.st_nlink!=1
            or stat.S_IMODE(row.st_mode)!=0o600 or not 0<row.st_size<=limit):
            raise Blocked('cohort_space_evidence_custody_invalid')
        raw=os.read(fd,row.st_size+1);after=os.fstat(fd)
        if len(raw)!=row.st_size or any(getattr(row,k)!=getattr(after,k) for k in
            ('st_dev','st_ino','st_size','st_mtime_ns','st_ctime_ns')):
            raise Blocked('cohort_space_evidence_changed')
        return raw
    finally:os.close(fd)


def _events(raw,session,archive,tool,identity):
    if not isinstance(raw,bytes) or not 0<len(raw)<=1<<20:raise Blocked('cohort_space_events_invalid')
    try:rows=[owner.private._json(line) for line in raw.splitlines() if line.strip()]
    except Exception:raise Blocked('cohort_space_events_invalid') from None
    if len(rows)!=8 or [r.get('event') for r in rows]!=list(STAGES) or any(set(r)!={'event','receipt'} for r in rows):
        raise Blocked('cohort_space_events_invalid')
    receipt=rows[-1]['receipt'];source=receipt['source24'];final=receipt['final']
    if (receipt.get('operation_id')!=session.context['operation_id']
        or receipt.get('snapshot_id')!=archive['binding']['snapshot_id']
        or receipt.get('source_revision')!=tool['source_sha']
        or receipt.get('source_sha256')!=tool['source_inventory']['build_source_sha256']
        or receipt.get('executable_sha256')!=tool['content']['executable_sha256']
        or receipt.get('migrator_sha256')!=tool['content']['migrator_sha256']
        or receipt.get('inventory_sha256')!=tool['content']['sql_inventory_sha256']
        or source.get('database_oid')!=identity['database_oid'] or final.get('database_oid')!=identity['database_oid']
        or source.get('version')!=24 or final.get('version')!=51
        or source.get('database_name')!='space_db' or final.get('database_name')!='space_db'
        or str(source.get('database_system_id'))!=identity['system_identifier']
        or str(final.get('database_system_id'))!=identity['system_identifier']):
        raise Blocked('cohort_space_events_binding_changed')
    return receipt


def native_space_rehearsal(session):
    """Run the original upgrade and verifier before shared clone cleanup."""
    owner._guard(session);owner._no_pending(session)
    archive=owner.load_backup_archive(session);metadata=owner.load_metadata(session,archive['metadata_ref'])
    migration=metadata['source']['migrations'].get('space_db')
    if (not isinstance(migration,dict) or migration.get('from')!=24 or migration.get('to')!=51):
        raise Blocked('cohort_space_legacy24_required')
    tool=load_tool(session);nonce=owner.restore_attempt(session);resources_path,resources=_resources(session)
    if owner._read(owner._base(session)/'restore-attempt.json').get('phase') not in ('local','offnode'):
        raise Blocked('cohort_space_attempt_changed')
    candidates=[r for r in resources['resources'].values() if r['kind']=='postgres' and r['status']=='ACTIVE']
    if len(candidates)!=1:raise Blocked('cohort_space_clone_required')
    clone=candidates[0];cid=clone['cid']
    rows=json.loads(owner.restore_command(session,DOCKER+['inspect',cid]))
    env=rows[0]['Config']['Env'];admins=[x.split('=',1)[1] for x in env if x.startswith('POSTGRES_USER=')]
    if len(admins)!=1 or not re.fullmatch('voice_cohort_[a-f0-9]{24}',admins[0]):
        raise Blocked('cohort_space_clone_admin_invalid')
    admin=admins[0];base=owner._base(session)/'restore'/nonce
    native_user=metadata['source']['catalog']['space_db']['database_owner']
    if not isinstance(native_user,str) or not re.fullmatch('[A-Za-z_][A-Za-z0-9_]{0,62}',native_user):
        raise Blocked('cohort_space_clone_owner_invalid')
    record_path=base/'native-space.json'
    record={'schema':'voice-staging-cohort-space-rehearsal-v1','operation':session.operation,'attempt':nonce,
        'plan_sha256':owner.digest(owner.load_plan(session)),'archive_sha256':owner.digest(archive),
        'tool_sha256':owner.digest(tool),'clone_id':cid,'status':'PENDING','resources':{},'commands':[]}
    owner._write(record_path,record)
    # The current administrator belongs only to the disposable clone. Give the
    # network-isolated tool a fresh password; no live role or shared credential.
    password=owner.secrets.token_hex(32)
    sql=('SET search_path=pg_catalog,public,pg_temp; ALTER ROLE "'+native_user+'"'+" PASSWORD '"+password+"';\n"
        "SELECT json_build_object('system_identifier',(SELECT system_identifier::text FROM pg_control_system()),"
        "'database_oid',(SELECT oid::bigint FROM pg_database WHERE datname='space_db'));\n").encode()
    identity=owner.private._json(owner.restore_command(session,DOCKER+['exec','-i',cid,'psql','-X','-qAt',
        '-v','ON_ERROR_STOP=1','-U',admin,'-d','postgres'],body=sql))
    if type(identity.get('database_oid')) is not int or not re.fullmatch('[0-9]+',identity.get('system_identifier','')):
        raise Blocked('cohort_space_clone_identity_invalid')
    record['clone_identity']=identity;owner.save(record_path,record)
    work=base/'native-work';work.mkdir(mode=0o700);os.chown(work,65532,65532)
    env_path=base/'native.env'
    owner.private._private_write(env_path,('SPACE_NATIVE_ADMIN_URL=postgres://'+native_user+':'+password+
        '@127.0.0.1:5432/postgres?sslmode=disable\nSPACE_NATIVE_REQUIRED_SOURCE_SHA256='+
        tool['source_inventory']['build_source_sha256']+'\n').encode())
    env_hash=hashlib.sha256(_private_generated(env_path,uid=0,limit=4096)).hexdigest()
    def check():
        owner._guard(session)
        if (owner.restore_attempt(session)!=nonce or load_tool(session)!=tool
            or owner._read(record_path)!=record or owner._read(resources_path)!=resources
            or hashlib.sha256(_private_generated(env_path,uid=0,limit=4096)).hexdigest()!=env_hash):
            raise Blocked('cohort_space_rehearsal_changed')
    def command(label,args,*,limit=2<<20,timeout=60):
        check();owner._no_pending(session)
        ident='space-'+nonce+'-'+str(len(record['commands']))+'-'+label
        intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':ident,
            'binding':session.binding(),'argv':DOCKER+args,'body_sha256':hashlib.sha256(b'').hexdigest(),'status':'PENDING'}
        path=owner._base(session)/'effects'/(ident+'.json');owner._write(path,intent)
        raw=owner._send(session,ident,DOCKER+args,b'',limit=limit,timeout=timeout)
        if owner._read(path)!=intent:raise Blocked('cohort_space_command_changed')
        done={**intent,'status':'DONE','response_sha256':hashlib.sha256(raw).hexdigest()}
        owner.save(path,done);check()
        record['commands'].append({'effect_id':ident,'sha256':owner.digest(done)})
        owner.save(record_path,record)
        return raw
    inspected=json.loads(command('image',['image','inspect',tool['config_digest']]))
    if (len(inspected)!=1 or inspected[0].get('Id')!=tool['config_digest'] or inspected[0].get('Os')!='linux'
        or inspected[0].get('Architecture')!='amd64' or inspected[0].get('Config',{}).get('User')!='65532:65532'
        or inspected[0]['Config'].get('Entrypoint')!=['/usr/local/bin/native-migrate']
        or inspected[0]['Config'].get('Labels',{}).get('org.opencontainers.image.revision')!=tool['source_sha']):
        raise Blocked('cohort_space_image_changed')
    outputs={}
    for mode in ('upgrade-legacy24','verify-legacy24'):
        name='voice-cohort-space-'+nonce+'-'+('upgrade' if mode=='upgrade-legacy24' else 'verify')
        args=['create','--name',name,'--label','voice.test.owner='+name,'--network','container:'+cid,
            '--user','65532:65532','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true',
            '--memory','256m','--pids-limit','64','--env-file',str(env_path),
            '--mount','type=bind,source='+str(work)+',target=/evidence'+('' if mode=='upgrade-legacy24' else ',readonly'),
            tool['config_digest'],'-mode',mode,'-migrations-dir','/migrations','-migrate','/usr/local/bin/migrate',
            '-origin-evidence','/evidence/origin.json']
        args+=(['-operation-id',session.context['operation_id'],'-snapshot-id',archive['binding']['snapshot_id']]
            if mode=='upgrade-legacy24' else ['-prior-evidence','/evidence/completed.jsonl'])
        created=command('create',args).decode().strip()
        if not re.fullmatch('[a-f0-9]{64}',created):raise Blocked('cohort_space_create_uncertain')
        record['resources'][created]='OWNED';owner.save(record_path,record)
        observed=json.loads(command('inspect',['inspect',created]))
        if (len(observed)!=1 or observed[0].get('Id')!=created or observed[0].get('Image')!=tool['config_digest']
            or observed[0]['HostConfig'].get('NetworkMode')!='container:'+cid
            or observed[0]['Config'].get('Labels',{}).get('voice.test.owner')!=name):
            raise Blocked('cohort_space_container_changed')
        raw=command('run',['start','-a',created],limit=1<<20,timeout=360);outputs[mode]=raw
        state=json.loads(command('inspect',['inspect',created]))[0]['State']
        if state.get('Running') is not False or state.get('ExitCode')!=0:
            raise Blocked('cohort_space_execution_incomplete')
        if mode=='upgrade-legacy24':
            receipt=_events(raw,session,archive,tool,identity)
            original=_private_generated(work/'origin.json',uid=65532,limit=64<<20)
            owner.private._private_write(base/'native-origin.json',original)
            owner.private._private_write(base/'native-upgrade.jsonl',raw)
            fd=os.open(work/'completed.jsonl',os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
            with os.fdopen(fd,'wb') as out:out.write(raw);out.flush();os.fsync(out.fileno())
            os.chown(work/'completed.jsonl',65532,65532)
        command('remove',['rm','-f',created]);record['resources'][created]='REMOVED';owner.save(record_path,record)
        if command('absence',['ps','-a','--filter','id='+created,'--format','{{.ID}}']).strip():
            raise Blocked('cohort_space_cleanup_incomplete')
    # Original executable verification parses all eight events and authenticates
    # the original sidecar against this clone, never a caller-decoded receipt.
    if (_private_generated(work/'origin.json',uid=65532,limit=64<<20)!=original
        or _private_generated(work/'completed.jsonl',uid=65532,limit=1<<20)!=outputs['upgrade-legacy24']):
        raise Blocked('cohort_space_evidence_changed')
    owner.private._private_write(base/'native-verify.jsonl',outputs['verify-legacy24'])
    verified=owner.private._json(outputs['verify-legacy24'])
    if verified!={'event':'verified-legacy24','receipt':receipt}:
        raise Blocked('cohort_space_verification_changed')
    check();record.update(status='COMPLETE',origin_sha256=hashlib.sha256(original).hexdigest(),
        events_sha256=hashlib.sha256(outputs['upgrade-legacy24']).hexdigest(),
        verification_sha256=hashlib.sha256(outputs['verify-legacy24']).hexdigest(),receipt=receipt)
    owner.save(record_path,record)
    return load_native_space_rehearsal(session)


def load_native_space_rehearsal(session):
    """Authenticate the current attempt's retained result, including after cleanup."""
    owner._guard(session);owner._no_pending(session)
    attempt=owner._read(owner._base(session)/'restore-attempt.json')
    result=_retained_rehearsal(session,attempt,owner.load_backup_archive(session),load_tool(session))
    owner._guard(session)
    return result


def _retained_rehearsal(session,attempt,archive,tool):
    """Verify retained bytes under the caller's already authenticated phase."""
    if (attempt.get('operation')!=session.operation or attempt.get('plan_sha256')!=owner.digest(owner.load_plan(session))
        or attempt.get('status') not in ('RUNNING','COMPLETE') or attempt.get('phase') not in ('local','offnode')
        or not re.fullmatch('[a-f0-9]{32}',str(attempt.get('nonce')))):
        raise Blocked('cohort_space_attempt_changed')
    nonce=attempt['nonce'];base=owner._base(session)/'restore'/nonce
    record=owner._read(base/'native-space.json')
    fields={'schema','operation','attempt','plan_sha256','archive_sha256','tool_sha256','clone_id','status',
        'resources','commands','clone_identity','origin_sha256','events_sha256','verification_sha256','receipt'}
    if (set(record)!=fields or record['schema']!='voice-staging-cohort-space-rehearsal-v1'
        or record['operation']!=session.operation or record['attempt']!=nonce or record['status']!='COMPLETE'
        or record['plan_sha256']!=attempt['plan_sha256'] or record['archive_sha256']!=owner.digest(archive)
        or record['tool_sha256']!=owner.digest(tool) or len(record['resources'])!=2
        or any(not re.fullmatch('[a-f0-9]{64}',cid) or status!='REMOVED' for cid,status in record['resources'].items())):
        raise Blocked('cohort_space_rehearsal_incomplete')
    resources=owner._read(base/'resources.json')
    clones=[r for r in resources['resources'].values() if r.get('cid')==record['clone_id'] and r.get('kind')=='postgres']
    if len(clones)!=1 or clones[0]['status'] not in ('ACTIVE','REMOVED'):
        raise Blocked('cohort_space_clone_changed')
    blobs={name:_private_generated(base/path,uid=0,limit=limit) for name,path,limit in (
        ('origin','native-origin.json',64<<20),('events','native-upgrade.jsonl',1<<20),
        ('verification','native-verify.jsonl',1<<20))}
    if any(hashlib.sha256(raw).hexdigest()!=record[name+'_sha256'] for name,raw in blobs.items()):
        raise Blocked('cohort_space_evidence_changed')
    receipt=_events(blobs['events'],session,archive,tool,record['clone_identity'])
    if receipt!=record['receipt'] or owner.private._json(blobs['verification'])!={'event':'verified-legacy24','receipt':receipt}:
        raise Blocked('cohort_space_verification_changed')
    labels=['image']+['create','inspect','run','inspect','remove','absence']*2
    if len(record['commands'])!=len(labels):raise Blocked('cohort_space_command_changed')
    for index,(ref,label) in enumerate(zip(record['commands'],labels)):
        ident='space-'+nonce+'-'+str(index)+'-'+label
        if set(ref)!={'effect_id','sha256'} or ref['effect_id']!=ident:raise Blocked('cohort_space_command_changed')
        command=owner._read(owner._base(session)/'effects'/(ident+'.json'))
        if (command.get('status')!='DONE' or command.get('effect_id')!=ident or command.get('operation')!=session.operation
            or owner.digest(command)!=ref['sha256'] or command.get('body_sha256')!=hashlib.sha256(b'').hexdigest()):
            raise Blocked('cohort_space_command_changed')
        if index in (3,9) and command['response_sha256']!=record['events_sha256' if index==3 else 'verification_sha256']:
            raise Blocked('cohort_space_command_changed')
    if owner._read(base/'native-space.json')!=record or owner._read(owner._base(session)/'restore-attempt.json')!=attempt:
        raise Blocked('cohort_space_rehearsal_changed')
    owner._phase_continuity(session,owner.load_plan(session))
    return copy.deepcopy(record)


def _retained_native_authority(session):
    """Revalidate the full qualified evidence without connecting to Space."""
    qualified=owner.load_qualified(session);plan=owner.load_plan(session);base=owner._base(session)
    archive=owner._read(base/'archive-record.json');offnode=owner._read(base/'offnode-restore.json')
    tool=load_tool(session)
    if (owner.digest(archive)!=qualified['archive_sha256']
        or owner.digest(offnode)!=qualified['offnode_restore_sha256']
        or offnode.get('operation')!=session.operation or offnode.get('plan_sha256')!=owner.digest(plan)
        or offnode.get('phase')!='offnode' or offnode.get('status')!='COMPLETE'
        or owner._read(base/'restore-attempt.json')!=offnode):
        raise Blocked('cohort_space_offnode_changed')
    rehearsal=_retained_rehearsal(session,offnode,archive,tool)
    if (offnode.get('proof',{}).get('native_space')!=rehearsal
        or offnode['proof'].get('space_native_rehearsal_sha256')!=owner.digest(rehearsal)):
        raise Blocked('cohort_space_offnode_changed')
    result={'schema':'voice-staging-space-native-authority-v1','operation':session.operation,
        'preservation_sha256':owner.digest(qualified),'tool':tool,'rehearsal':rehearsal}
    if (owner.load_qualified(session)!=qualified or load_tool(session)!=tool
        or owner._read(base/'archive-record.json')!=archive or owner._read(base/'offnode-restore.json')!=offnode
        or _retained_rehearsal(session,offnode,archive,tool)!=rehearsal):
        raise Blocked('cohort_space_authority_changed')
    return copy.deepcopy(result)


def load_native_authority(session):
    """Transfer qualified off-node evidence, never clone identity, to live code."""
    import staging_cohort_preservation_pg as pg
    original=pg.guard_space_identity(session)
    result={**_retained_native_authority(session),'original':original}
    if pg.guard_space_identity(session)!=original:
        raise Blocked('cohort_space_authority_changed')
    return result


def _running_native_job(session):
    """Only the response-owned upgrade or verifier permits exclusive polling."""
    import staging_space_native as live
    from stage_runtime import Kube
    row=session.check()
    if row.get('phase')!='prepare' or not row.get('target_sha256'):
        raise Blocked('cohort_space_upgrade_window_invalid')
    base=owner._base(session)/'space-live'
    plan=owner._read(base/'plan.json')
    kind='verify' if (base/'effects'/'verify-create.json').exists() else 'upgrade'
    created_path=base/'effects'/(kind+'-create.json');created=owner._read(created_path)
    fields={'schema','operation','binding','authority_sha256','archive_sha256','namespace_uid',
        'postgres_pvc_uid','postgres_pv_uid','tool_platform_digest','tool_config_digest',
        'config_sha256','secret_sha256','documents','node','storage_class_uid','storage_class_sha256','storage_provisioner'}
    effect_fields={'schema','operation','effect','plan_sha256','binding','argv','body_sha256','status','response'}
    if (set(plan)!=fields or plan.get('schema')!='voice-staging-space-live-plan-v1'
        or plan.get('operation')!=session.operation or plan.get('binding')!=session.binding()
        or set(created)!=effect_fields or created.get('schema')!='voice-staging-space-live-effect-v1'
        or created.get('operation')!=session.operation or created.get('effect')!=kind+'-create'
        or created.get('status')!='DONE' or created.get('binding')!=session.binding()
        or created.get('plan_sha256')!=owner.digest(plan)
        or created.get('argv')!=owner.PREFIX+['create','-f','/dev/stdin','-o','json']
        or any((base/path).exists() for path in (kind+'-closed.json','effects/'+kind+'-delete.json'))):
        raise Blocked('cohort_space_upgrade_window_invalid')
    live._live_closed(session,plan,'bind')
    if kind=='verify':live._live_closed(session,plan,'upgrade')
    response=created['response'];tool=load_tool(session)
    archive=owner._read(owner._base(session)/'archive-record.json');kube=Kube()
    node=kube.get('pod','voice-postgres-0').get('spec',{}).get('nodeName')
    storage=plan['documents']['pvc']['spec']['storageClassName']
    documents=live._live_documents(session,tool,node,storage,archive['binding']['snapshot_id'])
    if (plan['documents']!=documents or plan['node']!=node or plan['archive_sha256']!=owner.digest(archive)
        or plan['tool_platform_digest']!=tool['platform_digest'] or plan['tool_config_digest']!=tool['config_digest']
        or created['body_sha256']!=hashlib.sha256(owner.private._bytes(documents[kind])).hexdigest()
        or type(response) is not dict or set(response)!={'uid','resourceVersion','creationTimestamp','spec_sha256'}
        or not owner.guard.UUID.fullmatch(str(response['uid'])) or not response['resourceVersion']
        or not response['creationTimestamp'] or not owner.guard.SHA.fullmatch(str(response['spec_sha256']))):
        raise Blocked('cohort_space_upgrade_window_invalid')
    desired=documents[kind];job=kube.get('job',desired['metadata']['name']);meta=job.get('metadata',{})
    if (job.get('apiVersion')!='batch/v1' or job.get('kind')!='Job'
        or meta.get('name')!=desired['metadata']['name'] or meta.get('namespace')!='voice-staging'
        or meta.get('uid')!=response['uid'] or not meta.get('resourceVersion') or meta.get('deletionTimestamp')
        or meta.get('creationTimestamp')!=response['creationTimestamp']
        or any(meta.get('labels',{}).get(key)!=value for key,value in desired['metadata']['labels'].items())
        or owner.digest(job.get('spec'))!=response['spec_sha256']):
        raise Blocked('cohort_space_upgrade_job_changed')
    live._check_live_job_spec(job['spec'],desired)
    if owner._read(base/'plan.json')!=plan or owner._read(created_path)!=created:
        raise Blocked('cohort_space_upgrade_window_invalid')
    session.check()
    # Status and RV may advance while the same exact owned Job is running.
    return {'plan':plan,'kind':kind,'created':created,'job_uid':meta['uid'],'job_spec_sha256':response['spec_sha256']}


def load_native_authority_during_job(session):
    """Retain pre-submit Space identity while Go owns its exclusive session."""
    import staging_cohort_preservation_pg as pg
    window=_running_native_job(session)
    retained=_retained_native_authority(session)
    original=pg._preserved_space_identity(session)
    result={**retained,'original':original}
    plan=window['plan'];engine=original['engine']
    if (plan['authority_sha256']!=owner.digest(result) or plan['namespace_uid']!=engine['namespace_uid']
        or plan['postgres_pvc_uid']!=engine['pvc_uid'] or plan['postgres_pv_uid']!=engine['pv_uid']
        or _retained_native_authority(session)!=retained or pg._preserved_space_identity(session)!=original
        or _running_native_job(session)!=window):
        raise Blocked('cohort_space_authority_changed')
    return copy.deepcopy(result)
