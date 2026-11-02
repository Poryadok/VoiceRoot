"""Compose policy-shard Linux dispositions with exact Windows test evidence.

This module does not establish full CI-script job or whole-preflight success.
"""
import argparse
from pathlib import Path
import hashlib
import json
import os
import re
import stat
import sys

import local_ci_staging_dispatch as dispatch
import local_ci_matrix_inventory as matrix
import local_ci_staging_execution as staging
from local_docker_owned import DockerCLI, OwnedRun
import local_heavy_slots
import local_proto_execution
import local_source_snapshot as snapshot


def windows_test_names(root, rows):
    """Map source-pinned Linux Windows-only skips to exact unittest names."""
    dispositions = dispatch.policy_expected_skips(root, rows)
    names = []
    for index, tests in dispositions.items():
        path, _, count = dispatch.POLICY_SKIP_FILES[index]
        if len(tests) != count or not re.fullmatch(r'scripts/ci/[A-Za-z0-9_]+_test\.py', path):
            raise ValueError('Windows policy inventory differs')
        module = Path(path).stem
        for name, reason in tests:
            if not re.fullmatch(r'__main__\.[A-Za-z_]\w*\.[A-Za-z_]\w*', name) or not reason:
                raise ValueError('Windows policy test identity differs')
            names.append(module + name[len('__main__'):])
    if not names or len(names) != len(set(names)):
        raise ValueError('Windows policy test inventory is empty or duplicated')
    return sorted(names)


def windows_test_count(stderr, exit_code, expected):
    """Require the entire explicit unittest selection and no skips."""
    matches = re.findall(rb'(?m)^Ran ([0-9]+) tests? in [0-9.]+s\s*$', stderr)
    if (type(expected) is not int or expected < 1 or exit_code != 0 or len(matches) != 1 or
            int(matches[0]) != expected or not re.search(rb'(?m)^OK\s*$', stderr)):
        raise ValueError('Windows policy tests incomplete, failed or skipped')
    return expected


def _read(path, limit):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > limit:
        raise ValueError('unsafe or oversized policy evidence')
    data = path.read_bytes()
    if len(data) > limit:
        raise ValueError('oversized policy evidence')
    return data


def verify_owned_absence(owner):
    owner._check_daemon()
    if any(owner.engine.exists(row['kind'], row['id']) for row in owner.state['resources']):
        raise ValueError('policy owned resource remains')
    owner._check_daemon()


def verify_linux_evidence(root, output, source, base, report):
    """Revalidate each policy recipe, hosted tests and exact owned cleanup."""
    if (not isinstance(report, dict) or report.get('schema') != 'voice-local-ci-staging-v1' or
            report.get('status') != 'incomplete' or report.get('source') != source['head'] or
            report.get('base') != base or report.get('shard') != 'policy' or
            type(report.get('recipes')) is not int or report['recipes'] != 76 or
            type(report.get('hosted_policy_tests')) is not int or report['hosted_policy_tests'] < 1):
        raise ValueError('policy worker identity differs')
    rows = matrix.build(root)['shards']['policy']
    recipes = dispatch.inventory(rows, shard='policy')
    expected = dispatch.policy_expected_skips(root, rows)
    events = dispatch.parse_policy_skip_events(_read(output/'policy-skips.stdout', 1024*1024))
    if set(events) != set(expected) or any(events[index] != expected[index] for index in expected):
        raise ValueError('policy Linux skip inventory differs')
    dispatch.validate_events(_read(output/'recipe-events.stdout', 1024*1024), recipes)
    dispatch.validate_make_log(_read(output/'preservation.stdout', 32*1024*1024), shard='policy')
    for index in range(len(recipes)):
        dispatch.validate_policy_recipe(index, _read(output/('recipe-log-'+str(index)+'.stdout'), 16*1024*1024),
                                        events.get(index, set()), expected)
    hosted = _read(output/'hosted-policy.stdout', 16*1024*1024) + _read(output/'hosted-policy.stderr', 16*1024*1024)
    dispatch.validate_policy_recipe(0, hosted, set(), {})
    counts = re.findall(rb'(?m)^Ran ([0-9]+) tests? in ', hosted)
    if len(counts) != 1 or int(counts[0]) != report['hosted_policy_tests']:
        raise ValueError('hosted policy test count differs')
    identity = json.loads(_read(output/'identity.json', 1024*1024))
    if identity.get('source') != source['head'] or identity.get('journal') != report.get('journal') or identity.get('slot') != report.get('slot'):
        raise ValueError('policy ownership identity differs')
    staging._validate_journal(output.resolve(), identity, 'policy')
    owner = OwnedRun.load(Path(identity['journal']).parent, DockerCLI())
    try:
        verify_owned_absence(owner)
    finally:
        owner.close()
    slot = local_heavy_slots.host_slot_directory()/('heavy-'+str(identity['slot'])+'.lock')
    if slot.is_symlink(): raise ValueError('unsafe policy slot')
    fd = os.open(slot, os.O_RDWR | getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0))
    try:
        if not stat.S_ISREG(os.fstat(fd).st_mode) or not local_heavy_slots._try_lock(fd) or local_heavy_slots._marker(fd) != b'idle\n':
            raise ValueError('policy slot not released')
    finally:
        os.close(fd)
    return windows_test_names(root, rows)


