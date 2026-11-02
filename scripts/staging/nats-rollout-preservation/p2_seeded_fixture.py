"""Hosted-only populated NATS setup for an owned P2 integration fixture.

All Kubernetes objects here are synthetic. The broker, JWTs, authorization,
store, native archive, and selected-store custody checks are real. A caller
owns the VM and must retain and reconcile the handle on every ambiguous error.
"""
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import sys
import tempfile
import uuid


def require_hosted():
    if (sys.platform != 'linux' or os.geteuid() != 0 or
        os.environ.get('GITHUB_ACTIONS') != 'true' or
        os.environ.get('RUNNER_ENVIRONMENT') != 'github-hosted'):
        raise RuntimeError('owned_fixture_host_required')


def require_external(base, store):
    base, store = Path(base), Path(store)
    if store.is_relative_to(base):
        raise RuntimeError('owned_fixture_store_not_external')


def _root_directory(path, *, private=False):
    path = Path(path)
    row = path.lstat()
    if (not path.is_absolute() or path.resolve(strict=True) != path or
        not stat.S_ISDIR(row.st_mode) or row.st_uid != 0 or row.st_mode & (0o077 if private else 0o022)):
        raise RuntimeError('owned_fixture_root_untrusted')
    return path


class FixtureKube:
    """Private in-memory Kubernetes-shaped test data, never a live API."""
    def __init__(self, marker, old, operator):
        self.rows = {('configmap', marker['metadata']['name']): copy.deepcopy(marker),
                     ('secret', old['metadata']['name']): copy.deepcopy(old),
                     ('secret', operator['metadata']['name']): copy.deepcopy(operator)}
        self.sequence = 0

    def get(self, kind, name):
        return copy.deepcopy(self.rows[(kind, name)])

    def optional_secret(self, name):
        row = self.rows.get(('secret', name))
        return copy.deepcopy(row) if row is not None else None

    def create_secret(self, row):
        key = ('secret', row['metadata']['name'])
        if key in self.rows:
            raise RuntimeError('owned_fixture_secret_exists')
        row = copy.deepcopy(row)
        self.sequence += 1
        row['metadata'].update(uid=str(uuid.uuid4()), resourceVersion=str(self.sequence))
        self.rows[key] = row


class Retained:
    """Mutable custody handle supplied before the first broker side effect."""
    def __init__(self, base, store, checkpoint):
        self.base, self.store = base, store
        self._checkpoint = checkpoint
        self.runtimes = []
        self.runtime = None
        self.broker = None
        self.installation = None
        self.enrollment_v1 = self.enrollment_v4 = None
        self.kube = None
        self.census = self.manifest = self.descriptor = None
        self.input_sha256 = None
        self.claim = self.pv = self.hub = None
        self.binding_sha256 = None
        self._custody = None
        self._identity_custody = None
        self.identity_sha256 = None
        self.private_seed = None
        self.private_seed_sha256 = None

    def seal_identity_custody(self):
        row = self.base.lstat()
        self._identity_custody = (row.st_dev,row.st_ino,copy.deepcopy(self.input_sha256))

    def verify_identity_custody(self):
        if self._identity_custody is None:raise RuntimeError('owned_fixture_custody_unsealed')
        _root_directory(self.base)
        row = self.base.lstat()
        if (row.st_dev,row.st_ino,self.input_sha256)!=self._identity_custody:
            raise RuntimeError('owned_fixture_custody_changed')

    def identity_digest(self):
        from nats_contract_plan import digest
        return digest({'installation_sha256':self.installation.sha256,'enrollment_v1':self.enrollment_v1,
            'enrollment_v4':self.enrollment_v4,'input_sha256':self.input_sha256})

    def verify_identity(self):
        import bootstrap_selection
        self.verify_identity_custody()
        if self.identity_sha256 is None or self.identity_digest()!=self.identity_sha256:
            raise RuntimeError('owned_fixture_identity_changed')
        bootstrap_selection.select(self.kube,self.enrollment_v4,self.installation.generation,installation=self.installation)
        if hashlib.sha256((self.base/'inputs/bootstrap.creds').read_bytes()).hexdigest()!=self.enrollment_v4['credential_sha256']:
            raise RuntimeError('owned_fixture_identity_changed')

    def seal_custody(self):
        if self._custody is not None:
            raise RuntimeError('owned_fixture_custody_already_sealed')
        base, store = self.base.lstat(), self.store.lstat()
        self._custody = (base.st_dev,base.st_ino,store.st_dev,store.st_ino,
            copy.deepcopy(self.descriptor),copy.deepcopy(self.input_sha256))

    def verify_custody(self):
        if self._custody is None:
            raise RuntimeError('owned_fixture_custody_unsealed')
        _root_directory(self.base)
        _verify_selected_path(self.store,self.descriptor)
        base, store = self.base.lstat(),self.store.lstat()
        if (base.st_dev,base.st_ino,store.st_dev,store.st_ino,self.descriptor,self.input_sha256) != self._custody:
            raise RuntimeError('owned_fixture_custody_changed')

    def register(self, runtime):
        self.runtimes.append(runtime)
        # The operation label is allocated before any Docker create. The
        # callback persists public identity only, never kube rows or secrets.
        self._checkpoint(self)

    def binding_digest(self):
        from nats_contract_plan import digest
        return digest({'installation_sha256':self.installation.sha256,
            'enrollment_v1':self.enrollment_v1,'enrollment_v4':self.enrollment_v4,
            'descriptor':self.descriptor,'census':self.census,'manifest':self.manifest,
            'input_sha256':self.input_sha256,'claim':self.claim,'pv':self.pv,'hub':self.hub})

    def verify_binding(self):
        import bootstrap_selection
        self.verify_identity()
        self.verify_custody()
        if self.binding_sha256 is None or self.binding_digest()!=self.binding_sha256:
            raise RuntimeError('owned_fixture_binding_changed')
        bootstrap_selection.select(self.kube,self.enrollment_v4,self.installation.generation,installation=self.installation)
        _verify_selected_path(self.store,self.descriptor)


