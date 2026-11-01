"""Explicitly profiled full Go integration diagnostics, never a preflight pass.

Only explicitly listed services are implemented. Other services require a reviewed profile
for their own images, binaries, environment and dedicated CI checks. The caller
must supervise this process and maintain exclusive private daemon/socket custody.
"""
import json
import os
from pathlib import Path
import re
import sys

import affected_check_plan
import local_chat_full_integration as chat
import local_chat_orchestration as orchestration
from local_docker_owned import DockerCLI
from local_docker_scheduled import ScheduledRun
import local_go_dependencies
import local_go_plan
import local_source_snapshot as snapshot
import local_nats_hosted_diagnostic as bootstrap
import local_process_supervision as supervision
import local_auth_full_orchestration as auth_broker

BOT_IMAGES = ('postgres:16-bookworm', 'nats:2.10-alpine')
SEARCH_IMAGES = ('postgres:16-bookworm',)
USER_IMAGES = ('postgres:16-bookworm', 'migrate/migrate:v4.18.1')
REALTIME_IMAGES = ()
POSTGRES_SERVICES = frozenset({'search', 'controlledgame', 'moderation', 'social', 'subscription', 'story'})
EXTRA_SERVICES = POSTGRES_SERVICES | {'bot', 'user', 'realtime'}


def profile(service):
    if not isinstance(service, str):
        raise ValueError('unsupported Go integration service')
    if service == 'chat':
        return {'images': chat.IMAGES, 'module': 'src/backend/chat'}
    if service == 'bot':
        return {'images': BOT_IMAGES, 'module': 'src/backend/bot'}
    if service == 'user':
        return {'images': USER_IMAGES, 'module': 'src/backend/user'}
    if service == 'realtime':
        return {'images': REALTIME_IMAGES, 'module': 'src/backend/realtime'}
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


def run_selected(plan, service, root, output, helper, helper_digest, archives, go_binary='go', *, base, head_ref):
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
    result = run(service, root, source['head'], output, helper, helper_digest, archives, go_binary)
    if not isinstance(result, dict) or result.get('status') != 'incomplete' or result.get('source') != source:
        raise ValueError('Go integration diagnostic does not match planned source')
    return result


def run(service, root, head, output, helper, helper_digest, archives, go_binary='go'):
    """Run one allowlisted service under caller-enforced custody, status incomplete."""
    selected = profile(service)
    auth_broker._require_worker_job()
    if service == 'chat':
        return chat.run(root, head, output, helper, helper_digest, archives, go_binary)
    if os.name != 'nt':
        raise ValueError('Windows supervised diagnostic required; Linux is unsupported')
    chat.validate_archives(archives, selected['images'])
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('output must be new and outside source tree')
    orchestration._verify_file(helper, helper_digest)
    for item in archives.values():
        orchestration._verify_file(item['archive'], item['archive_sha256'])
    go_binary = local_go_plan._check_go_toolchain(go_binary)
    source = snapshot.capture(root, head)
    output.mkdir(parents=True)
    source_copy = snapshot.materialize(root, source, output / 'source')
    acquisition = local_go_dependencies.acquire(root, source, source_copy, selected['module'],
                                                 output / 'dependencies', go_binary)
    checkout = output / 'run'
    checkout.mkdir()
    engine = DockerCLI()
    scheduled = ScheduledRun.start_host(checkout, engine, 300)
    try:
        chat.publish_owner(scheduled, output)
    except BaseException as error:
        orchestration._finish(scheduled, error)
    result = chat.run_owned(scheduled, engine, source, root, source_copy, output, helper, helper_digest,
                            archives, acquisition, service=service)
    if snapshot.capture(root, head) != source:
        raise ValueError('source changed during cleanup')
    result['unbound_inputs'] = ['helper, image and tool provenance', 'exclusive daemon and socket access',
        'Go dependency and cached ziphash provenance', 'host acquisition supervision',
        'source inventory completeness audit and dedicated integration jobs',
        'Go client container CPU and memory limits not enforced']
    if service == 'realtime':
        result['unbound_inputs'].append('tagged Realtime lifecycle PostgreSQL fixture and dedicated job')
    if service == 'user':
        result['unbound_inputs'].append('User association, admission and capture dedicated fixtures and jobs')
    (output / 'diagnostic.json').write_text(json.dumps(result, sort_keys=True, indent=2) + '\n', encoding='utf-8')
    return result


def _worker_command():
    command = bootstrap._worker_command(Path(__file__))
    original = repr(sorted(bootstrap.PROJECT_MODULES))
    modules = bootstrap.PROJECT_MODULES | {'local_go_full_integration', 'local_chat_full_integration',
        'local_auth_full_orchestration', 'local_auth_full', 'local_auth_private', 'local_nats_hosted_diagnostic'}
    if command[5].count(original) != 1:
        raise ValueError('Go worker import allowlist changed')
    command[5] = command[5].replace(original, repr(sorted(modules)), 1)
    return command


def run_supervised(plan, service, root, output, helper, helper_digest, archives, go_binary='go', *, base, head_ref):
    """Fence the selected executor before source acquisition or owned resources."""
    profile(service)
    request = {'plan': plan, 'service': service, 'root': str(root), 'output': str(output),
               'helper': str(helper), 'helper_digest': helper_digest, 'archives': archives,
               'go_binary': str(go_binary), 'base': base, 'head_ref': head_ref}
    raw = json.dumps(request, allow_nan=False, separators=(',', ':')).encode('utf-8')
    if len(raw) > 1024 * 1024:
        raise ValueError('Go worker request exceeds limit')
    try:
        completed = supervision.run_diagnostic(_worker_command(), timeout=10800, input=raw,
                                               capture_output=True, check=False)
    except BaseException as error:
        error.add_note('Go worker cleanup may be uncertain; inspect its ownership journal and slot.')
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
    return report


def main():
    if sys.argv[1:] != ['--worker']:
        raise ValueError('Go integration worker requires supervised invocation')
    auth_broker._require_worker_job()
    raw = sys.stdin.buffer.read(1024 * 1024 + 1)
    if len(raw) > 1024 * 1024:
        raise ValueError('Go worker request exceeds limit')
    request = json.loads(raw, object_pairs_hook=chat._unique)
    keys = {'plan', 'service', 'root', 'output', 'helper', 'helper_digest', 'archives', 'go_binary', 'base', 'head_ref'}
    if not isinstance(request, dict) or set(request) != keys:
        raise ValueError('invalid Go worker request')
    result = run_selected(**request)
    result['input_identity'] = {'helper_sha256': request['helper_digest'],
        'archives': {tag: {'archive_sha256': item['archive_sha256'], 'image_id': item['image_id']}
                     for tag, item in request['archives'].items()}}
    (Path(request['output']) / 'diagnostic.json').write_text(
        json.dumps(result, sort_keys=True, indent=2) + '\n', encoding='utf-8')
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
