"""Source-owned DEV Space runtime transition (implementation in progress)."""
import base64
import copy
import hashlib
import hmac
import fcntl
import json
import os
import selectors
import secrets
import subprocess
import time
from pathlib import Path
import re

import full_cohort_schema_metadata as schema


class SpaceRuntimeError(RuntimeError):
    pass


RUNTIME_SQL_SHA256='688a094bdb4d500f60ec256728c49791663db7c6e51b1a1437a31be592ee0dd3'


def _mac(key, value):
    if not isinstance(key, bytes) or len(key) != 32: raise ValueError()
    return hmac.new(key, json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode(), hashlib.sha256).hexdigest()


def _runtime_sql(root, source):
    """Bind the fixed canonical grants script to the authenticated source tree."""
    import publish_candidate
    relative = 'docker/postgres/provision-space-runtime-role.sql'
    snapshot = publish_candidate.source_snapshot(Path(root), source)
    path = Path(root) / relative
    if path.is_symlink() or not path.is_file(): raise ValueError()
    raw = path.read_bytes()
    if (len(raw) > 65536 or hashlib.sha256(raw).hexdigest() != RUNTIME_SQL_SHA256 or
            hashlib.sha256(raw).hexdigest() != snapshot.get('files', {}).get(relative) or
            snapshot.get('source_sha') != source or not raw.startswith(b'\\getenv space_runtime_password SPACE_DB_PASSWORD\n') or
            b'\0' in raw): raise ValueError()
    return raw.decode('utf-8').split('\n', 1)[1]


def _role_check(pg, space_oid, *, require_zero_sessions):
    """Check only fixed role metadata, memberships, other-DB dependencies and sessions."""
    if type(space_oid) is not int or not 0 < space_oid < 2**32 or type(require_zero_sessions) is not bool:
        raise ValueError()
    sessions = "EXISTS(SELECT 1 FROM pg_catalog.pg_stat_activity WHERE usename='space_runtime')" if require_zero_sessions else 'false'
    sql = ("SELECT CASE WHEN " + sessions + " THEN 'unsafe' ELSE COALESCE((SELECT CASE WHEN r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR r.rolreplication "
           "OR r.rolbypassrls OR r.rolinherit OR NOT r.rolcanlogin "
           "OR EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid OR m.roleid=r.oid) "
           "OR EXISTS(SELECT 1 FROM pg_catalog.pg_shdepend d WHERE d.refclassid='pg_authid'::regclass "
           "AND d.refobjid=r.oid AND ((d.dbid<>0 AND d.dbid<>" + str(space_oid) + "::oid) OR "
           "(d.dbid=0 AND NOT (d.classid='pg_database'::regclass AND d.objid=" + str(space_oid) + "::oid AND d.deptype='a'))))" +
           " THEN 'unsafe' ELSE 'dedicated' END FROM pg_catalog.pg_roles r WHERE r.rolname='space_runtime'),'absent') END;\n")
    result=pg._run(sql)
    if result not in (b'absent\n',b'dedicated\n'): raise ValueError()
    return result.decode('ascii').strip()


def _admin_check(pg, role, space_oid, role_state):
    if (not isinstance(role,str) or not schema.SAFE.fullmatch(role) or type(space_oid) is not int or
            not 0 < space_oid < 2**32 or role_state not in ('absent','dedicated')): raise ValueError()
    existing = ' AND r.rolsuper' if role_state == 'dedicated' else ''
    sql=("SELECT CASE WHEN current_user=session_user AND current_user='"+role+"' AND "
         "(r.rolsuper OR r.rolcreatedb) AND (r.rolsuper OR r.rolcreaterole)"+existing+
         " AND (r.rolsuper OR d.datdba=r.oid) THEN 'ok' ELSE 'unsafe' END "
         "FROM pg_catalog.pg_roles r JOIN pg_catalog.pg_database d ON d.oid="+str(space_oid)+"::oid "
         "WHERE r.rolname=current_user;\n")
    if pg._run(sql)!=b'ok\n': raise ValueError()


