"""Run full Auth make target in a private daemon; result is always incomplete.

The caller must authenticate inputs and maintain exclusive daemon/socket and
workspace custody. Missing images, tools or required reports fail closed.
"""
import argparse
import ctypes
import json
import os
from pathlib import Path
import re
import tempfile
import sys

from local_docker_owned import DockerCLI
from local_docker_scheduled import ScheduledRun
import local_auth_full
import local_auth_private
import local_chat_orchestration as common
import local_rootless_profile as rootless
import local_source_snapshot as snapshot
import local_nats_hosted_diagnostic as bootstrap
import local_process_supervision as supervision

IMAGE_TAGS = ('postgres:16-alpine', 'redis:7-alpine', 'redis:7.4-alpine',
              'nats:2.11-alpine', 'migrate/migrate:v4.18.1')
MAKE_IMAGE = 'sha256:a9daab691df2dfe118e5c6d3d6bd8fd6767ea2c9bf8fa3c9356161055dd7b97d'
STAGES = frozenset({'source-copy', 'acquire-slot', 'resources', 'make-provisioning', 'socket-init',
                    'daemon-create', 'daemon-start', 'daemon-readiness', 'image-load', 'auth-suite',
                    'tool-verification', 'cleanup-verification', 'complete'})

CGROUP_KEYS = ('pids_current', 'pids_max', 'pids_events_max', 'memory_current', 'memory_max',
               'memory_events_max', 'memory_events_oom', 'memory_events_oom_kill')


def parse_cgroup_sample(raw):
    if not isinstance(raw, bytes) or len(raw) > 512 or not re.fullmatch(rb'(?:[0-9]{1,20}\s+){8}', raw):
        raise ValueError('Auth cgroup counters unavailable')
    return dict(zip(CGROUP_KEYS, map(int, raw.split())))


def sample_cgroup(owner, daemon):
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        if owner.read_rootless_cgroup(daemon, out, err):
            raise ValueError('Auth cgroup counters unavailable')
        out.seek(0); raw = out.read(513)
    sample = parse_cgroup_sample(raw)
    if sample['pids_max'] != 256 or sample['memory_max'] != 512 * 1024 * 1024:
        raise ValueError('Auth cgroup counters unavailable')
    return sample


def save_cgroup_counters(output, before, after):
    events = ('pids_events_max', 'memory_events_max', 'memory_events_oom', 'memory_events_oom_kill')
    if any(after[key] < before[key] for key in events):
        raise ValueError('Auth cgroup counters unavailable')
    (output / 'cgroup-counters.json').write_text(json.dumps(
        {'schema': 'voice-local-auth-cgroup-v1', 'before': before, 'after': after}, sort_keys=True) + '\n',
        encoding='ascii')
    if any(after[key] > before[key] for key in events):
        raise ValueError('Auth cgroup resource limit hit')


def record_stage(output, stage):
    """Atomically record only the last attempted fixed stage, never exception text."""
    if stage not in STAGES:
        raise ValueError('invalid Auth diagnostic stage')
    raw = (json.dumps({'schema': 'voice-local-auth-stage-v1', 'stage': stage}, sort_keys=True) + '\n').encode('ascii')
    with tempfile.NamedTemporaryFile(mode='wb', dir=output, prefix='.stage-', delete=False) as stream:
        temporary = Path(stream.name)
        stream.write(raw); stream.flush(); os.fsync(stream.fileno())
    try:
        os.replace(temporary, Path(output) / 'stage.json')
    finally:
        if temporary.exists():
            temporary.unlink()


def _verify_make(owner, client, expected):
    check = ('set -eu; test -f /workspace/tools/make; test ! -L /workspace/tools/make; '
             'for tool in bash mvn java curl tar sha256sum; do command -v "$tool" >/dev/null; done; '
             'ldd /workspace/tools/make > /workspace/make-ldd.txt; '
             '! grep -q "not found" /workspace/make-ldd.txt; '
             'sha256sum /workspace/tools/make; /workspace/tools/make --version | head -n 1')
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        if owner.execute(client, ['sh', '-ec', check], out, err, 60):
            raise ValueError('GNU Make provisioning or client compatibility check failed')
        out.seek(0); actual = out.read(4097)
    if actual != expected:
        raise ValueError('GNU Make copy identity or version differs in client')


