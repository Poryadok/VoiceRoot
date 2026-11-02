"""Explicitly profiled full Go integration diagnostics, never a preflight pass.

Only explicitly listed services are implemented. Other services require a reviewed profile
for their own images, binaries, environment and dedicated CI checks. The caller
must supervise this process and maintain exclusive private daemon/socket custody.
"""
import json
import hashlib
import os
import os as _os
from pathlib import Path
import re
import sys
import tempfile
import uuid

import affected_check_plan
import local_chat_full_integration as chat
import local_chat_orchestration as orchestration
from local_docker_owned import DockerCLI
from local_docker_owned import OwnedRun, OwnershipError
from local_docker_scheduled import ScheduledRun
import local_heavy_slots as heavy_slots
import local_go_dependencies
import local_go_multi_dependencies
import local_go_plan
import local_source_snapshot as snapshot
import local_ci_proofs_scheduler as proof_scheduler
import local_nats_hosted_diagnostic as bootstrap
import local_process_supervision as supervision
import local_auth_full_orchestration as auth_broker

BOT_IMAGES = ('postgres:16-bookworm', 'nats:2.10-alpine')
SEARCH_IMAGES = ('postgres:16-bookworm',)
USER_IMAGES = ('postgres:16-bookworm', 'migrate/migrate:v4.18.1')
ROLE_IMAGES = ('postgres:16-bookworm', 'migrate/migrate:v4.18.1', 'redis:7-alpine')
REALTIME_IMAGES = ()
POSTGRES_SERVICES = frozenset({'search', 'controlledgame', 'moderation', 'social', 'subscription', 'story'})
EXTRA_SERVICES = POSTGRES_SERVICES | {'bot', 'user', 'realtime', 'analytics', 'gameintegration', 'role', 'messaging', 'notification', 'voice'}


def profile(service):
    if not isinstance(service, str):
        raise ValueError('unsupported Go integration service')
    if service == 'chat':
        return {'images': chat.IMAGES, 'module': 'src/backend/chat'}
    if service == 'bot':
        return {'images': BOT_IMAGES, 'module': 'src/backend/bot'}
    if service == 'user':
        return {'images': USER_IMAGES, 'module': 'src/backend/user'}
    if service == 'notification':
        return {'images': ('postgres:16-bookworm', 'redis:7-alpine'), 'module': 'src/backend/notification'}
    if service == 'voice':
        return {'images': ('postgres:16-bookworm', 'redis:7.4-bookworm', 'nats:2.12-alpine'),
                'module': 'src/backend/voice'}
    if service == 'gameintegration':
        return {'images': USER_IMAGES, 'module': 'src/backend/gameintegration'}
    if service in ('role', 'messaging'):
        return {'images': ROLE_IMAGES, 'module': 'src/backend/' + service}
    if service in ('realtime', 'analytics'):
        return {'images': REALTIME_IMAGES, 'module': 'src/backend/' + service}
    if service in POSTGRES_SERVICES:
        return {'images': SEARCH_IMAGES, 'module': 'src/backend/' + service}
    raise ValueError('unsupported Go integration service: image, tool and dedicated CI prerequisites require review')


def read_required(source_copy, service, packages):
    profile(service)
    path = Path(source_copy) / 'scripts/ci/go_integration_required.json'
    if path.is_symlink() or not path.is_file():
        raise ValueError('source integration inventory is missing')
    inventory = json.loads(path.read_text(encoding='utf-8'), object_pairs_hook=chat._unique)
    required = inventory.get(service) if isinstance(inventory, dict) else None
    prefix = 'voice/backend/' + service
    if (not isinstance(required, list) or not required or
            any(not isinstance(item, list) or len(item) != 2 or
                not isinstance(item[0], str) or item[0] not in packages or
                not (item[0] == prefix or item[0].startswith(prefix + '/')) or
                not isinstance(item[1], str) or not re.fullmatch(r'Test[A-Za-z0-9_]+', item[1])
                for item in required) or len({tuple(item) for item in required}) != len(required)):
        raise ValueError('invalid or unselected required Go integration inventory')
    return required


