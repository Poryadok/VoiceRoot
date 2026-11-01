"""Run one hosted NATS script in a private daemon; never a full CI receipt.

Trusted same-user/daemon custody is required throughout. Filesystem operations
and disk growth are not independently bounded. Any uncertainty retains the slot.
"""
import hashlib
import json
from pathlib import Path
import shutil
import sys
import tempfile
import time
import subprocess
import os
import secrets
import re
import shlex

import local_ci_staging_request as request_contract
import local_ci_matrix_inventory as matrix_inventory

HELPER_HASH = '27107931deb9d34a1055c6fff20c00dd5d97cecc2732cb6d1f0bd27646dc35a1'
JQ_HASH = '5942c9b0934e510ee61eb3e30273f1b3fe2590df93933a93d7c58b81d19c8ff5'
KUBECTL_HASH = request_contract.KUBECTL_HASH
NODE_IMAGE = 'sha256:9214e73b07d3828f371e8d5d8bfebdfa1b0625c3c82253a4cbd09d9c00e20b4a'
GO_IMAGE = 'sha256:a9daab691df2dfe118e5c6d3d6bd8fd6767ea2c9bf8fa3c9356161055dd7b97d'
PYTHON_CLIENT = 'sha256:61d53b56a648da09aa966c5cc46a7e751aea2b38d01f33b04456a9f289685be9'
CLI_IMAGE = 'sha256:7691a001d1bf70d960395ef367f9f008db5f8d7d18556b2b88303596a4384d51'
from local_docker_owned import DockerCLI
from local_docker_scheduled import ScheduledRun
import local_chat_orchestration as chat
import local_go_execution as go
import local_rootless_profile as profile
import local_source_snapshot as snapshot


def verify_policy_kubectl(path):
    chat._verify_file(path, KUBECTL_HASH)


def configure(request):
    global ROOT, BASE, HELPER, JQ, SOURCE_COMMIT, SOURCE_TREE, SOURCE_BASE, SHARD, IMAGES, ASSETS, RUNTIME_IMAGES
    request = request_contract.validate(request)
    ROOT, BASE = Path(request['root']), Path(request['output'])
    SOURCE_COMMIT, SOURCE_TREE, SOURCE_BASE, SHARD = request['commit'], request['tree'], request['base'], request['shard']
    ASSETS = {key: Path(value) for key, value in request['assets'].items()}
    RUNTIME_IMAGES = request.get('runtime_images', {})
    HELPER, JQ = ASSETS['helper'], ASSETS['jq']
    IMAGES = (
    (ASSETS['postgres_image'], 'bcdd6a15d43a8a7ef1dbe8ae5b625d426163ce4419cdb07953aff03fed760222', 'sha256:6a2e324ebe59af6031c4c1cd96c1d5072c2ebc36563d2196365e43b8297c062d', 'postgres:16-alpine'),
    (ASSETS['go_image'], '250dec6007dd9cc790025b05a4fcf1b7c42f161ff1c8bf1416f34f36066c124b', 'sha256:c5fe2bbc8331458475b45686d82e0c28b1161addfe33f1ebac0a715a29587701', 'voice-ci/go:locked'),
    (ASSETS['python_image'], 'd7fb0e66f9a9f1e06e41acbc5cada84eeab9ddd6b2e216dc800c758322bee43d', 'sha256:979e5a8a56846771598d47ebf1f8a75b46155104e168a0b237c0783c1dedff8a', 'voice-ci/python:locked'),
    (ASSETS['nats_image'], 'b24e03bdee3b6c6091f21e0a88626f9a45364db538958cdeba2498d7b551637c', 'sha256:138f6ccc9fd034c76ac3cee7faed6c5fdb874fd333f477b91f52b4052f6669ce', 'nats:2.12-alpine'),
    (ASSETS['nats_box_image'], '09c27c2c8f09ff60c9c6c747a6bd82560181a61244595631db52ad7ea8c565eb', 'sha256:baf85f62859ab3ccc005820d83dd455345c9373f101fd64197e16a06aeeaac6e', 'natsio/nats-box:0.18.0'),
    (ASSETS['alpine_image'], '7738547cc655bffaeaa3a7c1b43292f3fde45ea00772918d04c9afc819f0b148', 'sha256:24ab38714e948bd49d24d566e97c1c104b63f85c2e642685e72393b37cfb9862', 'alpine:3.22'),
)

def host_git(*args, stdout=subprocess.PIPE):
    environment = dict(os.environ)
    for key in list(environment):
        if key.startswith('GIT_'):
            del environment[key]
    environment.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
                       GIT_TERMINAL_PROMPT='0')
    return subprocess.run(['git', *args], cwd=ROOT, env=environment,
                          check=True, stdout=stdout, stderr=subprocess.PIPE,
                          timeout=180).stdout

def verify_host():
    if (host_git('rev-parse', 'HEAD').strip().decode() != SOURCE_COMMIT or
        host_git('rev-parse', 'HEAD^{tree}').strip().decode() != SOURCE_TREE or
        host_git('status', '--porcelain=v1', '--untracked-files=all').strip()):
        raise RuntimeError('host source identity or cleanliness changed')

def prepare_source():
    verify_host()
    host_git('bundle', 'create', str(BASE / 'source.bundle'), 'HEAD')
    with (BASE / 'source.tar').open('xb') as stream:
        host_git('archive', '--format=tar', 'HEAD', stdout=stream)
    verify_host()

def build_adapter(text, hashes):
    marker = "  sh -eu -c '"
    if text.count(marker) != 1: raise RuntimeError('Python fixture call differs')
    original = text.split(marker, 1)[1].rsplit("'", 1)[0]
    original_sha = hashlib.sha256(original.encode()).hexdigest()
    goflags = 'GOFLAGS=-p=2'
    tests = '    python -m unittest "$@"'
    baseline = '    python -m unittest controller_test root_main_test docker_runtime_test stage_runtime_test'
    if original.count(goflags) != 1 or original.count(tests) != 1 or original.count(baseline) != 1:
        raise RuntimeError('canonical Python fixture shape differs')
    guard = '; '.join("echo '" + hashes[name] + '  /tmp/repo/scripts/staging/nats-known-baseline/' + name + "' | sha256sum -c - || exit 1" for name in ('go.mod','go.sum'))
    prewarm = (guard + '; cd /tmp/repo/scripts/staging/nats-known-baseline; '
               'timeout 600 go build github.com/nats-io/nats-server/v2/server; ' + guard +
               '; cd /tmp/repo/scripts/staging/nats-rollout-preservation; ' + tests.strip() + '; ' + guard)
    adapted = original.replace('GOMAXPROCS=2', 'GOMAXPROCS=1', 1).replace(goflags, "GOFLAGS='-p=1 -mod=readonly'", 1).replace(tests, '    ' + prewarm, 1).replace(baseline, baseline + '; ' + guard, 1)
    return original_sha, ('#!/bin/sh\nset -eu\n' + adapted).encode()