def provision_make(owner, engine, network, workspace, client):
    """Copy and compare GNU Make from a pinned image inside the owned workspace."""
    if engine._call('image', 'inspect', MAKE_IMAGE, '--format', '{{.Id}} {{.Os}}/{{.Architecture}}') != MAKE_IMAGE + ' linux/amd64':
        raise ValueError('pinned GNU Make source image is unavailable or has the wrong platform')
    tools = owner.create_container(MAKE_IMAGE, network, {'/workspace': workspace}, [],
                                   command=['sleep', 'infinity'], clear_entrypoint=True)
    owner.start_container(tools)
    copy = ('set -eu; test -f /usr/bin/make; test ! -L /usr/bin/make; '
            'test ! -e /workspace/tools; mkdir /workspace/tools; '
            'cp /usr/bin/make /workspace/tools/make; chmod 755 /workspace/tools/make; '
            'cmp /usr/bin/make /workspace/tools/make; '
            'sha256sum /workspace/tools/make; /workspace/tools/make --version | head -n 1')
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        if owner.execute(tools, ['sh', '-ec', copy], out, err, 60):
            raise ValueError('GNU Make provisioning or client compatibility check failed')
        out.seek(0); expected = out.read(4097)
    match = re.fullmatch(rb'([0-9a-f]{64})  /workspace/tools/make\n(GNU Make [0-9][^\r\n]*)\n', expected)
    if not match:
        raise ValueError('GNU Make identity or version is invalid')
    _verify_make(owner, client, expected)
    return {'image_id': MAKE_IMAGE, 'sha256': match.group(1).decode(), 'version': match.group(2).decode('ascii')}