def _checked_file(root, relative, expected):
    if not isinstance(expected,str) or not re.fullmatch(r'[a-f0-9]{64}',expected):
        raise RuntimeError('owned_fixture_input_pin_invalid')
    path = root/relative
    if path.resolve(strict=True) != path:
        raise RuntimeError('owned_fixture_input_untrusted')
    for parent in (path,*path.parents):
        row = parent.lstat()
        if row.st_uid != 0 or row.st_mode & 0o022 or stat.S_ISLNK(row.st_mode):
            raise RuntimeError('owned_fixture_input_untrusted')
        if parent == root:break
    fd = os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        row = os.fstat(fd)
        if not stat.S_ISREG(row.st_mode) or row.st_nlink != 1 or not 0 < row.st_size <= 32 << 20:
            raise RuntimeError('owned_fixture_input_untrusted')
        digest = hashlib.sha256()
        while chunk := os.read(fd,1 << 20):digest.update(chunk)
        after = os.fstat(fd)
        if ((row.st_dev,row.st_ino,row.st_size,row.st_mtime_ns,row.st_ctime_ns) !=
            (after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns) or digest.hexdigest()!=expected):
            raise RuntimeError('owned_fixture_input_changed')
    finally:os.close(fd)
    return path


def _verify_selected_path(path, descriptor):
    from stage_runtime import verify_selected_store_leaf
    fd = os.open('/',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        root = os.fstat(fd)
        if root.st_uid != 0 or root.st_mode & 0o022:
            raise RuntimeError('owned_fixture_selected_parent_untrusted')
        for index, name in enumerate(path.parts[1:]):
            child = os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=fd)
            os.close(fd); fd = child; row = os.fstat(fd)
            if index == len(path.parts)-2:
                verify_selected_store_leaf(row,descriptor)
            elif row.st_uid != 0 or row.st_mode & 0o022:
                raise RuntimeError('owned_fixture_selected_parent_untrusted')
    finally:os.close(fd)


def _copy_binary(source, target):
    if source.is_symlink() or not source.is_file():
        raise RuntimeError('owned_fixture_binary_invalid')
    shutil.copyfile(source, target)
    os.chown(target, 0, 65532)
    target.chmod(0o550)


def _run_owned(runtime, suffix, image, mounts, command, broker):
    name = runtime.create(suffix, image, mounts, command, broker)
    runtime.run(['start', name])
    if runtime.run(['wait', name], timeout=60) != '0' or runtime.inspect(name)['State']['Running']:
        raise RuntimeError('owned_fixture_seed_failed')