def run_profile(service, owner, daemon, client, root, source, source_copy, archive_digest, identities, output):
    if service not in EXTRA_SERVICES:
        raise ValueError('unsupported Go integration service')
    auth_broker._require_worker_job()
    return chat.run_full(owner, daemon, client, root, source, source_copy, archive_digest, identities, output,
                         service=service)


def _validate_resource_limits(value):
    if not isinstance(value, dict) or set(value) != {'client', 'daemon', 'host_job', 'capacity_budget'}:
        raise ValueError('explicit Go preflight resource limits are required')
    client = value['client']; daemon = value['daemon']; host = value['host_job']
    for item, fields in ((client, {'nano_cpus', 'memory_bytes', 'memory_swap_bytes', 'pids_limit'}),
                         (daemon, {'nano_cpus', 'memory_bytes', 'memory_swap_bytes', 'pids_limit'}),
                         (host, {'cpu_rate', 'job_memory_bytes', 'active_process_limit'})):
        if not isinstance(item, dict) or set(item) != fields or any(type(v) is not int or v <= 0 for v in item.values()):
            raise ValueError('invalid Go preflight resource limits')
    if client['memory_swap_bytes'] < client['memory_bytes'] or daemon != {
            'nano_cpus': 1_000_000_000, 'memory_bytes': 512 * 1024 * 1024,
            'memory_swap_bytes': 1024 * 1024 * 1024, 'pids_limit': 256}:
        raise ValueError('Go client limits or sealed daemon profile are invalid')
    from local_process_supervision import JobLimits
    JobLimits(**host)
    budget = heavy_slots._validate_capacity_budget(value['capacity_budget'])
    return {'client': dict(client), 'daemon': dict(daemon), 'host_job': dict(host),
            'capacity_budget': budget}


def _resource_claim(limits, measured):
    host_cpu = (measured['nano_cpus'] * limits['host_job']['cpu_rate'] + 9999) // 10000
    return {'nano_cpus': limits['client']['nano_cpus'] + limits['daemon']['nano_cpus'] + host_cpu,
            'memory_bytes': limits['client']['memory_bytes'] + limits['daemon']['memory_bytes'] +
                            limits['host_job']['job_memory_bytes'],
            'pids_limit': limits['client']['pids_limit'] + limits['daemon']['pids_limit'] +
                          limits['host_job']['active_process_limit']}


def run_selected(plan, service, root, output, helper, helper_digest, archives, go_binary='go', *, base, head_ref,
                 resource_limits=None, owner_id=None, ownership_journal=None, slot=None):
    """Require the caller's affected plan to select this service before work."""
    profile(service)
    auth_broker._require_worker_job()
    if not isinstance(plan, dict):
        raise ValueError('invalid affected plan')
    checks, components, source = plan.get('checks'), plan.get('components'), plan.get('source')
    if (not isinstance(base, str) or not re.fullmatch(r'[0-9a-f]{40}', base) or
            not isinstance(head_ref, str) or not head_ref or plan.get('base') != base or
            not isinstance(checks, dict) or 'backend-go-integration-pr' not in checks or
            not isinstance(components, dict) or not isinstance(components.get('go_integration'), list) or
            service not in components['go_integration'] or not isinstance(source, dict) or
            not isinstance(source.get('head'), str) or not re.fullmatch(r'[0-9a-f]{40}', source['head'])):
        raise ValueError('service is not selected by a valid Go integration plan')
    if affected_check_plan.build_local_plan(Path(root).resolve(), base, source['head'], head_ref) != plan:
        raise ValueError('affected Go integration plan differs from recomputed source plan')
    if snapshot.capture(Path(root).resolve(), source['head']) != source:
        raise ValueError('planned source changed before Go integration')
    result = run(service, root, source['head'], output, helper, helper_digest, archives, go_binary,
                 resource_limits=resource_limits, owner_id=owner_id,
                 ownership_journal=ownership_journal, slot=slot)
    if not isinstance(result, dict) or result.get('status') != 'incomplete' or result.get('source') != source:
        raise ValueError('Go integration diagnostic does not match planned source')
    return result


