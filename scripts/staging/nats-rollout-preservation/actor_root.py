"""Affected existing service actors: approved ACL, original mounted authority."""
import base64,copy,json,re,time
from pathlib import Path
from urllib.parse import urlsplit,parse_qs,unquote
import actor_auth,actor_verification
from bootstrap_root import secret_bytes,hash_bytes,ACCOUNT_SHA
from controller import Blocked

ROLES={'realtime','space','social','story','messaging'}
STORY_IMAGES={
 'ghcr.io/poryadok/voiceroot/story@sha256:8b142297965b3121fc19e0d970b137ad4ff8c8afa2c79eb49e490d59324f3c5f':('900de5490b7a688f4cd434f332ff4fc1d92d4cc9168d3a1f522618c749fdcbb7','46192e1ca61e898df57a2aac07657e574b9d209d91106dc0e18ef8a735684970','ab957f054b081573447888ed9550323bd413cbbf'),
 'ghcr.io/poryadok/voiceroot/story@sha256:900de5490b7a688f4cd434f332ff4fc1d92d4cc9168d3a1f522618c749fdcbb7':('900de5490b7a688f4cd434f332ff4fc1d92d4cc9168d3a1f522618c749fdcbb7','46192e1ca61e898df57a2aac07657e574b9d209d91106dc0e18ef8a735684970','ab957f054b081573447888ed9550323bd413cbbf'),
 'ghcr.io/poryadok/voiceroot/story@sha256:09c6481710665e056d5b29c092189bff899a78e6c457be5a521edec12bbb0fa9':('09c6481710665e056d5b29c092189bff899a78e6c457be5a521edec12bbb0fa9','56c0f5981f838b86a22595b8848bf2a960c8d060d41d4f200ab59d6e30111016','e0d635a8357bc9fbfa8b29202c33ac0e81b44644')}
ACL='deploy/nats/acl-intent.yaml'
STORY_UP={
    '000001_init.up.sql':'5d4957b2d80de5c28623109572966753567a1b5419caa451e012d5179f12e220',
    '000002_visibility_audience.up.sql':'63b9d54da053d0e9143bf09d993c123d7d479423452b92eb61f247e9e4a2ef16',
    '000003_hidden_from_feed.up.sql':'fe5396f4cb89865f12a31e1e79e7a993f9f9d4346f692539e9ab087b9776be0c',
    '000004_archive_purge_outbox.up.sql':'09117f527f298eceecbc7affbe940dc2d419c927e2ef349738c0e6229b01ecf1'}
STORY_P2_UP={**STORY_UP,
    '000005_lfp_consent_journal.up.sql':'a0b23aa978ae94b7fe74cd5b96286e54b0a5f7da04d634565cc9f0e92423f2be',
    '000006_lfp_actor_provenance.up.sql':'073cba0ddf42ddc74072546b095ac9799e836464fb0a3c237b865e5c5bcc73f8',
    '000007_media_reference_intents.up.sql':'80216c1e8e27fbb0d7728e06304876e72b8cfddf782b92ea85f31c760c84426f',
    '000008_reference_inventory.up.sql':'18c405c4ce24c323f9005a6634000e8341e4d5c748c86894c2628919ac89236c'}
def fail():raise Blocked('existing_service_actor_authority_changed')
def identity(row):return {k:row['metadata'][k] for k in ('name','uid','resourceVersion')}
def public_user(credentials,role):
    try:
        token=credentials.split(b'-----BEGIN NATS USER JWT-----',1)[1].split(b'------END NATS USER JWT------',1)[0].strip()
        payload=token.split(b'.')[1];claim=json.loads(base64.urlsafe_b64decode(payload+b'='*((-len(payload))%4)))
        public=claim['sub']
        if claim.get('name')!='voice-'+role:fail()
    except (IndexError,KeyError,ValueError,UnicodeError):fail()
    if not isinstance(public,str) or not re.fullmatch(r'U[A-Z2-7]{55}',public):fail()
    return public # Merely identity selection; the private verifier authenticates it.

