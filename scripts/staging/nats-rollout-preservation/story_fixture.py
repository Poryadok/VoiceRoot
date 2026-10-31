"""Actual immutable Story writer compatibility in a disposable clean-current4 store.

Called only by the locked root preflight. No staging address, credential, PVC,
namespace or selected store is accepted by this adapter.
"""
import hashlib,json,os,re,secrets,shutil,tempfile,time,io,tarfile
from pathlib import Path
from controller import Blocked
from docker_runtime import DockerRuntime
import guard,story_witness

POSTGRES='docker.io/library/postgres@sha256:e013e867e712fec275706a6c51c966f0bb0c93cfa8f51000f85a15f9865a28cb'
IMAGE='ghcr.io/poryadok/voiceroot/story@sha256:'
LABEL='voice.story-current4.fixture'
WRITER_ERRORS=frozenset(('story_compatibility_fixture_create_story_unavailable',
    'story_compatibility_fixture_positive_failed','story_compatibility_fixture_negative_control_failed'))
CALLER='nats-rollout-preservation/story-current4-probe'
FIXTURE='nats-rollout-preservation/story_fixture.py'
SQL='src/backend/migrations/story_db/'
CONTRACT='health/text-writer/sql-defaults/missing-profile/wrong-count/schema-preservation/cleanup/current4-v1'
PRESERVED_CALLER='nats-rollout-preservation/story-restored8-probe'
PRESERVED_CALLER_SHA256='458e4e3e5e940137cddddaf42dda1c22505f92de5792600364907a343b209be0'
PRESERVED_CONTRACT='restored-data8-v1'
CATALOG="SELECT 'R|'||c.relname||'|'||c.relkind::text FROM pg_class c WHERE c.relnamespace='public'::regnamespace\nUNION ALL SELECT 'A|'||c.relname||'|'||a.attnum||'|'||a.attname||'|'||format_type(a.atttypid,a.atttypmod)||'|'||a.attnotnull||'|'||a.attidentity::text||'|'||a.attgenerated::text||'|'||COALESCE(pg_get_expr(d.adbin,d.adrelid),'') FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE c.relnamespace='public'::regnamespace AND a.attnum>0 AND NOT a.attisdropped\nUNION ALL SELECT 'C|'||conname||'|'||pg_get_constraintdef(oid) FROM pg_constraint WHERE connamespace='public'::regnamespace\nUNION ALL SELECT 'I|'||indexname||'|'||indexdef FROM pg_indexes WHERE schemaname='public'\nUNION ALL SELECT 'T|'||tgname||'|'||pg_get_triggerdef(oid) FROM pg_trigger WHERE NOT tgisinternal\nUNION ALL SELECT 'F|'||proname||'|'||pg_get_function_identity_arguments(oid)||'|'||pg_get_functiondef(oid) FROM pg_proc WHERE pronamespace='public'::regnamespace AND prokind IN ('f','p') ORDER BY 1"

def validate_writer(positive,positive_exit,negative,negative_exit):
    wanted={'status':'PASS','create_grpc_code':'OK','response_identity_type_verified':True,
        'sql_fields_verified':True,'story_count':1,'default_ttl_24h_verified':True,
        'archive_plus30d_verified':True,'missing_profile_code':'Unauthenticated',
        'negative_count_unchanged':True,'unrelated_row_count':0,'row_retained':True}
    if positive=={'status':'FAIL','phase':'create_story','grpc_code':'Unavailable'} and positive_exit==17:
        raise Blocked('story_compatibility_fixture_create_story_unavailable')
    if positive!=wanted or positive_exit!=0:raise Blocked('story_compatibility_fixture_positive_failed')
    if negative!={'status':'FAIL','phase':'wrong_expected_row_count'} or negative_exit!=17:
        raise Blocked('story_compatibility_fixture_negative_control_failed')