def _prepare_output_root(root, output):
    """Create one new, real output directory before any Docker or slot work."""
    root = Path(root).resolve(strict=True)
    output = Path(output).absolute()
    preflight_output = output.parent.parent
    if (not output.is_absolute() or output.parent.name != 'go-full' or
            not preflight_output.is_dir() or preflight_output.is_symlink() or
            preflight_output.resolve(strict=True) != preflight_output):
        raise ValueError('Go output must be inside the existing real preflight output')
    if output == root or root in output.parents:
        raise ValueError('Go output must be outside the source tree')
    if output.is_symlink() or output.exists():
        raise ValueError('Go output must be a new real directory')
    parent = output.parent
    if parent.is_symlink():
        raise ValueError('Go output parent must not be a symlink')
    if parent.exists():
        if not parent.is_dir() or parent.resolve(strict=True) != parent:
            raise ValueError('Go output parent identity changed')
    else:
        parent.mkdir(exist_ok=False)
    if parent.resolve(strict=True) != parent or parent.is_symlink():
        raise ValueError('Go output parent identity changed during creation')
    output.mkdir(exist_ok=False)
    if output.is_symlink() or not output.is_dir() or output.resolve(strict=True) != output:
        raise ValueError('Go output directory identity changed during creation')
    return output


def _require_parent_owner_paths(output, owner_id, ownership_journal):
    output = Path(output).absolute()
    if (not isinstance(owner_id, str) or not re.fullmatch(r'voiceci-[0-9a-f]{32}', owner_id) or
            output.is_symlink() or not output.is_dir() or output.resolve(strict=True) != output):
        raise ValueError('bounded Go worker output is not the parent-created directory')
    expected = output / 'run' / 'tmp' / 'ci-local' / owner_id / 'ownership.json'
    journal = Path(ownership_journal).absolute() if isinstance(ownership_journal, str) else None
    chain = (output / 'run', output / 'run' / 'tmp', output / 'run' / 'tmp' / 'ci-local',
             expected.parent)
    if (journal != expected or any(path.is_symlink() or not path.is_dir() or
                                   path.resolve(strict=True) != path for path in chain) or
            expected.is_symlink() or not expected.is_file() or expected.resolve(strict=True) != expected):
        raise ValueError('bounded Go worker journal is not bound to its parent output and owner')
    return output, expected


def _write_diagnostic_atomic(output, report):
    output = Path(output)
    if output.is_symlink() or not output.is_dir() or output.resolve(strict=True) != output:
        raise ValueError('Go diagnostic output directory identity changed')
    final = output / 'diagnostic.json'
    if final.is_symlink() or (final.exists() and not final.is_file()):
        raise ValueError('Go diagnostic destination is not a regular file')
    data = (json.dumps(report, sort_keys=True, indent=2) + '\n').encode('utf-8')
    descriptor, temporary = tempfile.mkstemp(prefix='.diagnostic-', suffix='.tmp', dir=output)
    try:
        with _os.fdopen(descriptor, 'wb') as stream:
            stream.write(data)
            stream.flush()
            _os.fsync(stream.fileno())
        _os.replace(temporary, final)
    finally:
        try:
            _os.unlink(temporary)
        except FileNotFoundError:
            pass


