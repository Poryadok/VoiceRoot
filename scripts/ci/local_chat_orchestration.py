"""Run a bounded selected Chat diagnostic; never issue preflight PASS.

The caller must authenticate helper and image identities and maintain exclusive
control of the outer daemon, private network and socket for the entire run.
These assumptions are not established by local journal locks or inspections.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import tempfile
import time

from local_docker_owned import DockerCLI, OwnershipError
from local_docker_scheduled import ScheduledRun
import local_chat_private
import local_go_dependencies
import local_go_execution
import local_go_plan
import local_process_supervision as supervision
import local_source_snapshot as snapshot


def _verify_file(path, expected):
    path = Path(path)
    if (not isinstance(expected, str) or not re.fullmatch(r'[0-9a-f]{64}', expected) or
            not path.is_absolute() or path.resolve() != path or path.is_symlink()):
        raise ValueError('invalid caller file identity')
    with path.open('rb') as stream:
        before, current = os.fstat(stream.fileno()), path.lstat()
        if (not stat.S_ISREG(before.st_mode) or not stat.S_ISREG(current.st_mode) or
                (before.st_dev, before.st_ino) != (current.st_dev, current.st_ino)):
            raise ValueError('caller file identity changed')
        digest = hashlib.file_digest(stream, 'sha256').hexdigest()
        after, current = os.fstat(stream.fileno()), path.lstat()
        if ((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) !=
                (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) or
                (after.st_dev, after.st_ino) != (current.st_dev, current.st_ino) or digest != expected):
            raise ValueError('caller file bytes changed')


def _create_daemon(helper, digest, environment, request):
    _verify_file(helper, digest)
    result = supervision.run_diagnostic([str(helper)], timeout=90, env=environment,
                                        input=json.dumps(request, sort_keys=True, separators=(',', ':')).encode(),
                                        capture_output=True, check=False)
    if result.returncode != 0:
        raise OwnershipError('rootless creator outcome uncertain')
    _verify_file(helper, digest)
    try:
        identifier = result.stdout.decode('ascii').strip()
    except UnicodeError:
        raise OwnershipError('rootless creator output invalid') from None
    if not re.fullmatch(r'[0-9a-f]{64}', identifier):
        raise OwnershipError('rootless creator output invalid')
    return identifier


def _finish(scheduled, failure):
    try:
        scheduled.close()
    except BaseException as cleanup:
        if failure is not None:
            raise BaseExceptionGroup('Chat diagnostic and cleanup failed', [failure, cleanup])
        raise
    if failure is not None:
        raise failure


def _report(source, diagnostic, journal, slot):
    if diagnostic.get('status') != 'incomplete':
        raise ValueError('unexpected selected Chat diagnostic status')
    return {'schema': 'voice-local-chat-private-diagnostic-v1', 'status': 'incomplete',
            'source': source, 'diagnostic': diagnostic, 'ownership_journal': journal, 'slot': slot,
            'unbound_inputs': ['helper, image and tool provenance', 'exclusive daemon and socket access',
                               'Go dependency acquisition and cached ziphash source binding',
                               'host dependency acquisition process supervision']}


def _wait_daemon(owner, daemon, client, timeout=60):
    """Retry the exact Unix endpoint without mutating the daemon.

    Timeout is a retry budget, not a wall-clock bound: ownership inspection
    and Docker CLI operations have their own timeouts.
    """
    address = local_chat_private.inspect_pair(owner, daemon, client)
    deadline = time.monotonic() + timeout
    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise ValueError('private Docker socket did not become ready')
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            code = owner.execute(client, ['curl', '--silent', '--show-error', '--fail', '--max-time', '2',
                                          '--unix-socket', '/socket/docker.sock', 'http://localhost/_ping'],
                                 out, err, min(5, remaining))
            out.seek(0)
            ready = code == 0 and out.read(16) == b'OK'
        if local_chat_private.inspect_pair(owner, daemon, client) != address:
            raise ValueError('private Docker endpoint changed during readiness')
        if ready:
            return
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise ValueError('private Docker socket did not become ready')
        time.sleep(min(.5, remaining))


def _run_owned(scheduled, engine, source, root, source_copy, output, helper, helper_digest,
               archive, archive_sha256, image_id, acquisition=None):
    owner = scheduled.run
    failure = None
    result = None
    try:
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
            lambda request: _create_daemon(helper, helper_digest, engine.rootless_creator_env(), request))
        owner.start_container(daemon)
        _wait_daemon(owner, daemon, client)
        _verify_file(archive, archive_sha256)
        copied = owner.directory / 'postgres-source.tar'
        with Path(archive).open('rb') as source_file, copied.open('xb') as target:
            for block in iter(lambda: source_file.read(1024 * 1024), b''):
                target.write(block)
            target.flush()
            os.fsync(target.fileno())
        _verify_file(archive, archive_sha256)
        _verify_file(copied, archive_sha256)
        with (output / 'image.stdout').open('xb') as out, (output / 'image.stderr').open('xb') as err:
            owner.load_canonical_rootless_image(daemon, copied, archive_sha256, image_id,
                                                'postgres:16-bookworm', out, err, 300)
        source_digest = local_go_execution.transfer_source(owner, client, root, source, source_copy,
                                                            output / 'source.tar')
        local_go_execution.transfer_dependencies(owner, client, root, source, source_copy, source_digest,
                                                 acquisition, output / 'dependencies/download-cache.tar')
        diagnostic = local_chat_private.run_selected(owner, daemon, client, root, source, source_copy,
                                                     source_digest, output / 'selected')
        result = _report(source, diagnostic, str((owner.directory / 'ownership.json').relative_to(output)),
                         scheduled.slot)
        result['inputs'] = {'go_image_id': image, 'postgres_image_id': image_id,
                            'postgres_archive_sha256': archive_sha256, 'creator_sha256': helper_digest,
                            'dependency_archive_sha256': acquisition['archive_sha256']}
    except BaseException as error:
        failure = error
    _finish(scheduled, failure)
    return result


def run(root, head, output, helper, helper_digest, archive, archive_sha256, image_id, go_binary='go'):
    """Run on Windows under a caller-enforced exclusive Docker/socket boundary.

    Images and tools are caller-supplied identities, not authenticated inputs.
    Linux remains unsupported. All errors fail; uncertain operations retain slots.
    """
    if os.name != 'nt':
        raise ValueError('Windows supervised diagnostic required; Linux is unsupported')
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('output must be a new directory outside the source tree')
    if not isinstance(image_id, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id):
        raise ValueError('invalid Postgres image identity')
    _verify_file(helper, helper_digest)
    _verify_file(archive, archive_sha256)
    go_binary = local_go_plan._check_go_toolchain(go_binary)
    source = snapshot.capture(root, head)
    output.mkdir(parents=True)
    source_copy = snapshot.materialize(root, source, output / 'source')
    acquisition = local_go_dependencies.acquire(root, source, source_copy, 'src/backend/chat',
                                                 output / 'dependencies', go_binary)
    checkout = output / 'run'
    checkout.mkdir()
    engine = DockerCLI()
    scheduled = ScheduledRun.start_host(checkout, engine, 300)
    result = _run_owned(scheduled, engine, source, root, source_copy, output, helper, helper_digest,
                        archive, archive_sha256, image_id, acquisition)
    if snapshot.capture(root, head) != source:
        raise ValueError('source changed during Chat diagnostic cleanup')
    (output / 'diagnostic.json').write_text(json.dumps(result, sort_keys=True, indent=2) + '\n', encoding='utf-8')
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--head', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--helper', required=True)
    parser.add_argument('--helper-sha256', required=True)
    parser.add_argument('--postgres-archive', required=True)
    parser.add_argument('--postgres-archive-sha256', required=True)
    parser.add_argument('--postgres-image-id', required=True)
    parser.add_argument('--go', default='go')
    args = parser.parse_args()
    result = run(Path(__file__).resolve().parents[2], args.head, args.output, args.helper,
                 args.helper_sha256, args.postgres_archive, args.postgres_archive_sha256,
                 args.postgres_image_id, args.go)
    print(json.dumps(result, sort_keys=True))
    return 2  # Incomplete diagnostics cannot authorize a preflight handoff.


if __name__ == '__main__':
    raise SystemExit(main())