def _grants_check(pg):
    """Read only effective privileges required by the canonical runtime script."""
    table_required=('SELECT','INSERT','UPDATE','DELETE')
    table_forbidden=('TRUNCATE','REFERENCES','TRIGGER')
    sequence_required=('USAGE','SELECT')
    def lack(function, names):
        return ' OR '.join("NOT pg_catalog."+function+"('space_runtime',c.oid,'"+name+"')" for name in names)
    def extra(function, names):
        return ' OR '.join("pg_catalog."+function+"('space_runtime',c.oid,'"+name+"') OR pg_catalog."+function+
                           "('space_runtime',c.oid,'"+name+" WITH GRANT OPTION')" for name in names)
    sql=("\\connect space_db\nSELECT CASE WHEN pg_catalog.has_schema_privilege('space_runtime','public','USAGE') "
         "AND NOT pg_catalog.has_schema_privilege('space_runtime','public','CREATE') "
         "AND NOT pg_catalog.has_schema_privilege('space_runtime','public','USAGE WITH GRANT OPTION') "
         "AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace "
         "WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f') AND ("+
         lack('has_table_privilege',table_required)+' OR '+extra('has_table_privilege',table_forbidden)+' OR '+
         ' OR '.join("pg_catalog.has_table_privilege('space_runtime',c.oid,'"+name+" WITH GRANT OPTION')" for name in table_required)+
         " OR pg_catalog.has_any_column_privilege('space_runtime',c.oid,'REFERENCES') OR "+
         ' OR '.join("pg_catalog.has_any_column_privilege('space_runtime',c.oid,'"+name+" WITH GRANT OPTION')" for name in ('SELECT','INSERT','UPDATE','REFERENCES'))+")) "
         "AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace "
         "WHERE n.nspname='public' AND c.relkind='S' AND ("+
         lack('has_sequence_privilege',sequence_required)+' OR '+extra('has_sequence_privilege',('UPDATE',))+' OR '+
         ' OR '.join("pg_catalog.has_sequence_privilege('space_runtime',c.oid,'"+name+" WITH GRANT OPTION')" for name in sequence_required)+")) "
         "THEN 'ok' ELSE 'unsafe' END;\n")
    if pg._run(sql)!=b'ok\n': raise ValueError()


def _secret_patch(before, dsn, key):
    """Build only the exact key replacement; payload is private and never journaled."""
    meta = before['metadata']; data = before['data']
    if (before.get('kind') != 'Secret' or before.get('apiVersion') != 'v1' or
            meta.get('name') != 'voice-app-secrets' or meta.get('namespace') != 'voice-dev' or
            meta.get('deletionTimestamp') or before.get('immutable') is True or
            any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion')) or
            not isinstance(data,dict) or 'SPACE_DATABASE_URL' not in data or
            any(not isinstance(k,str) or not isinstance(v,str) for k,v in data.items()) or
            not isinstance(dsn,str) or not dsn): raise ValueError()
    old = copy.deepcopy(data)
    new = copy.deepcopy(old); new['SPACE_DATABASE_URL'] = base64.b64encode(dsn.encode()).decode('ascii')
    patch = [{'op':'test','path':'/metadata/uid','value':meta['uid']},
             {'op':'test','path':'/metadata/resourceVersion','value':meta['resourceVersion']},
             {'op':'replace','path':'/data/SPACE_DATABASE_URL','value':new['SPACE_DATABASE_URL']}]
    other = {k:v for k,v in old.items() if k != 'SPACE_DATABASE_URL'}
    return patch, {'secret_uid':meta['uid'], 'old_rv':meta['resourceVersion'],
                   'old_data_hmac_sha256':_mac(key,old), 'new_data_hmac_sha256':_mac(key,new),
                   'unchanged_other_keys_hmac_sha256':_mac(key,other)}, new


def _secret_unchanged(before, actual, target):
    """Accept only the fixed key replacement and API revision bookkeeping."""
    original=copy.deepcopy(before); observed=copy.deepcopy(actual)
    original['data']=copy.deepcopy(target)
    for item in (original,observed):
        item['metadata'].pop('resourceVersion',None)
        item['metadata'].pop('managedFields',None)
    if original != observed: raise ValueError()