def _unbound_inputs(service, bounded):
    items = ['helper, image and tool provenance', 'exclusive daemon and socket access',
        'Go dependency and cached ziphash provenance', 'source inventory completeness audit and dedicated integration jobs']
    if bounded:
        items.extend(['host PID limit is caller supplied and not measured as a physical capacity ceiling',
            'nested Testcontainers and Ryuk cgroup/custody behavior',
            'source snapshot Git object writes, materialization, dependency caches and logs lack storage bounds',
            'foreign and non-cooperating host workloads are outside shared capacity accounting'])
    else:
        items.extend(['Go client container CPU and memory limits are not enforced',
            'host acquisition and execution are not supervised by an owner-bound Windows Job',
            'foreign and non-cooperating host workloads are outside shared capacity accounting'])
    service_gaps = {
        'realtime': 'tagged Realtime lifecycle PostgreSQL fixture and dedicated job',
        'user': 'User association, admission and capture dedicated fixtures and jobs',
        'gameintegration': 'T30 Compose Chat and Game Integration dedicated fixtures and jobs',
        'role': 'Role migration, bootstrap and tagged Role-Space dedicated fixtures and jobs',
        'analytics': 'tagged Analytics ClickHouse schema, fixture and dedicated job',
        'messaging': 'T16 cross-service, nonroot mounted-key, threadlist DSN and BE131 projection dedicated fixtures and jobs',
        'notification': 'BE195 real Redis endpoint and Notification quota dedicated integration job',
        'voice': 'Voice R18 real Redis endpoint and dedicated Voice ledger/lifecycle integration jobs'}
    if service in service_gaps:
        items.append(service_gaps[service])
    return items