def source_grants(raw):
    """The canonical flat ACL subset only; no YAML tags/aliases/executable input."""
    if not 0<len(raw)<=1<<20:fail()
    services={};section=None;role=None;field=None;version=False;sections=set()
    for line in raw.decode('utf-8').splitlines():
        if not line.strip() or line.lstrip().startswith('#'):continue
        if line=='version: 1' and not version:version=True;continue
        if line in ('services:','bootstrap:'):
            section=line[:-1];role=None;field=None
            if section in sections:fail()
            sections.add(section)
            if section=='bootstrap':services['bootstrap']={};role='bootstrap'
            continue
        match=re.fullmatch(r'  ([a-z][a-z0-9-]{0,63}):',line)
        if match and section=='services':
            role=match[1]
            if role in services:fail()
            services[role]={};field=None;continue
        indent='  ' if section=='bootstrap' else '    '
        if role is not None and line==indent+'no_response: true':
            if 'no_response' in services[role]:fail()
            services[role]['no_response']=True;field=None;continue
        if role is not None and line in (indent+'publish:',indent+'subscribe:'):
            field=line.strip()[:-1]
            if field in services[role]:fail()
            services[role][field]=[];continue
        match=re.fullmatch(re.escape(indent)+r'(publish|subscribe): \[([A-Za-z0-9_.$*>-]+(?:, [A-Za-z0-9_.$*>-]+)*)\]',line)
        if match and role is not None:
            field=match[1]
            if field in services[role]:fail()
            services[role][field]=match[2].split(', ');field=None;continue
        match=re.fullmatch(re.escape(indent)+r'  - ([A-Za-z0-9_.$*>-]+)',line)
        if match and role is not None and field in ('publish','subscribe'):
            services[role][field].append(match[1]);continue
        fail()
    if not version or any(set(grant)!={'publish','subscribe','no_response'} for grant in services.values()):fail()
    return services

def mounted_role(kube,stage,role,secret_name):
    """Bind the actual captured leaf mount, not an optional preflight hint."""
    try:
        name='voice-'+role;row=stage.snapshots[name];metadata=row['metadata']
        if (metadata['name']!=name or metadata['namespace']!='voice-staging'
            or metadata['uid']!=stage.expected['deployment_uids'][name]):fail()
        current=kube.get('deployment',name)
        if ({k:current['metadata'][k] for k in ('name','uid','namespace')}!={k:metadata[k] for k in ('name','uid','namespace')}
            or current['spec']!=row['spec']):fail() # controller status/RV are observations, never mutation authority
        if hasattr(stage,'marker') and stage.marker['data']['phase']!='active':stage.owned_marker()
        spec=row['spec']['template']['spec'];key=role+'.creds'
        leaves=[c for c in spec['containers'] if c.get('name')=='nats-leaf']
        if len(leaves)!=1:fail()
        leaf=leaves[0];path='/var/run/nats/creds/'+key
        if [e for e in leaf.get('env',[]) if e.get('name')=='NATS_CREDS']!=[{'name':'NATS_CREDS','value':path}]:fail()
        mounts=[m for m in leaf.get('volumeMounts',[]) if m.get('name')=='nats-service-creds' or m.get('mountPath')==path]
        if mounts!=[{'name':'nats-service-creds','mountPath':path,'subPath':key,'readOnly':True}]:fail()
        volumes=[v for v in spec.get('volumes',[]) if v.get('name')=='nats-service-creds']
        wanted={'name':'nats-service-creds','secret':{'secretName':secret_name,'defaultMode':256,'items':[{'key':key,'path':key}]}}
        if volumes!=[wanted]:fail()
    except (KeyError,TypeError,AttributeError):fail()
    return {'deployment':{k:metadata[k] for k in ('name','uid','namespace')},'container':'nats-leaf','key':key,'path':path,
        'mount_sha256':hash_bytes(json.dumps({'env':{'name':'NATS_CREDS','value':path},'mount':mounts[0],'volume':wanted},sort_keys=True,separators=(',',':')).encode())}

