"""Run two named hosted NATS scripts in a private daemon; never a CI pass.

Trusted same-user/daemon custody is required throughout. Filesystem operations
and disk growth are not independently bounded. Any uncertainty retains the slot.
"""
import hashlib
import json
import math
import os
from pathlib import Path
import shutil
import re
import sys
import tempfile
import time

GO_IMAGE = 'sha256:a9daab691df2dfe118e5c6d3d6bd8fd6767ea2c9bf8fa3c9356161055dd7b97d'
CLI_IMAGE = 'sha256:7691a001d1bf70d960395ef367f9f008db5f8d7d18556b2b88303596a4384d51'
IMAGE_TAGS = ('nats:2.12-alpine', 'natsio/nats-box:0.18.0', 'alpine:3.22')
SCRIPTS = {
    'nats-jwt-leaf-hosted-proof.sh': 'PASS: hosted JWT resolver + TLS leaf + exact ACL proof',
    'nats-canonical-acl-hosted-proof.sh': 'PASS: canonical hosted JWT/TLS bootstrap and least-privilege ACL proof',
}
MAX_LOG_BYTES = 16 * 1024 * 1024
REQUEST_KEYS = frozenset({'root', 'source', 'source_copy', 'deps', 'deps_archive', 'output',
                          'helper', 'helper_hash', 'jq', 'jq_hash', 'images'})
PROJECT_MODULES = frozenset({'affected_check_plan', 'go_integration_receipts', 'local_chat_orchestration',
    'local_chat_private', 'local_docker_owned', 'local_docker_scheduled', 'local_go_dependencies',
    'local_go_execution', 'local_go_plan', 'local_go_verify', 'local_heavy_slots', 'local_image_archive',
    'local_process_supervision', 'local_rootless_profile', 'local_source_snapshot'})
from local_docker_owned import DockerCLI
from local_docker_scheduled import ScheduledRun
import local_chat_orchestration as chat
import local_go_execution as go
import local_rootless_profile as profile
import local_source_snapshot as snapshot
import local_process_supervision as supervision


class DiagnosticError(RuntimeError):
    """A failed worker is not a receipt; its journal may require recovery."""


def _windows():
    return os.name == 'nt'


def validate_script_result(script, code, stdout, stderr):
    if script not in SCRIPTS or type(code) is not int or code != 0:
        raise ValueError('hosted NATS script failed or is unknown')
    paths = (Path(stdout), Path(stderr))
    if any(path.stat().st_size > MAX_LOG_BYTES for path in paths):
        raise ValueError('hosted NATS log exceeds diagnostic read limit')
    out, err = (path.read_bytes() for path in paths)
    if b'SKIP:' in out + err or SCRIPTS[script].encode() not in out.splitlines():
        raise ValueError('hosted NATS script result incomplete or skipped')
    return {'status': 'incomplete', 'script': script, 'exit_code': code,
            'stdout_sha256': hashlib.sha256(out).hexdigest(), 'stderr_sha256': hashlib.sha256(err).hexdigest()}