def _proof_legacy(handle, credentials, operator, installation):
    """Prove the legacy single-consumer receipt against a live empty broker."""
    from actor_auth import AuthScratch
    from bootstrap_root import secret_bytes, hash_bytes
    from docker_runtime import DockerRuntime, NATS_IMAGE
    from nats_contract_actor import Actor
    from nats_contract_plan import DECLARATION, digest
    from scenario import ready
    root = handle.base/'auth-custody'
    root.mkdir(mode=0o700, exist_ok=True)
    with AuthScratch('previous-', root) as scratch:
        base = scratch.path
        base.chmod(0o750); os.chown(base, 0, 65532)
        inputs = base/'inputs'; inputs.mkdir(mode=0o750); os.chown(inputs, 0, 65532)
        p = inputs/'bootstrap.creds'; p.write_bytes(credentials); p.chmod(0o440); os.chown(p, 0, 65532)
        values = {key: secret_bytes(operator, key).decode().strip() for key in
                  ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
        if hash_bytes(values['account.public'].encode()) != installation.account_sha256:
            raise RuntimeError('owned_fixture_account_changed')
        q = json.dumps
        config = ('host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\nmax_control_line: 32768\noperator: '+q(values['operator.jwt'])+
                  '\nsystem_account: '+q(values['system-account.public'])+'\nresolver: MEMORY\nresolver_preload: { '+q(values['account.public'])+
                  ': '+q(values['account.jwt'])+', '+q(values['system-account.public'])+': '+q(values['system-account.jwt'])+' }\njetstream { store_dir: /data }\n')
        p = base/'server.conf'; p.write_text(config); p.chmod(0o440); os.chown(p, 0, 65532)
        store = base/'store'; store.mkdir(mode=0o700); os.chown(store, 65532, 65532)
        runtime = DockerRuntime(base, 'prior'+uuid.uuid4().hex[:12]); handle.register(runtime)
        scratch.hold()
        broker = runtime.start_broker('proof', store); ready(runtime, broker)
        actor = Actor(runtime, broker, runtime.owned[broker]['id'], store, lambda: None)
        stream = 'social_events'; declaration = DECLARATION
        actor._request('$JS.API.STREAM.CREATE.'+stream, {'name':stream,'subjects':[declaration['filter_subject']],
            'storage':'file','retention':'limits','max_age':604800000000000,'duplicate_window':120000000000})
        actor.mutate('$JS.API.CONSUMER.CREATE.'+stream+'.'+declaration['durable_name'],
                     {'stream_name':stream,'action':'create','config':copy.deepcopy(declaration)})
        normalized = actor.info_config(stream+'/'+declaration['durable_name'])
        if any(normalized.get(k) != v for k,v in declaration.items()):
            raise RuntimeError('owned_fixture_legacy_auth_invalid')
        tree = runtime.monitor_jsz(broker)
        if (tree['messages'] != 0 or tree['streams'] != 1 or tree['consumers'] != 1 or
            [a['id'] for a in tree['account_details'] if a.get('stream_detail')] != [values['account.public']]):
            raise RuntimeError('owned_fixture_legacy_auth_invalid')
        runtime.stop(broker)
        for name in reversed(list(runtime.owned)):
            row = runtime.inspect(name); runtime.run(['rm','-f',row['Id']])
            if runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}']):
                raise RuntimeError('owned_fixture_cleanup_uncertain')
        runtime.no_operation_containers(); scratch.release()
        return {'verified':True,'server_image':NATS_IMAGE,'account_sha256':installation.account_sha256,
                'reply_prefix':'_INBOX.voice.bootstrap.reply','normalized_consumer':normalized,
                'normalized_consumer_sha256':digest(normalized)}