def story_content(image,automatic=None):
    """Immutable alias role chain; component witness is not current CI approval."""
    import source_authority
    if not isinstance(image,str) or not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/story@sha256:[a-f0-9]{64}',image):fail()
    chain=source_authority.immutable_image_identity(image,time.monotonic()+120)
    if chain.get('requested_image')!=image:fail()
    child=chain.get('manifest_image','').removeprefix('ghcr.io/poryadok/voiceroot/story@sha256:')
    config=chain.get('config_sha256')
    matches={revision for a,b,revision in STORY_IMAGES.values() if (a,b)==(child,config)}
    if len(matches)==1:return child,config,matches.pop()
    if matches:fail()
    import story_witness
    authority=story_witness.lookup(child,config) if automatic is None else automatic(child,config)
    return child,config,authority['component_source_sha']

STORY_SCHEMA_SQL="""BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';
SELECT json_build_object('database',current_database(),
 'username',current_user,
 'version',(SELECT version FROM schema_migrations),
 'dirty',(SELECT dirty FROM schema_migrations));
ROLLBACK;
"""
STORY_SCHEMA_SCRIPT=('export PGPASSWORD="$POSTGRES_PASSWORD"; exec psql -X -A -t -q -v ON_ERROR_STOP=1 '
    '-h 127.0.0.1 -U "$POSTGRES_USER" -d story_db -c "$1"')
STORY_SCHEMA_ARGS=('exec','voice-postgres-0','-c','postgres','--','sh','-ceu',STORY_SCHEMA_SCRIPT,'--',STORY_SCHEMA_SQL)


def story_schema(kube,source,stage):
    """Fixed compatible candidate schema; isolated fixture PASS is not input."""
    folder=Path(source)/'src/backend/migrations/story_db'
    hashes={path.name:hash_bytes(path.read_bytes()) for path in folder.glob('*.up.sql')}
    if hashes!=STORY_UP:fail()
    return _story_database(kube,stage,hashes)

def story_schema_p2(kube,source,stage,plan,binding):
    """Describe exact candidate8 and actual baseline4, without qualifying8.

    The caller authenticates this binding to the protected source receipt and
    producer inputs. A descriptor is never final migration or image admission.
    """
    canonical=lambda value:hash_bytes(json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False).encode())
    try:
        fields={'operation','source_sha','source_capture_sha256','helper_sha256','context_sha256','migration_plan_sha256'}
        if type(binding) is not dict or set(binding)!=fields:fail()
        for key,length in (('operation',12),('source_sha',40),('source_capture_sha256',64),
                           ('helper_sha256',64),('context_sha256',64),('migration_plan_sha256',64)):
            if not isinstance(binding[key],str) or not re.fullmatch('[a-f0-9]{'+str(length)+'}',binding[key]):fail()
        if type(plan) is not list or canonical(plan)!=binding['migration_plan_sha256']:fail()
        bound=copy.deepcopy(binding)
        if stage.operation!=bound['operation']:fail()
        rows=[row for row in plan if row['database']=='story']
        if len(rows)!=1:fail()
        selected=rows[0];folder=Path(source)/'src/backend/migrations/story_db'
        names=set(STORY_P2_UP)|{name.replace('.up.sql','.down.sql') for name in STORY_P2_UP if not name.startswith('000001_')}
        paths={path.name:path for path in folder.iterdir() if path.suffix=='.sql'}
        if set(paths)!=names or any(path.is_symlink() or not path.is_file() for path in paths.values()):fail()
        files={name:path.read_bytes().decode('utf-8') for name,path in paths.items()}
        hashes={name:hash_bytes(raw.encode()) for name,raw in files.items()}
        ups={name:sha for name,sha in hashes.items() if name.endswith('.up.sql')}
        if ups!=STORY_P2_UP or selected['files']!=files or selected['hashes']!=hashes:fail()
        image=stage.old_images['voice-story/story']
        if not isinstance(image,str) or not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/story@sha256:[a-f0-9]{64}',image):fail()
    except (KeyError,TypeError,ValueError,UnicodeError,OSError,AttributeError):fail()
    baseline=_story_database(kube,stage,dict(STORY_UP))
    # Fresh DB/secret/config observations above cannot authorize a different
    # source plan or retained image introduced while SQL was being observed.
    if (binding!=bound or canonical(plan)!=bound['migration_plan_sha256']
        or stage.operation!=bound['operation'] or stage.old_images['voice-story/story']!=image):fail()
    try:
        after={path.name:path for path in folder.iterdir() if path.suffix=='.sql'}
        if (set(after)!=names or any(path.is_symlink() or not path.is_file() for path in after.values())
            or {name:path.read_bytes().decode('utf-8') for name,path in after.items()}!=files):fail()
    except (OSError,UnicodeError):fail()
    return {'schema':'voice-story-baseline4-target8-composition-v1',
        'baseline':baseline,'candidate':{'version':8,'up_sha256':ups,'migration_files_sha256':canonical(hashes)},
        'retained_image':image,'binding':bound}

