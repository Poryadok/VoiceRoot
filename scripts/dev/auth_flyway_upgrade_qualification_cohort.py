"""Authenticate actual cohort readback before a separate Auth clone rehearsal.

The caller owns provenance, the common fence and the exact readback/restore receipts.
No method in this module authorizes a live migration or feature activation.
"""
import re
import io
import json
import pathlib
import secrets
import tarfile
import tempfile
import time

import story_media_backup_cohort as cohort
import story_media_backup_cohort_finalize as finalize
import story_media_backup_cohort_restore as restore_engine
from auth_flyway_upgrade_qualification import snapshot_protected, source_catalog, candidate_blob, marker_snapshot, validate_history
from auth_flyway_upgrade_qualification_docker import OwnedDocker
from auth_user_adoption_runtime import digest, QUALIFIED_TOOLS, validate_ddl_gate
import story_media_backup_restore as media_restore


def validate_cohort(root, key, binding, databases, manifest, metadata, custody, restore, admission, *, phase=None):
    binding, databases, manifest, metadata, custody, restore, admission = (
        snapshot_protected(v) for v in (binding, databases, manifest, metadata, custody, restore, admission))
    if (not isinstance(admission, dict) or set(admission) != {'source_sha','target','flyway_enabled',
            'selector_mode','persistence','plan_sha256','custody_sha256','restore_sha256'}
            or not re.fullmatch(r'[0-9a-f]{40}', str(admission['source_sha']))
            or admission['source_sha'] == '0'*40 or admission['source_sha'] != binding.get('source_sha')
            or type(admission['target']) is not int or admission['target'] != 32
            or admission['flyway_enabled'] is not False or admission['selector_mode'] != 'absent'
            or admission['persistence'] != 'jdbc'
            or any(not re.fullmatch(r'[0-9a-f]{64}',str(admission[k])) or admission[k]=='0'*64
                   for k in ('plan_sha256','custody_sha256','restore_sha256'))):
        raise ValueError('source or configuration admission unavailable')
    if (not isinstance(key, bytes) or len(key) != 32 or not isinstance(binding,dict)
            or set(binding) != finalize.BINDING_KEYS | {'dump_receipts'} | ({'full_catalog'} if 'full_catalog' in binding else set())
            or not re.fullmatch(r'media-c1-[0-9a-f]{32}',str(binding.get('operation_id','')))
            or not isinstance(databases,list) or 'auth_db' not in databases):
        raise ValueError('C1 cohort identity invalid')
    base = {k:v for k,v in binding.items() if k != 'dump_receipts'}
    members = finalize.validate_binding(base, databases)
    receipts = binding['dump_receipts']
    if (not isinstance(receipts,dict) or set(receipts) != members
            or any(not isinstance(v,str) or not re.fullmatch(r'[0-9a-f]{64}',v) for v in receipts.values())):
        raise ValueError('terminal dump receipts incomplete')
    cohort.verify(root, key, binding, databases, manifest)
    inputs = binding['restore_inputs']
    restore_engine._source(binding, databases, metadata, inputs['postgres_image'], inputs['migrate_image'])
    plan = metadata['migrations']['auth_db']
    if plan['engine'] != 'flyway' or plan['from'] != 26 or plan['to'] != 26:
        raise ValueError('prior cohort restore must preserve Auth Flyway 26')
    custody_fields = {'schema','destination','artifact_id','operation_id','snapshot_id','challenge',
                      'run_id','head_sha','bundle_bytes','bundle_sha256'}
    if (not isinstance(custody,dict) or set(custody) != custody_fields
            or custody['schema'] != 'voice-cohort-custody-v1' or custody['destination'] != 'github:Poryadok/VoiceRoot'
            or custody['operation_id'] != binding['operation_id'] or custody['snapshot_id'] != binding['snapshot_id']
            or custody['head_sha'] != admission['source_sha']
            or type(custody['artifact_id']) is not int or custody['artifact_id'] <= 0
            or type(custody['run_id']) is not int or custody['run_id'] <= 0
            or type(custody['bundle_bytes']) is not int or custody['bundle_bytes'] <= 0
            or not re.fullmatch(r'[0-9a-f]{32}',str(custody['challenge']))
            or not re.fullmatch(r'[0-9a-f]{64}',str(custody['bundle_sha256']))
            or digest(custody) != admission['custody_sha256']):
        raise ValueError('protected cohort readback receipt unavailable')
    extended = 'migration_logins' in restore if isinstance(restore,dict) else False
    if 'full_catalog' in binding:
        import auth_live_migration_phase as auth_live
        protected, provenance = auth_live.verified_sources(phase,metadata,26)
        if (protected != binding or provenance['custody_sha256'] != digest(custody)
                or provenance['restore_sha256'] != digest(restore)):
            raise ValueError('protected mixed cohort authority unavailable')
    elif extended:
        raise ValueError('unexpected mixed cohort receipt')
    if (not isinstance(restore,dict) or set(restore) != {'operation_id','snapshot_id','source_sha','manifest_sha256',
            'databases','scope','cleanup'} | ({'migration_logins'} if extended else set()) or restore['operation_id'] != binding['operation_id']
            or restore['snapshot_id'] != binding['snapshot_id'] or restore['source_sha'] != binding['source_sha']
            or restore['manifest_sha256'] != digest(manifest)
            or restore['scope'] != 'selected-cohort-roles-catalog-and-ledger-only' or restore['cleanup'] != 'verified'
            or not isinstance(restore['databases'],dict) or set(restore['databases']) != set(databases)
            or restore['databases'].get('auth_db') != {'mode':'restore-only','from':26,'to':26}
            or digest(restore) != admission['restore_sha256']):
        raise ValueError('protected cohort restore receipt unavailable')
    return {'binding':binding,'databases':databases,'manifest':manifest,'metadata':metadata,
            'custody':custody,'restore':restore,'admission':admission}