def _identity(handle, old, operator):
    """Enroll one actual generated identity through both scoped receipt formats."""
    import bootstrap_auth
    import bootstrap_enrollment
    import bootstrap_selection
    import nats_contract_integration as contract
    from bootstrap_renewal import invoke
    from nats_contract_plan import digest
    token = old.decode().split('-----BEGIN NATS USER JWT-----\n',1)[1].split('\n',1)[0]
    claims = json.loads(base64.urlsafe_b64decode(token.split('.')[1]+'=='))
    actor_sha = hashlib.sha256(claims['sub'].encode()).hexdigest()
    account = (handle.base/'inputs/account.public').read_text().strip()
    account_sha = hashlib.sha256(account.encode()).hexdigest()
    installation = contract.fixture_installation(account_sha, actor_sha, operator)
    generation = installation.generation
    def metadata(name):
        return {'name':name,'namespace':'voice-staging','uid':str(uuid.uuid4()),'resourceVersion':'1'}
    marker = {'metadata':metadata('voice-nats-generation'),'data':{'generation':generation,'phase':'active'}}
    old_secret = {'metadata':metadata('voice-nats-bootstrap-credentials-'+generation),
        'type':'Opaque','immutable':True,'data':{'bootstrap.creds':base64.b64encode(old).decode()}}
    operator = copy.deepcopy(operator)
    operator['metadata'] = metadata('voice-nats-operator-'+generation)
    kube = FixtureKube(marker, old_secret, operator)
    handle.installation, handle.kube = installation, kube
    binding = {'generation':generation,'marker_uid':marker['metadata']['uid'],
        'marker_rv':marker['metadata']['resourceVersion'],'account_sha256':account_sha,'actor_sha256':actor_sha,
        'old_secret':{k:old_secret['metadata'][k] for k in ('name','uid','resourceVersion')} |
                     {'credential_sha256':hashlib.sha256(old).hexdigest()},
        'operator_secret':{k:operator['metadata'][k] for k in ('name','uid','resourceVersion')}}
    def unchanged():
        if (kube.get('configmap','voice-nats-generation') != marker or
            kube.get('secret',old_secret['metadata']['name']) != old_secret or
            kube.get('secret',operator['metadata']['name']) != operator):
            raise RuntimeError('owned_fixture_identity_changed')
    prior_auth = {}
    def authenticated_prior(value):
        prior_auth.update(_proof_legacy(handle,value,operator,installation))
        return prior_auth.get('verified') is True
    committed = {}
    def commit_prior(row):
        row = dict(row,schema='voice-nats-bootstrap-enrollment-scoped-v1',
                   installation_sha256=installation.sha256,auth_proof=prior_auth)
        bootstrap_selection.select(kube,row,generation,installation=installation)
        committed.update(row)
    bootstrap_enrollment.enroll(kube,binding,old,unchanged,authenticated_prior,commit_prior)
    previous = copy.deepcopy(committed)
    request = {'schema':'voice-nats-existing-bootstrap-renewal-v1','old_creds':old.decode(),
        'account_jwt':(handle.base/'inputs/account.jwt').read_text().strip(),
        'operator_jwt':(handle.base/'inputs/operator.jwt').read_text().strip(),
        'signer_seed':(handle.base/'inputs/synthetic-account.seed').read_text(),
        'actor_sha256':actor_sha,'account_sha256':account_sha}
    renewed = invoke(handle.base/'bootstrap-renewer',request)
    current_binding = dict(binding,previous_enrollment_sha256=digest(previous))
    fresh_auth = {}
    def authenticated(value):
        # Track real runtime operations before they can create a container, so
        # an ambiguous create can be reconciled by its exact operation label.
        original = bootstrap_auth.DockerRuntime
        class RecordedRuntime(original):
            def __init__(self,*args,**kwargs):
                super().__init__(*args,**kwargs)
                handle.register(self)
        bootstrap_auth.DockerRuntime = RecordedRuntime
        try:result = bootstrap_auth.prove(value,operator,unchanged,installation=installation)
        finally:bootstrap_auth.DockerRuntime = original
        fresh_auth.update(result)
        return result.get('verified') is True
    current = {}
    def commit_current(row):
        row = dict(row,schema='voice-nats-bootstrap-enrollment-scoped-v4',
            installation_sha256=installation.sha256,auth_proof=copy.deepcopy(fresh_auth),
            previous_enrollment=previous,previous_enrollment_sha256=digest(previous))
        bootstrap_selection.select(kube,row,generation,installation=installation)
        current.update(row)
    bootstrap_enrollment.enroll(kube,current_binding,renewed,unchanged,authenticated,commit_current)
    handle.enrollment_v1, handle.enrollment_v4 = previous, copy.deepcopy(current)
    return renewed


