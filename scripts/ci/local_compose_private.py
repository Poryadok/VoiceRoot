"""One fixed Compose fixture on a caller-controlled private daemon.

This diagnostic is incomplete. The caller owns provisioning, exclusive socket
custody, image/tool authentication, supervision and whole-daemon cleanup.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import tempfile
import time

import local_chat_orchestration as orchestration
import local_chat_private
import local_compose_audit
from local_docker_owned import DockerCLI
from local_docker_scheduled import ScheduledRun

MARKER = b'voice-compose-private-fixture-ok\n'
COMMAND = ['env', '-i', 'PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
           'HOME=/tmp', 'DOCKER_HOST=unix:///socket/docker.sock', 'COMPOSE_DISABLE_ENV_FILE=1',
           'docker', 'compose', '--project-directory', '/workspace']
CLIENT_IMAGE = 'docker@sha256:5e6baf67325a79f64ed3bed6863954d5e05d0f3d4e57cbdb14809346814bbfdb'


def model(project):
    return {'name': project, 'services': {'fixture': {
        'image': 'postgres:16-bookworm', 'entrypoint': ['/bin/sh', '-c'],
        'command': ['test -d /fixture && printf "voice-compose-private-fixture-ok\\n"'],
        'network_mode': 'none',
        'volumes': [{'type': 'volume', 'source': 'data', 'target': '/fixture'}],
    }}, 'volumes': {'data': {'name': project + '_data'}}}


def _unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate rendered Compose key')
        result[key] = value
    return result


def run_fixture(owner, daemon, client, checkout, stdout, stderr, model_output):
    """Execute the exact audited JSON once; never call Compose cleanup on host."""
    checkout = Path(checkout).resolve()
    project = owner.run_id
    config = model(project)
    findings = local_compose_audit.audit(config, project, checkout, owner.directory)
    if findings:
        raise ValueError('Compose audit rejected fixed fixture')
    address = local_chat_private.inspect_pair(owner, daemon, client)
    source = json.dumps(config, sort_keys=True, separators=(',', ':')).encode('utf-8')
    # The read and accepted model are bounded; the subprocess output file can
    # grow before execution returns and is not a disk-usage limit.
    with tempfile.TemporaryFile() as stream, tempfile.TemporaryFile() as rendered:
        stream.write(source)
        stream.seek(0)
        code = owner.execute(client, COMMAND + ['-p', project, '-f', '-', 'config', '--format', 'json'],
                             rendered, stderr, 30, stdin=stream)
        if code != 0:
            raise ValueError('Compose rendering failed')
        rendered.seek(0)
        raw = rendered.read(65537)
    if len(raw) > 65536 or b'$' in raw:
        raise ValueError('rendered Compose model is oversized or interpolated')
    try:
        parsed = json.loads(raw, object_pairs_hook=_unique_object)
    except (ValueError, UnicodeDecodeError) as error:
        raise ValueError('invalid rendered Compose model') from error
    if parsed != config or local_compose_audit.audit(parsed, project, checkout, owner.directory):
        raise ValueError('rendered Compose model differs or failed audit')
    command = COMMAND + ['-p', project, '-f', '-', 'run', '--rm', '--no-deps',
                         '--pull', 'never', '-T', 'fixture']
    with Path(model_output).open('x+b') as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())
        stream.seek(0)
        code = owner.execute(client, command, stdout, stderr, 120, stdin=stream)
        stream.seek(0)
        if stream.read(len(raw) + 1) != raw:
            raise ValueError('retained Compose model changed during execution')
    if code != 0:
        raise ValueError('Compose fixture failed')
    if local_chat_private.inspect_pair(owner, daemon, client) != address:
        raise ValueError('private daemon address changed')
    stdout.seek(0)
    if stdout.read(len(MARKER) + 1) != MARKER:
        raise ValueError('Compose fixture output differs')
    return {'status': 'incomplete', 'project': project, 'model_sha256': hashlib.sha256(raw).hexdigest()}


def wait_daemon(owner, daemon, client, timeout=60):
    address = local_chat_private.inspect_pair(owner, daemon, client)
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            code = owner.execute(client, COMMAND[:5] + ['docker', 'version', '--format', '{{.Server.Version}}'],
                                 out, err, 5)
        if local_chat_private.inspect_pair(owner, daemon, client) != address:
            raise ValueError('private daemon address changed during readiness')
        if code == 0:
            return
        time.sleep(.5)
    raise ValueError('private Docker socket did not become ready')


def run_owned(scheduled, engine, checkout, output, helper, helper_digest, archive, archive_digest, image_id):
    owner, failure, result = scheduled.run, None, None
    try:
        network = owner.create_resource('network')
        mounts = {name: owner.create_resource('volume') for name in
                  ('/home/rootless/.local/share/docker', '/var/lib/docker', '/run/user/1000')}
        workspace = owner.create_resource('volume')
        client_id = engine._call('image', 'inspect', CLIENT_IMAGE, '--format', '{{.Id}}')
        client = owner.create_container(client_id, network, {'/socket': mounts['/run/user/1000'],
                                                             '/workspace': workspace}, [],
                                        command=['sleep', 'infinity'], clear_entrypoint=True)
        owner.start_container(client)
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            if owner.execute(client, ['chown', '1000:1000', '/socket'], out, err, 30):
                raise ValueError('private socket initialization failed')
        daemon = owner.create_rootless_container(network, mounts,
            lambda request: orchestration._create_daemon(helper, helper_digest, engine.rootless_creator_env(), request))
        owner.start_container(daemon)
        wait_daemon(owner, daemon, client)
        orchestration._verify_file(archive, archive_digest)
        copied = owner.directory / 'postgres-source.tar'
        with Path(archive).open('rb') as source, copied.open('xb') as target:
            for block in iter(lambda: source.read(1024 * 1024), b''):
                target.write(block)
            target.flush()
            os.fsync(target.fileno())
        orchestration._verify_file(archive, archive_digest)
        orchestration._verify_file(copied, archive_digest)
        with (output / 'image.stdout').open('xb') as out, (output / 'image.stderr').open('xb') as err:
            owner.load_canonical_rootless_image(daemon, copied, archive_digest, image_id,
                                                'postgres:16-bookworm', out, err, 300)
        with (output / 'compose.stdout').open('x+b') as out, (output / 'compose.stderr').open('xb') as err:
            result = run_fixture(owner, daemon, client, checkout, out, err, output / 'compose-model.json')
        result.update(client_image_id=client_id, postgres_image_id=image_id,
                      archive_sha256=archive_digest, slot=scheduled.slot,
                      ownership_journal=str(owner.directory / 'ownership.json'))
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


def run(output, helper, helper_digest, archive, archive_digest, image_id):
    if os.name != 'nt':
        raise ValueError('Windows supervised diagnostic required; Linux is unsupported')
    output = Path(output).resolve()
    if output.exists() or not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id):
        raise ValueError('new output directory and image ID required')
    orchestration._verify_file(helper, helper_digest)
    orchestration._verify_file(archive, archive_digest)
    output.mkdir(parents=True)
    checkout = output / 'run'
    checkout.mkdir()
    engine = DockerCLI()
    scheduled = ScheduledRun.start_host(checkout, engine, 300)
    try:
        publish_owner(scheduled, output)
    except BaseException as error:
        orchestration._finish(scheduled, error)
    result = run_owned(scheduled, engine, checkout, output, helper, helper_digest, archive, archive_digest, image_id)
    result.update(schema='voice-local-compose-private-v1', unbound_inputs=[
        'helper, image and Compose tool provenance', 'exclusive daemon and socket access',
        'general Compose models and interruption acceptance'])
    (output / 'diagnostic.json').write_text(json.dumps(result, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    return result