def validate_preserved_migration(before,after):
    """Account for every captured row and the single SQL8-created closed gate."""
    try:
        if (type(before) is not dict or type(after) is not dict
            or before.get('schema_migrations')!=[{'version':4,'dirty':False}]
            or after.get('schema_migrations')!=[{'version':8,'dirty':False}]
            or not set(before)<set(after)):raise ValueError()
        for table,rows in before.items():
            if table!='schema_migrations' and (type(rows) is not list or after[table]!=rows):raise ValueError()
        gates=after['story_reference_inventory_gate']
        if type(gates) is not list or len(gates)!=1:raise ValueError()
        gate=gates[0]
        if (set(gate)!={'singleton','owner_incarnation_id','revision','capture_started'}
            or gate['singleton'] is not True or type(gate['revision']) is not int or gate['revision']!=0
            or gate['capture_started'] is not False
            or not re.fullmatch('[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}',gate['owner_incarnation_id'])):raise ValueError()
        if any(rows!=[] for table,rows in after.items() if table not in before and table!='story_reference_inventory_gate'):
            raise ValueError()
        return gate['owner_incarnation_id']
    except (KeyError,TypeError,ValueError):raise Blocked('story_preserved_migration_data_changed') from None


def validate_preserved_writer(positive,positive_exit,negative,negative_exit,before,after,unrelated):
    """The returned identity permits exactly one added Story and no other write."""
    try:
        new_id=positive['new_story_id']
        if not isinstance(new_id,str) or not re.fullmatch('[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}',new_id):raise ValueError()
        baseline=len(before['stories'])
        wanted={'status':'PASS','contract':PRESERVED_CONTRACT,'create_grpc_code':'OK',
            'response_identity_type_verified':True,'sql_fields_verified':True,'baseline_story_count':baseline,
            'story_count':baseline+1,'new_story_id':new_id,'default_ttl_24h_verified':True,
            'archive_plus30d_verified':True,'missing_profile_code':'Unauthenticated',
            'negative_count_unchanged':True,'unrelated_row_count':unrelated,'row_retained':True}
        if (positive!=wanted or type(positive_exit) is not int or positive_exit!=0
            or type(negative_exit) is not int or negative_exit!=17
            or negative!={'status':'FAIL','phase':'wrong_expected_row_count'}
            or set(before)!=set(after) or any(row['id']==new_id for row in before['stories'])
            or sum(row['id']==new_id for row in after['stories'])!=1):raise ValueError()
        projection={table:([row for row in rows if row['id']!=new_id] if table=='stories' else rows)
                    for table,rows in after.items()}
        if projection!=before:raise ValueError()
        return new_id
    except (KeyError,TypeError,ValueError):raise Blocked('story_preserved_writer_data_changed') from None


PRESERVED_AUTHORITY_SQL="""SELECT jsonb_build_object(
 'database',(SELECT jsonb_build_object('owner',pg_get_userbyid(datdba),'acl',datacl::text,
   'collation',datcollate,'ctype',datctype) FROM pg_database WHERE datname=current_database()),
 'schema',(SELECT jsonb_build_object('owner',pg_get_userbyid(nspowner),'acl',nspacl::text)
   FROM pg_namespace WHERE nspname='public'),
 'relations',(SELECT coalesce(jsonb_object_agg(c.relname,jsonb_build_object('owner',pg_get_userbyid(c.relowner),
   'acl',c.relacl::text,'rls',c.relrowsecurity,'force_rls',c.relforcerowsecurity,
   'replica_identity',c.relreplident,'options',c.reloptions)),'{}'::jsonb)
   FROM pg_class c WHERE c.relnamespace='public'::regnamespace),
 'policies',(SELECT coalesce(jsonb_agg(to_jsonb(p) ORDER BY p.tablename,p.policyname),'[]'::jsonb)
   FROM pg_policies p WHERE schemaname='public'),
 'sequences',(SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY s.sequencename),'[]'::jsonb)
   FROM pg_sequences s WHERE schemaname='public'),
 'functions',(SELECT coalesce(jsonb_agg(jsonb_build_object('name',p.proname,
   'arguments',pg_get_function_identity_arguments(p.oid),'owner',pg_get_userbyid(p.proowner),
   'acl',p.proacl::text,'security_definer',p.prosecdef,'config',p.proconfig)
   ORDER BY p.proname,pg_get_function_identity_arguments(p.oid)),'[]'::jsonb)
   FROM pg_proc p WHERE pronamespace='public'::regnamespace),
 'defaults',(SELECT coalesce(jsonb_agg(jsonb_build_object('owner',pg_get_userbyid(d.defaclrole),
   'schema',n.nspname,'type',d.defaclobjtype,'acl',d.defaclacl::text)
   ORDER BY pg_get_userbyid(d.defaclrole),n.nspname,d.defaclobjtype),'[]'::jsonb)
   FROM pg_default_acl d LEFT JOIN pg_namespace n ON n.oid=d.defaclnamespace))"""


