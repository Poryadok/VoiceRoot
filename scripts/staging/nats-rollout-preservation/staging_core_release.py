"""STAGING terminal release authority, held under the bridge's root lock."""
import copy
import hashlib
from pathlib import Path

import commands
import guard
import resource_claim
import root_main
import source_authority
import staging_prerequisites as prerequisites
from stage_runtime import Kube
from runtime_stage import revision_template
from gis_existing_update import verify_pod_spec


class Blocked(RuntimeError):
    pass


def _guard(session):
    if (type(session) is not resource_claim.Session or type(session.journal) is not resource_claim.RootJournal
        or session.journal.path != guard.ROOT/'installed'/resource_claim.JOURNAL
        or commands.authority_guard != session.check or not session.admitted):
        raise Blocked('core_release_session_invalid')
    proof = vars(session).get('operation_lock')
    if type(proof) is not root_main.OperationLockProof:
        raise Blocked('core_release_lock_invalid')
    proof.check()
    row = session.check()
    if row.get('phase') not in ('apply-ready', 'awaiting-ci') or not row.get('target_sha256'):
        raise Blocked('core_release_phase_invalid')
    return row


def _read(path, limit=2 << 20):
    return prerequisites._json(prerequisites._read_private(path, limit=limit))


def _controller(row, kind, uid):
    refs = [r for r in row.get('metadata', {}).get('ownerReferences', []) if r.get('controller') is True]
    return len(refs) == 1 and refs[0].get('kind') == kind and refs[0].get('uid') == uid


def _inventory(kube, name, limit):
    listing = kube.run(['get', name, '-o', 'json'])
    if (type(listing) is not dict or type(listing.get('items')) is not list
        or len(listing['items']) > limit or not listing.get('metadata', {}).get('resourceVersion')):
        raise Blocked('core_release_inventory_invalid')
    return listing['items']


