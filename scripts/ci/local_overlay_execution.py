"""Run MatchSquad overlay tests in an owned Linux client; never issue PASS.

Image provenance and exclusive Docker custody are caller assumptions. The
result is a scoped diagnostic, not a complete local preflight receipt.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import sys

import affected_check_plan
import local_auth_full_orchestration as auth_broker
import local_chat_full_integration as chat
import local_chat_orchestration as orchestration
from local_docker_owned import DockerCLI
from local_docker_scheduled import ScheduledRun
import local_go_execution
import local_nats_hosted_diagnostic as bootstrap
import local_process_supervision as supervision
import local_source_snapshot


PYTHON_IMAGE = 'sha256:61d53b56a648da09aa966c5cc46a7e751aea2b38d01f33b04456a9f289685be9'
MODULES = ('scripts.dev.match_squad_overlay_test', 'scripts.dev.match_squad_runtime_overlay_test')
DISCOVER = ('import unittest; s=unittest.defaultTestLoader.loadTestsFromNames(' + repr(list(MODULES)) +
            '); print("VOICE_OVERLAY_DISCOVERED=" + str(s.countTestCases()))')
OUTPUT_LIMIT = 1024 * 1024


def _windows():
    return os.name == 'nt'


def validate_discovery(raw):
    text = raw.decode('utf-8', errors='strict').replace('\r\n', '\n')
    rows = re.findall(r'(?m)^VOICE_OVERLAY_DISCOVERED=([0-9]+)$', text)
    if len(rows) != 1 or int(rows[0]) < 1:
        raise ValueError('invalid overlay test discovery')
    return int(rows[0])


def validate_summary(raw, code, count):
    text = raw.decode('utf-8', errors='strict').replace('\r\n', '\n')
    rows = re.findall(r'(?m)^Ran ([0-9]+) tests? in [0-9.]+s$', text)
    if (code != 0 or type(count) is not int or count < 1 or len(rows) != 1 or int(rows[0]) != count or
            not re.search(r'(?m)^OK\s*$', text) or re.search(r'(?im)^\s*(?:skip|skipped|FAILED)\b', text) or
            re.search(r'(?m)^OK\s*\(', text)):
        raise ValueError('overlay tests failed, skipped or differ from discovery')
    return count


def _command(owner, client, output, label, command, timeout):
    prefix = output / label
    script = chat.bounded_script(command, event_limit=OUTPUT_LIMIT, stderr_limit=OUTPUT_LIMIT)
    with prefix.with_suffix('.stdout').open('xb') as stdout, prefix.with_suffix('.stderr').open('xb') as stderr:
        code = owner.execute(client, ['bash', '-c', script], stdout, stderr, timeout)
    out, err = prefix.with_suffix('.stdout'), prefix.with_suffix('.stderr')
    if out.stat().st_size > OUTPUT_LIMIT or err.stat().st_size > OUTPUT_LIMIT:
        raise ValueError('overlay command output limit exceeded')
    return code, out.read_bytes(), err.read_bytes()


def run(root, plan, output, image_id, *, base, head_ref):
    auth_broker._require_worker_job()
    if not _windows():
        raise ValueError('Windows supervised overlay diagnostic required')
    root, output = Path(root).resolve(), Path(output).resolve()
    source = plan.get('source') if isinstance(plan, dict) else None
    if not isinstance(source, dict) or 'match-squad-overlay' not in plan.get('checks', {}):
        raise ValueError('planner-selected overlay check required')
    if (not isinstance(base, str) or not re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', base) or
            not isinstance(head_ref, str) or not head_ref or
            affected_check_plan.build_local_plan(root, base, source['head'], head_ref) != plan):
        raise ValueError('affected overlay plan differs')
    if image_id != PYTHON_IMAGE:
        raise ValueError('pinned Python image identity required')
    if output.exists() or output == root or root in output.parents:
        raise ValueError('overlay output must be new and outside source')
    if local_source_snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before overlay diagnostic')
    output.mkdir(parents=True)
    snapshot = local_source_snapshot.materialize(root, source, output / 'source')
    engine = DockerCLI()
    if engine._call('image', 'inspect', image_id, '--format', '{{.Os}}/{{.Architecture}}') != 'linux/amd64':
        raise ValueError('pinned Python image platform differs')
    checkout = output / 'run'
    checkout.mkdir()
    scheduled = ScheduledRun.start_host(checkout, engine, 300)
    failure, result = None, None
    try:
        owner = scheduled.run
        network = owner.create_resource('network')
        volume = owner.create_resource('volume')
        loader = owner.create_container(image_id, network, {'/workspace': volume}, [], command=['sleep', 'infinity'],
                                        clear_entrypoint=True)
        owner.start_container(loader)
        source_digest = local_go_execution.transfer_source(owner, loader, root, source, snapshot, output / 'source.tar')
        client = owner.create_phase0_container(image_id, volume)
        owner.start_container(client)
        prefix = ('cd /workspace/repo/source && exec env -i PATH=/usr/local/bin:/usr/bin:/bin '
                  'HOME=/tmp PYTHONDONTWRITEBYTECODE=1 python3 -B ')
        code, stdout, stderr = _command(owner, client, output, 'version', prefix + '--version', 30)
        version = stdout.decode('utf-8', errors='strict').strip()
        if code or not re.fullmatch(r'Python 3\.12\.[0-9]+', version):
            raise ValueError('pinned Python image requires Python 3.12')
        code, stdout, stderr = _command(owner, client, output, 'discovery', prefix + '-c ' + shlex.quote(DISCOVER), 120)
        if code:
            raise ValueError('overlay discovery failed')
        count = validate_discovery(stdout)
        command = prefix + '-m unittest ' + ' '.join(MODULES)
        code, stdout, stderr = _command(owner, client, output, 'tests', command, 600)
        passed = validate_summary(stderr, code, count)
        local_go_execution._verify_workspace(owner, loader, root, source, source_digest)
        if (local_source_snapshot.capture(root, source['head']) != source or
                engine._call('image', 'inspect', image_id, '--format', '{{.Os}}/{{.Architecture}}') != 'linux/amd64'):
            raise ValueError('source or Python image changed during overlay diagnostic')
        result = {'schema': 'voice-local-overlay-diagnostic-v1', 'status': 'incomplete', 'source': source,
                  'image_id': image_id, 'version': version, 'command': command, 'source_archive_sha256': source_digest,
                  'modules': list(MODULES), 'tests': {'discovered': count, 'passed': passed, 'skipped': []},
                  'stdout_sha256': hashlib.sha256(stdout).hexdigest(), 'stderr_sha256': hashlib.sha256(stderr).hexdigest(),
                  'slot': scheduled.slot, 'ownership_journal': str((owner.directory / 'ownership.json').relative_to(output)),
                  'unbound_inputs': ['image provenance', 'exclusive daemon and socket custody',
                                     'interruption and crash recovery acceptance']}
    except BaseException as error:
        failure = error
    orchestration._finish(scheduled, failure)
    if local_source_snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during overlay cleanup')
    (output / 'diagnostic.json').write_text(json.dumps(result, sort_keys=True, indent=2) + '\n', encoding='utf-8')
    return result


def _worker_command():
    command = bootstrap._worker_command(Path(__file__))
    original = repr(sorted(bootstrap.PROJECT_MODULES))
    modules = bootstrap.PROJECT_MODULES | {'local_overlay_execution', 'local_auth_full_orchestration',
        'local_auth_full', 'local_auth_private', 'local_chat_full_integration', 'local_nats_hosted_diagnostic'}
    if command[5].count(original) != 1:
        raise ValueError('overlay worker import allowlist changed')
    command[5] = command[5].replace(original, repr(sorted(modules)), 1)
    return command


def run_supervised(root, plan, output, image_id, *, base, head_ref):
    request = {'root': str(root), 'plan': plan, 'output': str(output), 'image_id': image_id,
               'base': base, 'head_ref': head_ref}
    raw = json.dumps(request, allow_nan=False, separators=(',', ':')).encode('utf-8')
    if len(raw) > 1024 * 1024:
        raise ValueError('overlay worker request exceeds limit')
    try:
        completed = supervision.run_diagnostic(_worker_command(), timeout=7200, input=raw,
                                               capture_output=True, check=False)
    except BaseException as error:
        error.add_note('Overlay cleanup may be uncertain; inspect its ownership journal and slot.')
        raise
    if completed.returncode:
        raise ValueError('overlay worker failed; inspect its journal before recovery')
    try:
        result = json.loads(completed.stdout, object_pairs_hook=chat._unique)
        saved = json.loads((Path(output) / 'diagnostic.json').read_text(encoding='utf-8'), object_pairs_hook=chat._unique)
        if result != saved or result.get('source') != plan['source'] or result.get('status') != 'incomplete':
            raise ValueError()
    except (OSError, ValueError, TypeError, KeyError, UnicodeError):
        raise ValueError('invalid overlay worker diagnostic') from None
    return result


def main():
    if sys.argv[1:] != ['--worker']:
        raise ValueError('overlay worker requires supervised invocation')
    auth_broker._require_worker_job()
    raw = sys.stdin.buffer.read(1024 * 1024 + 1)
    if len(raw) > 1024 * 1024:
        raise ValueError('overlay worker request exceeds limit')
    request = json.loads(raw, object_pairs_hook=chat._unique)
    if not isinstance(request, dict) or set(request) != {'root', 'plan', 'output', 'image_id', 'base', 'head_ref'}:
        raise ValueError('invalid overlay worker request')
    result = run(**request)
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
