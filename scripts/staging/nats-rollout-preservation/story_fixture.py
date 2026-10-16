"""Actual immutable Story writer compatibility in a disposable clean-current4 store.

Called only by the locked root preflight. No staging address, credential, PVC,
namespace or selected store is accepted by this adapter.
"""
import hashlib,json,os,re,secrets,shutil,tempfile,time
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