def _restore_to_selected(archive, manifest, selected):
    """Copy verified native records into a caller-owned, empty selected leaf."""
    from native_store import restore_closed_store, archive_closed_store
    incoming = selected/'incoming'
    restore_closed_store(archive,incoming,manifest)
    for child in incoming.iterdir():
        os.rename(child,selected/child.name)
    incoming.rmdir()
    for root, dirs, files in os.walk(selected,topdown=True,followlinks=False):
        for name in dirs + files:
            path = Path(root)/name; row = path.lstat()
            if not (stat.S_ISDIR(row.st_mode) or stat.S_ISREG(row.st_mode)):
                raise RuntimeError('owned_fixture_restored_entry_invalid')
            os.chown(path,65532,10000)
            path.chmod(0o2770 if stat.S_ISDIR(row.st_mode) else 0o660)
    check = selected.parent/('fixture-verify-'+uuid.uuid4().hex+'.tar')
    try:
        actual = archive_closed_store(selected,check)
        if actual != manifest:
            raise RuntimeError('owned_fixture_restored_store_changed')
    finally:
        if check.exists():check.unlink()


def _restore_selected_after_fence(check, archive, manifest, selected):
    check()
    _restore_to_selected(archive,manifest,selected)


def _check_seed_boundary(handle, session, row, stage):
    handle.verify_identity()
    if session.check()!=row:raise RuntimeError('owned_fixture_session_changed')
    stage.verify_final_storage()
    handle.verify_custody()


def _check_preflight_seed(session, stage, seed_census):
    import staging_cohort_preservation as cohort
    import transaction
    from rollout_census import semantic, validate_snapshot
    state=cohort._state(session)
    live=transaction.require_original_live_census(stage,state['events'])
    if semantic(validate_snapshot(live))!=semantic(validate_snapshot(seed_census)):
        raise RuntimeError('owned_fixture_preflight_census_changed')


def _check_private_seed(handle):
    from native_store import verify_archive
    from nats_contract_plan import digest
    if (handle.private_seed is None or digest(handle.private_seed)!=handle.private_seed_sha256
        or not verify_archive(handle.base/'seed.tar',handle.private_seed['manifest'])
        or handle.runtime.inspect(handle.private_seed['broker'])['State']['Running']):
        raise RuntimeError('owned_fixture_private_seed_changed')


def reconcile_containers(handle, assert_reconciled):
    """After caller reconciliation remove only exactly owned test containers.

    Neither the selected leaf nor the guard root is removed here. Unknown IDs,
    failed inspection and ambiguous stop/remove retain all remaining evidence.
    assert_reconciled must establish that every issued Docker operation,
    including a create that may complete late, reached a terminal result and
    that the daemon is reachable. Empty inventories alone do not establish
    quiescence. After SIGKILL, lost handles or daemon outage, retain the paths
    and operation labels until external reconciliation or disposable-VM teardown.
    """
    if not callable(assert_reconciled) or assert_reconciled(handle) is not True:
        raise RuntimeError('owned_fixture_reconciliation_required')
    if handle._custody is None:handle.verify_identity_custody()
    elif handle.binding_sha256 is None:handle.verify_custody()
    else:handle.verify_binding()
    from docker_runtime import LABEL
    inventories = []
    for runtime in handle.runtimes:
        rows = runtime.run(['ps','-a','--filter','label='+LABEL+'='+runtime.operation,'--format','{{.ID}}','--no-trunc']).splitlines()
        if len(rows)!=len(set(rows)):
            raise RuntimeError('owned_fixture_cleanup_uncertain')
        known = {row['id']:name for name,row in runtime.owned.items()}
        if (len(known)!=len(runtime.owned) or any(not re.fullmatch(r'[a-f0-9]{64}',item) for item in known)
            or set(rows)-set(known)):
            raise RuntimeError('owned_fixture_cleanup_uncertain')
        for container_id in rows:runtime.inspect(known[container_id])
        inventories.append((runtime,rows,known))
    for runtime,rows,known in inventories:
        current = runtime.run(['ps','-a','--filter','label='+LABEL+'='+runtime.operation,'--format','{{.ID}}','--no-trunc']).splitlines()
        if len(current)!=len(set(current)) or set(current)!=set(rows):
            raise RuntimeError('owned_fixture_cleanup_uncertain')
        for container_id in rows:
            name = known[container_id]; state = runtime.inspect(name)['State']
            if state['Running']:runtime.stop(name)
            runtime.run(['rm','-f',container_id])
            if runtime.run(['ps','-a','--filter','id='+container_id,'--format','{{.ID}}']):
                raise RuntimeError('owned_fixture_cleanup_uncertain')
        runtime.no_operation_containers()