def _private_patch(patch, *, dry_run):
    """Pass the fixed Secret patch through a sealed anonymous Linux FD."""
    import frontend_runtime_executor
    fd=process=selector=None
    try:
        if type(dry_run) is not bool: raise ValueError()
        raw=json.dumps(patch,separators=(',', ':'),allow_nan=False).encode()
        if not raw or len(raw)>65536: raise ValueError()
        fd=os.memfd_create('voice-space-private-patch',os.MFD_CLOEXEC|os.MFD_ALLOW_SEALING)
        os.fchmod(fd,0o600)
        view=memoryview(raw)
        while view:
            count=os.write(fd,view)
            if count<=0: raise ValueError()
            view=view[count:]
        fcntl.fcntl(fd,fcntl.F_ADD_SEALS,fcntl.F_SEAL_WRITE|fcntl.F_SEAL_GROW|fcntl.F_SEAL_SHRINK|fcntl.F_SEAL_SEAL)
        os.lseek(fd,0,os.SEEK_SET)
        args=frontend_runtime_executor.KubectlTransport.PREFIX+['patch','secret','voice-app-secrets','--type=json',
              '--patch-file=/proc/self/fd/'+str(fd)]
        if dry_run: args.append('--dry-run=server')
        args += ['-o','json']
        process=subprocess.Popen(args,stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,pass_fds=(fd,))
        selector=selectors.DefaultSelector(); selector.register(process.stdout,selectors.EVENT_READ)
        deadline=time.monotonic()+25; output=bytearray()
        while selector.get_map():
            remaining=deadline-time.monotonic()
            if remaining<=0 or not selector.select(remaining): raise ValueError()
            block=os.read(process.stdout.fileno(),min(8192,65537-len(output)))
            if not block: selector.unregister(process.stdout); break
            output.extend(block)
            if len(output)>65536: raise ValueError()
        if process.wait(timeout=max(.01,deadline-time.monotonic())): raise ValueError()
        return json.loads(output)
    except BaseException:
        raise SpaceRuntimeError('private Secret patch unavailable or uncertain') from None
    finally:
        if selector is not None: selector.close()
        if process is not None:
            if process.poll() is None: process.kill(); process.wait()
            if process.stdout is not None: process.stdout.close()
        if fd is not None: os.close(fd)


def _source_routes(observed, controllers, reader):
    """Resolve every observed Space route from the exact pre-cut controllers.

    The caller must authenticate the inventory and CUT. This helper does not
    establish authority or permit a Secret update by itself.
    """
    workloads = observed['workloads']
    if set(workloads) != set(schema.CONTAINERS): raise ValueError()
    by_service = {}
    for obj in controllers:
        if obj.get('kind') != 'Deployment': continue
        name = obj.get('metadata', {}).get('name', '')
        if name.startswith('voice-'):
            service = name.removeprefix('voice-')
            if service in by_service: raise ValueError()
            by_service[service] = obj
    consumers = set()
    for service in sorted(set(schema.OWNERS) | {'gateway'}):
        entries = workloads[service]
        if not isinstance(entries, list) or not entries: raise ValueError()
        for entry in entries:
            if entry.get('status') != 'observed': raise ValueError()
            result = entry['result']; candidates = [(result['binding'], None)]
            if len(result.get('secondary', [])) != len(schema.SECONDARY.get(service, {})): raise ValueError()
            candidates += [(x['binding'], x) for x in result.get('secondary', [])]
            for binding, secondary in candidates:
                if binding.get('state') != 'resolved': raise ValueError()
                if secondary is not None and secondary.get('owner_consistency') != 'matched': raise ValueError()
                if binding.get('role') == 'space_runtime' and binding.get('database') != 'space_db': raise ValueError()
                if binding.get('database') != 'space_db': continue
                variable = 'DATABASE_URL' if service == 'space' and secondary is None else 'SPACE_DATABASE_URL'
                if (service != 'space' and variable not in schema.SECONDARY.get(service, {}) or
                        service == 'space' and secondary is not None): raise ValueError()
                obj = by_service.get(service)
                if (not obj or not obj.get('metadata', {}).get('uid') or
                        binding.get('deployment_uid') != obj['metadata']['uid']): raise ValueError()
                containers = obj['spec']['template']['spec']['containers']
                selected = [x for x in containers if x.get('name') == schema.CONTAINERS[service]]
                if len(selected) != 1: raise ValueError()
                value, refs = schema._env(service, selected[0], variable, reader)
                if (len(refs) != 1 or refs[0][:3] != ('secret', 'voice-app-secrets', 'SPACE_DATABASE_URL') or
                        refs[0][3] is None or value != refs[0][3]['value']): raise ValueError()
                host, database, role = schema._url(value, 'voice-dev')
                if (host != binding['host'] or database != 'space_db' or role != binding['role'] or
                        {'kind':'secret','uid':refs[0][3]['uid'],'resource_version':refs[0][3]['resource_version']}
                        not in binding['references']): raise ValueError()
                consumers.add(service)
    if 'space' not in consumers: raise ValueError()
    return sorted(consumers)