def _observe(session, state, approved, originals):
    names = set(source_authority.IMAGE_NAMES) - {'nats-hub-config-renderer'}
    if len(names) != 23 or set(approved.get('images', {})) != set(source_authority.IMAGE_NAMES):
        raise Blocked('core_release_source_inventory_invalid')
    namespace_uid = state['context']['expected']['namespace_uid']
    snapshots = state['context']['snapshots']
    target = state['target']
    kube = Kube()
    _guard(session)
    namespace = kube.get('namespace', 'voice-staging')
    meta = namespace.get('metadata', {})
    if (namespace.get('kind') != 'Namespace' or meta.get('name') != 'voice-staging'
        or meta.get('uid') != namespace_uid or not namespace_uid or meta.get('deletionTimestamp')):
        raise Blocked('core_release_namespace_changed')
    replicasets = _inventory(kube, 'replicasets', 2048)
    pods = _inventory(kube, 'pods', 4096)
    observations = {}
    for service in sorted(names):
        name = 'voice-' + service
        container_name = 'portal' if service == 'developer-portal' else service
        deployment = kube.get('deployment', name)
        md = deployment['metadata']; spec = deployment['spec']; status = deployment.get('status', {})
        template = spec['template']; dep_uid = md.get('uid'); generation = md.get('generation')
        if (deployment.get('kind') != 'Deployment' or md.get('name') != name
            or md.get('namespace') != 'voice-staging' or md.get('deletionTimestamp') or not dep_uid
            or type(generation) is not int or generation < 1 or spec.get('replicas', 1) != 1
            or status.get('observedGeneration', 0) < generation or status.get('readyReplicas') != 1
            or status.get('updatedReplicas') != 1 or status.get('availableReplicas') != 1):
            raise Blocked('core_release_deployment_not_ready')
        snapshot = snapshots.get(name)
        expected_uid = snapshot.get('metadata', {}).get('uid') if type(snapshot) is dict else None
        if name == 'voice-gameintegration' and expected_uid is None:
            expected_uid = state.get('gis_first_install_ready', {}).get('deployment_uid')
        if dep_uid != expected_uid or not expected_uid:
            raise Blocked('core_release_deployment_identity_changed')
        wanted = target.get('template_hashes', {}).get(name)
        original = originals.get(name)
        if wanted is None and type(original) is dict:
            if original.get('metadata', {}).get('uid') != dep_uid:
                raise Blocked('core_release_deployment_identity_changed')
            wanted = resource_claim.digest(original['spec']['template'])
        if not wanted or resource_claim.digest(template) != wanted:
            raise Blocked('core_release_template_changed')
        containers = template['spec'].get('containers', [])
        primary = [row for row in containers if row.get('name') == container_name]
        if len(primary) != 1 or primary[0].get('image') != approved['images'][service]:
            raise Blocked('core_release_image_changed')
        owned = [rs for rs in replicasets if _controller(rs, 'Deployment', dep_uid)
                 and rs.get('spec', {}).get('replicas', 0) != 0]
        if len(owned) != 1:
            raise Blocked('core_release_revision_invalid')
        rs = owned[0]; rm = rs['metadata']; rs_uid = rm.get('uid')
        if (not rs_uid or rm.get('namespace') != 'voice-staging' or rm.get('deletionTimestamp')
            or rs['spec'].get('replicas') != 1
            or revision_template(rs['spec']['template']) != revision_template(template)):
            raise Blocked('core_release_revision_invalid')
        selected = [p for p in pods if _controller(p, 'ReplicaSet', rs_uid)]
        if len(selected) != 1:
            raise Blocked('core_release_pod_invalid')
        old_uids = {r.get('metadata', {}).get('uid') for r in replicasets
                    if _controller(r, 'Deployment', dep_uid)} - {rs_uid}
        if any(_controller(p, 'ReplicaSet', uid) and p.get('status', {}).get('phase') not in ('Succeeded', 'Failed')
               for uid in old_uids for p in pods):
            raise Blocked('core_release_pod_invalid')
        pod = selected[0]; pm = pod['metadata']; ps = pod.get('status', {})
        if (not pm.get('uid') or pm.get('namespace') != 'voice-staging' or pm.get('deletionTimestamp')
            or ps.get('phase') != 'Running'
            or not any(c.get('type') == 'Ready' and c.get('status') == 'True' for c in ps.get('conditions', []))):
            raise Blocked('core_release_pod_invalid')
        verify_pod_spec(template['spec'], pod['spec'])
        statuses = ps.get('containerStatuses', [])
        if (len(statuses) != len(containers) or {r.get('name') for r in statuses} != {r.get('name') for r in containers}
            or any(r.get('ready') is not True or not r.get('imageID') for r in statuses)):
            raise Blocked('core_release_pod_invalid')
        primary_status = next(row for row in statuses if row['name'] == container_name)
        if (not isinstance(primary_status['imageID'], str)
            or primary_status['imageID'].removeprefix('docker-pullable://') != approved['images'][service]):
            raise Blocked('core_release_image_changed')
        observations[name] = {'deployment_uid': dep_uid, 'generation': generation, 'template_sha256': wanted,
                              'replicaset_uid': rs_uid, 'pod_uid': pm['uid'],
                              'images': {r['name']: r['imageID'] for r in statuses}}
    again = kube.get('namespace', 'voice-staging')
    if (again.get('metadata', {}).get('uid') != namespace_uid
        or again.get('metadata', {}).get('deletionTimestamp')):
        raise Blocked('core_release_namespace_changed')
    _guard(session)
    return namespace_uid, observations


def _migration_history(state, approved):
    import migrations
    plan = state['migrations']; target = state['target']
    migrations.validate_plan(plan, target['mode'])
    if (set(row['database'] for row in plan) != set(migrations.DATABASES) | {'auth'}
        or resource_claim.digest(plan) != target['migration_sha256']):
        raise Blocked('core_release_migration_changed')
    source_hashes = target['source_hashes']
    if (type(source_hashes) is not dict or not source_hashes
        or any(approved['source_files'].get(path) != value for path, value in source_hashes.items())):
        raise Blocked('core_release_source_changed')
    for row in plan:
        prefix = ('src/backend/auth/src/main/resources/db/migration/' if row['database'] == 'auth'
                  else 'src/backend/migrations/' + row['database'] + '_db/')
        approved_files = {path[len(prefix):]: value for path, value in approved['source_files'].items()
                          if path.startswith(prefix) and '/' not in path[len(prefix):] and path.endswith('.sql')}
        if (approved_files != row['hashes']
            or any(source_hashes.get(prefix + name) != digest for name, digest in row['hashes'].items())):
            raise Blocked('core_release_source_changed')
    documents = migrations.documents(plan, state['operation'])
    expected = [(row, cm, job) for row, cm, job in zip(plan, documents[::2], documents[1::2])
                if row['database'] != 'space']
    jobs = state.get('migration_jobs')
    if type(jobs) is not list or len(jobs) != len(expected):
        raise Blocked('core_release_migration_changed')
    complete = [event for event in state.get('events', []) if event.get('kind') == 'migration_complete']
    if complete != [{'kind': 'migration_complete', **row} for row in jobs]:
        raise Blocked('core_release_migration_changed')
    for saved, (_, cm, job) in zip(jobs, expected):
        if (set(saved) != {'name', 'uid', 'configmap_uid'} or saved['name'] != job['metadata']['name']
            or any(not guard.UUID.fullmatch(str(saved[key])) for key in ('uid', 'configmap_uid'))):
            raise Blocked('core_release_migration_changed')
        for kind, obj, uid in (('ConfigMap', cm, saved['configmap_uid']), ('Job', job, saved['uid'])):
            event = {'kind': 'migration_resource_created', 'resource_kind': kind,
                     'name': obj['metadata']['name'], 'uid': uid}
            if state['events'].count(event) != 1:
                raise Blocked('core_release_migration_changed')


