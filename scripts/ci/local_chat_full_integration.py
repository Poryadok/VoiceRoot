"""Full Chat Go integration diagnostic on a caller-owned private daemon.

This never issues a preflight pass. Caller must provision and authenticate the
three images, enforce exclusive private daemon/socket custody, and retain the
host slot through exact outer-resource cleanup and supervised execution.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import tempfile
from urllib.parse import quote

import go_integration_receipts as receipts
import local_chat_orchestration as orchestration
import local_chat_private
from local_docker_owned import DockerCLI
from local_docker_scheduled import ScheduledRun
import local_go_dependencies
import local_go_execution
import local_go_plan
import local_source_snapshot as snapshot

MODULE = 'src/backend/chat'
IMAGES = ('postgres:16-bookworm', 'redis:7.4-alpine', 'migrate/migrate:v4.18.1')
EVENT_LIMIT = 64 * 1024 * 1024
STDERR_LIMIT = 8 * 1024 * 1024


def bounded_script(command, event_limit=EVENT_LIMIT, stderr_limit=STDERR_LIMIT):
    """Use the maintained worker's in-container pipe caps and exit propagation."""
    if any(type(limit) is not int or limit < 1 for limit in (event_limit, stderr_limit)):
        raise ValueError('positive output limits required')
    return ('set -o pipefail; exec 3> >(head -c ' + str(stderr_limit + 1) + ' >&2); stderr_pid=$!; { ' +
            command + '; } 2>&3 | head -c ' + str(event_limit + 1) + '; '
            'codes=("${PIPESTATUS[@]}"); exec 3>&-; wait "$stderr_pid" || exit 125; '
            '[ "${codes[1]}" = 0 ] || exit 125; exit "${codes[0]}"')


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate integration inventory key')
        result[key] = value
    return result


def read_required(source_copy, packages):
    path = Path(source_copy) / 'scripts/ci/go_integration_required.json'
    if path.is_symlink() or not path.is_file():
        raise ValueError('source integration inventory is missing')
    inventory = json.loads(path.read_text(encoding='utf-8'), object_pairs_hook=_unique)
    required = inventory.get('chat') if isinstance(inventory, dict) else None
    if (not isinstance(required, list) or not required or
            any(not isinstance(item, list) or len(item) != 2 or
                not isinstance(item[0], str) or item[0] not in packages or
                not (item[0] == 'voice/backend/chat' or item[0].startswith('voice/backend/chat/')) or
                not isinstance(item[1], str) or not re.fullmatch(r'Test[A-Za-z0-9_]+', item[1])
                for item in required) or len({tuple(item) for item in required}) != len(required)):
        raise ValueError('invalid or unselected Chat required inventory')
    return required


def validate_events(events, packages, no_tests, required):
    summary = receipts.validate_test_run(events, packages, no_tests)
    missing = receipts.verify_required(events, required, 0)
    if missing:
        raise ValueError('required Chat integration tests missing or skipped')
    return summary