class SpaceRuntimeRolePhase:
    """Private Space-only role preparation; effect lifecycle remains in progress."""
    STEP = 'space-runtime-role'
    LEDGER = 'space_runtime_role'

    def __init__(self, journal, fence, client, lock, namespace_uid, catalog, cut, source_root, token):
        import frontend_runtime_executor
        import full_cohort_affected_plan
        import media_operation_journal
        import resource_fencing
        import writer_cut
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence or
                type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                getattr(fence.ledger, 'client', None) is not client or
                type(catalog) is not full_cohort_affected_plan.FullCatalogPhase or type(cut) is not writer_cut.WriterCut or
                getattr(catalog.fence.ledger,'client',None) is not catalog.client or
                getattr(cut.fence.ledger,'client',None) is not cut.client or
                cut.fence.context != fence.context or catalog.fence.context != fence.context or
                cut.lock != lock or catalog.lock != lock or cut.namespace_uid != namespace_uid or
                catalog.namespace_uid != namespace_uid or journal.intent.get('resource_context') != fence.context or
                journal.intent.get('namespace_uid') != namespace_uid or not isinstance(token,str) or not token):
            raise SpaceRuntimeError('Space runtime identity invalid')
        self.journal,self.fence,self.client,self.catalog,self.cut = journal,fence,client,catalog,cut
        self.lock,self.namespace_uid,self.source_root,self.token = copy.deepcopy(lock),namespace_uid,source_root,token
        self.intent=copy.deepcopy(journal.intent); self.poisoned=False
        self._bound=(journal,fence,client,catalog,cut,source_root,token)

    def _guard(self):
        import media_operation_guard
        if (self.poisoned or self.journal.poisoned or self._bound != (self.journal,self.fence,self.client,
                self.catalog,self.cut,self.source_root,self.token) or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent): raise schema.GuardLost('Space runtime identity changed')
        schema._fence(lambda: media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid))

    def _ledger(self):
        try:
            row,revision=self.fence.ledger.read()
            if (not isinstance(revision,tuple) or len(revision)!=2 or not all(isinstance(x,str) and x for x in revision) or
                    self.catalog.fence.ledger.read()!=(row,revision) or self.cut.fence.ledger.read()!=(row,revision) or
                    not self.fence.owned(row) or row.get('status')!='active'): raise ValueError()
            return row,revision
        except BaseException: raise schema.GuardLost('Space runtime ledger changed') from None

    def prepare(self):
        """Bind current source-known consumers before drain; no DB or Secret mutation."""
        import full_cohort_go_migration as go
        import space_fresh_install_phase_pg as space_pg
        pending=False
        try:
            self._guard(); schema._fence(self.cut._guard)
            try:
                inventory=self.catalog.inventory.verify_current(self.token)
                catalog=self.catalog.verify_current(self.token)
            except BaseException: raise schema.GuardLost('Space runtime observation unavailable') from None
            source_sql=_runtime_sql(self.source_root,self.fence.context['commit'])
            pg=space_pg.KubeSpacePostgres(self._guard)
            if pg.snapshot()!=catalog['engine']: raise ValueError()
            role_state=_role_check(pg,catalog['all_databases']['space_db'],require_zero_sessions=False)
            admin_role=self.catalog.inventory._admin_role()
            _admin_check(pg,admin_role,catalog['all_databases']['space_db'],role_state)
            row,rev=self._ledger()
            if (self.STEP in self.journal.state['steps'] or self.LEDGER in row or not self.fence.owned(row) or
                    row.get('status')!='active'): raise ValueError()
            def read(kind,name,key):
                self._guard(); value=schema.kube_key_reader(schema.linux_private_bytes)(kind,name,key); self._guard(); return value
            consumers=_source_routes(inventory['database_observations'],self.cut.admission['controllers'],read)
            secret=go._private_kube(['get','secret','voice-app-secrets','-o','json'])
            key=secrets.token_bytes(32)
            prospective,safe,target = _secret_patch(secret,'postgres://space_runtime:placeholder@voice-postgres/space_db?sslmode=disable',key)
            dry=_private_patch(prospective,dry_run=True)
            _secret_unchanged(secret,dry,target)
            self._guard()
            try:
                inventory_after=self.catalog.inventory.verify_current(self.token)
                catalog_after=self.catalog.verify_current(self.token)
            except BaseException: raise schema.GuardLost('Space runtime observation unavailable') from None
            if (inventory_after!=inventory or catalog_after!=catalog or pg.snapshot()!=catalog['engine'] or
                    _role_check(pg,catalog['all_databases']['space_db'],require_zero_sessions=False)!=role_state or
                    go._private_kube(['get','secret','voice-app-secrets','-o','json'])!=secret): raise ValueError()
            _admin_check(pg,admin_role,catalog['all_databases']['space_db'],role_state)
            facts={'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                   'namespace_uid':self.namespace_uid,'secret_uid':safe['secret_uid'],'old_rv':safe['old_rv'],
                   'old_data_hmac_sha256':safe['old_data_hmac_sha256'],
                   'unchanged_other_keys_hmac_sha256':safe['unchanged_other_keys_hmac_sha256'],
                   'consumers':consumers,'original_space_oid':catalog['all_databases']['space_db'],
                   'runtime_sql_sha256':hashlib.sha256(source_sql.encode()).hexdigest(),'role_state':role_state,'admin_role':admin_role}
            marker={'status':'prepared','nonce':secrets.token_hex(16),'facts_sha256':_digest_public(facts),
                    'hmac_key_sha256':hashlib.sha256(key).hexdigest()}
            pending=True
            local=copy.deepcopy(marker)|{'facts':facts,'private_hmac_key':key.hex()}
            self.journal.state['steps'][self.STEP]=local; self.journal.save(); self._guard()
            current,actual=self._ledger()
            if current!=row or actual!=rev: raise ValueError()
            current[self.LEDGER]=marker; current['step']=self.STEP+'-prepared'
            self.fence.ledger.write(actual,current); self._guard()
            after,_=self._ledger()
            if after.get(self.LEDGER)!=marker or self.journal.state['steps'].get(self.STEP)!=local: raise ValueError()
            return copy.deepcopy(facts)
        except BaseException as error:
            if pending:
                self.poisoned=True
                try: self.fence.uncertain('Space runtime preparation uncertain')
                except BaseException: pass
            if isinstance(error, schema.GuardLost): raise
            raise SpaceRuntimeError('Space runtime preparation unavailable') from None

    def _local(self):
        self._guard(); local=copy.deepcopy(self.journal.state['steps'][self.STEP])
        if not isinstance(local,dict) or not isinstance(local.get('facts'),dict): raise ValueError()
        marker={k:v for k,v in local.items() if k not in ('facts','private_hmac_key','receipt')}
        key=bytes.fromhex(local['private_hmac_key']); facts=local['facts']
        row,_=self._ledger()
        if (len(key)!=32 or marker.get('hmac_key_sha256')!=hashlib.sha256(key).hexdigest() or
                marker.get('facts_sha256')!=_digest_public(facts) or row.get(self.LEDGER)!=marker or
                facts.get('operation_id')!=self.fence.context['operation_id'] or facts.get('source_sha')!=self.fence.context['commit'] or
                facts.get('namespace_uid')!=self.namespace_uid): raise ValueError()
        return local,marker,key

    def _advance(self, old, new, *, receipt=None):
        current,revision=self._ledger()
        local=copy.deepcopy(self.journal.state['steps'][self.STEP])
        if current.get(self.LEDGER)!=old or {k:v for k,v in local.items() if k not in ('facts','private_hmac_key','receipt')}!=old:
            raise ValueError()
        updated={**local,**new}
        if receipt is not None: updated['receipt']=copy.deepcopy(receipt)
        self.journal.state['steps'][self.STEP]=updated; self.journal.save()
        self._guard(); again,rev=self._ledger()
        if again!=current or rev!=revision: raise ValueError()
        again[self.LEDGER]=copy.deepcopy(new); again['step']=self.STEP+'-'+new['status']
        self.fence.ledger.write(rev,again); self._guard()
        if self._ledger()[0].get(self.LEDGER)!=new or self.journal.state['steps'].get(self.STEP)!=updated: raise ValueError()

    def _ack_secret(self, old, new, before, revision):
        """Persist the accepted UID/RV locally before any further remote operation."""
        local=copy.deepcopy(self.journal.state['steps'][self.STEP])
        if {k:v for k,v in local.items() if k not in ('facts','private_hmac_key','receipt')}!=old: raise ValueError()
        updated={**local,**new}; self.journal.state['steps'][self.STEP]=updated; self.journal.save()
        self._guard(); current,actual=self._ledger()
        if current!=before or actual!=revision or current.get(self.LEDGER)!=old: raise ValueError()
        current[self.LEDGER]=copy.deepcopy(new); current['step']=self.STEP+'-'+new['status']
        self.fence.ledger.write(actual,current); self._guard()
        if self._ledger()[0].get(self.LEDGER)!=new or self.journal.state['steps'].get(self.STEP)!=updated: raise ValueError()

    def run(self, space_phase):
        """One-shot dedicated role/grants and fixed-key CAS after genuine native51."""
        import full_cohort_go_migration as go
        import space_fresh_install_phase as fresh
        import space_fresh_install_phase_pg as space_pg
        pending=False
        try:
            local,marker,key=self._local(); facts=local['facts']
            if marker.get('status')!='prepared': raise ValueError()
            native=fresh.verify_native_completed_before_runtime(space_phase,self.token)
            if (space_phase.cut is not self.cut or space_phase.runtime is not self or
                    space_phase.journal is not self.journal or space_phase.fence is not self.fence or
                    native.get('operation_id')!=facts['operation_id'] or
                    native.get('source_sha')!=facts['source_sha'] or native.get('namespace_uid')!=self.namespace_uid or
                    native.get('installer_role')!=facts['admin_role'] or
                    type(native.get('new_oid')) is not int or native['new_oid']<=0 or
                    native['new_oid']==facts['original_space_oid']): raise ValueError()
            catalog=self.catalog.verify_receipt(self.token)
            source_sql=_runtime_sql(self.source_root,facts['source_sha'])
            if hashlib.sha256(source_sql.encode()).hexdigest()!=facts['runtime_sql_sha256']: raise ValueError()
            pg=space_pg.KubeSpacePostgres(self._guard)
            if (pg.snapshot()!=catalog['engine'] or pg.space_oid()!=native['new_oid'] or
                    _role_check(pg,native['new_oid'],require_zero_sessions=True)!=facts['role_state']):
                raise ValueError()
            _admin_check(pg,facts['admin_role'],native['new_oid'],facts['role_state'])
            before=go._private_kube(['get','secret','voice-app-secrets','-o','json'])
            password=secrets.token_hex(32)
            dsn='postgres://space_runtime:'+password+'@voice-postgres:5432/space_db?sslmode=disable'
            patch,safe,target=_secret_patch(before,dsn,key)
            if (safe['secret_uid']!=facts['secret_uid'] or safe['old_rv']!=facts['old_rv'] or
                    safe['old_data_hmac_sha256']!=facts['old_data_hmac_sha256'] or
                    safe['unchanged_other_keys_hmac_sha256']!=facts['unchanged_other_keys_hmac_sha256']): raise ValueError()
            dry=_private_patch(patch,dry_run=True)
            _secret_unchanged(before,dry,target)
            self.cut.verify_space_running(); self._guard()
            pending=True; role_pending=marker|{'status':'role_pending'}; self._advance(marker,role_pending)
            if (fresh.verify_native_completed_before_runtime(space_phase,self.token)!=native or
                    pg.snapshot()!=catalog['engine'] or pg.space_oid()!=native['new_oid'] or
                    _role_check(pg,native['new_oid'],require_zero_sessions=True)!=facts['role_state'] or
                    go._private_kube(['get','secret','voice-app-secrets','-o','json'])!=before or
                    _runtime_sql(self.source_root,facts['source_sha'])!=source_sql): raise ValueError()
            _admin_check(pg,facts['admin_role'],native['new_oid'],facts['role_state'])
            self.cut.verify_space_running(); self._guard()
            pg._run("\\connect space_db\n\\set space_runtime_password '"+password+"'\n"+source_sql)
            if (pg.snapshot()!=catalog['engine'] or pg.space_oid()!=native['new_oid'] or
                    _role_check(pg,native['new_oid'],require_zero_sessions=True)!='dedicated'): raise ValueError()
            _grants_check(pg)
            role_done=role_pending|{'status':'role_done'}; self._advance(role_pending,role_done)
            if (fresh.verify_native_completed_before_runtime(space_phase,self.token)!=native or
                    go._private_kube(['get','secret','voice-app-secrets','-o','json'])!=before): raise ValueError()
            secret_pending=role_done|{'status':'secret_pending'}; self._advance(role_done,secret_pending)
            if (fresh.verify_native_completed_before_runtime(space_phase,self.token)!=native or
                    pg.snapshot()!=catalog['engine'] or pg.space_oid()!=native['new_oid'] or
                    _role_check(pg,native['new_oid'],require_zero_sessions=True)!='dedicated' or
                    go._private_kube(['get','secret','voice-app-secrets','-o','json'])!=before): raise ValueError()
            _grants_check(pg)
            self.cut.verify_space_running(); self._guard()
            before_row,before_revision=self._ledger()
            if before_row.get(self.LEDGER)!=secret_pending: raise ValueError()
            changed=_private_patch(patch,dry_run=False)
            meta=changed.get('metadata',{})
            _secret_unchanged(before,changed,target)
            if (meta.get('uid')!=facts['secret_uid'] or
                    not isinstance(meta.get('resourceVersion'),str) or not meta['resourceVersion'] or
                    meta['resourceVersion']==facts['old_rv']): raise ValueError()
            # Preserve the accepted identity locally before any further remote check.
            acknowledged=secret_pending|{'status':'secret_owned','new_rv':meta['resourceVersion']}
            self._ack_secret(secret_pending,acknowledged,before_row,before_revision)
            actual=go._private_kube(['get','secret','voice-app-secrets','-o','json'])
            _secret_unchanged(before,actual,target)
            if actual.get('metadata',{}).get('uid')!=facts['secret_uid'] or actual.get('metadata',{}).get('resourceVersion')!=meta['resourceVersion']:
                raise ValueError()
            credential={'role':'space_runtime','password':password,'sslmode':'disable','host':'127.0.0.1'}
            source={'engine':catalog['engine'],'server_start':catalog['server_start'],'database_oid':native['new_oid']}
            go._login('space_db',credential,source,self._guard)
            receipt={'operation_id':facts['operation_id'],'source_sha':facts['source_sha'],'namespace_uid':self.namespace_uid,
                     'database_oid':native['new_oid'],'role':'space_runtime','native_receipt_sha256':native['native_receipt_sha256'],
                     'secret_uid':facts['secret_uid'],'old_rv':facts['old_rv'],'new_rv':meta['resourceVersion'],
                     'old_data_hmac_sha256':safe['old_data_hmac_sha256'],'new_data_hmac_sha256':safe['new_data_hmac_sha256'],
                     'unchanged_other_keys_hmac_sha256':safe['unchanged_other_keys_hmac_sha256'],'consumers':facts['consumers']}
            done=acknowledged|{'status':'done','receipt_sha256':_digest_public(receipt)}
            self._advance(acknowledged,done,receipt=receipt)
            return copy.deepcopy(receipt)
        except BaseException as error:
            if pending:
                self.poisoned=True
                try: self.fence.uncertain('Space runtime effect uncertain')
                except BaseException: pass
            if isinstance(error,schema.GuardLost): raise
            raise SpaceRuntimeError('Space runtime effect uncertain' if pending else 'Space runtime not admitted') from None