def validate_images(images):
    if not isinstance(images, dict) or set(images) != set(IMAGE_TAGS):
        raise ValueError('exact full Auth image inventory required')
    for tag in IMAGE_TAGS:
        item = images[tag]
        if (not isinstance(item, dict) or set(item) != {'archive', 'sha256', 'image_id'} or
                not isinstance(item['archive'], str) or not Path(item['archive']).is_absolute() or
                not isinstance(item['sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', item['sha256']) or
                not isinstance(item['image_id'], str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', item['image_id'])):
            raise ValueError('invalid full Auth image input')
    for item in images.values():
        common._verify_file(item['archive'], item['sha256'])


def load_images(owner, daemon, images, output):
    image_inputs, loaded = {}, {}
    for index, tag in enumerate(IMAGE_TAGS):
        item = images[tag]
        common._verify_file(item['archive'], item['sha256'])
        copied = owner.directory / ('image-' + str(index) + '.tar')
        with Path(item['archive']).open('rb') as source_file, copied.open('xb') as target:
            for block in iter(lambda: source_file.read(1024 * 1024), b''):
                target.write(block)
            target.flush(); os.fsync(target.fileno())
        common._verify_file(item['archive'], item['sha256'])
        common._verify_file(copied, item['sha256'])
        with (output / ('image-' + str(index) + '.stdout')).open('xb') as out, \
             (output / ('image-' + str(index) + '.stderr')).open('xb') as err:
            if item['image_id'] in loaded:
                owner.load_canonical_rootless_alias(daemon, copied, item['sha256'], item['image_id'],
                                                    loaded[item['image_id']], tag, out, err, 300)
            else:
                owner.load_canonical_rootless_image(daemon, copied, item['sha256'], item['image_id'], tag, out, err, 300)
                loaded[item['image_id']] = tag
        image_inputs[tag] = {'image_id': item['image_id'], 'archive_sha256': item['sha256']}
    return image_inputs


def _run_owned(root, head, output, helper, helper_digest, client_image_id, cache, cache_digest, images):
    _require_worker_job()
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('output must be new and outside source tree')
    if not isinstance(client_image_id, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', client_image_id):
        raise ValueError('invalid Auth client image identity')
    common._verify_file(helper, helper_digest)
    common._verify_file(cache, cache_digest)
    if local_auth_private.cache_digest(cache) != cache_digest:
        raise ValueError('Maven cache identity differs')
    validate_images(images)
    source = snapshot.capture(root, head)
    output.mkdir(parents=True)
    record_stage(output, 'source-copy')
    source_copy = snapshot.materialize(root, source, output / 'source')
    checkout = output / 'run'; checkout.mkdir()
    engine = DockerCLI()
    if engine._call('image', 'inspect', client_image_id, '--format', '{{.Id}}') != client_image_id:
        raise ValueError('Auth client image identity differs')
    record_stage(output, 'acquire-slot')
    scheduled = ScheduledRun.start_host(checkout, engine, 300)
    owner = scheduled.run
    failure, result = None, None
    try:
        record_stage(output, 'resources')
        network = owner.create_resource('network')
        mounts = {target: owner.create_resource('volume') for target in sorted(rootless.WORKSPACE_TARGETS)}
        client = owner.create_container(client_image_id, network,
            {'/socket': mounts['/run/user/1000'], '/workspace': mounts['/workspace']}, [],
            command=['sleep', 'infinity'], clear_entrypoint=True)
        owner.start_container(client)
        record_stage(output, 'socket-init')
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            if owner.execute(client, ['chown', '1000:1000', '/socket'], out, err, 30):
                raise ValueError('socket initialization failed')
        record_stage(output, 'daemon-create')
        daemon = owner.create_rootless_container(network, mounts,
            lambda request: common._create_daemon(helper, helper_digest, engine.rootless_creator_env(), request))
        record_stage(output, 'daemon-start')
        owner.start_container(daemon)
        # Readiness validates the shared profile and exact network on each pass.
        record_stage(output, 'daemon-readiness')
        local_auth_full.wait_daemon(owner, daemon, client)
        record_stage(output, 'image-load')
        image_inputs = load_images(owner, daemon, images, output)
        record_stage(output, 'auth-suite')
        make_tool = None
        def prepare_tools():
            nonlocal make_tool
            record_stage(output, 'make-provisioning')
            tool_network = owner.create_resource('network')
            if tool_network == network:
                raise ValueError('GNU Make tool network is not isolated from the daemon')
            make_tool = provision_make(owner, engine, tool_network, mounts['/workspace'], client)
            record_stage(output, 'auth-suite')
        before = sample_cgroup(owner, daemon)
        try:
            diagnostic = local_auth_full.run_full(owner, daemon, client, root, source, source_copy, cache,
                                                  output / 'full', prepare_tools=prepare_tools)
        finally:
            after = sample_cgroup(owner, daemon)
            save_cgroup_counters(output, before, after)
        record_stage(output, 'tool-verification')
        _verify_make(owner, client, (make_tool['sha256'] + '  /workspace/tools/make\n' +
                                     make_tool['version'] + '\n').encode('ascii'))
        if diagnostic['maven_cache_sha256'] != cache_digest:
            raise ValueError('Maven cache changed during diagnostic')
        result = {'schema': 'voice-local-auth-full-orchestration-v1', 'status': 'incomplete',
                  'diagnostic': diagnostic, 'slot': scheduled.slot,
                  'ownership_journal': str((owner.directory / 'ownership.json').relative_to(output)),
                  'inputs': {'creator_sha256': helper_digest, 'client_image_id': client_image_id,
                             'maven_cache_sha256': cache_digest, 'images': image_inputs,
                             'make_tool': make_tool}}
    except BaseException as error:
        failure = error
    common._finish(scheduled, failure)
    record_stage(output, 'cleanup-verification')
    if snapshot.capture(root, head) != source:
        raise ValueError('source changed during full Auth cleanup')
    (output / 'diagnostic.json').write_text(json.dumps(result, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    record_stage(output, 'complete')
    return result


def _worker_command():
    command = bootstrap._worker_command(Path(__file__))
    original = repr(sorted(bootstrap.PROJECT_MODULES))
    modules = bootstrap.PROJECT_MODULES | {'local_auth_full', 'local_auth_private', 'local_nats_hosted_diagnostic'}
    if command[5].count(original) != 1:
        raise ValueError('Auth worker import allowlist changed')
    command[5] = command[5].replace(original, repr(sorted(modules)), 1)
    return command


def failure_category(stderr):
    """Return only fixed labels, never worker output or test-controlled text."""
    if not isinstance(stderr, bytes) or len(stderr) > 1024 * 1024:
        return 'unclassified worker failure'
    known = {
        b'Auth source transfer failed': 'Auth source transfer failed',
        b'Auth source normalization failed': 'Auth source normalization failed',
        b'canonical Auth make target failed': 'canonical Auth target failed',
        b'GNU Make provisioning or client compatibility check failed': 'GNU Make provisioning or compatibility failed',
        b'GNU Make copy identity or version differs in client': 'GNU Make identity check failed',
        b'pinned GNU Make source image is unavailable or has the wrong platform': 'GNU Make source image unavailable',
        b'private Docker socket did not become ready': 'private Docker socket unavailable',
    }
    matches = [label for message, label in known.items()
               if re.search(rb'(?m)^ValueError: ' + re.escape(message) + rb'\r?$', stderr)]
    return matches[0] if len(matches) == 1 else 'unclassified worker failure'


def run(root, head, output, helper, helper_digest, client_image_id, cache, cache_digest, images):
    """Run the owned worker through the Windows process broker, never inline."""
    request = {'root': str(root), 'head': head, 'output': str(output), 'helper': str(helper),
               'helper_digest': helper_digest, 'client_image_id': client_image_id, 'cache': str(cache),
               'cache_digest': cache_digest, 'images': images}
    raw = json.dumps(request, allow_nan=False, separators=(',', ':')).encode('utf-8')
    if len(raw) > 1024 * 1024:
        raise ValueError('Auth worker request exceeds limit')
    try:
        result = supervision.run_diagnostic(_worker_command(), timeout=10800, input=raw, capture_output=True, check=False)
    except BaseException as error:
        error.add_note('Auth worker cleanup may be uncertain; inspect its ownership journal and slot.')
        raise
    if result.returncode != 0:
        raise ValueError('full Auth worker failed (' + failure_category(result.stderr) +
                         '); inspect its journal before recovery')
    try:
        report = json.loads(result.stdout, object_pairs_hook=_object)
        saved = json.loads((Path(output) / 'diagnostic.json').read_text(encoding='utf-8'), object_pairs_hook=_object)
        if report != saved or report.get('status') != 'incomplete':
            raise ValueError()
    except (OSError, ValueError, TypeError, UnicodeError):
        raise ValueError('full Auth worker returned invalid diagnostic') from None
    return report


def _object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate image manifest field')
        result[key] = value
    return result


def read_images(path, digest):
    common._verify_file(path, digest)
    images = json.loads(Path(path).read_text(encoding='utf-8'), object_pairs_hook=_object)
    common._verify_file(path, digest)
    validate_images(images)
    return images


def _require_worker_job():
    """Refuse an unfenced worker; this is not proof of exclusive job custody."""
    if os.name != 'nt':
        raise ValueError('Auth worker requires Windows job containment')
    from ctypes import wintypes
    class Basic(ctypes.Structure):
        _fields_ = [('PerProcessUserTimeLimit', ctypes.c_longlong), ('PerJobUserTimeLimit', ctypes.c_longlong),
                    ('LimitFlags', wintypes.DWORD), ('MinimumWorkingSetSize', ctypes.c_size_t),
                    ('MaximumWorkingSetSize', ctypes.c_size_t), ('ActiveProcessLimit', wintypes.DWORD),
                    ('Affinity', ctypes.c_size_t), ('PriorityClass', wintypes.DWORD), ('SchedulingClass', wintypes.DWORD)]
    class IO(ctypes.Structure):
        _fields_ = [(key, ctypes.c_ulonglong) for key in ('ReadOperationCount', 'WriteOperationCount',
                    'OtherOperationCount', 'ReadTransferCount', 'WriteTransferCount', 'OtherTransferCount')]
    class Extended(ctypes.Structure):
        _fields_ = [('BasicLimitInformation', Basic), ('IoInfo', IO), ('ProcessMemoryLimit', ctypes.c_size_t),
                    ('JobMemoryLimit', ctypes.c_size_t), ('PeakProcessMemoryUsed', ctypes.c_size_t),
                    ('PeakJobMemoryUsed', ctypes.c_size_t)]
    kernel = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel.GetCurrentProcess.restype = wintypes.HANDLE
    kernel.IsProcessInJob.argtypes = [wintypes.HANDLE, wintypes.HANDLE, ctypes.POINTER(wintypes.BOOL)]
    kernel.QueryInformationJobObject.argtypes = [wintypes.HANDLE, ctypes.c_int, ctypes.c_void_p, wintypes.DWORD, ctypes.c_void_p]
    in_job, limits = wintypes.BOOL(), Extended()
    if (not kernel.IsProcessInJob(kernel.GetCurrentProcess(), None, ctypes.byref(in_job)) or not in_job.value or
            not kernel.QueryInformationJobObject(None, 9, ctypes.byref(limits), ctypes.sizeof(limits), None) or
            not limits.BasicLimitInformation.LimitFlags & 0x2000):
        raise ValueError('Auth worker requires kill-on-close job containment')


def main():
    if sys.argv[1:] == ['--worker']:
        _require_worker_job()
        raw = sys.stdin.buffer.read(1024 * 1024 + 1)
        if len(raw) > 1024 * 1024:
            raise ValueError('Auth worker request exceeds limit')
        request = json.loads(raw, object_pairs_hook=_object)
        if not isinstance(request, dict) or set(request) != {'root', 'head', 'output', 'helper', 'helper_digest',
                'client_image_id', 'cache', 'cache_digest', 'images'}:
            raise ValueError('invalid Auth worker request')
        print(json.dumps(_run_owned(**request), sort_keys=True))
        return 0
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('head', 'output', 'helper', 'helper-sha256', 'client-image-id',
                 'maven-cache', 'maven-cache-sha256', 'images', 'images-sha256'):
        parser.add_argument('--' + name, required=True)
    args = parser.parse_args()
    images = read_images(args.images, args.images_sha256)
    result = run(Path(__file__).resolve().parents[2], args.head, args.output, args.helper,
                 args.helper_sha256, args.client_image_id, args.maven_cache, args.maven_cache_sha256, images)
    print(json.dumps(result, sort_keys=True))
    return 2


if __name__ == '__main__':
    raise SystemExit(main())