def run(service, root, head, output, helper, helper_digest, archives, go_binary='go', *,
        resource_limits=None, owner_id=None, ownership_journal=None, slot=None):
    """Run one allowlisted service under caller-enforced custody, status incomplete.

    Resource-limited runs require a clean committed checkout whose file bytes
    match the Git blobs (including line endings) for the capped source archive;
    on Windows this generally requires ``core.autocrlf=false`` at checkout.
    Other snapshot, materialization and acquisition writes are not
    storage-bounded; the archive limit does not bound the full run.
    """
    selected = profile(service)
    auth_broker._require_worker_job()
    limits = _validate_resource_limits(resource_limits) if resource_limits is not None else None
    if limits is not None and (not isinstance(owner_id, str) or not ownership_journal or type(slot) is not int):
        raise ValueError('bounded Go worker requires its parent owner receipt')
    parent_output = None
    if limits is not None:
        parent_output, _ = _require_parent_owner_paths(output, owner_id, ownership_journal)
    if service == 'chat':
        if limits is not None:
            root = Path(root).resolve(strict=True)
            output = parent_output
            journal = Path(ownership_journal).resolve()
            if (output == root or root in output.parents or
                    journal.name != 'ownership.json' or not journal.is_file()):
                raise ValueError('bounded Chat worker owner paths differ')
            selected = profile(service)
            chat.validate_archives(archives, selected['images'])
            orchestration._verify_file(helper, helper_digest)
            for item in archives.values():
                orchestration._verify_file(item['archive'], item['archive_sha256'])
            go_binary = local_go_plan._check_go_toolchain(go_binary)
            import local_bounded_source
            bounded_archive = local_bounded_source.capture_archive(
                root, head, output / 'source.tar', max_files=local_bounded_source.MAX_FILES,
                max_source_bytes=local_bounded_source.MAX_SOURCE_BYTES,
                max_archive_bytes=local_bounded_source.MAX_ARCHIVE_BYTES, timeout=local_bounded_source.MAX_SECONDS)
            source = bounded_archive['source']
            if not isinstance(source, dict) or source.get('head') != head:
                raise ValueError('bounded archive source differs from requested head')
            source_copy = snapshot.materialize(root, source, output / 'source')
            acquisition = local_go_dependencies.acquire(root, source, source_copy, selected['module'],
                                                         output / 'dependencies', go_binary)
            engine = chat.DockerCLI()
            scheduled = chat.ScheduledRun.attach_worker(journal.parent, engine, slot, owner_id, limits)
            result = chat.run_owned(scheduled, engine, source, root, source_copy, output, helper,
                                    helper_digest, archives, acquisition, service='chat',
                                    resource_limits=limits, bounded_archive=bounded_archive)
            if snapshot.capture(root, head) != source:
                raise ValueError('source changed during bounded Chat integration')
            result['unbound_inputs'] = _unbound_inputs(service, bounded=True)
            return result
        return chat.run(root, head, output, helper, helper_digest, archives, go_binary)
    if os.name != 'nt':
        raise ValueError('Windows supervised diagnostic required; Linux is unsupported')
    chat.validate_archives(archives, selected['images'])
    root = Path(root).resolve(strict=True)
    output = parent_output if limits is not None else Path(output).absolute()
    if (output.exists() and limits is None) or output == root or root in output.parents:
        raise ValueError('output must be new and outside source tree')
    orchestration._verify_file(helper, helper_digest)
    for item in archives.values():
        orchestration._verify_file(item['archive'], item['archive_sha256'])
    go_binary = local_go_plan._check_go_toolchain(go_binary)
    bounded_archive = None
    if limits is not None:
        import local_bounded_source
        bounded_archive = local_bounded_source.capture_archive(
            root, head, output / 'source.tar', max_files=local_bounded_source.MAX_FILES,
            max_source_bytes=local_bounded_source.MAX_SOURCE_BYTES,
            max_archive_bytes=local_bounded_source.MAX_ARCHIVE_BYTES, timeout=local_bounded_source.MAX_SECONDS)
    source = snapshot.capture(root, head) if bounded_archive is None else bounded_archive['source']
    if not isinstance(source, dict) or source.get('head') != head:
        raise ValueError('bounded archive source differs from requested head')
    if limits is None:
        output.mkdir(parents=True, exist_ok=False)
    source_copy = snapshot.materialize(root, source, output / 'source')
    if service == 'notification':
        acquisition = local_go_multi_dependencies.acquire(root, source, source_copy,
                                                            output / 'dependencies', go_binary)
    else:
        acquisition = local_go_dependencies.acquire(root, source, source_copy, selected['module'],
                                                     output / 'dependencies', go_binary)
    checkout = output / 'run'
    if limits is not None:
        if checkout.is_symlink() or not checkout.is_dir() or checkout.resolve(strict=True) != checkout:
            raise ValueError('bounded Go worker checkout is not the exact parent-created directory')
    else:
        checkout.mkdir(exist_ok=False)
    engine = DockerCLI()
    if limits is not None:
        scheduled = ScheduledRun.attach_worker(Path(ownership_journal).parent, engine, slot, owner_id, limits)
    else:
        scheduled = ScheduledRun.start_host(checkout, engine, 300)
        try:
            chat.publish_owner(scheduled, output)
        except BaseException as error:
            orchestration._finish(scheduled, error)
    run_options = {'service': service, 'resource_limits': limits}
    if bounded_archive is not None:
        run_options['bounded_archive'] = bounded_archive
    result = chat.run_owned(scheduled, engine, source, root, source_copy, output, helper, helper_digest,
                            archives, acquisition, **run_options)
    if snapshot.capture(root, head) != source:
        raise ValueError('source changed during cleanup')
    result['unbound_inputs'] = _unbound_inputs(service, bounded=limits is not None)
    _write_diagnostic_atomic(output, result)
    return result


def _worker_command():
    command = bootstrap._worker_command(Path(__file__))
    original = repr(sorted(bootstrap.PROJECT_MODULES))
    modules = bootstrap.PROJECT_MODULES | {'local_go_full_integration', 'local_chat_full_integration',
        'local_auth_full_orchestration', 'local_auth_full', 'local_auth_private', 'local_nats_hosted_diagnostic',
        'local_go_multi_dependencies', 'local_bounded_source', 'local_current_diff'}
    if command[5].count(original) != 1:
        raise ValueError('Go worker import allowlist changed')
    command[5] = command[5].replace(original, repr(sorted(modules)), 1)
    return command