def _evidence(session):
    import staging_space_native
    _guard(session)
    operation = session.operation
    base = guard.ROOT / ('rollout-' + operation)
    state = _read(base/'checkpoint.json', 128 << 20)
    binding = resource_claim.historical_binding(state)
    if (state.get('status') != 'PASS' or state.get('phase') != 'POST_RESTART'
        or state.get('fence_status') != 'RELEASED' or binding != session.binding()):
        raise Blocked('core_release_terminal_changed')
    capture = prerequisites.validate_historical_capture(operation)
    native = staging_space_native.validate_historical_live_completed(operation)
    if native != state.get('native_space_migration'):
        raise Blocked('core_release_native_changed')
    parent = guard.ROOT/'installed'/'prerequisites'/operation
    approved = _read(parent/'approved.json', 32 << 20)
    _migration_history(state, approved)
    prerequisites._verify_helper_source(session, approved, guard.ROOT/'installed'/'sources'/operation)
    custody = _read(parent/'code-custody.json')
    relative = 'nats-rollout-preservation/staging_core_release.py'
    if (Path(__file__) != Path(custody['root'])/relative
        or custody['manifest'].get(relative) != hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
        or approved.get('source_sha') != session.context['commit']):
        raise Blocked('core_release_source_changed')
    active = _read(guard.ROOT/'installed'/'active-release.json')
    expected = state['context']['expected']
    if (active.get('operation') != operation or active.get('namespace_uid') != expected['namespace_uid']
        or active.get('pvc_uid') != expected['source_claim_uid']
        or active.get('target_sha256') != state['target']['manifest_sha256']):
        raise Blocked('core_release_active_changed')
    originals = _read(parent/'capture-snapshots.json', 32 << 20)
    if resource_claim.digest(originals) != capture['stage']['original_snapshots_sha256']:
        raise Blocked('core_release_custody_changed')
    namespace_uid, workloads = _observe(session, state, approved, originals)
    if (_read(base/'checkpoint.json', 128 << 20) != state
        or prerequisites.validate_historical_capture(operation) != capture
        or staging_space_native.validate_historical_live_completed(operation) != native
        or _read(guard.ROOT/'installed'/'active-release.json') != active):
        raise Blocked('core_release_custody_changed')
    _guard(session)
    return {'schema': 'voice-staging-core-release-v1', 'operation': operation,
            'context': copy.deepcopy(session.context), 'helper_sha256': session.helper,
            'target_sha256': binding['target_sha256'], 'namespace_uid': namespace_uid,
            'checkpoint_sha256': resource_claim.digest(state), 'capture_sha256': resource_claim.digest(capture),
            'native_sha256': resource_claim.digest(native), 'approved_sha256': resource_claim.digest(approved),
            'active_sha256': resource_claim.digest(active), 'workloads': workloads}


class CoreReleasePhase:
    def __init__(self, session):
        _guard(session)
        self.session = session

    def produce(self):
        path = guard.ROOT/('rollout-' + self.session.operation)/'core-release.json'
        if path.exists() or path.is_symlink():
            return verify_completed(self, self.session)
        first = _evidence(self.session)
        if _evidence(self.session) != first:
            raise Blocked('core_release_observation_changed')
        _guard(self.session)
        prerequisites._private_write(path, prerequisites._bytes(first))
        if _read(path, 8 << 20) != first:
            raise Blocked('core_release_receipt_changed')
        _guard(self.session)
        return verify_completed(self, self.session)


def verify_completed(phase, session):
    if type(phase) is not CoreReleasePhase or phase.session is not session:
        raise Blocked('core_release_phase_invalid')
    _guard(session)
    path = guard.ROOT/('rollout-' + session.operation)/'core-release.json'
    receipt = _read(path, 8 << 20)
    if _evidence(session) != receipt or _read(path, 8 << 20) != receipt:
        raise Blocked('core_release_receipt_changed')
    _guard(session)
    return {'operation_id': session.context['operation_id'], 'source_sha': session.context['commit'],
            'namespace_uid': receipt['namespace_uid'], 'target_sha256': receipt['target_sha256']}