def validate_images(images):
    if (not isinstance(images, (list, tuple)) or len(images) != len(IMAGE_TAGS) or
            any(not isinstance(row, (list, tuple)) or len(row) != 4 for row in images) or
            tuple(row[3] for row in images) != IMAGE_TAGS or
            any(not isinstance(row[1], str) or not re.fullmatch(r'[0-9a-f]{64}', row[1]) or
                not isinstance(row[2], str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', row[2]) for row in images) or
            len({row[2] for row in images}) != len(IMAGE_TAGS)):
        raise ValueError('exact NATS image inventory required')
    return tuple((Path(path), digest, image, tag) for path, digest, image, tag in images)


def _run_owned(root, source, source_copy, deps, deps_archive, output, helper, helper_hash, jq, jq_hash, images):
    """Run the fixed diagnostic inventory, under caller-enforced trusted custody.

    Images are (archive path, archive SHA-256, image ID, fixed tag) tuples. The
    caller authenticates helper, image and tool provenance and maintains exclusive
    access to the private network/socket. This does not establish CI parity.
    """
    if not _windows():
        raise ValueError('hosted NATS diagnostic is supported only on Windows')
    ROOT, SOURCE_COPY, DEPS_ARCHIVE, BASE = map(lambda x: Path(x).resolve(), (root, source_copy, deps_archive, output))
    HELPER, JQ = Path(helper), Path(jq)
    HELPER_HASH, JQ_HASH = helper_hash, jq_hash
    if (BASE.exists() or BASE == ROOT or ROOT in BASE.parents or BASE == SOURCE_COPY or SOURCE_COPY in BASE.parents):
        raise ValueError('diagnostic output must be new and outside source trees')
    IMAGES = validate_images(images)
    BASE.mkdir(exist_ok=False)
    chat._verify_file(HELPER, HELPER_HASH)
    chat._verify_file(JQ, JQ_HASH)
    snapshot.verify_materialized(ROOT, source, SOURCE_COPY)
    if snapshot.capture(ROOT, source['head']) != source:
        raise RuntimeError('source drift before run')
    engine = DockerCLI()
    daemon_id = engine.daemon()
    for image in (GO_IMAGE, CLI_IMAGE):
        if engine._call('image', 'inspect', image, '--format', '{{.Id}}') != image:
            raise RuntimeError('client image identity changed')
    owner_root = BASE / 'owner'
    owner_root.mkdir()
    scheduled = ScheduledRun.start_host(owner_root, engine, 0)
    owner = scheduled.run
    failure = None
    try:
        (BASE / 'identity.json').write_text(json.dumps({'daemon': daemon_id, 'slot': scheduled.slot,
            'journal': str(owner.directory / 'ownership.json'), 'source': source,
            'images': [row[2:] for row in IMAGES]}, indent=2))
        print('SCRIPT_PROBE_OWNER', scheduled.slot, owner.directory, file=sys.stderr, flush=True)
        network = owner.create_resource('network')
        mounts = {path: owner.create_resource('volume') for path in
                  ('/home/rootless/.local/share/docker', '/var/lib/docker', '/run/user/1000')}
        workspace = owner.create_resource('volume')
        cli = owner.create_container(CLI_IMAGE, network, {'/run/user/1000': mounts['/run/user/1000']}, [],
                                     command=['sleep', 'infinity'], clear_entrypoint=True)
        owner.start_container(cli)
        client = owner.create_container(GO_IMAGE, network, {'/run/user/1000': mounts['/run/user/1000'],
                                        '/workspace': workspace}, [], command=['sleep', 'infinity'])
        owner.start_container(client)

        def execute(name, who, command, timeout=120, stdin=None, allow_failure=False):
            with (BASE / (name + '.stdout')).open('xb') as out, (BASE / (name + '.stderr')).open('xb') as err:
                code = owner.execute(who, command, out, err, timeout, stdin=stdin) if stdin else owner.execute(who, command, out, err, timeout)
            if code and not allow_failure:
                raise RuntimeError(name + ' failed: ' + str(code))
            return code

        execute('socket-permissions', client, ['chown', '1000:1000', '/run/user/1000'])
        execute('copy-docker-cli', cli, ['sh', '-ec', 'mkdir /run/user/1000/tools; '
            'cp /usr/local/bin/docker /run/user/1000/tools/docker-cli; chmod 755 /run/user/1000/tools/docker-cli'])
        with JQ.open('rb') as jq:
            execute('copy-jq', client, ['sh', '-ec', 'set -C; cat > /run/user/1000/tools/jq; '
                'chmod 755 /run/user/1000/tools/jq'], stdin=jq)
        chat._verify_file(JQ, JQ_HASH)
        execute('tool-check', client, ['sh', '-ec', 'sha256sum /run/user/1000/tools/jq /run/user/1000/tools/docker-cli; '
            '/run/user/1000/tools/jq --version; /run/user/1000/tools/docker-cli --version; go version; '
            'chown -R 1000:1000 /run/user/1000/tools'])
        if (BASE / 'tool-check.stdout').read_text().split()[0] != JQ_HASH:
            raise RuntimeError('transferred jq differs')
        private = owner.create_rootless_container(network, mounts,
            lambda request: chat._create_daemon(HELPER, HELPER_HASH, engine.rootless_creator_env(), request))
        owner.start_container(private)

        def verify_peers():
            owner._validate(); owner._check_daemon()
            items = {r['name']: r for r in owner.state['resources']}
            for name in (network, cli, client, private):
                owner._inspect(items[name])
            net = items[network]
            raw = json.loads(engine._call('network', 'inspect', net['id']))[0]
            labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': net['nonce'], 'voice.ci.kind': 'network'}
            profile.verify_owned_network(raw, net['id'], network, labels,
                {items[name]['id']: name for name in (cli, client, private)})
            raw_daemon = json.loads(engine._call('container', 'inspect', items[private]['id']))[0]
            profile.verify_runtime_network(raw_daemon, net['id'])
            owner._check_daemon()

        docker = ['/run/user/1000/tools/docker-cli', '-H', 'unix:///run/user/1000/docker.sock']
        verify_peers()
        for attempt in range(30):
            if execute('ping-' + str(attempt), client, docker + ['info'], timeout=5, allow_failure=True) == 0:
                break
            time.sleep(1)
        else:
            raise RuntimeError('private daemon unavailable')
        for number, (path, digest, image, tag) in enumerate(IMAGES):
            chat._verify_file(path, digest)
            copy = owner.directory / ('image-' + str(number) + '.tar')
            with path.open('rb') as source_file, copy.open('xb') as target:
                shutil.copyfileobj(source_file, target, 1024 * 1024)
            chat._verify_file(path, digest); chat._verify_file(copy, digest)
            with (BASE / ('load-' + str(number) + '.stdout')).open('xb') as out, \
                 (BASE / ('load-' + str(number) + '.stderr')).open('xb') as err:
                owner.load_canonical_rootless_image(private, copy, digest, image, None, out, err, 300)
            execute('tag-' + str(number), client, docker + ['image', 'tag', image, tag])
            execute('tag-check-' + str(number), client, docker + ['image', 'inspect', tag, '--format', '{{.Id}}'])
            if (BASE / ('tag-check-' + str(number) + '.stdout')).read_text().strip() != image:
                raise RuntimeError('private tag identity differs')
        archive_digest = go.transfer_source(owner, client, ROOT, source, SOURCE_COPY, BASE / 'source.tar')
        go.transfer_dependencies(owner, client, ROOT, source, SOURCE_COPY, archive_digest, deps, DEPS_ARCHIVE)
        execute('prepare-user', client, ['sh', '-ec', 'mkdir /run/user/1000/tmp /run/user/1000/gocache; '
            'chown -R 1000:1000 /run/user/1000/tmp /run/user/1000/gocache /workspace/go-mod'])
        wrapper = '#!/bin/sh\nif [ "$1" = run ]; then shift; exec /run/user/1000/tools/docker-cli -H unix:///run/user/1000/docker.sock run --pull=never "$@"; fi\nexec /run/user/1000/tools/docker-cli -H unix:///run/user/1000/docker.sock "$@"\n'
        with tempfile.TemporaryFile() as stream:
            stream.write(wrapper.encode()); stream.seek(0)
            execute('docker-wrapper', client, ['sh', '-ec', 'set -C; cat > /run/user/1000/tools/docker; '
                'chmod 755 /run/user/1000/tools/docker; chown 1000:1000 /run/user/1000/tools/docker'], stdin=stream)
        verify_peers()
        go._verify_workspace(owner, client, ROOT, source, archive_digest)
        results = []
        for number, script in enumerate(SCRIPTS):
            prefix = 'script-' + str(number)
            execute(prefix + '-containers-before', client, docker + ['ps', '-aq', '--no-trunc'])
            execute(prefix + '-networks-before', client, docker + ['network', 'ls', '--format', '{{.ID}} {{.Name}}'])
            if (BASE / (prefix + '-containers-before.stdout')).read_bytes().strip():
                raise RuntimeError('private daemon has unexpected containers before script')
            command = ['setpriv', '--reuid=1000', '--regid=1000', '--clear-groups', 'env',
                'GITHUB_ACTIONS=true', 'TMPDIR=/run/user/1000/tmp', 'HOME=/run/user/1000', 'USER=voice-ci',
                'PATH=/run/user/1000/tools:/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin',
                'GOPROXY=off', 'GOSUMDB=off', 'GOTOOLCHAIN=local', 'GOFLAGS=-mod=readonly',
                'GOMODCACHE=/workspace/go-mod', 'GOCACHE=/run/user/1000/gocache', 'CGO_ENABLED=0',
                'GOMAXPROCS=2', 'bash', '/workspace/source/scripts/ci/' + script]
            code = execute(prefix, client, command, timeout=1800, allow_failure=True)
            results.append(validate_script_result(script, code, BASE / (prefix + '.stdout'), BASE / (prefix + '.stderr')))
            execute(prefix + '-containers-after', client, docker + ['ps', '-aq', '--no-trunc'])
            execute(prefix + '-networks-after', client, docker + ['network', 'ls', '--format', '{{.ID}} {{.Name}}'])
            if ((BASE / (prefix + '-containers-after.stdout')).read_bytes().strip() or
                    (BASE / (prefix + '-networks-before.stdout')).read_bytes() != (BASE / (prefix + '-networks-after.stdout')).read_bytes()):
                raise RuntimeError('private daemon resources remain after script')
            verify_peers(); go._verify_modules(owner, client, 'src/backend/pkg')
            go._verify_workspace(owner, client, ROOT, source, archive_digest)
            snapshot.verify_materialized(ROOT, source, SOURCE_COPY)
            if snapshot.capture(ROOT, source['head']) != source:
                raise RuntimeError('source drift after run')
        report = {'status': 'incomplete', 'source': source, 'scripts': results,
            'images': [{'archive_sha256': row[1], 'image_id': row[2], 'tag': row[3]} for row in IMAGES],
            'tools': {'helper_sha256': HELPER_HASH, 'jq_sha256': JQ_HASH, 'go_image_id': GO_IMAGE,
                      'cli_image_id': CLI_IMAGE, 'dependency_archive_sha256': deps['archive_sha256']},
            'slot': scheduled.slot,
            'ownership_journal': str(owner.directory / 'ownership.json'),
            'omitted': ['remaining CI scripts and shards', 'registry tag drift after acquisition',
                        'Go ziphash provenance', 'adversarial same-user Docker administrators',
                        'bounded filesystem operations and disk growth', 'crash and foreign sentinel proof']}
    except BaseException as exc:
        failure = exc
    chat._finish(scheduled, failure)
    (BASE / 'diagnostic.json').write_text(json.dumps(report, indent=2))
    return report


def _request(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError('duplicate diagnostic request key')
            result[key] = value
        return result
    request = json.loads(raw, object_pairs_hook=unique)
    if not isinstance(request, dict) or set(request) != REQUEST_KEYS:
        raise ValueError('invalid diagnostic request fields')
    return request


def _worker_command(path):
    path = Path(path).resolve()
    # The project directory never enters sys.path: only named project modules
    # can be loaded from it, including when an optional stdlib module is absent.
    bootstrap = ('import importlib.abc,importlib.util,os,runpy,sys\n'
                 'root=sys.argv[1]\n'
                 'allowed=frozenset(' + repr(sorted(PROJECT_MODULES)) + ')\n'
                 'class ProjectFinder(importlib.abc.MetaPathFinder):\n'
                 ' def find_spec(self,name,path=None,target=None):\n'
                 '  if name in allowed:\n'
                 '   return importlib.util.spec_from_file_location(name,os.path.join(root,name+".py"))\n'
                 'sys.meta_path.append(ProjectFinder())\n'
                 'sys.argv=sys.argv[2:]\n'
                 'runpy.run_path(sys.argv[0],run_name="__main__")')
    return [sys.executable, '-I', '-S', '-B', '-c', bootstrap, str(path.parent), str(path), '--worker']


def run(request, *, timeout=7200):
    """Supervise the worker; failure may leave a journal requiring recovery.

    The Windows broker bounds worker execution and its child processes, not
    daemon cancellation, process startup, filesystem operations or disk growth.
    Never recover an uncertain slot automatically.
    """
    if not isinstance(request, dict) or set(request) != REQUEST_KEYS:
        raise ValueError('invalid diagnostic request fields')
    if (isinstance(timeout, bool) or not isinstance(timeout, (int, float)) or
            not math.isfinite(timeout) or not 0 < timeout <= 7200):
        raise ValueError('invalid diagnostic timeout')
    raw = json.dumps(request, allow_nan=False, separators=(',', ':')).encode('utf-8')
    if len(raw) > 1024 * 1024:
        raise ValueError('diagnostic request exceeds limit')
    try:
        result = supervision.run_diagnostic(_worker_command(__file__),
            timeout=timeout, input=raw, capture_output=True, check=False)
    except BaseException as error:
        error.add_note('Worker cleanup may be uncertain; inspect the requested output ownership journal and slot.')
        raise
    if result.returncode != 0:
        error = DiagnosticError('NATS worker failed; inspect output logs and ownership journal before recovery')
        error.worker_returncode = result.returncode
        error.worker_stderr = result.stderr
        raise error
    try:
        report = json.loads(result.stdout)
        scripts = report['scripts']
        if (report['status'] != 'incomplete' or report['source'] != request['source'] or
                not isinstance(scripts, list) or len(scripts) != len(SCRIPTS) or
                [row['script'] for row in scripts] != list(SCRIPTS) or
                any(row['status'] != 'incomplete' or type(row['exit_code']) is not int or row['exit_code'] != 0 for row in scripts)):
            raise ValueError()
    except (ValueError, TypeError, KeyError, UnicodeError):
        raise DiagnosticError('NATS worker returned an invalid incomplete report') from None
    return report


def main(argv=None):
    args = sys.argv[1:] if argv is None else argv
    if args == ['--worker']:
        raw = sys.stdin.buffer.read(1024 * 1024 + 1)
        if len(raw) > 1024 * 1024:
            raise ValueError('diagnostic request exceeds limit')
        print(json.dumps(_run_owned(**_request(raw)), sort_keys=True))
        return 0
    if len(args) != 1:
        raise ValueError('expected one diagnostic request JSON path')
    with Path(args[0]).open('rb') as stream:
        raw = stream.read(1024 * 1024 + 1)
    if len(raw) > 1024 * 1024:
        raise ValueError('diagnostic request exceeds limit')
    request = _request(raw)
    print(json.dumps(run(request), sort_keys=True))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