def prepare_owned_identity(base, *, bundle_root, source_root, input_sha256, on_retained):
    """Create scoped enrollment before Session admission; retain on failure.

    Caller supplies input_sha256 derived from the reviewed captured source and
    pinned build provenance; hashes of untrusted observed files are insufficient.
    Caller creates private `base` and the exclusive guard.ROOT on a disposable hosted
    VM. `on_retained(handle)` must durably retain custody before Docker calls.
    """
    require_hosted()
    import guard
    base = _root_directory(base,private=True)
    bundle_root = _root_directory(bundle_root)
    source_root = _root_directory(source_root)
    roles = ('realtime','notification','analytics-chat','search')
    bundle_names = ('helper','kernel','bootstrap-renewer','nats-9ae-predecessor.json')
    expected = {'bundle/'+name for name in bundle_names} | {'source/'+role for role in roles}
    if type(input_sha256) is not dict or set(input_sha256)!=expected:
        raise RuntimeError('owned_fixture_input_pin_invalid')
    pinned = {name:_checked_file(bundle_root,name,input_sha256['bundle/'+name]) for name in bundle_names}
    scripts = {role:_checked_file(source_root,
        'scripts/staging/nats-known-baseline/testdata/deployed-bootstrap-20261004/'+role+'-bootstrap.sh',
        input_sha256['source/'+role]) for role in roles}
    _root_directory(guard.ROOT,private=True)
    if any(Path(guard.ROOT).iterdir()):
        raise RuntimeError('owned_fixture_guard_root_not_empty')
    if any(base.iterdir()) or not callable(on_retained):
        raise RuntimeError('owned_fixture_base_not_empty')
    handle = Retained(base,None,on_retained)
    handle.input_sha256 = copy.deepcopy(input_sha256)
    handle.seal_identity_custody()
    on_retained(handle)
    import nats_contract_integration as contract
    contract.fixture.call([str(pinned['helper']),'prepare',str(base)])
    for name in ('helper','kernel','bootstrap-renewer'):
        _copy_binary(pinned[name],base/name)
    for role in roles:
        source = scripts[role]
        target = base/('bootstrap-'+role+'.sh');shutil.copyfile(source,target);os.chown(target,0,65532);target.chmod(0o440)
    baseline = pinned['nats-9ae-predecessor.json']
    shutil.copyfile(baseline,base/'inputs/predecessor.json');os.chown(base/'inputs/predecessor.json',0,65532);(base/'inputs/predecessor.json').chmod(0o440)
    for p in [base/'inputs',base/'server.conf',*(base/'inputs').iterdir()]:os.chown(p,0,65532)
    base.chmod(0o750);os.chown(base,0,65532)
    old = (base/'inputs/bootstrap.creds').read_bytes()
    operator = {'data':{key:base64.b64encode((base/'inputs'/key).read_bytes()).decode() for key in
        ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}}
    renewed = _identity(handle,old,operator)
    (base/'inputs/bootstrap.creds').write_bytes(renewed)
    handle.identity_sha256=handle.identity_digest();handle.verify_identity();on_retained(handle)
    return handle


def prepare_private_seed(handle):
    """Populate and close the private broker before a Session observes its census."""
    require_hosted()
    from docker_runtime import DockerRuntime, NATS_IMAGE
    from native_store import archive_closed_store
    from rollout_census import census
    from scenario import ready
    from nats_contract_plan import digest
    from types import SimpleNamespace
    handle.verify_identity()
    if handle.private_seed is not None:raise RuntimeError('owned_fixture_seed_already_prepared')
    base=handle.base
    runtime = DockerRuntime(base,'ownedp2'+uuid.uuid4().hex[:12]);handle.runtime=runtime;handle.register(runtime)
    store = base/'seed-store';store.mkdir(mode=0o700);os.chown(store,65532,65532)
    broker = runtime.start_broker('seed',store);ready(runtime,broker);runtime.bootstrap(broker)
    seeded = runtime.kernel(broker,'seed')/'fixture.json'
    snapshot = base/'inputs/fixture.json';snapshot.write_bytes(seeded.read_bytes());snapshot.chmod(0o440);os.chown(snapshot,0,65532)
    for suffix, command in (('baseline-record-seed','seed-contract'),('extra-seed','extra'),('record-verify','verify')):
        _run_owned(runtime,suffix,NATS_IMAGE,[(base/'helper','/kernel',False),(base/'inputs','/inputs',False)],['/kernel',command],broker)
    account = (base/'inputs/account.public').read_text().strip()
    tree=runtime.monitor_jsz(broker)
    before = census(SimpleNamespace(monitor_jsz=lambda _:copy.deepcopy(tree)),broker,account)
    if sum(row['state']['messages'] for row in before['streams']) != 9 or len(before['consumers']) != 45:
        raise RuntimeError('owned_fixture_census_invalid')
    runtime.stop(broker)
    if runtime.inspect(broker)['State']['Running']:raise RuntimeError('owned_fixture_seed_not_closed')
    archive = base/'seed.tar';manifest = archive_closed_store(store,archive)
    handle.verify_identity()
    handle.private_seed={'broker':broker,'account':account,'census':before,'monitor_tree':tree,'manifest':manifest}
    handle.private_seed_sha256=digest(handle.private_seed)
    handle._checkpoint(handle)
    return {'census':copy.deepcopy(before),'monitor_tree':copy.deepcopy(tree)}