def qualify_cohort(source_root, root, key, binding, databases, manifest, metadata, custody, restore, admission, journal):
    """Qualify one protected C1 readback on a disposable Auth clone.

    `journal` is the dedicated fence/CAS adapter, prebound to these inputs.
    It must implement guard(), stage(name, status), state and durable save().
    """
    frozen = validate_cohort(root, key, binding, databases, manifest, metadata, custody, restore, admission,
                             phase=journal.phase if 'full_catalog' in binding else None)
    binding, databases, manifest, metadata, custody, restore, admission = (frozen[k] for k in
        ('binding','databases','manifest','metadata','custody','restore','admission'))
    source = admission['source_sha']
    sources, catalog = source_catalog(source_root, source)
    if catalog != metadata['migrations']['auth_db']['files']:
        raise ValueError('candidate migration inventory differs from protected source metadata')
    gate = candidate_blob(source_root, source, 'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql')
    journal.guard()
    postgres_id = media_restore._check_image(binding['restore_inputs']['postgres_image'])
    if postgres_id != media_restore._check_image(QUALIFIED_TOOLS['postgres']):
        raise ValueError('unqualified PostgreSQL image')
    try:
        docker = OwnedDocker(journal)
    except BaseException:
        journal.uncertain()
        raise
    call = docker.call
    admin = 'voice_auth_clone_' + secrets.token_hex(12)
    password = secrets.token_hex(24)
    receipt = None
    try:
        def decrypt(member):
            stream = io.BytesIO()
            cohort.decrypt(root, key, binding, databases, manifest, member, stream)
            return stream.getvalue()
        journal.guard()
        pg = docker.create(QUALIFIED_TOOLS['postgres'], ['--network','none','--memory','1g','--cpus','2',
            '-e','POSTGRES_USER='+admin, '-e','POSTGRES_PASSWORD='+password,
            '-e','POSTGRES_HOST_AUTH_METHOD=trust'], ['-c','cluster_name=voice-auth-flyway-qualification'])
        call('start', pg)
        def psql(db, sql):
            return call('exec','-i','-e','PGPASSWORD='+password,pg,'psql','-h','127.0.0.1',
                        '-X','-qAt','-v','ON_ERROR_STOP=1','-U',admin,'-d',db,data=sql.encode())
        for _ in range(100):
            try:
                if psql('postgres','SELECT 1').strip() == b'1': break
            except ValueError: time.sleep(.2)
        else: raise ValueError('private PostgreSQL not ready')
        if psql('postgres','SHOW cluster_name').strip() != b'voice-auth-flyway-qualification':
            raise ValueError('private PostgreSQL identity changed')
        roles = decrypt('roles')
        role_sql = roles.decode('utf-8')
        if re.search(r'\bPASSWORD\s', role_sql, re.I):
            raise ValueError('role archive contains passwords')
        psql('postgres', role_sql)
        def query(db, sql): return json.loads(psql(db,sql))
        role_query = media_restore.role_catalog_sql(admin)
        before_roles = query('postgres',role_query)
        if before_roles != metadata['roles']:
            raise ValueError('restored role catalog differs')
        voice = [r for r in before_roles['roles'] if r['name'] == 'voice']
        if len(voice) != 1 or voice[0]['super'] is not True or voice[0]['login'] is not True:
            raise ValueError('actual migration role authority unavailable')
        owner = media_restore._quote_identifier(metadata['catalog']['auth_db']['database_owner'])
        psql('postgres','CREATE DATABASE auth_db OWNER ' + owner)
        call('exec','-i','-e','PGPASSWORD='+password,pg,'pg_restore','-h','127.0.0.1','-U',admin,
             '--exit-on-error','--dbname=auth_db',data=decrypt('auth_db'))
        before_catalog = query('auth_db',restore_engine.AUTH_CATALOG)
        if before_catalog != metadata['catalog']['auth_db']:
            raise ValueError('restored Auth catalog differs from protected snapshot')
        history_sql = "SELECT coalesce(json_agg(row_to_json(h) ORDER BY installed_rank),'[]'::json) FROM public.flyway_schema_history h"
        before_history = query('auth_db',history_sql)
        if not before_history or before_history[-1].get('version') != '26':
            raise ValueError('original Auth history is not clean Flyway 26')
        cols = query('auth_db', "SELECT coalesce(json_object_agg(c.relname,(SELECT json_agg(a.attname ORDER BY a.attnum) "
            "FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped)),'{}'::json) "
            "FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='r'")
        q = lambda sql: query('auth_db',sql)
        before_markers = marker_snapshot(q,cols,before=True)
        expected_markers = marker_snapshot(q,cols,before=True,normalize=True)
        archive = io.BytesIO()
        with tarfile.open(fileobj=archive,mode='w') as tar:
            for name, raw in sorted(sources.items()):
                info=tarfile.TarInfo('sql/'+name); info.size=len(raw); info.mode=0o644
                tar.addfile(info,io.BytesIO(raw))
        def flyway(action, ignore_pending=False):
            args=['-connectRetries=0','-validateMigrationNaming=true','-baselineOnMigrate=false',
                  '-outOfOrder=false','-locations=filesystem:/flyway/sql','-target=32']
            if ignore_pending: args.append('-ignoreMigrationPatterns=*:pending')
            cid=docker.create(QUALIFIED_TOOLS['flyway'],['--network','container:'+pg,'-e','FLYWAY_USER=voice',
                '-e','FLYWAY_URL=jdbc:postgresql://127.0.0.1:5432/auth_db'],args+[action])
            call('cp','-',cid+':/flyway',data=archive.getvalue())
            call('start','-a',cid,timeout=240)
        journal.guard(); flyway('validate',True)
        journal.stage('flyway-migrate','pending')
        flyway('migrate')
        journal.stage('flyway-migrate','done')
        journal.guard(); flyway('validate')
        after_history=query('auth_db',history_sql)
        validate_history(before_history,after_history)
        with tempfile.TemporaryDirectory(prefix='auth-flyway-v31-') as temp:
            pinned=pathlib.Path(temp)/'src/backend/auth/src/main/resources/db/migration/V31__auth_identity_ddl_barrier.sql'
            pinned.parent.mkdir(parents=True); pinned.write_bytes(sources['V31__auth_identity_ddl_barrier.sql'])
            validate_ddl_gate(q(gate.decode('utf-8')),pathlib.Path(temp))
        after_markers=marker_snapshot(q,cols,before=False)
        if expected_markers != after_markers or before_roles != query('postgres',role_query):
            raise ValueError('original data or roles changed outside admitted transformations')
        after_catalog=query('auth_db',restore_engine.AUTH_CATALOG)
        for field in ('database_owner','schema_owner','schema_acl','defaults'):
            if before_catalog[field] != after_catalog[field]: raise ValueError('catalog authority changed')
        old={o['name']:o for o in before_catalog['objects']}; new={o['name']:o for o in after_catalog['objects']}
        if any(new.get(n) != obj for n,obj in old.items()): raise ValueError('original object authority changed')
        receipt={'protocol':'voice-auth-flyway-cohort-clone-v1','operation_id':binding['operation_id'],
            'snapshot_id':binding['snapshot_id'],'source_sha':source,'target':32,'plan_sha256':admission['plan_sha256'],
            'manifest_sha256':digest(manifest),'custody_sha256':digest(custody),'restore_sha256':digest(restore),
            'metadata_sha256':digest(metadata),'source_catalog':catalog,'postgres_image':QUALIFIED_TOOLS['postgres'],
            'flyway_image':QUALIFIED_TOOLS['flyway'],'before_history':before_history,'after_history':after_history,
            'before_markers':before_markers,'expected_after_markers':expected_markers,'after_markers':after_markers,
            'original_table_columns':cols,'before_catalog_sha256':digest(before_catalog),
            'after_catalog_sha256':digest(after_catalog),'roles_sha256':digest(before_roles),
            'scope':'isolated-auth-clone-only'}
    except BaseException:
        try:
            docker.cleanup()
        finally:
            journal.uncertain()
        raise
    try:
        docker.cleanup()
    except BaseException:
        journal.uncertain()
        raise
    receipt['cleanup']='confirmed'; receipt['resources']=snapshot_protected(docker.records)
    try:
        journal.state['flyway_clone']['status']='complete'
        journal.state['flyway_clone']['receipt']=receipt
        journal.save(); journal.guard()
    except BaseException:
        journal.uncertain()
        raise
    return receipt