def validate_preservation_output(output):
    marker = b'NATS_ROLLOUT_PRESERVATION=PASS valid-receipt-paused-apply-and-second-use-veto-all-modes'
    summaries = re.findall(rb'^Ran ([0-9]+) tests? in [^\r\n]+\r?$', output, flags=re.MULTILINE)
    oks = re.findall(rb'^OK\r?$', output, flags=re.MULTILINE)
    if (len(output) > 16 * 1024 * 1024 or b'SKIP' in output.upper() or
        len(summaries) != 3 or any(int(count) <= 0 for count in summaries) or len(oks) != 3 or
        re.search(rb'^FAILED(?:\s|\()', output, flags=re.MULTILINE) or output.splitlines().count(marker) != 1):
        raise RuntimeError('preservation output incomplete, skipped or too large')
    return [int(count) for count in summaries]


def hosted_go_script(directory):
    """Keep go test's JSON and diagnostics in separate bounded streams."""
    return ('set -o pipefail; cd ' + shlex.quote(directory) + '; '
            'exec 3> >(head -c 16777217 >&2); stderr_pid=$!; '
            'go test -json -count=1 -timeout=90s ./... 2>&3 | head -c 16777217; '
            'codes=("${PIPESTATUS[@]}"); exec 3>&-; wait "$stderr_pid" || exit 125; '
            '[ "${codes[1]}" = 0 ] || exit 125; exit "${codes[0]}"')


def hosted_policy_script():
    return ('set -o pipefail; cd /workspace/repo; '
            'exec 3> >(head -c 16777217 >&2); stderr_pid=$!; '
            '/usr/local/bin/python3 scripts/ci/compiled_source_build_test.py 2>&3 | head -c 16777217; '
            'codes=("${PIPESTATUS[@]}"); exec 3>&-; wait "$stderr_pid" || exit 125; '
            '[ "${codes[1]}" = 0 ] || exit 125; exit "${codes[0]}"')


def validate_hosted_go_logs(output, packages, dispatch):
    if (output / 'hosted-go.stderr').stat().st_size > 16 * 1024 * 1024:
        raise RuntimeError('hosted Go diagnostics exceeded limit')
    return dispatch.validate_go_json((output / 'hosted-go.stdout').read_bytes(), packages)