def verify_component(root, output, source, base, report):
    """Revalidate component evidence before it is composed into a job."""
    if (not isinstance(report, dict) or report.get('schema') != 'voice-local-policy-component-v1' or
            report.get('status') != 'complete' or report.get('scope') != 'ci-script-tests/policy' or
            report.get('source') != source or report.get('base') != base or
            report.get('remaining_shards') != ['runtime', 'staging', 'proofs']):
        raise ValueError('policy component identity differs')
    linux = json.loads(_read(output/'linux/diagnostic.json', 1024*1024))
    names = verify_linux_evidence(root, output/'linux', source, base, linux)
    if len(names) != 37 or names != report.get('windows_tests'):
        raise ValueError('policy Windows inventory differs')
    for suffix in ('stdout', 'stderr'):
        raw = _read(output/('windows.'+suffix), 16*1024*1024)
        if hashlib.sha256(raw).hexdigest() != report.get('windows_'+suffix+'_sha256'):
            raise ValueError('policy Windows log differs')
        if suffix == 'stderr': windows_test_count(raw, 0, len(names))


def run(root, base, head, head_ref, output, assets):
    """Execute the selected policy shard and its exact Windows-only cases.

    This returns only a policy component receipt, never a whole-job handoff.
    Fetched refs, daemon custody and external tool provenance remain assumptions.
    """
    import affected_check_plan
    import local_preflight
    if os.name != 'nt' or sys.version_info[:2] != (3, 12):
        raise ValueError('policy component requires Windows Python 3.12 supervision')
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('policy output must be new and outside checkout')
    local_preflight._handoff_refs(root, base, head, head_ref)
    plan = affected_check_plan.build_local_plan(root, base, head, head_ref)
    source = plan['source']
    if ('ci-script-tests' not in plan['checks'] or source.get('head') != head or
            source.get('changed_paths') or source.get('raw_mismatch_paths')):
        raise ValueError('policy requires selected clean committed source')
    output.mkdir(parents=True)
    request = {'root':str(root), 'output':str(output/'linux'), 'commit':head,
               'tree':source['tree'], 'base':base, 'shard':'policy', 'assets':assets}
    linux = staging.run(request)
    names = verify_linux_evidence(root, output/'linux', source, base, linux)
    if len(names) != 37:
        raise ValueError('Windows policy inventory count differs')
    materialized = snapshot.materialize(root, source, output/'windows-source')
    env = {key:value for key,value in os.environ.items() if not key.upper().startswith(('GIT_', 'VOICE_', 'PYTHON'))}
    env.update(PYTHONDONTWRITEBYTECODE='1', PYTHONNOUSERSITE='1')
    result = local_proto_execution._execute([sys.executable, '-B', '-m', 'unittest', *names],
                                             cwd=materialized/'scripts/ci', env=env, timeout=1800)
    (output/'windows.stdout').write_bytes(result.stdout)
    (output/'windows.stderr').write_bytes(result.stderr)
    stdout = _read(output/'windows.stdout', 16*1024*1024)
    stderr = _read(output/'windows.stderr', 16*1024*1024)
    windows_test_count(stderr, result.returncode, len(names))
    snapshot.verify_materialized(root, source, materialized, allowed_extra=['tmp/'])
    if snapshot.capture(root, head) != source or affected_check_plan.build_local_plan(root, base, head, head_ref) != plan:
        raise ValueError('policy source or selection changed')
    local_preflight._handoff_refs(root, base, head, head_ref)
    report = {'schema':'voice-local-policy-component-v1', 'status':'complete', 'scope':'ci-script-tests/policy',
              'source':source, 'base':base, 'head_ref':head_ref, 'linux':'linux/diagnostic.json',
              'windows_tests':names, 'windows_stdout_sha256':hashlib.sha256(stdout).hexdigest(),
              'windows_stderr_sha256':hashlib.sha256(stderr).hexdigest(),
              'remaining_shards':['runtime','staging','proofs'],
              'limitations':['fetched refs are not a live remote observation',
                             'tool and dependency publisher provenance is not authenticated',
                             'exclusive Docker daemon and socket custody is assumed']}
    verify_component(root, output, source, base, report)
    (output/'policy-component.json').write_text(json.dumps(report, indent=2, sort_keys=True)+'\n', encoding='utf-8')
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', required=True)
    parser.add_argument('--head', required=True)
    parser.add_argument('--head-ref', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--assets', required=True, type=Path)
    args = parser.parse_args()
    if not args.assets.is_absolute(): parser.error('assets path must be absolute')
    assets = staging.contract.parse_assets(_read(args.assets, 1024*1024))
    report = run(Path(__file__).resolve().parents[2], args.base, args.head, args.head_ref, args.output, assets)
    print(json.dumps(report, sort_keys=True))


if __name__ == '__main__':
    main()