def verify_images(owner, client, identities, image_tags=IMAGES):
    if (not isinstance(identities, dict) or set(identities) != set(image_tags) or
            any(not isinstance(value, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', value)
                for value in identities.values())):
        raise ValueError('explicit private image identities must match the selected profile')
    for tag in image_tags:
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            command = ['curl', '--silent', '--show-error', '--fail', '--max-time', '10',
                       '--unix-socket', '/socket/docker.sock',
                       'http://localhost/images/' + quote(tag, safe='') + '/json']
            if owner.execute(client, command, out, err, 30):
                raise ValueError('private required image unavailable')
            out.seek(0)
            try:
                image = json.load(out)
            except (UnicodeDecodeError, ValueError) as error:
                raise ValueError('invalid private image inspection') from error
            if not isinstance(image, dict) or image.get('Id') != identities[tag]:
                raise ValueError('private required image identity differs')


def run_full(owner, daemon, client, root, source, source_copy, archive_digest, identities, output, *, service='chat'):
    from local_auth_full_orchestration import _require_worker_job
    _require_worker_job()
    if service == 'chat':
        module, image_tags, inventory_reader = MODULE, IMAGES, read_required
        test_args = '-p 2'
    elif service in ('bot', 'search'):
        import local_go_full_integration as full_go
        selected = full_go.profile(service)
        module, image_tags = selected['module'], selected['images']
        inventory_reader = lambda source_copy, packages: full_go.read_required(source_copy, service, packages)
        test_args = '-p 1 -parallel 1'
    else:
        raise ValueError('unsupported Go service profile')
    root, output = Path(root).resolve(), Path(output).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('output must be new and outside source tree')
    address = local_chat_private.inspect_pair(owner, daemon, client)
    verify_images(owner, client, identities, image_tags)
    output.mkdir(parents=True)
    inventory = local_go_execution.discover_packages(owner, client, root, source, source_copy,
                                                      archive_digest, module, output / 'inventory')
    packages = sorted(inventory['packages_with_tests'] + inventory['packages_without_tests'])
    snapshot.verify_materialized(root, source, source_copy)
    required = inventory_reader(source_copy, inventory['packages_with_tests'])
    inventory_path = Path(source_copy) / 'scripts/ci/go_integration_required.json'
    inventory_digest = hashlib.sha256(inventory_path.read_bytes()).hexdigest()
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before full Chat integration')
    if local_chat_private.inspect_pair(owner, daemon, client) != address:
        raise ValueError('private daemon address changed')
    go = ('cd /workspace/source/' + module + ' && '
               'exec env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly '
               'GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOMAXPROCS=2 '
               'GOCACHE=/workspace/go-build GOMODCACHE=/workspace/go-mod '
               'DOCKER_HOST=unix:///socket/docker.sock TESTCONTAINERS_RYUK_DISABLED=true '
               'TESTCONTAINERS_HOST_OVERRIDE="$1" '
               'go test -json -count=1 -timeout 20m ' + test_args + ' ./...')
    command = ['bash', '-c', bounded_script(go), 'voice-' + service + '-full', address]
    with (output / 'events.jsonl').open('xb') as out, (output / 'stderr.log').open('xb') as err:
        code = owner.execute(client, command, out, err, 1800)
    if ((output / 'events.jsonl').stat().st_size > EVENT_LIMIT or
            (output / 'stderr.log').stat().st_size > STDERR_LIMIT):
        raise ValueError('Go diagnostic output limit exceeded')
    if code != 0:
        raise ValueError('full Chat integration command failed')
    if local_chat_private.inspect_pair(owner, daemon, client) != address:
        raise ValueError('private daemon address changed')
    verify_images(owner, client, identities, image_tags)
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during full Chat integration')
    local_go_execution._verify_modules(owner, client, module)
    local_go_execution._verify_workspace(owner, client, root, source, archive_digest)
    raw = (output / 'events.jsonl').read_bytes()
    events = receipts.read_events(raw.decode('utf-8'))
    summary = validate_events(events, packages, inventory['packages_without_tests'], required)
    return {'schema': 'voice-local-' + service + '-full-integration-v1', 'status': 'incomplete', 'source': source,
            'command': command, 'exit_code': code, 'inventory': inventory,
            'required_count': len(required), 'required_inventory_sha256': inventory_digest,
            'events_sha256': hashlib.sha256(raw).hexdigest(), 'tests': summary, 'images': identities}


def validate_archives(archives, image_tags=IMAGES):
    if not isinstance(archives, dict) or set(archives) != set(image_tags):
        raise ValueError('explicit image archives must match the selected profile')
    for item in archives.values():
        if (not isinstance(item, dict) or set(item) != {'archive', 'archive_sha256', 'image_id'} or
                not isinstance(item['archive'], str) or not Path(item['archive']).is_absolute() or
                not isinstance(item['archive_sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', item['archive_sha256']) or
                not isinstance(item['image_id'], str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', item['image_id'])):
            raise ValueError('invalid Chat image archive identity')


def run_owned(scheduled, engine, source, root, source_copy, output, helper, helper_digest, archives, acquisition,
              *, service='chat'):
    from local_auth_full_orchestration import _require_worker_job
    _require_worker_job()
    owner, failure, result = scheduled.run, None, None
    try:
        if service == 'chat':
            image_tags, executor = IMAGES, run_full
        elif service in ('bot', 'search'):
            import local_go_full_integration as full_go
            image_tags = full_go.profile(service)['images']
            executor = lambda *args: full_go.run_profile(service, *args)
        else:
            raise ValueError('unsupported Go service profile')
        validate_archives(archives, image_tags)
        network = owner.create_resource('network')
        mounts = {name: owner.create_resource('volume') for name in
                  ('/home/rootless/.local/share/docker', '/var/lib/docker', '/run/user/1000')}
        workspace = owner.create_resource('volume')
        image = engine._call('image', 'inspect', local_go_plan.GO_IMAGE, '--format', '{{.Id}}')
        client = owner.create_container(image, network, {'/socket': mounts['/run/user/1000'],
                                                          '/workspace': workspace}, [], command=['sleep', 'infinity'])
        owner.start_container(client)
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            if owner.execute(client, ['chown', '1000:1000', '/socket'], out, err, 30):
                raise ValueError('private socket initialization failed')
        daemon = owner.create_rootless_container(network, mounts,
            lambda request: orchestration._create_daemon(helper, helper_digest, engine.rootless_creator_env(), request))
        owner.start_container(daemon)
        orchestration._wait_daemon(owner, daemon, client)
        for index, tag in enumerate(image_tags):
            item = archives[tag]
            orchestration._verify_file(item['archive'], item['archive_sha256'])
            copied = owner.directory / ('image-' + str(index) + '.tar')
            with Path(item['archive']).open('rb') as source_file, copied.open('xb') as target:
                for block in iter(lambda: source_file.read(1024 * 1024), b''):
                    target.write(block)
                target.flush()
                os.fsync(target.fileno())
            orchestration._verify_file(item['archive'], item['archive_sha256'])
            orchestration._verify_file(copied, item['archive_sha256'])
            with (output / ('image-' + str(index) + '.stdout')).open('xb') as out, \
                 (output / ('image-' + str(index) + '.stderr')).open('xb') as err:
                owner.load_canonical_rootless_image(daemon, copied, item['archive_sha256'], item['image_id'],
                                                    tag, out, err, 300)
        source_digest = local_go_execution.transfer_source(owner, client, root, source, source_copy, output / 'source.tar')
        local_go_execution.transfer_dependencies(owner, client, root, source, source_copy, source_digest,
                                                  acquisition, output / 'dependencies/download-cache.tar')
        identities = {tag: archives[tag]['image_id'] for tag in image_tags}
        result = executor(owner, daemon, client, root, source, source_copy, source_digest, identities, output / 'full')
        result.update(slot=scheduled.slot, ownership_journal=str(owner.directory / 'ownership.json'),
                      source_archive_sha256=source_digest, go_image_id=image)
    except BaseException as error:
        failure = error
    orchestration._finish(scheduled, failure)
    return result


def publish_owner(scheduled, output):
    receipt = {'run_id': scheduled.run.run_id, 'slot': scheduled.slot,
               'directory': str(scheduled.run.directory)}
    with (output / 'owner.json').open('x', encoding='utf-8') as stream:
        json.dump(receipt, stream, sort_keys=True)
        stream.flush()
        os.fsync(stream.fileno())


def run(root, head, output, helper, helper_digest, archives, go_binary='go'):
    """Run one full Chat diagnostic with caller-provided inputs and custody.

    Windows only; helper, image and module identities are not authenticated.
    The caller must supervise this process and maintain exclusive daemon/socket
    custody. This is not yet wired to the preflight or the general Go matrix.
    """
    from local_auth_full_orchestration import _require_worker_job
    _require_worker_job()
    if os.name != 'nt':
        raise ValueError('Windows supervised diagnostic required; Linux is unsupported')
    validate_archives(archives)
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
    acquisition = local_go_dependencies.acquire(root, source, source_copy, MODULE, output / 'dependencies', go_binary)
    checkout = output / 'run'
    checkout.mkdir()
    engine = DockerCLI()
    scheduled = ScheduledRun.start_host(checkout, engine, 300)
    try:
        publish_owner(scheduled, output)
    except BaseException as error:
        orchestration._finish(scheduled, error)
    result = run_owned(scheduled, engine, source, root, source_copy, output, helper, helper_digest, archives, acquisition)
    if snapshot.capture(root, head) != source:
        raise ValueError('source changed during cleanup')
    result['unbound_inputs'] = ['helper, image and tool provenance', 'exclusive daemon and socket access',
                                'Go dependency and cached ziphash provenance', 'host acquisition supervision',
                                'source inventory completeness audit and dedicated integration jobs']
    (output / 'diagnostic.json').write_text(json.dumps(result, sort_keys=True, indent=2) + '\n', encoding='utf-8')
    return result
