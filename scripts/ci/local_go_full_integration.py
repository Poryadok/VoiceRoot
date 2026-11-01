"""Explicitly profiled full Go integration diagnostics, never a preflight pass.

Only explicitly listed services are implemented. Other services require a reviewed profile
for their own images, binaries, environment and dedicated CI checks. The caller
must supervise this process and maintain exclusive private daemon/socket custody.
"""
import json
import os
from pathlib import Path
import re

import affected_check_plan
import local_chat_full_integration as chat
import local_chat_orchestration as orchestration
from local_docker_owned import DockerCLI
from local_docker_scheduled import ScheduledRun
import local_go_dependencies
import local_go_plan
import local_source_snapshot as snapshot

BOT_IMAGES = ('postgres:16-bookworm', 'nats:2.10-alpine')
SEARCH_IMAGES = ('postgres:16-bookworm',)
POSTGRES_SERVICES = frozenset({'search', 'controlledgame', 'moderation', 'social', 'subscription', 'story'})
EXTRA_SERVICES = POSTGRES_SERVICES | {'bot'}


def profile(service):
    if not isinstance(service, str):
        raise ValueError('unsupported Go integration service')
    if service == 'chat':
        return {'images': chat.IMAGES, 'module': 'src/backend/chat'}
    if service == 'bot':
        return {'images': BOT_IMAGES, 'module': 'src/backend/bot'}
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
    return chat.run_full(owner, daemon, client, root, source, source_copy, archive_digest, identities, output,
                         service=service)


def run_selected(plan, service, root, output, helper, helper_digest, archives, go_binary='go', *, base, head_ref):
    """Require the caller's affected plan to select this service before work."""
    profile(service)
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
    (output / 'diagnostic.json').write_text(json.dumps(result, sort_keys=True, indent=2) + '\n', encoding='utf-8')
    return result