def preserved_snapshot(sql):
    """One read-only repeatable-read snapshot of all public rows and authority.

    The clone remains isolated and quiescent while the table names are read.
    Full personal/actor/intent/inventory bytes stay in memory; durable receipts
    contain their digests and the already authenticated dump remains the source.
    """
    names=json.loads(sql("SELECT coalesce(json_agg(tablename ORDER BY tablename),'[]'::json) FROM pg_tables WHERE schemaname='public'"))
    if (not isinstance(names,list) or not names or len(names)>128 or len(set(names))!=len(names)
        or any(not isinstance(name,str) or not re.fullmatch('[a-z_][a-z0-9_]{0,62}',name) for name in names)):
        raise Blocked('story_preserved_table_inventory_invalid')
    pairs=[]
    for name in names:
        pairs.extend(("'"+name+"'", "(SELECT coalesce(jsonb_agg(x ORDER BY x::text),'[]'::jsonb) FROM (SELECT to_jsonb(r) x FROM public.\""+name+"\" r) q)"))
    structure=CATALOG.removesuffix(' ORDER BY 1')
    query=("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY; SET LOCAL statement_timeout='60s'; "
        "SELECT jsonb_build_object('rows',jsonb_build_object("+','.join(pairs)+"),'structure',"
        "(SELECT jsonb_agg(v ORDER BY v) FROM ("+structure+") c(v)),'authority',("+PRESERVED_AUTHORITY_SQL+")); ROLLBACK;")
    result=json.loads(sql(query))
    if (type(result) is not dict or set(result)!={'rows','structure','authority'}
        or set(result['rows'])!=set(names) or any(type(rows) is not list for rows in result['rows'].values())
        or type(result['structure']) is not list or type(result['authority']) is not dict):
        raise Blocked('story_preserved_snapshot_invalid')
    return result


def retained_authority(before,after,reference=None):
    """New schema objects may be added; captured owners/ACL/RLS may not drift."""
    try:
        for field in ('database','schema','defaults'):
            if before[field]!=after[field]:raise ValueError()
        for field in ('policies','sequences','functions'):
            if any(value not in after[field] for value in before[field]):raise ValueError()
            extra=[value for value in after[field] if value not in before[field]]
            if extra and (reference is None or any(value not in reference[field] for value in extra)):raise ValueError()
        if any(after['relations'].get(name)!=value for name,value in before['relations'].items()):raise ValueError()
        if reference is not None and any(reference['relations'].get(name)!=value for name,value in after['relations'].items()
                                        if name not in before['relations']):raise ValueError()
    except (KeyError,TypeError,ValueError):raise Blocked('story_preserved_authority_changed') from None