def seed_prepared(handle, stage, *, session):
    """Restore the pre-observed seed into the captured, fenced selected store."""
    require_hosted()
    from stage_runtime import selected_store_descriptor
    from rollout_census import census, semantic
    from scenario import ready
    from resource_claim import Session
    from runtime_stage import RolloutStage, HUB
    handle.verify_identity()
    if not isinstance(session,Session) or session.installation.sha256!=handle.installation.sha256:
        raise RuntimeError('owned_fixture_session_changed')
    if not isinstance(stage,RolloutStage) or stage.operation!=session.operation:
        raise RuntimeError('owned_fixture_stage_changed')
    _check_private_seed(handle)
    row=copy.deepcopy(session.check())
    stage.verify_final_storage()
    _check_preflight_seed(session,stage,handle.private_seed['census'])
    if handle._custody is not None:raise RuntimeError('owned_fixture_store_already_bound')
    base=handle.base;selected_path=Path(stage.final_path)
    claim,pv,hub=stage.final_claim,stage.final_pv,stage.snapshots[HUB]
    require_external(base,selected_path)
    descriptor=selected_store_descriptor(hub,claim,pv)
    if (not selected_path.is_absolute() or selected_path.resolve(strict=True)!=selected_path
        or descriptor['path']!=str(selected_path) or any(selected_path.iterdir())):
        raise RuntimeError('owned_fixture_selected_store_invalid')
    _verify_selected_path(selected_path,descriptor)
    handle.store=selected_path;handle.descriptor=copy.deepcopy(descriptor)
    handle.claim,handle.pv,handle.hub=copy.deepcopy(claim),copy.deepcopy(pv),copy.deepcopy(hub)
    handle.seal_custody();handle._checkpoint(handle)
    def still_fenced():
        _check_seed_boundary(handle,session,row,stage)
        _check_preflight_seed(session,stage,handle.private_seed['census'])
        _check_private_seed(handle)
    runtime=handle.runtime;broker=handle.private_seed['broker'];account=handle.private_seed['account']
    before=handle.private_seed['census'];manifest=handle.private_seed['manifest'];archive=base/'seed.tar'
    if runtime.inspect(broker)['State']['Running']:raise RuntimeError('owned_fixture_seed_not_closed')
    _restore_selected_after_fence(still_fenced,archive,manifest,selected_path)
    def closed():
        still_fenced()
        if runtime.inspect(broker)['State']['Running']:raise RuntimeError('owned_fixture_seed_not_closed')
        _verify_selected_path(selected_path,descriptor)
    still_fenced()
    runtime.allow_bound_store(selected_path,claim,pv,closed,descriptor=descriptor)
    still_fenced()
    selected = runtime.start_broker('selected',selected_path);handle.broker = selected
    ready(runtime,selected)
    observed = census(runtime,selected,account)
    if semantic(observed)!=semantic(before):raise RuntimeError('owned_fixture_restored_census_changed')
    runtime.stop(selected)
    if runtime.inspect(selected)['State']['Running']:raise RuntimeError('owned_fixture_selected_not_closed')
    handle.census,handle.manifest = observed,manifest
    still_fenced()
    handle.binding_sha256 = handle.binding_digest()
    handle.verify_binding()
    handle._checkpoint(handle)
    return handle