def _digest_public(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',', ':'),allow_nan=False).encode()).hexdigest()


def verify_prepared(phase, token):
    """Recheck pre-cut facts against the drained live target before any DROP."""
    import full_cohort_affected_plan as affected
    import full_cohort_go_migration as go
    import space_fresh_install_phase_pg as space_pg
    try:
        if type(phase) is not SpaceRuntimeRolePhase or token != phase.token: raise ValueError()
        local,marker,key=phase._local(); facts=local['facts']; row,revision=phase._ledger()
        if marker.get('status')!='prepared': raise ValueError()
        phase.cut.verify_drained(); phase._guard()
        inventory=phase.catalog.inventory.verify_receipt(token); catalog=phase.catalog.verify_receipt(token)
        def read(kind,name,key_name):
            phase._guard(); value=schema.kube_key_reader(schema.linux_private_bytes)(kind,name,key_name); phase._guard(); return value
        if _source_routes(inventory['database_observations'],phase.cut.admission['controllers'],read)!=facts['consumers']:
            raise ValueError()
        actual=go._private_kube(['get','secret','voice-app-secrets','-o','json']); meta=actual['metadata']; data=actual['data']
        if (meta.get('uid')!=facts['secret_uid'] or meta.get('resourceVersion')!=facts['old_rv'] or
                _mac(key,data)!=facts['old_data_hmac_sha256'] or
                _mac(key,{k:v for k,v in data.items() if k!='SPACE_DATABASE_URL'})!=facts['unchanged_other_keys_hmac_sha256']):
            raise ValueError()
        pg=space_pg.KubeSpacePostgres(phase._guard)
        if (pg.snapshot()!=catalog['engine'] or pg.space_oid()!=facts['original_space_oid'] or
                _role_check(pg,facts['original_space_oid'],require_zero_sessions=True)!=facts['role_state'] or
                hashlib.sha256(_runtime_sql(phase.source_root,facts['source_sha']).encode()).hexdigest()!=facts['runtime_sql_sha256'] or
                affected._full_catalog(phase.catalog,catalog['engine'],catalog['server_start']) !=
                    {k:catalog[k] for k in ('all_databases','databases','excluded','properties')}): raise ValueError()
        _admin_check(pg,facts['admin_role'],facts['original_space_oid'],facts['role_state'])
        phase.cut.verify_drained(); phase._guard(); after,end=phase._ledger()
        if after!=row or end!=revision or phase.journal.state['steps'].get(phase.STEP)!=local: raise ValueError()
        return copy.deepcopy(facts)
    except schema.GuardLost: raise
    except BaseException: raise SpaceRuntimeError('Space runtime preparation no longer current') from None