def main():
    BASE.mkdir(exist_ok=False)
    chat._verify_file(HELPER, HELPER_HASH)
    chat._verify_file(JQ, JQ_HASH)
    if SHARD == 'policy': verify_policy_kubectl(ASSETS['kubectl'])
    engine = DockerCLI()
    daemon_id = engine.daemon()
    for image in (GO_IMAGE, CLI_IMAGE, PYTHON_CLIENT, NODE_IMAGE):
        if engine._call('image', 'inspect', image, '--format', '{{.Id}}') != image:
            raise RuntimeError('client image identity changed')
    prepare_source()
    owner_root = BASE / 'owner'
    owner_root.mkdir()
    scheduled = ScheduledRun.start_host(owner_root, engine, 0)
    owner = scheduled.run
    failure = None
    try:
        (BASE / 'identity.json').write_text(json.dumps({'daemon': daemon_id, 'slot': scheduled.slot,
            'journal': str(owner.directory / 'ownership.json'), 'source': SOURCE_COMMIT,
            'images': [row[2:] for row in IMAGES], 'runtime_images': RUNTIME_IMAGES}, indent=2))
        print('SCRIPT_PROBE_OWNER', scheduled.slot, owner.directory, flush=True)
        network = owner.create_resource('network')
        mounts = {path: owner.create_resource('volume') for path in
                  ('/home/rootless/.local/share/docker', '/var/lib/docker', '/run/user/1000')}
        workspace = owner.create_resource('volume')
        cli = owner.create_container(CLI_IMAGE, network, {'/run/user/1000': mounts['/run/user/1000']}, [],
                                     command=['sleep', 'infinity'], clear_entrypoint=True)
        owner.start_container(cli)
        client = owner.create_container(PYTHON_CLIENT, network, {'/run/user/1000': mounts['/run/user/1000'],
                                        '/workspace': workspace}, [], command=['sleep', 'infinity'])
        owner.start_container(client)
        gopeer = owner.create_container(GO_IMAGE, network, {'/run/user/1000': mounts['/run/user/1000']}, [], command=['sleep','infinity'])
        owner.start_container(gopeer)
        nodepeer = owner.create_container(NODE_IMAGE, network, {'/run/user/1000': mounts['/run/user/1000']}, [], command=['sleep','infinity'], clear_entrypoint=True)
        owner.start_container(nodepeer)

        def execute(name, who, command, timeout=120, stdin=None, allow_failure=False):
            with (BASE / (name + '.stdout')).open('xb') as out, (BASE / (name + '.stderr')).open('xb') as err:
                code = owner.execute(who, command, out, err, timeout, stdin=stdin) if stdin else owner.execute(who, command, out, err, timeout)
            if code and not allow_failure:
                raise RuntimeError(name + ' failed: ' + str(code))
            return code

        execute('socket-permissions', client, ['chown', '1000:1000', '/run/user/1000'])
        execute('copy-docker-cli', cli, ['sh', '-ec', 'mkdir /run/user/1000/tools; '
            'cp /usr/local/bin/docker /run/user/1000/tools/docker-cli; chmod 755 /run/user/1000/tools/docker-cli'])
        execute('copy-node', nodepeer, ['sh','-ec','cp /usr/local/bin/node /run/user/1000/tools/node; sha256sum /usr/local/bin/node /run/user/1000/tools/node'])
        node_hashes = (BASE/'copy-node.stdout').read_text().splitlines()
        if len(node_hashes) != 2 or node_hashes[0].split()[0] != node_hashes[1].split()[0]: raise RuntimeError('Node transfer differs')
        execute('copy-go', gopeer, ['sh','-ec','cp -a /usr/local/go /run/user/1000/tools/go'])
        execute('install-node', client, ['install','-o','0','-g','0','-m','0555','/run/user/1000/tools/node','/usr/local/bin/node'])
        execute('link-go', client, ['ln','-s','/run/user/1000/tools/go/bin/go','/usr/local/bin/go'])
        with JQ.open('rb') as jq:
            execute('copy-jq', client, ['sh', '-ec', 'set -C; cat > /run/user/1000/tools/jq; '
                'chmod 755 /run/user/1000/tools/jq'], stdin=jq)
        chat._verify_file(JQ, JQ_HASH)
        execute('tool-check', client, ['sh', '-ec', 'sha256sum /run/user/1000/tools/jq /run/user/1000/tools/docker-cli; '
            '/run/user/1000/tools/jq --version; /run/user/1000/tools/docker-cli --version; go version; '
            'chown -R 1000:1000 /run/user/1000/tools'])
        if (BASE / 'tool-check.stdout').read_text().split()[0] != JQ_HASH:
            raise RuntimeError('transferred jq differs')
        execute('install-trusted-tools', client, ['sh','-ec','install -o 0 -g 0 -m 0555 /run/user/1000/tools/jq /usr/local/bin/jq; install -D -o 0 -g 0 -m 0555 /run/user/1000/tools/docker-cli /usr/local/libexec/voice-ci-docker'])
        if SHARD == 'policy':
            with ASSETS['kubectl'].open('rb') as binary:
                execute('copy-kubectl', client, ['sh','-ec','set -C; cat > /run/user/1000/tools/kubectl; install -o 0 -g 0 -m 0555 /run/user/1000/tools/kubectl /usr/local/bin/kubectl'], stdin=binary)
            verify_policy_kubectl(ASSETS['kubectl'])
            execute('check-kubectl', client, ['sha256sum','/usr/local/bin/kubectl'])
            if (BASE/'check-kubectl.stdout').read_text().split() != [KUBECTL_HASH, '/usr/local/bin/kubectl']:
                raise RuntimeError('transferred kubectl differs')
        docker_cli_sha = (BASE/'tool-check.stdout').read_text().splitlines()[1].split()[0]
        private = owner.create_rootless_container(network, mounts,
            lambda request: chat._create_daemon(HELPER, HELPER_HASH, engine.rootless_creator_env(), request))
        owner.start_container(private)

        def verify_peers():
            owner._validate(); owner._check_daemon()
            items = {r['name']: r for r in owner.state['resources']}
            for name in (network, cli, client, gopeer, nodepeer, private):
                owner._inspect(items[name])
            net = items[network]
            raw = json.loads(engine._call('network', 'inspect', net['id']))[0]
            labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': net['nonce'], 'voice.ci.kind': 'network'}
            profile.verify_owned_network(raw, net['id'], network, labels,
                {items[name]['id']: name for name in (cli, client, gopeer, nodepeer, private)})
            raw_daemon = json.loads(engine._call('container', 'inspect', items[private]['id']))[0]
            profile.verify_runtime_network(raw_daemon, net['id'])
            owner._check_daemon()

        docker = ['/usr/local/libexec/voice-ci-docker', '-H', 'unix:///run/user/1000/docker.sock']
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
        if SHARD == 'runtime':
            for key, tag in (('postgres', None), ('docker_cli', 'docker:27-cli'), ('python', 'python:3.12-bookworm')):
                row = RUNTIME_IMAGES[key]
                path, digest, image = Path(row['archive']), row['sha256'], row['image_id']
                chat._verify_file(path, digest)
                copy = owner.directory / ('runtime-' + key + '.tar')
                with path.open('rb') as source_file, copy.open('xb') as target:
                    shutil.copyfileobj(source_file, target, 1024 * 1024)
                chat._verify_file(path, digest); chat._verify_file(copy, digest)
                with (BASE / ('runtime-load-' + key + '.stdout')).open('xb') as out, \
                     (BASE / ('runtime-load-' + key + '.stderr')).open('xb') as err:
                    owner.load_canonical_rootless_image(private, copy, digest, image, None, out, err, 300)
                execute('runtime-platform-' + key, client, docker + ['image', 'inspect', image, '--format', '{{.Os}}/{{.Architecture}}'])
                if (BASE / ('runtime-platform-' + key + '.stdout')).read_text().strip() != 'linux/amd64':
                    raise RuntimeError('runtime image platform differs')
                if tag is not None:
                    execute('runtime-tag-' + key, client, docker + ['image', 'tag', image, tag])
                    execute('runtime-tag-check-' + key, client, docker + ['image', 'inspect', tag, '--format', '{{.Id}}'])
                    if (BASE / ('runtime-tag-check-' + key + '.stdout')).read_text().strip() != image:
                        raise RuntimeError('runtime private tag differs')
        execute('git-prep', client, ['sh', '-ec', 'mkdir -p /workspace/repo; git -c init.templateDir=/dev/null init /workspace/repo'])
        with (BASE / 'source.bundle').open('rb') as bundle:
            execute('git-transfer', client, ['sh', '-ec', 'set -C; cat > /workspace/source.bundle'], timeout=180, stdin=bundle)
        git_env = ['env', 'HOME=/tmp', 'GIT_CONFIG_NOSYSTEM=1', 'GIT_CONFIG_GLOBAL=/dev/null', 'GIT_TERMINAL_PROMPT=0']
        execute('git-fetch', client, git_env + ['git', '-C', '/workspace/repo', 'fetch', '/workspace/source.bundle', 'HEAD'], timeout=180)
        execute('git-checkout', client, git_env + ['git', '-c', 'core.hooksPath=/dev/null', '-C', '/workspace/repo', 'checkout', '--detach', 'FETCH_HEAD'], timeout=180)
        def check_git(label):
            execute(label, client, git_env + ['sh', '-ec', 'cd /workspace/repo; git rev-parse HEAD; git rev-parse HEAD^{tree}; git status --porcelain=v1 --untracked-files=all; git archive HEAD | sha256sum'], timeout=180)
            with (BASE / 'source.tar').open('rb') as stream:
                digest = hashlib.file_digest(stream, 'sha256').hexdigest()
            expected = [SOURCE_COMMIT, SOURCE_TREE, digest + '  -']
            if (BASE / (label + '.stdout')).read_text().splitlines() != expected:
                raise RuntimeError('private Git identity, cleanliness or archive differs')
        check_git('git-before')
        execute('containers-before', client, docker + ['ps', '-aq', '--no-trunc'])
        if (BASE / 'containers-before.stdout').read_bytes().strip():
            raise RuntimeError('private daemon is not empty')
        marker = secrets.token_hex(32)
        # Packages were acquired separately; this run has an internal-only network.
        package_bundle = ASSETS['packages']
        package_hash = 'bf2ada572dc9ae244abf7a55ea0583edb59e740adc02c9927183d07a0726c4c1'
        chat._verify_file(package_bundle, package_hash)
        import tarfile
        with tarfile.open(package_bundle, 'r:') as tar:
            seen = set(); total = 0
            for member in tar:
                from pathlib import PurePosixPath
                path = PurePosixPath(member.name)
                if (len(seen) >= 5000 or str(path) in seen or path.is_absolute() or '\\' in member.name or ':' in member.name or
                    str(path) != member.name.rstrip('/') or not path.parts or path.parts[0] != 'offline' or
                    any(part in ('', '.', '..') for part in path.parts) or not (member.isdir() or member.isfile()) or
                    member.size < 0 or member.size > 128 * 1024**2 or
                    (len(path.parts) > 1 and path.parts[1] not in ('debian', 'apk', 'wheels'))):
                    raise RuntimeError('unsafe package bundle')
                seen.add(str(path)); total += member.size
                if total > 1024**3: raise RuntimeError('package bundle too large')
        with package_bundle.open('rb') as stream:
            execute('package-transfer', client, ['sh', '-ec', 'set -C; cat > /run/user/1000/packages.tar'], timeout=300, stdin=stream)
        chat._verify_file(package_bundle, package_hash)
        execute('package-transfer-hash', client, ['sha256sum', '/run/user/1000/packages.tar'])
        if (BASE / 'package-transfer-hash.stdout').read_text().split()[0] != package_hash:
            raise RuntimeError('transferred package bundle differs')
        execute('package-extract', client, ['sh', '-ec', 'tar --no-same-owner -xf /run/user/1000/packages.tar -C /run/user/1000; mkdir /run/user/1000/offline/apk/x86_64; mv /run/user/1000/offline/apk/*.apk /run/user/1000/offline/apk/APKINDEX.tar.gz /run/user/1000/offline/apk/x86_64/; printf "file:///offline/apk\\n" > /run/user/1000/offline/repositories'])
        setup = ('set -o pipefail; export DEBIAN_FRONTEND=noninteractive; '
                 'apt-get -o Dir::State::lists=/run/user/1000/offline/debian/lists -o Dir::Cache::archives=/run/user/1000/offline/debian/archives --no-download install -y --no-install-recommends sudo python3 python3-yaml 2>&1 | head -c 16777217')
        execute('client-packages', client, ['bash', '-ec', setup], timeout=600)
        if (BASE / 'client-packages.stdout').stat().st_size > 32 * 1024 * 1024:
            raise RuntimeError('client package output exceeded limit')
        import importlib.util
        dispatch_path = Path(__file__).with_name('local_ci_staging_dispatch.py')
        dispatch_sha = '3dd045d98e2e1cad0ccce50131a3bdcd6bb1adb42556df58700df9590687e0a1'
        chat._verify_file(dispatch_path, dispatch_sha)
        spec = importlib.util.spec_from_file_location('staging_dispatch', dispatch_path)
        dispatch = importlib.util.module_from_spec(spec); spec.loader.exec_module(dispatch)
        # Derive the Python fixture shell exactly from the bound git archive.
        source_tar = BASE / 'source.tar'
        source_hash = (BASE / 'git-before.stdout').read_text().splitlines()[2].split()[0]
        chat._verify_file(source_tar, source_hash)
        import tarfile
        with tarfile.open(source_tar, 'r:') as tar:
            item = tar.getmember('scripts/ci/nats-rollout-preservation_test.sh')
            if not item.isfile() or item.size > 1024**2: raise RuntimeError('fixture script invalid')
            text = tar.extractfile(item).read().decode('utf-8')
            hashes = {}
            for name in ('go.mod','go.sum'):
                item = tar.getmember('scripts/staging/nats-known-baseline/' + name)
                if not item.isfile() or item.size > 1024**2: raise RuntimeError('Go source invalid')
                hashes[name] = hashlib.sha256(tar.extractfile(item).read()).hexdigest()
            if SHARD == 'runtime':
                story = tar.getmember('scripts/ci/story-media-cohort-chain.sh')
                dockerfile = tar.getmember('scripts/ci/story-media-cohort-chain.Dockerfile')
                if not story.isfile() or not dockerfile.isfile() or story.size > 1024**2 or dockerfile.size > 1024**2:
                    raise RuntimeError('runtime fixture source invalid')
                story_adapter = dispatch.story_media_adapter(tar.extractfile(story).read(), tar.extractfile(dockerfile).read(),
                    RUNTIME_IMAGES['docker_cli']['image_id'], RUNTIME_IMAGES['python']['image_id'])
        chat._verify_file(source_tar, source_hash)
        original_sha, adapter_bytes = build_adapter(text, hashes)
        adapter_sha = hashlib.sha256(adapter_bytes).hexdigest()
        with tempfile.TemporaryFile() as stream:
            stream.write(adapter_bytes); stream.seek(0)
            execute('fixture-adapter', client, ['sh','-ec','mkdir /run/user/1000/adapter-root; chmod 755 /run/user/1000/adapter-root; set -C; cat > /run/user/1000/adapter-root/offline-runner.sh; chmod 444 /run/user/1000/adapter-root/offline-runner.sh'], stdin=stream)
        if SHARD == 'runtime':
            story_sha = hashlib.sha256(story_adapter).hexdigest()
            with tempfile.TemporaryFile() as stream:
                stream.write(story_adapter); stream.seek(0)
                execute('story-adapter', client, ['sh','-ec','set -C; cat > /run/user/1000/adapter-root/story-media.sh; chmod 444 /run/user/1000/adapter-root/story-media.sh'], stdin=stream)
            with source_tar.open('rb') as stream:
                execute('runtime-source-transfer', client, ['sh','-ec','set -C; cat > /run/user/1000/source.tar'], timeout=300, stdin=stream)
            chat._verify_file(source_tar, source_hash)
            execute('runtime-source-hash', client, ['sha256sum','/run/user/1000/source.tar'])
            if (BASE/'runtime-source-hash.stdout').read_text().split()[0] != source_hash:
                raise RuntimeError('runtime source transfer differs')
            execute('runtime-source-extract', client, ['sh','-ec','mkdir /run/user/1000/source-mirror; tar --no-same-owner -xf /run/user/1000/source.tar -C /run/user/1000/source-mirror; tar -df /run/user/1000/source.tar -C /run/user/1000/source-mirror'], timeout=300)
        (BASE / 'fixture-adapter.json').write_text(json.dumps({'canonical_argument_sha256':original_sha,
            'adapted_sha256':adapter_sha,'go_source_sha256':hashes}, indent=2))
        wrapper = """#!/bin/sh
set -eu
python_image=0
if [ "${1-}" != run ]; then
  exec /run/user/1000/tools/docker-cli -H unix:///run/user/1000/docker.sock "$@"
fi
shift
n=$#
while [ "$n" -gt 0 ]; do
  arg=$1; shift
  case "$arg" in
    --network)
      [ "$n" -ge 2 ] && [ "$1" = none ] || { echo 'unsupported network option' >&2; exit 2; }
      shift; n=$((n-2)); continue ;;
    --net|--net=*|--network=*|--network-*|--pull|--pull=*)
      echo 'unsupported network or pull option' >&2; exit 2 ;;
    golang@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c) arg=voice-ci/go:locked ;;
    python@sha256:1b668429b3511ab407d8e00648891631b0b1a4d7e15e3ca70f38ab5b91ad4ab4) arg=voice-ci/python:locked; python_image=1 ;;
  esac
  set -- "$@" "$arg"
  n=$((n-1))
done
if [ "$python_image" = 1 ]; then
  set -- --mount type=bind,source=/run/user/1000/offline/apk,target=/offline/apk,readonly --mount type=bind,source=/run/user/1000/offline/wheels,target=/offline/wheels,readonly --mount type=bind,source=/run/user/1000/offline/repositories,target=/etc/apk/repositories,readonly --env PIP_NO_INDEX=1 --env PIP_FIND_LINKS=/offline/wheels "$@"
fi
set -- run --pull=never --network none "$@"
exec /run/user/1000/tools/docker-cli -H unix:///run/user/1000/docker.sock "$@"
"""
        wrapper = wrapper.replace('python_image=0', 'python_image=0\nadapter_count=0')
        wrapper = wrapper.replace('  set -- "$@" "$arg"\n  n=$((n-1))',
            '  if [ "$python_image" = 1 ] && [ "$(printf %s "$arg" | sha256sum | cut -d " " -f 1)" = "' + original_sha + '" ]; then arg="exec /bin/sh /offline-runner.sh"; adapter_count=$((adapter_count+1)); fi\n  set -- "$@" "$arg"\n  n=$((n-1))')
        wrapper = wrapper.replace('if [ "$python_image" = 1 ]; then\n  set --',
            'if [ "$python_image" = 1 ]; then\n  [ "$adapter_count" = 1 ] || { echo "fixture adapter identity differs" >&2; exit 2; }\n  set -- --mount type=bind,source=/run/user/1000/adapter-root/offline-runner.sh,target=/offline-runner.sh,readonly --mount type=bind,source=/run/user/1000/go-proxy/download,target=/offline-go-proxy,readonly --env GOPROXY=file:///offline-go-proxy --env GOSUMDB=off')
        # Retain only the diagnostic Python container for exact postmortem.
        inner_name = 'ci-python-' + owner.run_id
        wrapper = wrapper.replace('python_image=0', 'python_image=0\nremove_requested=0\nidentity_override=0')
        wrapper = wrapper.replace('  case "$arg" in\n', '  case "$arg" in\n    --rm) remove_requested=1; n=$((n-1)); continue ;;\n    --name|--name=*|--cidfile|--cidfile=*|--label|--label=*|-l|-l=*) identity_override=1 ;;\n', 1)
        wrapper = wrapper.replace('if [ "$python_image" = 1 ]; then\n  [ "$adapter_count" = 1 ]',
            'if [ "$python_image" = 1 ]; then\n  [ "$identity_override" = 0 ] || { echo "unsupported identity option" >&2; exit 2; }\n  set -- --name ' + inner_name + ' --cidfile /run/user/1000/inner-python.cid --label voice.ci.run=' + owner.run_id + ' "$@"\n  [ "$adapter_count" = 1 ]')
        wrapper = wrapper.replace('fi\nset -- run --pull=never --network none "$@"',
            'else\n  if [ "$remove_requested" = 1 ]; then set -- --rm "$@"; fi\nfi\nset -- run --pull=never --network none "$@"', 1)
        import importlib.util
        policy_file = Path(__file__).with_name('local_ci_staging_network.py')
        chat._verify_file(policy_file,'946c461d30c556a289fa6a973809136a61a94e2c1eb6f29427ee37ae36538ed4')
        spec = importlib.util.spec_from_file_location('staging_network_policy',policy_file)
        policy = importlib.util.module_from_spec(spec); spec.loader.exec_module(policy)
        wrapper = policy.apply(wrapper)
        wrapper = wrapper.replace('/run/user/1000/tools/docker-cli', '/usr/local/libexec/voice-ci-docker')
        if wrapper.count('-H unix:///run/user/1000/docker.sock') != 2: raise RuntimeError('private socket wrapper differs')
        wrapper_sha = hashlib.sha256(wrapper.encode()).hexdigest()
        chat._verify_file(policy_file,'946c461d30c556a289fa6a973809136a61a94e2c1eb6f29427ee37ae36538ed4')
        with tempfile.TemporaryFile() as stream:
            stream.write(wrapper.encode()); stream.seek(0)
            execute('docker-wrapper', client, ['sh', '-ec', 'set -C; cat > /usr/local/bin/docker; chmod 755 /usr/local/bin/docker'], stdin=stream)
        wheel = ASSETS['wheel']
        wheel_sha = 'ba1cc08a7ccde2d2ec775841541641e4548226580ab850948cbfda66a1befcdc'
        chat._verify_file(wheel,wheel_sha)
        with wheel.open('rb') as stream:
            execute('wheel-transfer',client,['sh','-ec','set -C; cat > /tmp/staging-pyyaml.whl'],timeout=120,stdin=stream)
        chat._verify_file(wheel,wheel_sha)
        execute('wheel-hash',client,['sha256sum','/tmp/staging-pyyaml.whl'])
        if (BASE/'wheel-hash.stdout').read_text().split()[0] != wheel_sha: raise RuntimeError('wheel transfer differs')
        execute('wheel-name',client,['mv','/tmp/staging-pyyaml.whl','/tmp/pyyaml-6.0.3-cp312-cp312-manylinux2014_x86_64.manylinux_2_17_x86_64.manylinux_2_28_x86_64.whl'])
        execute('wheel-install',client,['python3','-m','pip','install','--no-index','--disable-pip-version-check','/tmp/pyyaml-6.0.3-cp312-cp312-manylinux2014_x86_64.manylinux_2_17_x86_64.manylinux_2_28_x86_64.whl'],timeout=120)
        execute('client-user', client, ['sh', '-ec', 'useradd -u 1000 -M -d /run/user/1000 voiceci; printf "voiceci ALL=(root) NOPASSWD: ALL\\n" > /etc/sudoers.d/voiceci; chmod 440 /etc/sudoers.d/voiceci; chown -R 1000:1000 /workspace/repo; mkdir -m 700 /run/user/1000/fixture-root; chown 1000:1000 /run/user/1000/fixture-root; python3 -c "import yaml"'])
        cache = ASSETS['preservation_go_cache']
        cache_sha = 'dcdbd0ce8baf9dd15d6bb8e1cdf8e2b2428e529f7bcdabf94ab5318865343be4'
        chat._verify_file(cache, cache_sha)
        with tarfile.open(cache, 'r:') as tar:
            seen = set(); total = 0
            for member in tar:
                from pathlib import PurePosixPath
                path = PurePosixPath(member.name)
                if (len(seen) >= 5000 or str(path) in seen or path.is_absolute() or '\\' in member.name or ':' in member.name or
                    str(path) != member.name.rstrip('/') or not path.parts or path.parts[0] != 'download' or
                    any(part in ('', '.', '..') for part in path.parts) or not (member.isdir() or member.isfile()) or
                    member.size < 0 or member.size > 128 * 1024**2 or member.name.endswith(('.ziphash', '.lock'))):
                    raise RuntimeError('unsafe Go cache')
                seen.add(str(path)); total += member.size
                if total > 1024**3: raise RuntimeError('Go cache too large')
        with cache.open('rb') as stream:
            execute('go-cache-transfer', client, ['sh','-ec','set -C; cat > /run/user/1000/go-cache.tar'], timeout=300, stdin=stream)
        chat._verify_file(cache, cache_sha)
        execute('go-cache-hash', client, ['sha256sum','/run/user/1000/go-cache.tar'])
        if (BASE/'go-cache-hash.stdout').read_text().split()[0] != cache_sha: raise RuntimeError('Go cache transfer differs')
        execute('go-cache-extract', client, ['sh','-ec','mkdir /run/user/1000/go-proxy; tar --no-same-owner -xf /run/user/1000/go-cache.tar -C /run/user/1000/go-proxy'])
        proxy = ASSETS['staging_go_proxy']
        proxy_sha = '30e81706b31b16c1c4252d0874a2fc38ab1885c8677d76210ad8a4c6a1ca5334'
        chat._verify_file(proxy,proxy_sha)
        with proxy.open('rb') as stream:
            execute('staging-proxy-transfer',client,['sh','-ec','set -C; cat > /run/user/1000/staging-proxy.tar'],timeout=300,stdin=stream)
        chat._verify_file(proxy,proxy_sha)
        execute('staging-proxy-hash',client,['sha256sum','/run/user/1000/staging-proxy.tar'])
        if (BASE/'staging-proxy-hash.stdout').read_text().split()[0] != proxy_sha: raise RuntimeError('proxy transfer differs')
        execute('staging-proxy-extract',client,['sh','-ec','mkdir /run/user/1000/staging-proxy /run/user/1000/tmp /run/user/1000/gocache; tar --no-same-owner -xf /run/user/1000/staging-proxy.tar -C /run/user/1000/staging-proxy; chown -R 1000:1000 /run/user/1000/tmp /run/user/1000/gocache'])
        inventory = matrix_inventory.build(ROOT)
        recipes = dispatch.inventory(inventory['shards'][SHARD], shard=SHARD)
        wrappers = dispatch.wrappers(recipes, shard=SHARD)
        if SHARD == 'policy':
            expected_skips = dispatch.policy_expected_skips(ROOT, inventory['shards']['policy'])
            skip_logger = dispatch.policy_skip_logger()
            skip_logger_sha = hashlib.sha256(skip_logger).hexdigest()
            with tempfile.TemporaryFile() as stream:
                stream.write(skip_logger); stream.seek(0)
                execute('policy-skip-logger', client, ['sh','-ec','set -C; cat > /run/user/1000/adapter-root/sitecustomize.py; chmod 444 /run/user/1000/adapter-root/sitecustomize.py'], stdin=stream)
            execute('policy-skip-init', client, ['sh','-ec','set -C; : > /run/user/1000/policy-skips; chown 1000:1000 /run/user/1000/policy-skips'])
        for kind,name in (('bash','bash-guard'),('python','python3')):
            with tempfile.TemporaryFile() as stream:
                stream.write(wrappers[kind]); stream.seek(0)
                execute('dispatch-'+kind,client,['sh','-ec','set -C; cat > /run/user/1000/adapter-root/'+name+'; chmod 555 /run/user/1000/adapter-root/'+name],stdin=stream)
        execute('recipe-events-init',client,['sh','-ec','set -C; : > /run/user/1000/recipe-events; chown 1000:1000 /run/user/1000/recipe-events'])
        execute('seal-adapter-parent', client, ['sh','-ec','chown 0:0 /run/user/1000; chmod 1777 /run/user/1000; chown 0:0 /run/user/1000/adapter-root; chmod 555 /run/user/1000/adapter-root'])
        execute('runtime-mode',client,['stat','-c','%u:%a','/run/user/1000'])
        if (BASE/'runtime-mode.stdout').read_text().strip() != '0:1777': raise RuntimeError('sticky parent differs')
        execute('test-uid',client,['setpriv','--reuid=1000','--regid=1000','--clear-groups','id','-u'])
        if (BASE/'test-uid.stdout').read_text().strip() != '1000': raise RuntimeError('unexpected test uid')
        denied_rename = execute('adapter-rename-denied',client,['setpriv','--reuid=1000','--regid=1000','--clear-groups','env','LC_ALL=C','mv','/run/user/1000/adapter-root','/run/user/1000/replaced'],allow_failure=True)
        denied_write = execute('adapter-write-denied',client,['setpriv','--reuid=1000','--regid=1000','--clear-groups','env','LC_ALL=C','sh','-ec','printf replacement > /run/user/1000/adapter-root/offline-runner.sh'],allow_failure=True)
        if denied_rename == 0 or denied_write == 0: raise RuntimeError('adapter replacement permitted')
        for name in ('adapter-rename-denied','adapter-write-denied'):
            message = (BASE/(name+'.stderr')).read_text().lower()
            if 'permission denied' not in message and 'operation not permitted' not in message: raise RuntimeError('denial was not a permission check')
        execute('adapter-before', client, ['sha256sum','/run/user/1000/adapter-root/offline-runner.sh'])
        if (BASE/'adapter-before.stdout').read_text().split()[0] != adapter_sha: raise RuntimeError('adapter file differs before execution')
        if SHARD == 'runtime':
            execute('story-adapter-before', client, ['sha256sum','/run/user/1000/adapter-root/story-media.sh'])
            if (BASE/'story-adapter-before.stdout').read_text().split()[0] != story_sha:
                raise RuntimeError('runtime adapter differs before execution')
            execute('runtime-mirror-writable', client, ['setpriv','--reuid=1000','--regid=1000','--clear-groups',
                'find','/run/user/1000/source-mirror','-writable','-print','-quit'], timeout=300)
            if (BASE/'runtime-mirror-writable.stdout').read_bytes().strip():
                raise RuntimeError('runtime source mirror writable by test user')
        if SHARD == 'policy':
            execute('policy-logger-before', client, ['sha256sum','/run/user/1000/adapter-root/sitecustomize.py'])
            if (BASE/'policy-logger-before.stdout').read_text().split()[0] != skip_logger_sha:
                raise RuntimeError('policy skip logger differs')
        verify_peers()
        common = ['setpriv','--reuid=1000','--regid=1000','--clear-groups','env',
            'HOME=/run/user/1000','USER=voiceci','GITHUB_ACTIONS=true','PYTHONDONTWRITEBYTECODE=1','VOICE_CONTRACT_PROFILE=current',
            'VOICE_CI_EVENT_NAME=pull_request','VOICE_CI_PR_BASE_SHA=' + SOURCE_BASE + '',
            'VOICE_R22_BASE_SHA=' + SOURCE_BASE + '',
            'TMPDIR=/run/user/1000/tmp','PATH=/run/user/1000/adapter-root:/usr/local/bin:/run/user/1000/tools:/usr/bin:/bin',
            'GOPROXY=file:///run/user/1000/staging-proxy/download','GOSUMDB=off','GOTOOLCHAIN=local',
            'GOFLAGS=-p=1 -mod=readonly','GOMAXPROCS=1','CGO_ENABLED=0','GOCACHE=/run/user/1000/gocache']
        if SHARD == 'runtime': common.append('DOCKER_BUILDKIT=0')
        if SHARD == 'policy':
            common.extend(('PYTHONPATH=/run/user/1000/adapter-root', 'VOICE_CI_SKIP_LOG=/run/user/1000/policy-skips'))
        execute('resolve-base',client,common+['bash','/workspace/repo/scripts/ci/resolve-voice-r22-base.sh'])
        if (BASE/'resolve-base.stdout').read_text().strip() != SOURCE_BASE: raise RuntimeError('hosted event base differs')
        execute('verify-base',client,common+['git','-C','/workspace/repo','cat-file','-e','' + SOURCE_BASE + '^{commit}'])
        if SHARD == 'policy':
            execute('user-kubectl-check', client, common + ['sh','-ec','[ "$(command -v kubectl)" = /usr/local/bin/kubectl ]; [ ! -w /usr/local/bin/kubectl ]; sha256sum /usr/local/bin/kubectl; kubectl version --client'])
            if (BASE/'user-kubectl-check.stdout').read_text().splitlines()[0] != KUBECTL_HASH + '  /usr/local/bin/kubectl': raise RuntimeError('unprivileged kubectl differs')
        execute('user-tool-check', client, common + ['sh','-ec','command -v go; go version; [ "$(command -v node)" = /usr/local/bin/node ]; node --version; [ "$(command -v jq)" = /usr/local/bin/jq ]; jq --version; [ "$(command -v docker)" = /usr/local/bin/docker ]; docker --version; [ ! -w /usr/local/bin ]; [ ! -w /usr/local/libexec ]; sha256sum /usr/local/bin/node /usr/local/bin/jq /usr/local/bin/docker /usr/local/libexec/voice-ci-docker; python3 --version; git --version; make --version | head -1; for x in bash openssl curl wget base64 tar timeout sha256sum awk sed; do command -v "$x"; done'])
        tool_output = (BASE/'user-tool-check.stdout').read_text()
        if SHARD == 'policy': tool_output += (BASE/'user-kubectl-check.stdout').read_text()
        expected_tools = [(node_hashes[0].split()[0],'/usr/local/bin/node'),(JQ_HASH,'/usr/local/bin/jq'),(wrapper_sha,'/usr/local/bin/docker'),(docker_cli_sha,'/usr/local/libexec/voice-ci-docker')]
        if SHARD == 'policy': expected_tools.append((KUBECTL_HASH, '/usr/local/bin/kubectl'))
        if 'go version go1.26.10 ' not in tool_output or not any(line.startswith('v24.') for line in tool_output.splitlines()) or any(digest + '  ' + path not in tool_output.splitlines() for digest,path in expected_tools): raise RuntimeError('unprivileged tools differ')
        hosted_policy_tests = 0
        if SHARD == 'policy':
            execute('hosted-policy', client, common + ['env','VOICE_CI_RECIPE_INDEX=69','bash','-ec',hosted_policy_script()], timeout=600)
            if any((BASE/('hosted-policy.' + suffix)).stat().st_size > 16 * 1024 * 1024 for suffix in ('stdout', 'stderr')):
                raise RuntimeError('hosted policy output exceeded limit')
            hosted_output = (BASE/'hosted-policy.stdout').read_bytes() + (BASE/'hosted-policy.stderr').read_bytes()
            dispatch.validate_policy_recipe(0, hosted_output, set(), {})
            hosted_policy_tests = int(re.search(rb'(?m)^Ran ([0-9]+) tests? in ', hosted_output).group(1))
        execute('make-dry-run',client,common+['make','-C','/workspace/repo','-n','BASH=/run/user/1000/adapter-root/bash-guard','ci-script-tests-' + SHARD])
        lines = (BASE/'make-dry-run.stdout').read_text().splitlines()
        lines = [line for line in lines if not line.startswith('make: Entering directory') and not line.startswith('make: Leaving directory')]
        expected = [dispatch.command(recipe) for recipe in recipes]
        if [shlex.split(line) for line in lines] != expected: raise RuntimeError('expanded Make command inventory differs')
        command = common + ['bash','-ec','set -o pipefail; cd /workspace/repo; make BASH=/run/user/1000/adapter-root/bash-guard ci-script-tests-' + SHARD + ' 2>&1 | head -c 33554433']
        code = execute('preservation', client, command, timeout=5400, allow_failure=True)
        execute('tools-after-make', client, ['sha256sum'] + [path for _,path in expected_tools])
        if (BASE/'tools-after-make.stdout').read_text().splitlines() != [digest + '  ' + path for digest,path in expected_tools]: raise RuntimeError('tools changed during Make')
        execute('recipe-events',client,['cat','/run/user/1000/recipe-events'])
        cid_status = execute('inner-cid', client, ['cat','/run/user/1000/inner-python.cid'], allow_failure=True)
        if cid_status == 0:
            cid = (BASE/'inner-cid.stdout').read_text().strip()
            if not re.fullmatch(r'[0-9a-f]{64}',cid): raise RuntimeError('inner identity invalid')
            execute('inner-inspect', client, docker + ['inspect',cid])
            info = json.loads((BASE/'inner-inspect.stdout').read_text())
            if (not isinstance(info,list) or len(info) != 1 or info[0].get('Id') != cid or
                info[0].get('Name') != '/' + inner_name or info[0].get('Image') != 'sha256:979e5a8a56846771598d47ebf1f8a75b46155104e168a0b237c0783c1dedff8a' or
                (info[0].get('Config',{}).get('Labels') or {}).get('voice.ci.run') != owner.run_id or
                info[0].get('HostConfig',{}).get('NetworkMode') != 'none' or
                info[0].get('State',{}).get('Running') is not False):
                raise RuntimeError('inner ownership or terminal state differs')
            (BASE/'inner-state.json').write_text(json.dumps({'id':cid,'name':inner_name,
                'exit_code':info[0]['State'].get('ExitCode'),'oom_killed':info[0]['State'].get('OOMKilled'),
                'memory_limit':info[0]['HostConfig'].get('Memory')},indent=2))
            execute('inner-remove',client,docker+['rm','-f','-v',cid],timeout=120)
        raw_events = (BASE/'recipe-events.stdout').read_text().splitlines()
        started = []
        for event in raw_events:
            parts = event.split('\t')
            if len(parts) == 2 and parts[1] == 'start':
                if not parts[0].isdigit() or not 0 <= int(parts[0]) < len(recipes): raise RuntimeError('recipe event invalid')
                index = int(parts[0])
                if index in started: raise RuntimeError('duplicate recipe start')
                started.append(index)
        for index in started:
            path = '/run/user/1000/recipe-' + str(index) + '.log'
            execute('recipe-log-'+str(index),client,['sh','-ec','[ "$(wc -c < "$1")" -le 16777216 ]; cat "$1"','sh',path])
        if code != 0: raise RuntimeError('preservation failed: ' + str(code))
        if SHARD == 'staging' and cid_status != 0: raise RuntimeError('successful run lacks inner container identity')
        if SHARD in ('proofs', 'runtime', 'policy') and cid_status == 0: raise RuntimeError('unexpected preservation container')
        output = (BASE / 'preservation.stdout').read_bytes()
        dispatch.validate_make_log(output, shard=SHARD)
        count = dispatch.validate_events((BASE/'recipe-events.stdout').read_bytes(),recipes)
        skip_events = {}
        if SHARD == 'policy':
            execute('policy-skips', client, ['sh','-ec','[ "$(wc -c < /run/user/1000/policy-skips)" -le 1048576 ]; cat /run/user/1000/policy-skips'])
            skip_events = dispatch.parse_policy_skip_events((BASE/'policy-skips.stdout').read_bytes())
            if any(index not in expected_skips for index in skip_events):
                raise RuntimeError('unexpected policy skip recipe')
        for index in range(len(recipes)):
            data = (BASE/('recipe-log-'+str(index)+'.stdout')).read_bytes()
            if SHARD == 'policy': dispatch.validate_policy_recipe(index, data, skip_events.get(index, set()), expected_skips)
            else: dispatch.validate_recipe(index,data,shard=SHARD)
        tests = 0
        if SHARD == 'staging':
            for marker in (b'PASS: hosted JWT resolver + TLS leaf + exact ACL proof', b'PASS: canonical hosted JWT/TLS bootstrap and least-privilege ACL proof', b'NATS_ROLLOUT_PRESERVATION=PASS valid-receipt-paused-apply-and-second-use-veto-all-modes'):
                if marker not in output.splitlines(): raise RuntimeError('required staging marker missing')
            execute('hosted-go-list',client,common+['bash','-ec','cd /workspace/repo/scripts/staging/nats-leaf-routing-test; go list ./...'],timeout=600)
            packages = (BASE/'hosted-go-list.stdout').read_text().splitlines()
            execute('hosted-go',client,common+['bash','-ec',hosted_go_script('/workspace/repo/scripts/staging/nats-leaf-routing-test')],timeout=1200)
            tests = validate_hosted_go_logs(BASE, packages, dispatch)
        execute('tools-after-hosted', client, ['sha256sum'] + [path for _,path in expected_tools])
        if (BASE/'tools-after-hosted.stdout').read_text().splitlines() != [digest + '  ' + path for digest,path in expected_tools]: raise RuntimeError('tools changed during hosted steps')
        (BASE / 'suite-counts.json').write_text(json.dumps({'shard':SHARD,'recipes':count,'hosted_go_tests':tests,
            'hosted_policy_tests':hosted_policy_tests},indent=2))
        chat._verify_file(dispatch_path,dispatch_sha)
        execute('adapter-after', client, ['sha256sum','/run/user/1000/adapter-root/offline-runner.sh'])
        if (BASE/'adapter-after.stdout').read_text().split()[0] != adapter_sha: raise RuntimeError('adapter file differs after execution')
        if SHARD == 'runtime':
            execute('story-adapter-after', client, ['sha256sum','/run/user/1000/adapter-root/story-media.sh'])
            if (BASE/'story-adapter-after.stdout').read_text().split()[0] != story_sha:
                raise RuntimeError('runtime adapter differs after execution')
            execute('runtime-source-compare-after', client, ['tar','-df','/run/user/1000/source.tar','-C','/run/user/1000/source-mirror'], timeout=300)
        if SHARD == 'policy':
            execute('policy-logger-after', client, ['sha256sum','/run/user/1000/adapter-root/sitecustomize.py'])
            if (BASE/'policy-logger-after.stdout').read_text().split()[0] != skip_logger_sha:
                raise RuntimeError('policy skip logger changed')
        execute('containers-after', client, docker + ['ps', '-aq', '--no-trunc'])
        if (BASE / 'containers-after.stdout').read_bytes().strip():
            raise RuntimeError('inner container remains')
        execute('restore-repo-owner', client, ['chown', '-R', '0:0', '/workspace/repo'], timeout=180)
        check_git('git-after')
        verify_host(); verify_peers()
        print('STAGING_SHARD_DIAGNOSTIC_OK', flush=True)
    except BaseException as exc:
        if any(row['state'] in ('creating','starting','executing','stopping','removing') for row in owner.state['resources']):
            scheduled.abandon()
            exc.add_note('Uncertain operation: journal and slot retained; timeout postmortem is not available automatically.')
            raise
        failure = exc
    chat._finish(scheduled, failure)
    print('OWNED_RESOURCES_CLEANED', flush=True)

if __name__ == '__main__':
    configure(request_contract.parse(sys.stdin.buffer.read(1024 * 1024 + 1)))
    main()