def run_supervised(plan, service, root, output, helper, helper_digest, archives, go_binary='go', *, base, head_ref,
                   resource_limits):
    """Fence the selected executor before source acquisition or owned resources."""
    profile(service)
    limits = _validate_resource_limits(resource_limits)
    measured = heavy_slots.measured_host_capacity()
    capacity_budget = heavy_slots._validate_capacity_budget(limits['capacity_budget'])
    claim = _resource_claim(limits, measured)
    if any(claim[key] > capacity_budget[key] for key in claim):
        raise ValueError('single Go run exceeds the operator capacity budget')
    root = Path(root).resolve(strict=True)
    output = Path(output).absolute()
    if output == root or root in output.parents:
        raise ValueError('Go output must be outside the source tree')
    source = plan.get('source') if isinstance(plan, dict) else None
    if not isinstance(source, dict) or not isinstance(source.get('head'), str):
        raise ValueError('Go plan source identity is required before admission')
    run_id = 'voiceci-' + uuid.uuid4().hex
    job_name = 'Local\\VoiceCIProof-Go-' + run_id.removeprefix('voiceci-')
    job_limits = supervision.JobLimits(**limits['host_job'])
    source_bytes = json.dumps(source, allow_nan=False, separators=(',', ':'), sort_keys=True).encode('utf-8')
    host_job = {'schema': 1, 'slot': None, 'job_names': [job_name], 'limits': limits['host_job'],
                'session_identity': proof_scheduler._session_identity(),
                'source_identity': {'root': str(root), 'head': source['head'],
                                    'sha256': hashlib.sha256(source_bytes).hexdigest()}}
    output = _prepare_output_root(root, output)
    engine = DockerCLI()
    checkout = output / 'run'
    checkout.mkdir(exist_ok=False)
    scheduled = ScheduledRun.start_host(checkout, engine, 300, resource_claim=claim,
        capacity_budget=capacity_budget, host_job=host_job, resource_limits=limits, run_id=run_id)
    try:
        chat.publish_owner(scheduled, output)
    except BaseException:
        scheduled.close()
        raise
    owner_journal = str(scheduled.run.directory / 'ownership.json')
    request = {'plan': plan, 'service': service, 'root': str(root), 'output': str(output),
               'helper': str(helper), 'helper_digest': helper_digest, 'archives': archives,
               'go_binary': str(go_binary), 'base': base, 'head_ref': head_ref,
               'resource_limits': limits, 'owner_id': run_id,
               'ownership_journal': owner_journal, 'slot': scheduled.slot}
    raw = json.dumps(request, allow_nan=False, separators=(',', ':')).encode('utf-8')
    if len(raw) > 1024 * 1024:
        scheduled.close()
        raise ValueError('Go worker request exceeds limit')
    scheduled.release_owner_handle()
    try:
        completed = supervision.run_diagnostic(_worker_command(), timeout=10800, input=raw,
            capture_output=True, check=False, job_limits=job_limits, job_name=job_name)
    except BaseException as error:
        error.add_note('Go worker cleanup may be uncertain; inspect its ownership journal and slot.')
        try:
            if proof_scheduler._session_identity() != host_job['session_identity'] or supervision.named_job_exists(job_name) is not False:
                scheduled.abandon()
            else:
                scheduled.close()
        except BaseException:
            scheduled.abandon()
        raise
    try:
        if (proof_scheduler._session_identity() != host_job['session_identity'] or
                supervision.named_job_exists(job_name) is not False):
            scheduled.abandon()
            raise ValueError('Go worker named Job termination is not proven')
        if completed.returncode != 0:
            scheduled.close()
            raise ValueError('full Go worker failed; inspect its ownership journal before retry')
        close_evidence = scheduled.close()
    except BaseException:
        if scheduled._lease is not None:
            scheduled.abandon()
        raise
    if completed.returncode != 0:
        raise ValueError('full Go worker failed; inspect its journal before recovery')
    try:
        report = json.loads(completed.stdout, object_pairs_hook=chat._unique)
        saved = json.loads((Path(output) / 'diagnostic.json').read_text(encoding='utf-8'), object_pairs_hook=chat._unique)
        if report != saved or report.get('status') != 'incomplete' or report.get('source') != plan['source']:
            raise ValueError()
    except (OSError, ValueError, TypeError, KeyError, UnicodeError):
        raise ValueError('full Go worker returned invalid diagnostic') from None
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during supervised Go integration')
    job_evidence = getattr(completed, 'job_evidence', None)
    if (not isinstance(job_evidence, dict) or
            any(job_evidence.get(key) != value for key, value in limits['host_job'].items()) or
            job_evidence.get('job_name') != job_name or job_evidence.get('active_processes_after_cleanup') != 0):
        raise ValueError('Go worker returned no Windows Job limit evidence')
    slot_release = close_evidence.get('slot_release') if isinstance(close_evidence, dict) else None
    expected_release = {'slot': scheduled.slot, 'owner_id': run_id,
                        'marker_idle': True, 'reservation_released': True}
    if (not isinstance(close_evidence, dict) or slot_release != expected_release or
            close_evidence.get('owned_resources_remaining') != 0 or
            close_evidence.get('daemon_id') != report.get('resource_limit_readback', {}).get('daemon_id')):
        raise ValueError('Go host slot cleanup is not proven')
    report['resource_limit_readback'] = report.get('resource_limit_readback', {}) | {'host_job': job_evidence}
    report['cleanup_evidence'] = {'completed': True, 'named_job_absent': True,
        'owned_resources_remaining': close_evidence['owned_resources_remaining'],
        'slot_released': slot_release['marker_idle'],
        'capacity_reservation_released': slot_release['reservation_released'],
        'daemon_id': close_evidence['daemon_id'], 'slot': slot_release['slot'],
        'owner_id': slot_release['owner_id']}
    _write_diagnostic_atomic(output, report)
    return report