def verify_receipt(phase, token):
    """Authenticate historical protected completion without asserting live state."""
    try:
        if type(phase) is not SpaceRuntimeRolePhase or token != phase.token: raise ValueError()
        local,marker,key=phase._local(); row,revision=phase._ledger(); receipt=local['receipt']
        facts=local['facts']
        if (marker.get('status')!='done' or marker.get('receipt_sha256')!=_digest_public(receipt) or
                any(receipt.get(k)!=facts.get(k) for k in ('operation_id','source_sha','namespace_uid','secret_uid','old_rv',
                    'old_data_hmac_sha256','unchanged_other_keys_hmac_sha256','consumers')) or
                receipt.get('role')!='space_runtime' or receipt.get('new_rv')!=marker.get('new_rv') or
                type(receipt.get('database_oid')) is not int or receipt['database_oid']<=0 or
                receipt['database_oid']==facts['original_space_oid']): raise ValueError()
        phase._guard(); after,end=phase._ledger()
        if after!=row or end!=revision or phase.journal.state['steps'].get(phase.STEP)!=local: raise ValueError()
        return copy.deepcopy(receipt)
    except schema.GuardLost: raise
    except BaseException: raise SpaceRuntimeError('Space runtime historical receipt unavailable') from None


def verify_secret_successor(phase, token):
    """Re-read the exact Secret privately and verify protected HMAC custody."""
    import full_cohort_go_migration as go
    try:
        receipt=verify_receipt(phase,token)
        local,marker,key=phase._local(); row,revision=phase._ledger()
        if local.get('receipt')!=receipt: raise ValueError()
        actual=go._private_kube(['get','secret','voice-app-secrets','-o','json'])
        meta=actual['metadata']; data=actual['data']
        other={k:v for k,v in data.items() if k!='SPACE_DATABASE_URL'}
        if (actual.get('kind')!='Secret' or actual.get('apiVersion')!='v1' or meta.get('name')!='voice-app-secrets' or
                meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp') or meta.get('uid')!=receipt['secret_uid'] or
                meta.get('resourceVersion')!=receipt['new_rv'] or 'SPACE_DATABASE_URL' not in data or
                _mac(key,data)!=receipt['new_data_hmac_sha256'] or
                _mac(key,other)!=receipt['unchanged_other_keys_hmac_sha256']): raise ValueError()
        phase._guard(); after,end=phase._ledger()
        if after!=row or end!=revision or phase.journal.state['steps'].get(phase.STEP)!=local: raise ValueError()
        return {k:receipt[k] for k in ('secret_uid','old_rv','new_rv','old_data_hmac_sha256','new_data_hmac_sha256',
                                        'unchanged_other_keys_hmac_sha256','source_sha','operation_id')} | {'receipt_sha256':marker['receipt_sha256']}
    except schema.GuardLost: raise
    except BaseException: raise SpaceRuntimeError('Space runtime Secret successor unavailable') from None