def prove_preserved(session,postgres,admin,before,reference,resources,journal,binding,metadata):
    """Exercise the actual selected/recovery images on the captured migrated DB.

    Called inside the fixed whole-cohort restore before its owned cleanup. No
    result is qualified until the enclosing restore receipt proves cleanup.
    """
    import staging_cohort_preservation as owner
    import staging_cohort_preservation_runtime as runtime
    import story_media_backup_restore as restore
    loaded=owner.story_restore_inputs(session,binding,metadata,postgres)
    if loaded is None:raise Blocked('cohort_story_restore_profile_required')
    context,caller=loaded
    def unchanged():
        if owner.story_restore_inputs(session,binding,metadata,postgres)!=(context,caller):
            raise Blocked('cohort_story_restore_inputs_changed')
    def sql(query):return restore._psql(postgres,admin,'story_db',query)
    migrated=preserved_snapshot(sql)
    gate=validate_preserved_migration(before['rows'],migrated['rows'])
    retained_authority(before['authority'],migrated['authority'],reference['authority'])
    if migrated['structure']!=reference['structure'] or set(migrated['rows'])!=set(reference['rows']):
        raise Blocked('story_preserved_target_catalog_changed')
    # Preserve the actual captured runtime login and its restored grants. Only
    # its password is replaced inside this disposable clone; the app never uses
    # the clone administrator as a substitute for the admitted runtime role.
    runtime_user=context['runtime_user'];password=secrets.token_hex(24)
    sql('ALTER ROLE "'+runtime_user+'" PASSWORD \''+password+'\'')
    dsn='postgres://'+runtime_user+':'+password+'@127.0.0.1:5432/story_db?sslmode=disable'
    def create(kind,image,baseline=0,unrelated=0,mode='positive'):
        name='voice-media-restore-'+secrets.token_hex(16);labels={'voice.test.owner':name}
        nonce=journal.begin(kind,name,labels)
        try:
            args=runtime.story_create(name,postgres,image,kind,dsn,baseline,unrelated,mode)
            cid=restore._docker(*args).decode().strip()
            if not re.fullmatch('[a-f0-9]{64}',cid):raise Blocked('story_preserved_container_unknown')
            restore._docker('inspect',cid);journal.created(kind,nonce,cid,labels)
            resources.append((kind,nonce,cid,name,[]))
            return cid
        except BaseException:
            journal.uncertain(kind,nonce);raise
    def invoke(image,baseline,unrelated,mode):
        cid=create('story-probe',image,baseline,unrelated,mode)
        payload=io.BytesIO()
        with tarfile.open(fileobj=payload,mode='w',format=tarfile.USTAR_FORMAT) as archive:
            entry=tarfile.TarInfo('voice-story-restored8-probe');entry.mode=0o555;entry.size=len(caller)
            archive.addfile(entry,io.BytesIO(caller))
        restore._docker('cp','-',cid+':/tmp/',input=payload.getvalue())
        restore._docker('start',cid)
        status=restore._docker('wait',cid,timeout=60).decode().strip()
        output=restore._docker('logs',cid)
        if status not in ('0','17'):raise Blocked('story_preserved_probe_exit_invalid')
        return json.loads(output),int(status)
    results=[];current=migrated
    for key,image in context['images'].items():
        unchanged();image_id=restore._check_image(image)
        import source_authority
        chain=source_authority.immutable_image_identity(image,time.monotonic()+120)
        child=chain.get('manifest_image','').removeprefix(IMAGE)
        story_witness.digest(child)
        if chain.get('requested_image')!=image or chain.get('config_sha256')!=image_id.removeprefix('sha256:'):
            raise Blocked('story_preserved_image_chain_changed')
        app=create('story',image_id)
        restore._docker('start',app)
        binary=restore._docker('exec',app,'sha256sum','/service').decode().split()[0]
        story_witness.digest(binary)
        health=json.loads(restore._docker('exec',app,'sh','-ec',runtime.STORY_HEALTH,timeout=60))
        if health!={'service':'story','status':'ok'}:raise Blocked('story_preserved_health_invalid')
        baseline=len(current['rows']['stories'])
        unrelated=sum(len(current['rows'][name]) for name in ('story_views','story_reactions','highlights','highlight_stories','story_media_deletion_outbox'))
        invalid,invalid_exit=invoke(image_id,baseline,unrelated,'invalid')
        if (invalid!={'status':'FAIL','phase':'baseline_stories_input'} or invalid_exit!=17
            or preserved_snapshot(sql)!=current):raise Blocked('story_preserved_invalid_input_mutated')
        positive,positive_exit=invoke(image_id,baseline,unrelated,'positive')
        after=preserved_snapshot(sql)
        negative,negative_exit=invoke(image_id,baseline,unrelated,'wrong')
        if preserved_snapshot(sql)!=after:raise Blocked('story_preserved_negative_mutated')
        new_id=validate_preserved_writer(positive,positive_exit,negative,negative_exit,current['rows'],after['rows'],unrelated)
        if after['structure']!=current['structure'] or after['authority']!=current['authority']:
            raise Blocked('story_preserved_writer_catalog_changed')
        restore._docker('stop','--time','5',app)
        if preserved_snapshot(sql)!=after:raise Blocked('story_preserved_stop_mutated')
        results.append({'image':image,'child_sha256':child,'config_sha256':image_id.removeprefix('sha256:'),'binary_sha256':binary,
            'contract':PRESERVED_CONTRACT,'positive':positive,'positive_exit':positive_exit,
            'negative':negative,'negative_exit':negative_exit,'invalid_input':invalid,'invalid_exit':invalid_exit,
            'before_sha256':owner.digest(current),'after_sha256':owner.digest(after),'allowed_new_story_id':new_id})
        current=after;unchanged()
    if not results:raise Blocked('story_preserved_writer_required')
    return {'schema':'voice-story-preserved-target8-proof-v1','input':context,
        'baseline_sha256':owner.digest(before),'target_sha256':owner.digest(migrated),
        'catalog_sha256':owner.digest(migrated['structure']),'authority_sha256':owner.digest(migrated['authority']),
        'baseline_rows_sha256':owner.digest(before['rows']),'target_rows_sha256':owner.digest(migrated['rows']),
        'table_count':len(migrated['rows']),'created_gate_owner_incarnation':gate,
        'writer_results':results,'final_sha256':owner.digest(current)}