def _story_database(kube,stage,hashes):
    """Shared actual clean4/DSN/PG/secret checks; source profiles stay separate."""
    import space_authority
    try:
        snapshot=stage.snapshots['voice-story'];metadata=snapshot['metadata']
        if (metadata['name']!='voice-story' or metadata['namespace']!='voice-staging'
            or metadata['uid']!=stage.expected['deployment_uids']['voice-story']):fail()
        containers=snapshot['spec']['template']['spec']['containers']
        selected=[row for row in containers if row['name']=='story']
        if len(selected)!=1:fail()
        rows=[row for row in selected[0].get('env',[]) if row.get('name')=='DATABASE_URL']
        wanted={'name':'DATABASE_URL','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'STORY_DATABASE_URL'}}}
        if rows!=[wanted]:fail()
        database=kube.get('secret','voice-app-secrets');config=kube.get('configmap','voice-app-config')
        raw=secret_bytes(database,'STORY_DATABASE_URL')
        if not 1<=len(raw)<=8192:fail()
        url=urlsplit(raw.decode('utf-8'))
        user=config['data']['POSTGRES_USER'];password=secret_bytes(database,'POSTGRES_PASSWORD').decode('utf-8')
        if (not re.fullmatch(r'[a-z_][a-z0-9_]{0,31}',user) or not password
            or url.scheme not in ('postgres','postgresql') or url.port not in (None,5432)
            or url.hostname not in ('voice-postgres','voice-postgres.voice-staging.svc','voice-postgres.voice-staging.svc.cluster.local')
            or url.path!='/story_db' or url.fragment or parse_qs(url.query,strict_parsing=True)!={'sslmode':['disable']}
            or unquote(url.username or '')!=user or unquote(url.password or '')!=password):fail()
    except (KeyError,TypeError,ValueError,UnicodeError,AttributeError):fail()
    authority=space_authority.capture(kube)
    observed=kube.run(list(STORY_SCHEMA_ARGS),timeout=40)
    if (not isinstance(observed,dict) or set(observed)!={'database','username','version','dirty'}
        or observed['database']!='story_db' or type(observed['version']) is not int
        or observed['version']!=4 or observed['dirty'] is not False or observed['username']!=user):fail()
    if space_authority.capture(kube)!=authority:fail()
    if kube.get('secret','voice-app-secrets')!=database or kube.get('configmap','voice-app-config')!=config:fail()
    return {'authority':authority,'observation':{k:v for k,v in observed.items() if k!='username'},
        'migration_hashes':hashes,'database_secret':identity(database),'database_config':identity(config),
        'database_url_sha256':hash_bytes(raw),'postgres_user_sha256':hash_bytes(user.encode()),
        'database_reference_sha256':hash_bytes(json.dumps(wanted,sort_keys=True,separators=(',',':')).encode())}

def preflight(kube,source,code,binding,stage,services,decoder,*,story_composition=None,installation=None):
    from resource_claim import require_installation
    installation=require_installation(installation)
    roles=sorted(ROLES.intersection(services))
    if not roles:return None
    if stage.expected['generation']!=installation.generation:fail()
    generation=stage.expected['generation']
    operator=kube.get('secret','voice-nats-operator-'+generation)
    account=secret_bytes(operator,'account.public').decode().strip()
    if hash_bytes(account.encode())!=installation.account_sha256:fail()
    def schema_read():
        if story_composition is None:return story_schema(kube,source,stage)
        return story_schema_p2(kube,source,stage,*story_composition)
    schema=schema_read() if 'story' in roles else None
    candidate=None
    if schema is not None:
        image=stage.old_images.get('voice-story/story')
        import story_fixture,story_witness,guard
        snapshot=stage.snapshots['voice-story'];marker=kube.get('configmap',guard.MARKER)
        def story_unchanged():
            current=kube.get('deployment','voice-story')
            if (kube.get('secret','voice-nats-operator-'+generation)!=operator
                or {k:current['metadata'].get(k) for k in ('name','namespace','uid')}!={k:snapshot['metadata'].get(k) for k in ('name','namespace','uid')}
                or current['spec']!=snapshot['spec'] or kube.get('configmap',guard.MARKER)!=marker
                or schema_read()!=schema):fail()
        def automatic(child,config):
            return story_witness.ensure_content(child,config,
                lambda:story_fixture.prove(child,config,source,code,binding,story_unchanged),story_unchanged)
        child,config,revision=story_content(image,automatic)
        candidate={'schema':('voice-reviewed-story-compatible-pair-v1' if story_composition is None
            else 'voice-reviewed-story-baseline4-compatible-pair-v1'),'image':image,'child_sha256':child,
            'config_sha256':config,'component_source_sha':revision,'schema_sha256':hash_bytes(json.dumps(schema,sort_keys=True,separators=(',',':')).encode())}
        import story_witness
        selected=story_witness.approve_content((child,config,revision))
        if selected is not None:candidate['root_witness_authority']=selected
    raw=(Path(source)/ACL).read_bytes()
    acl=source_grants(raw)
    secret_name='voice-nats-service-credentials-'+generation
    services_secret=kube.get('secret',secret_name)
    binary=Path(code)/'nats-rollout-preservation/bootstrap-renewer'
    binary_sha=binding['nats-rollout-preservation/bootstrap-renewer']
    captured={'operator':identity(operator),'services':identity(services_secret),
        'operator_body_sha256':hash_bytes(json.dumps(operator,sort_keys=True,separators=(',',':')).encode()),
        'services_body_sha256':hash_bytes(json.dumps(services_secret,sort_keys=True,separators=(',',':')).encode())}
    def unchanged():
        if kube.get('secret',secret_name)!=services_secret or kube.get('secret',captured['operator']['name'])!=operator:fail()
        if hash_bytes((Path(source)/ACL).read_bytes())!=hash_bytes(raw):fail()
    proofs={};mounts={}
    for role in roles:
        mounts[role]=mounted_role(kube,stage,role,secret_name)
        grant=acl.get(role)
        if not isinstance(grant,dict) or set(grant)!={'publish','subscribe','no_response'} or grant['no_response'] is not True:fail()
        required={'pub':grant['publish'],'sub':grant['subscribe']}
        credentials=secret_bytes(services_secret,role+'.creds')
        proofs[role]=actor_verification.verify(binary,binary_sha,credentials,
            secret_bytes(operator,'account.jwt').decode().strip(),secret_bytes(operator,'operator.jwt').decode().strip(),
            hash_bytes(public_user(credentials,role).encode()),hash_bytes(account.encode()),required,
            lambda c:actor_auth.prove(c,operator,binary,unchanged),unchanged)
    unchanged()
    if candidate is not None:
        candidate['actor_sha256']=hash_bytes(json.dumps({'binding':captured,'mount':mounts['story'],'proof':proofs['story']},sort_keys=True,separators=(',',':')).encode())
    return {'schema':'voice-nats-service-actor-authority-v2','installation_sha256':installation.sha256,'generation':generation,'source_acl_sha256':hash_bytes(raw),
        'roles':roles,'binding':captured,'mounts':mounts,'proofs':proofs,'story_schema':schema,'compatible_story_candidate':candidate}

def revalidate(kube,source,code,binding,stage,services,decoder,previous,*,historical_code=None,installation=None):
    from resource_claim import require_installation,production_installation,require_production
    installation=require_installation(installation)
    if historical_code is not None:require_production(installation)
    if previous is not None:
        fields={'schema','generation','source_acl_sha256','roles','binding','mounts','proofs','story_schema','compatible_story_candidate'}
        if type(previous) is not dict:fail()
        if previous.get('schema')=='voice-nats-service-actor-authority-v2':
            if set(previous)!=fields|{'installation_sha256'} or previous['installation_sha256']!=installation.sha256:fail()
        elif previous.get('schema')=='voice-nats-service-actor-authority-v1':
            if set(previous)!=fields or installation!=production_installation():fail()
        else:fail()
    # Fresh private signed-time/revocation evaluation and actual scratch auth,
    # not a replay of prior booleans. No signing seed or event/ACK operation.
    profile=previous.get('story_schema') if isinstance(previous,dict) else None
    if isinstance(profile,dict) and profile.get('schema')=='voice-story-baseline4-target8-composition-v1':
        if historical_code is not None:fail()
        import compiler
        # Source is still the complete captured tree, not a filtered baseline.
        plan=compiler.compile_migrations(source,'app-only',lambda path:(Path(source)/path).read_bytes())
        current=preflight(kube,source,code,binding,stage,services,decoder,
            story_composition=(plan,profile.get('binding')),installation=installation)
    else:current=preflight(kube,source,code,binding,stage,services,decoder,installation=installation)
    if previous is not None and previous['schema']=='voice-nats-service-actor-authority-v1' and current is not None:
        current=dict(current);current.pop('installation_sha256');current['schema']='voice-nats-service-actor-authority-v1'
    if historical_code is None:
        if current!=previous:fail()
        return
    # Only the adopted V9 recovery may compare historical helper provenance.
    # Authenticate the preserved code without executing it; all verification
    # above used the CURRENT helper and current signed credentials.
    import copy
    import expired_recovery
    import root_cli
    canonical=lambda value:hash_bytes(json.dumps(value,sort_keys=True,separators=(',',':')).encode())
    old_binding=root_cli.code_binding(Path(historical_code))
    if canonical(old_binding)!=expired_recovery.V9_BINDING:fail()
    if current is None and previous is None:return
    try:
        old_verifier=old_binding['nats-rollout-preservation/bootstrap-renewer']
        new_verifier=binding['nats-rollout-preservation/bootstrap-renewer']
        def actor_hash(authority):
            return canonical({'binding':authority['binding'],'mount':authority['mounts']['story'],
                'proof':authority['proofs']['story']})
        for authority,verifier in ((previous,old_verifier),(current,new_verifier)):
            if set(authority['proofs'])!=set(authority['roles']):fail()
            for proof in authority['proofs'].values():
                if proof['verifier_sha256']!=verifier:fail()
            candidate=authority['compatible_story_candidate']
            if candidate is not None and candidate['actor_sha256']!=actor_hash(authority):fail()
        comparable=copy.deepcopy(previous)
        for proof in comparable['proofs'].values():proof['verifier_sha256']=new_verifier
        if comparable['compatible_story_candidate'] is not None:
            comparable['compatible_story_candidate']['actor_sha256']=actor_hash(comparable)
    except (KeyError,TypeError,AttributeError,ValueError):
        fail()
    if current!=comparable:fail()