def verify_completed(phase, token):
    import full_cohort_go_migration as go
    verify_secret_successor(phase,token)
    receipt=verify_receipt(phase,token)
    import space_fresh_install_phase_pg as space_pg
    try: catalog=phase.catalog.verify_receipt(token)
    except BaseException: raise schema.GuardLost('Space runtime catalog authority unavailable') from None
    pg=space_pg.KubeSpacePostgres(phase._guard)
    if (pg.snapshot()!=catalog['engine'] or pg.space_oid()!=receipt['database_oid'] or
            _role_check(pg,receipt['database_oid'],require_zero_sessions=False)!='dedicated'):
        raise SpaceRuntimeError('Space runtime role changed')
    try: _grants_check(pg)
    except schema.GuardLost: raise
    except BaseException: raise SpaceRuntimeError('Space runtime grants changed') from None
    try:
        actual=go._private_kube(['get','secret','voice-app-secrets','-o','json'])
        dsn=base64.b64decode(actual['data']['SPACE_DATABASE_URL'],validate=True).decode('ascii')
        match=re.fullmatch(r'postgres://space_runtime:([0-9a-f]{64})@voice-postgres:5432/space_db\?sslmode=disable',dsn)
        if not match: raise ValueError()
        credential={'role':'space_runtime','password':match.group(1),'sslmode':'disable','host':'127.0.0.1'}
        go._login('space_db',credential,{'engine':catalog['engine'],'server_start':catalog['server_start'],
                                        'database_oid':receipt['database_oid']},phase._guard)
    except schema.GuardLost: raise
    except BaseException: raise SpaceRuntimeError('Space runtime login unavailable') from None
    verify_secret_successor(phase,token)
    return copy.deepcopy(receipt)