def main():
    if sys.argv[1:] != ['--worker']:
        raise ValueError('Go integration worker requires supervised invocation')
    auth_broker._require_worker_job()
    raw = sys.stdin.buffer.read(1024 * 1024 + 1)
    if len(raw) > 1024 * 1024:
        raise ValueError('Go worker request exceeds limit')
    request = json.loads(raw, object_pairs_hook=chat._unique)
    keys = {'plan', 'service', 'root', 'output', 'helper', 'helper_digest', 'archives', 'go_binary', 'base', 'head_ref',
            'resource_limits', 'owner_id', 'ownership_journal', 'slot'}
    if not isinstance(request, dict) or set(request) != keys:
        raise ValueError('invalid Go worker request')
    _validate_resource_limits(request['resource_limits'])
    if (not isinstance(request['owner_id'], str) or not re.fullmatch(r'voiceci-[0-9a-f]{32}', request['owner_id']) or
            not isinstance(request['ownership_journal'], str) or not Path(request['ownership_journal']).is_absolute() or
            type(request['slot']) is not int or request['slot'] not in (0, 1)):
        raise ValueError('invalid Go worker owner binding')
    result = run_selected(**request)
    result['input_identity'] = {'helper_sha256': request['helper_digest'],
        'archives': {tag: {'archive_sha256': item['archive_sha256'], 'image_id': item['image_id']}
                     for tag, item in request['archives'].items()}, 'resource_limits': request['resource_limits']}
    (Path(request['output']) / 'diagnostic.json').write_text(
        json.dumps(result, sort_keys=True, indent=2) + '\n', encoding='utf-8')
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