def fixture_failure(error,phase,*,invalid=False):
    from deployment_report import capture_failure
    result=error if not invalid and error.args in [(label,) for label in WRITER_ERRORS] else Blocked('story_compatibility_fixture_'+phase+('_invalid' if invalid else '_blocked'))
    if hasattr(error,'diagnostic'):result.diagnostic=error.diagnostic.copy()
    capture_failure(result) # Snapshot the failing action before finally runs Docker cleanup.
    return result

def prove(child,config,source,code,binding,unchanged):
    """Return evidence only after real probe PASS and complete owned cleanup."""
    import actor_root
    story_witness.digest(child);story_witness.digest(config)
    source=Path(source);code=Path(code)
    caller=code/CALLER;raw=caller.read_bytes()
    if (caller.is_symlink() or not raw.startswith(b'\x7fELF') or not 0<len(raw)<=32<<20
        or hashlib.sha256(raw).hexdigest()!=binding.get(CALLER)
        or hashlib.sha256((code/FIXTURE).read_bytes()).hexdigest()!=binding.get(FIXTURE)):
        raise Blocked('story_compatibility_fixture_code_invalid')
    migrations={name:(source/SQL/name).read_bytes() for name in actor_root.STORY_UP}
    if {n:hashlib.sha256(v).hexdigest() for n,v in migrations.items()}!=actor_root.STORY_UP:
        raise Blocked('story_compatibility_fixture_schema_invalid')
    base=guard.ROOT/'story-fixtures'
    try:base.mkdir(mode=0o700)
    except FileExistsError:pass
    story_witness.directory(base)
    work=Path(tempfile.mkdtemp(prefix='current4-',dir=base));nonce=secrets.token_hex(16)
    probe=work/'probe';probe.write_bytes(raw);probe.chmod(0o555)
    # Only this executable is mounted; scratch SQL/database stay inside tmpfs.
    owned=[];deadline=time.monotonic()+300;phase='admission';result=None
    run=DockerRuntime._run
    def docker(args,timeout=30):
        if time.monotonic()>=deadline:raise Blocked('story_compatibility_fixture_timeout')
        return run(args,timeout=min(timeout,max(0.1,deadline-time.monotonic())),limit=1<<20)
    def start(part,image,args):
        name='voice-story-current4-'+nonce+'-'+part;owned.append(name)
        identifier=docker(['run','-d','--name',name,'--label',LABEL+'='+nonce,
            '--read-only','--cap-drop=ALL','--security-opt=no-new-privileges',
            '--pids-limit=128','--memory=512m','--cpus=1','--tmpfs','/tmp:mode=1777',*args,image])
        if not re.fullmatch('[a-f0-9]{64}',identifier):raise Blocked('story_compatibility_fixture_container_invalid')
        return identifier
    try:
        unchanged()
        phase='image'
        image=IMAGE+child
        for ref in (POSTGRES,image):
            try:docker(['image','inspect',ref,'--format','{{.Id}}'])
            except Blocked:docker(['pull',ref],timeout=120)
        metadata=json.loads(docker(['image','inspect',image,'--format','{{json .}}']))
        if metadata.get('Id')!='sha256:'+config or metadata.get('Architecture')!='amd64' or metadata.get('Os')!='linux':
            raise Blocked('story_compatibility_fixture_image_invalid')
        revision=(metadata.get('Config',{}).get('Labels') or {}).get('org.opencontainers.image.revision')
        if revision is not None:story_witness.digest(revision,40)
        phase='postgres'
        pg=start('pg',POSTGRES,['--network','none','--user','70:70',
            '--tmpfs','/var/lib/postgresql/data:uid=70,gid=70,mode=0700',
            '--tmpfs','/var/run/postgresql:uid=70,gid=70,mode=0700',
            '-e','POSTGRES_USER=voice','-e','POSTGRES_PASSWORD=voice','-e','POSTGRES_DB=story_db'])
        for _ in range(45):
            try:docker(['exec',pg,'pg_isready','-U','voice','-d','story_db'],timeout=5);break
            except Blocked:time.sleep(1)
        else:raise Blocked('story_compatibility_fixture_postgres_not_ready')
        def sql(query):
            return docker(['exec',pg,'psql','-U','voice','-d','story_db','-X','-A','-t','-q','-v','ON_ERROR_STOP=1','-c',query],timeout=30)
        phase='schema'
        sql('CREATE TABLE schema_migrations(version bigint NOT NULL PRIMARY KEY,dirty boolean NOT NULL)')
        for version,(name,migration) in enumerate(sorted(migrations.items()),1):
            sql('INSERT INTO schema_migrations VALUES(1,true)' if version==1 else 'UPDATE schema_migrations SET version='+str(version)+',dirty=true')
            if sql('SELECT version,dirty FROM schema_migrations')!=str(version)+'|t':raise Blocked('story_compatibility_fixture_schema_invalid')
            sql(migration.decode('utf-8'));sql('UPDATE schema_migrations SET dirty=false')
        def schema():
            if sql('SELECT version,dirty FROM schema_migrations')!='4|f':raise Blocked('story_compatibility_fixture_schema_invalid')
            return hashlib.sha256(sql(CATALOG).encode()).hexdigest()
        before=schema()
        phase='story'
        app=start('app',image,['--network','container:'+pg,'--user','65532:65532',
            '-e','DATABASE_URL=postgres://voice:voice@127.0.0.1:5432/story_db?sslmode=disable',
            '-e','LISTEN_ADDR=:8080','-e','STORY_GRPC_LISTEN=:9090',
            '-e','USER_GRPC_ADDR=127.0.0.1:9091'])
        binary=docker(['exec',app,'sha256sum','/service']).split()[0];story_witness.digest(binary)
        for _ in range(45):
            try:
                if json.loads(docker(['exec',app,'wget','-q','-O','-','http://127.0.0.1:8080/health'],timeout=5))=={'service':'story','status':'ok'}:break
            except (Blocked,ValueError):pass
            time.sleep(1)
        else:raise Blocked('story_compatibility_fixture_health_failed')
        phase='writer'
        def invoke(part,negative=False):
            name='voice-story-current4-'+nonce+'-'+part;owned.append(name)
            # This static shell envelope exposes exact exit without dropping the
            # canonical probe's negative control to a subprocess exception.
            command='/fixture-probe'+(' wrong' if negative else '')+'; status=$?; printf "\\nEXIT=%s\\n" "$status"; exit 0'
            output=docker(['run','--name',name,'--label',LABEL+'='+nonce,
                '--network','container:'+pg,'--user','65532:65532','--read-only','--cap-drop=ALL',
                '--security-opt=no-new-privileges','--pids-limit=64','--memory=128m','--cpus=1',
                '--mount','type=bind,src='+str(probe)+',dst=/fixture-probe,readonly',
                '-e','FIXTURE_STORY=postgres://voice:voice@127.0.0.1:5432/story_db?sslmode=disable',
                '--entrypoint','/bin/sh',image,'-c',command],timeout=35)
            value,separator,exit_code=output.rpartition('\nEXIT=')
            if not separator or not exit_code.isdigit():raise Blocked('story_compatibility_fixture_probe_invalid')
            return json.loads(value.strip()),int(exit_code)
        positive,positive_exit=invoke('positive');negative,negative_exit=invoke('negative',True)
        validate_writer(positive,positive_exit,negative,negative_exit)
        phase='schema_after';after=schema()
        if before!=after or sql('SELECT count(*) FROM stories')!='1':raise Blocked('story_compatibility_fixture_schema_changed')
        unchanged()
        evidence={'positive':positive,'negative':negative,'catalog_before':before,'catalog_after':after}
        canonical=lambda row:hashlib.sha256(story_witness.encoded(row)).hexdigest()
        result={'schema':'voice-story-current4-automatic-witness-v1','repository':'Poryadok/VoiceRoot','service':'story','platform':'linux/amd64',
            'content':{'child_sha256':child,'config_sha256':config,'binary_sha256':binary,'build_source_sha':revision},
            'component':{'source_sha':revision,'manifest_sha256':child},
            'database':{'name':'story_db','version':4,'dirty':False,'up_sha256':actor_root.STORY_UP,
                'catalog_before_sha256':before,'catalog_after_sha256':after},
            'fixture':{'contract_sha256':hashlib.sha256(CONTRACT.encode()).hexdigest(),
                'input_sha256':canonical({'child':child,'config':config,'migrations':actor_root.STORY_UP}),
                'source_sha256':binding[FIXTURE],'result_sha256':canonical(evidence),
                'cases':['health','text_writer','sql_defaults','missing_profile','wrong_count','schema_preservation','cleanup'],
                'positive_exit':positive_exit,'negative_exit':negative_exit,'missing_profile_code':'Unauthenticated','row_count':1,'cleanup_verified':False},
            'tools':{'postgres':POSTGRES,'caller':image},
            'execution':{'helper_binding_sha256':canonical(binding),'caller_sha256':binding[CALLER],'fixture_sha256':binding[FIXTURE]}}
    except Blocked as error:
        captured=fixture_failure(error,phase)
        if captured is error:raise
        raise captured from error
    except (OSError,ValueError,KeyError,TypeError) as error:
        raise fixture_failure(error,phase,invalid=True) from None
    finally:
        cleanup=True
        for name in reversed(owned):
            try:
                ids=run(['ps','-a','--filter','name=^/'+name+'$','--format','{{.ID}}'],timeout=10)
                if ids:
                    label=run(['inspect',name,'--format','{{index .Config.Labels "'+LABEL+'"}}'],timeout=10)
                    if label!=nonce:raise Blocked('story_compatibility_fixture_cleanup_owner_invalid')
                    run(['rm','-f',name],timeout=15)
                    if run(['ps','-a','--filter','name=^/'+name+'$','--format','{{.ID}}'],timeout=10):cleanup=False
            except Blocked:cleanup=False
        try:shutil.rmtree(work)
        except OSError:cleanup=False
        if not cleanup or work.exists():raise Blocked('story_compatibility_fixture_cleanup_failed')
    result['fixture']['cleanup_verified']=True
    unchanged()
    return story_witness.validate(result)
