"""Run the hosted CI-script proofs shard on an exact fetched PR merge ref.

The component proves one source-bound shard only. It never promotes the full
ci-script-tests matrix or aggregate local preflight to success.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

import local_ci_matrix_inventory as matrix
import local_current_diff
import local_ci_proofs_scheduler as scheduler
import local_proto_execution as execution
import local_process_supervision as supervision
import local_source_snapshot


SHARD = 'proofs'
RECIPE_COUNT = 15
OUTPUT_LIMIT = 16 * 1024 * 1024
TIMEOUT_SECONDS = 1800
WINDOWS_PATH_LIMIT = 240
SHA = re.compile(r'[0-9a-f]{40}')
PR_MERGE_REF = re.compile(r'refs/pull/[1-9][0-9]*/merge')
LIMITATIONS = [
    'only the proofs shard is complete',
    'other selected checks and all remaining CI shards are not_run',
    'candidate test sources are treated as cooperative, matching hosted CI; this does not prove resistance to adversarial PR scripts',
    'same-user tool custody is assumed; executable publisher provenance is not authenticated',
    'caller-supplied Windows Job Object limits bound only this proofs-shard process tree',
    'host slot scheduling is cooperative and limited to participating local proofs runs; same-user journal and Job Object custody is assumed',
    'recovery is bound to the creating Windows boot, session, and logon identity; mismatched contexts retain the slot for separate manual reconciliation',
    'Windows directory-entry durability across sudden power loss is unproven; missing or corrupt journals fail closed and do not establish general crash recovery',
    'this component does not prove host-wide saturation prevention or constrain unrelated processes and Docker',
    'aggregate local preflight remains incomplete',
]


def _windows():
    return os.name == 'nt'


def _git(root, *args, timeout=120):
    result = local_current_diff._execute(['git', '-C', str(root), *args], timeout=timeout)
    if result.returncode:
        raise ValueError('proof candidate Git check failed: ' + args[0])
    return result.stdout.decode('ascii', errors='strict').strip()


def validate_candidate(root, event_base, base, head, candidate):
    """Require exact PR parents, event ancestry and the actual clean merge tree."""
    if any(not isinstance(value, str) or not SHA.fullmatch(value)
           for value in (event_base, base, head, candidate)):
        raise ValueError('full PR candidate identities are required')
    if _git(root, 'rev-parse', '--verify', candidate + '^{commit}') != candidate:
        raise ValueError('candidate identity is not a commit')
    parents = _git(root, 'rev-list', '--parents', '-n', '1', candidate).split()[1:]
    if parents != [base, head]:
        raise ValueError('candidate parents differ from exact base and head')
    if local_current_diff._execute(['git', '-C', str(root), 'merge-base', '--is-ancestor',
                                    event_base, base], timeout=30).returncode:
        raise ValueError('event base is not an ancestor of actual merge base')
    merged = local_current_diff._execute(['git', '-C', str(root), 'merge-tree', '--write-tree',
                                          base, head], timeout=120)
    if merged.returncode:
        raise ValueError('PR base and head do not merge cleanly')
    expected_tree = merged.stdout.decode('ascii', errors='strict').splitlines()[0]
    actual_tree = _git(root, 'rev-parse', candidate + '^{tree}')
    if not SHA.fullmatch(expected_tree) or actual_tree != expected_tree:
        raise ValueError('candidate tree differs from exact clean merge tree')
    return {'event_base': event_base, 'base': base, 'head': head,
            'candidate': candidate, 'parents': parents, 'tree': actual_tree}


def validate_execution(dry_run, stdout, stderr, returncode, *, expected_recipes):
    """Require every planned Make recipe and a nonempty, skip-free test run."""
    if (type(expected_recipes) is not int or expected_recipes != RECIPE_COUNT or
            type(returncode) is not int or returncode != 0):
        raise ValueError('proof Make shard failed or recipe inventory differs')
    try:
        planned = [line for line in dry_run.decode('utf-8', errors='strict').splitlines() if line.strip()]
        out_text = stdout.decode('utf-8', errors='strict')
        err_text = stderr.decode('utf-8', errors='strict')
    except UnicodeError as error:
        raise ValueError('proof Make output is not valid UTF-8') from error
    if len(planned) != expected_recipes:
        raise ValueError('proof Make dry run recipe count differs')
    actual_lines = out_text.replace('\r\n', '\n').replace('\r', '\n').splitlines()
    if any(actual_lines.count(command) != 1 for command in planned):
        raise ValueError('proof Make execution did not echo every exact recipe once')
    combined = out_text + '\n' + err_text
    if re.search(r'(?im)^(?:.*\bskipped(?:=|\s+)|SKIP\s*[:(])', combined):
        raise ValueError('proof shard unittest skips are not accepted')
    ran = re.findall(r'(?m)^Ran ([0-9]+) tests? in [0-9.]+s\s*$', combined)
    ok = re.findall(r'(?m)^OK\s*$', combined)
    if not ran or len(ran) != len(ok) or any(int(count) < 1 for count in ran):
        raise ValueError('proof shard unittest inventory is empty or incomplete')
    if re.search(r'(?m)^FAILED(?:\s|$)', combined):
        raise ValueError('proof shard contains a failed test summary')
    return {'recipes': expected_recipes,
            'unittest_runs': len(ran), 'unittest_tests': sum(map(int, ran)),
            'skipped': 0}


def validate_job_evidence(evidence, job_limits, job_names=None):
    """Require exact Job Object configuration and verified post-cleanup counters."""
    if not isinstance(job_limits, supervision.JobLimits):
        raise ValueError('explicit Windows Job Object limits are required')
    if not isinstance(evidence, dict) or set(evidence) != {'make_dry_run', 'make'}:
        raise ValueError('proof Job Object evidence inventory differs')
    expected = job_limits.to_dict()
    fields = set(expected) | {'job_name', 'peak_job_memory_bytes', 'total_processes',
                              'active_processes_after_cleanup'}
    observed_names = []
    for name in ('make_dry_run', 'make'):
        row = evidence[name]
        if (not isinstance(row, dict) or set(row) != fields or
                any(row.get(key) != value for key, value in expected.items()) or
                type(row.get('peak_job_memory_bytes')) is not int or
                row['peak_job_memory_bytes'] < 0 or
                row['peak_job_memory_bytes'] > job_limits.job_memory_bytes or
                type(row.get('total_processes')) is not int or row['total_processes'] < 1 or
                type(row.get('active_processes_after_cleanup')) is not int or
                row['active_processes_after_cleanup'] != 0 or
                not isinstance(row.get('job_name'), str) or
                not re.fullmatch(r'Local\\VoiceCIProof-[A-Za-z0-9_-]{1,96}', row['job_name'])):
            raise ValueError('proof Job Object limits or cleanup evidence differ')
        observed_names.append(row['job_name'])
    if len(set(observed_names)) != 2 or (job_names is not None and tuple(observed_names) != tuple(job_names)):
        raise ValueError('proof named Job Object inventory differs')
    return evidence


def execute_bounded_recipe(command, *, cwd, env, timeout, job_limits, job_name):
    """Run one recipe under explicitly configured and verified Windows job limits."""
    if not isinstance(job_limits, supervision.JobLimits):
        raise ValueError('explicit Windows Job Object limits are required')
    if not isinstance(job_name, str) or not re.fullmatch(
            r'Local\\VoiceCIProof-[A-Za-z0-9_-]{1,96}', job_name):
        raise ValueError('a reserved named Job Object is required')
    result = execution._execute(command, cwd=cwd, env=env, timeout=timeout,
                                job_limits=job_limits, job_name=job_name)
    evidence = result.job_evidence
    expected = job_limits.to_dict()
    fields = set(expected) | {'job_name', 'peak_job_memory_bytes', 'total_processes',
                              'active_processes_after_cleanup'}
    if (not isinstance(evidence, dict) or set(evidence) != fields or
            any(evidence.get(key) != value for key, value in expected.items()) or
            evidence.get('job_name') != job_name or
            type(evidence.get('peak_job_memory_bytes')) is not int or
            not 0 <= evidence['peak_job_memory_bytes'] <= job_limits.job_memory_bytes or
            type(evidence.get('total_processes')) is not int or evidence['total_processes'] < 1 or
            type(evidence.get('active_processes_after_cleanup')) is not int or
            evidence['active_processes_after_cleanup'] != 0):
        raise ValueError('proof named Job Object evidence differs')
    return result


def run_proofs_recipe_suite(make_path, candidate_repo, env, output, expected_commands,
                            job_limits, timeout=TIMEOUT_SECONDS):
    """Hold one shared slot across bounded Make runs and exact cleanup evidence."""
    if not isinstance(job_limits, supervision.JobLimits):
        raise ValueError('explicit Windows Job Object limits are required')
    if type(timeout) is not int or not 0 < timeout <= TIMEOUT_SECONDS:
        raise ValueError('invalid proof recipe timeout')
    output = Path(output)
    if not output.is_dir() or output.is_symlink():
        raise ValueError('proof output directory must already exist and be real')
    scheduler.recover_proof_runs(capacity=2)
    run = scheduler.ProofRun.start(timeout=300, capacity=2)
    safe_to_release = True
    evidence = {}
    try:
        command = [str(make_path), '--no-print-directory', '-n', 'ci-script-tests-proofs']
        dry_job_name = run.reserve_job_name('make-dry-run')
        safe_to_release = False
        dry = execute_bounded_recipe(command, cwd=candidate_repo, env=env,
                                     timeout=120, job_limits=job_limits,
                                     job_name=dry_job_name)
        evidence['make_dry_run'] = dry.job_evidence
        safe_to_release = True
        (output / 'make-dry-run.stdout').write_bytes(dry.stdout)
        (output / 'make-dry-run.stderr').write_bytes(dry.stderr)
        if dry.returncode:
            raise ValueError('proof Make dry run failed')
        if len(dry.stdout) > OUTPUT_LIMIT or len(dry.stderr) > OUTPUT_LIMIT:
            raise ValueError('proof Make dry-run output exceeded its bound')
        if dry.stdout.decode('utf-8', errors='strict').splitlines() != expected_commands:
            raise ValueError('proof Make dry run differs from source-pinned recipes')

        safe_to_release = False
        make_job_name = run.reserve_job_name('make')
        result = execute_bounded_recipe(
            [str(make_path), '--no-print-directory', 'ci-script-tests-proofs'],
            cwd=candidate_repo, env=env, timeout=timeout, job_limits=job_limits,
            job_name=make_job_name)
        evidence['make'] = result.job_evidence
        safe_to_release = True
        (output / 'make.stdout').write_bytes(result.stdout)
        (output / 'make.stderr').write_bytes(result.stderr)
        if len(result.stdout) > OUTPUT_LIMIT or len(result.stderr) > OUTPUT_LIMIT:
            raise ValueError('proof Make output exceeded its bound')
        summary = validate_execution(dry.stdout, result.stdout, result.stderr,
                                     result.returncode, expected_recipes=RECIPE_COUNT)
        validate_job_evidence(evidence, job_limits,
                              job_names=(dry_job_name, make_job_name))
        return summary, evidence
    finally:
        if safe_to_release:
            run.close()
        else:
            run.abandon()


def child_environment(source, bash_path):
    """Drop inherited Git, Make and shell startup controls before execution."""
    blocked = {'MAKEFLAGS', 'GNUMAKEFLAGS', 'MFLAGS', 'MAKEFILES', 'MAKEOVERRIDES',
               'MAKELEVEL', 'MAKE', 'BASH_ENV', 'ENV', 'SHELLOPTS', 'BASHOPTS'}
    env = {key: value for key, value in source.items()
           if not key.upper().startswith(('GIT_', 'PYTHON', 'BASH_FUNC_')) and key.upper() not in blocked}
    env.update(PYTHONDONTWRITEBYTECODE='1', PYTHONNOUSERSITE='1',
               VOICE_TEST_BASH=str(bash_path))
    env['BASH'] = ('"' + Path(bash_path).as_posix() + '"' if _windows()
                   else str(bash_path))
    return env


def expected_recipe_commands(rows, root, bash_path):
    """Expand only the two Make variables allowed in the source inventory."""
    root_value = Path(root).resolve().as_posix()
    bash_value = ('"' + Path(bash_path).resolve().as_posix() + '"'
                  if _windows() else str(Path(bash_path).resolve()))
    commands = []
    for row in rows:
        recipe = row.get('recipe')
        if not isinstance(recipe, str):
            raise ValueError('proof recipe inventory is malformed')
        expanded = recipe.replace('$(ROOT)', root_value).replace('$(BASH)', bash_value)
        if '$(' in expanded or '${' in expanded:
            raise ValueError('proof Make recipe has an unsupported expansion')
        commands.append(expanded)
    return commands


def _tool(value, name):
    located = shutil.which(str(value)) if value else None
    if not located:
        raise ValueError(name + ' executable is unavailable')
    path = Path(located).resolve(strict=True)
    if not path.is_file():
        raise ValueError(name + ' executable is invalid')
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    return path, digest


def _tool_version(path, name, env):
    result = execution._execute([str(path), '--version'], env=env, timeout=30)
    if result.returncode or len(result.stdout) > 4096 or len(result.stderr) > 4096:
        raise ValueError(name + ' version probe failed or exceeded its bound')
    try:
        version = result.stdout.decode('utf-8', errors='strict').strip()
    except UnicodeError as error:
        raise ValueError(name + ' version output is not UTF-8') from error
    first_line = version.splitlines()[0] if version else ''
    if (name == 'GNU Make' and not re.fullmatch(r'GNU Make [0-9]+(?:\.[0-9]+)+', first_line) or
            name == 'Bash' and not version.startswith('GNU bash, version ')):
        raise ValueError(name + ' executable identity is not recognized')
    return version


def _read(path, limit=OUTPUT_LIMIT):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > limit:
        raise ValueError('proof evidence file is unsafe or oversized')
    data = path.read_bytes()
    if len(data) > limit:
        raise ValueError('proof evidence file is oversized')
    return data


def validate_windows_checkout_paths(candidate_repo, relative_paths):
    """Reject evidence roots whose private Windows checkout would hit MAX_PATH."""
    if not _windows():
        return
    root = Path(candidate_repo).resolve()
    def units(path):
        return len(str(path).encode('utf-16-le', errors='surrogatepass')) // 2
    longest = max((units(root / name) for name in relative_paths), default=units(root))
    if longest > WINDOWS_PATH_LIMIT:
        raise ValueError('proof output path is too deep for Windows checkout; choose a shorter output path')


def configure_private_git(git_path, candidate_repo):
    """Pin checkout bytes independently of the invoking user's Git defaults."""
    for key, value in (('core.autocrlf', 'false'), ('core.longpaths', 'false')):
        result = local_current_diff._execute(
            [str(git_path), '-C', str(candidate_repo), 'config', key, value], timeout=30)
        if result.returncode:
            raise ValueError('private candidate Git checkout configuration failed')


def verify_component(root, output, report, *, candidate_repo=None):
    """Recheck exact candidate, source manifest, bounded logs and incomplete scope."""
    root, output = Path(root).resolve(), Path(output).resolve()
    if (not isinstance(report, dict) or report.get('schema') != 'voice-local-ci-proofs-component-v1' or
            report.get('status') != 'complete' or report.get('scope') != 'ci-script-tests/proofs' or
            report.get('aggregate_preflight') != 'incomplete' or
            report.get('remaining_shards') != ['policy', 'runtime', 'staging'] or
            report.get('remaining_selected_checks') != 'not_run' or
            report.get('recipes') != RECIPE_COUNT or report.get('skipped') != 0 or
            report.get('limitations') != LIMITATIONS):
        raise ValueError('proof component status or scope differs')
    try:
        resource_limits = supervision.JobLimits(**report.get('resource_limits', {}))
    except (TypeError, ValueError):
        raise ValueError('proof resource limits are missing or invalid') from None
    validate_job_evidence(report.get('job_evidence'), resource_limits)
    if candidate_repo is None:
        candidate_repo = output / 'candidate-repo'
    candidate_repo = Path(candidate_repo).resolve()
    identity = validate_candidate(candidate_repo, report['event_base'], report['base'],
                                  report['head'], report['candidate'])
    if identity != report.get('candidate_identity'):
        raise ValueError('proof candidate identity differs')
    if (_git(candidate_repo, 'rev-parse', 'HEAD') != report['candidate'] or
            _git(candidate_repo, 'rev-parse', 'HEAD^{tree}') != report['candidate_tree']):
        raise ValueError('private proof checkout differs from fetched candidate')
    inventory = matrix.build(candidate_repo)
    if (inventory['schema'] != 'voice-local-ci-matrix-inventory-v1' or
            inventory['shards'].get(SHARD) is None or
            len(inventory['shards'][SHARD]) != RECIPE_COUNT or
            inventory.get('makefile_sha256') != report.get('makefile_sha256') or
            inventory.get('workflow_sha256') != report.get('workflow_sha256') or
            hashlib.sha256(json.dumps(inventory['shards'][SHARD], sort_keys=True,
                                      separators=(',', ':')).encode()).hexdigest() !=
            report.get('recipe_manifest_sha256')):
        raise ValueError('proof recipe or trusted source identity differs')
    if hashlib.sha256(Path(__file__).read_bytes()).hexdigest() != report.get('producer_sha256'):
        raise ValueError('proof producer source changed')
    dry_run = _read(output / 'make-dry-run.stdout')
    stdout, stderr = _read(output / 'make.stdout'), _read(output / 'make.stderr')
    bash_path = report.get('tools', {}).get('bash', {}).get('path')
    expected = expected_recipe_commands(inventory['shards'][SHARD], candidate_repo, bash_path)
    if dry_run.decode('utf-8', errors='strict').splitlines() != expected:
        raise ValueError('proof Make dry run differs from source-pinned recipes')
    summary = validate_execution(dry_run, stdout, stderr, 0, expected_recipes=RECIPE_COUNT)
    if summary != report.get('execution'):
        raise ValueError('proof execution summary differs')
    for name, raw in (('make-dry-run.stdout', dry_run), ('make.stdout', stdout),
                      ('make.stderr', stderr)):
        if hashlib.sha256(raw).hexdigest() != report.get('logs', {}).get(name):
            raise ValueError('proof execution log digest differs')
    tools = report.get('tools', {})
    for name in ('bash', 'make'):
        row = tools.get(name)
        if (not isinstance(row, dict) or not isinstance(row.get('path'), str) or
                not isinstance(row.get('version'), str)):
            raise ValueError('proof tool identity is missing')
        path, digest = _tool(row['path'], name)
        if str(path) != row['path'] or digest != row.get('sha256'):
            raise ValueError('proof tool bytes changed after execution')
        env = child_environment(os.environ, tools['bash']['path'])
        if _tool_version(path, 'GNU Make' if name == 'make' else 'Bash', env) != row['version']:
            raise ValueError('proof tool version changed after execution')


def run(root, event_base, head, head_ref, candidate_ref, output, *, job_limits,
        bash_binary=None, make_binary='make', remote='origin', timeout=TIMEOUT_SECONDS):
    """Fetch, validate and run only `ci-script-tests-proofs` in an isolated repo."""
    root, output = Path(root).resolve(), Path(output).resolve()
    if (output.exists() or output == root or root in output.parents or
            not isinstance(event_base, str) or not SHA.fullmatch(event_base) or
            not isinstance(head, str) or not SHA.fullmatch(head) or
            not isinstance(head_ref, str) or head_ref.startswith('-') or
            not PR_MERGE_REF.fullmatch(str(candidate_ref)) or
            not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]*', str(remote)) or
            type(timeout) is not int or not 0 < timeout <= TIMEOUT_SECONDS):
        raise ValueError('invalid fetched proofs component request')
    if not _windows():
        raise ValueError('proof shard requires Windows descendant supervision')
    if not isinstance(job_limits, supervision.JobLimits):
        raise ValueError('caller-supplied Windows Job Object limits are required')
    candidate_repo = output / 'candidate-repo'
    validate_windows_checkout_paths(candidate_repo, ())
    _git(root, 'check-ref-format', 'refs/heads/' + head_ref)
    source = local_source_snapshot.capture(root, head)
    if source.get('changed_paths') or source.get('raw_mismatch_paths'):
        raise ValueError('proof component requires exact clean committed head source')
    refs = ['refs/heads/master', 'refs/heads/' + head_ref, candidate_ref]
    before = local_current_diff._remote_refs(root, remote, refs)
    base, candidate = before['refs/heads/master'], before[candidate_ref]
    if before['refs/heads/' + head_ref] != head:
        raise ValueError('live PR head ref differs from requested head')
    output.mkdir(parents=True, exist_ok=False)
    git_path, git_sha = _tool('git', 'Git')
    bash_path, bash_sha = _tool(bash_binary or os.environ.get('VOICE_TEST_BASH') or 'bash', 'Bash')
    make_path, make_sha = _tool(make_binary, 'Make')
    env = child_environment(os.environ, bash_path)
    bash_version = _tool_version(bash_path, 'Bash', env)
    make_version = _tool_version(make_path, 'GNU Make', env)
    url = _git(root, 'remote', 'get-url', remote)
    producer_sha = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()

    def git_private(*args, timeout=120):
        result = local_current_diff._execute([str(git_path), '-C', str(candidate_repo), *args],
                                             timeout=timeout)
        if result.returncode:
            raise ValueError('private candidate Git operation failed: ' + args[0])
        return result.stdout.decode('ascii', errors='strict').strip()

    try:
        local_current_diff._execute([str(git_path), 'init', '-q', str(candidate_repo)], timeout=30).check_returncode()
        configure_private_git(git_path, candidate_repo)
        git_private('remote', 'add', remote, url)
        git_private('fetch', '--no-tags', remote,
                    '+' + candidate_ref + ':refs/heads/voice-pr-candidate')
        if git_private('rev-parse', '--verify', 'refs/heads/voice-pr-candidate^{commit}') != candidate:
            raise ValueError('private candidate fetch differs from live merge ref')
        path_result = local_current_diff._execute(
            [str(git_path), '-C', str(candidate_repo), 'ls-tree', '-r', '--name-only', '-z', candidate],
            timeout=120)
        if path_result.returncode or len(path_result.stdout) > OUTPUT_LIMIT:
            raise ValueError('private candidate path inventory failed or exceeded its bound')
        try:
            relative_paths = [row.decode('utf-8', errors='strict')
                              for row in path_result.stdout.split(b'\0') if row]
        except UnicodeError as error:
            raise ValueError('private candidate path inventory is not UTF-8') from error
        validate_windows_checkout_paths(candidate_repo, relative_paths)
        identity = validate_candidate(candidate_repo, event_base, base, head, candidate)
        git_private('checkout', '--detach', candidate)
        if (git_private('rev-parse', 'HEAD') != candidate or
                git_private('status', '--porcelain=v1', '--untracked-files=all')):
            raise ValueError('private candidate checkout is not exact and clean')
        inventory = matrix.build(candidate_repo)
        recipes = inventory['shards'].get(SHARD, [])
        if len(recipes) != RECIPE_COUNT:
            raise ValueError('proof shard recipe count differs from trusted inventory')
        recipe_manifest_sha = hashlib.sha256(json.dumps(
            recipes, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        expected_commands = expected_recipe_commands(recipes, candidate_repo, bash_path)
        summary, job_evidence = run_proofs_recipe_suite(
            make_path, candidate_repo, env, output, expected_commands, job_limits, timeout)
        dry_stdout = _read(output / 'make-dry-run.stdout')
        result_stdout, result_stderr = _read(output / 'make.stdout'), _read(output / 'make.stderr')
        if matrix.build(candidate_repo)['shards'][SHARD] != recipes:
            raise ValueError('proof recipe sources changed during execution')
        if hashlib.sha256(Path(__file__).read_bytes()).hexdigest() != producer_sha:
            raise ValueError('proof producer source changed during execution')
        if local_source_snapshot.capture(root, head) != source:
            raise ValueError('PR head source changed during proofs shard')
        after = local_current_diff._remote_refs(root, remote, refs)
        if after != before:
            raise ValueError('live PR refs changed during proofs shard')
        report = {'schema': 'voice-local-ci-proofs-component-v1', 'status': 'complete',
                  'scope': 'ci-script-tests/proofs', 'aggregate_preflight': 'incomplete',
                  'event_base': event_base, 'base': base, 'head': head, 'head_ref': head_ref,
                  'candidate_ref': candidate_ref, 'candidate': candidate, 'candidate_tree': identity['tree'],
                  'candidate_identity': identity, 'remote_refs_before': before, 'remote_refs_after': after,
                  'source': source, 'recipe_manifest_sha256': recipe_manifest_sha,
                  'makefile_sha256': inventory['makefile_sha256'],
                  'workflow_sha256': inventory['workflow_sha256'],
                  'producer_sha256': producer_sha,
                  'tools': {'git': {'path': str(git_path), 'sha256': git_sha},
                            'bash': {'path': str(bash_path), 'sha256': bash_sha,
                                     'version': bash_version},
                            'make': {'path': str(make_path), 'sha256': make_sha,
                                     'version': make_version}},
                  'execution': summary, 'recipes': RECIPE_COUNT, 'skipped': 0,
                  'resource_limits': job_limits.to_dict(), 'job_evidence': job_evidence,
                  'logs': {'make-dry-run.stdout': hashlib.sha256(dry_stdout).hexdigest(),
                           'make.stdout': hashlib.sha256(result_stdout).hexdigest(),
                           'make.stderr': hashlib.sha256(result_stderr).hexdigest()},
                  'remaining_shards': ['policy', 'runtime', 'staging'],
                  'remaining_selected_checks': 'not_run',
                  'limitations': LIMITATIONS}
        verify_component(root, output, report, candidate_repo=candidate_repo)
        (output / 'proofs-component.json').write_text(
            json.dumps(report, indent=2, sort_keys=True) + '\n', encoding='utf-8')
        return report
    except (subprocess.SubprocessError, OSError, UnicodeError) as error:
        raise ValueError('fetched proofs component did not complete') from error


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--event-base', required=True)
    parser.add_argument('--head', required=True)
    parser.add_argument('--head-ref', required=True)
    parser.add_argument('--candidate-ref', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--bash', dest='bash_binary')
    parser.add_argument('--make', dest='make_binary', default='make')
    parser.add_argument('--remote', default='origin')
    parser.add_argument('--cpu-rate', type=int, required=True,
                        help='Windows Job Object CPU hard cap in 1/10000 units')
    parser.add_argument('--job-memory-bytes', type=int, required=True,
                        help='Windows Job Object memory limit in bytes')
    parser.add_argument('--active-process-limit', type=int, required=True,
                        help='Windows Job Object active-process limit, including broker and command')
    args = parser.parse_args()
    job_limits = supervision.JobLimits(args.cpu_rate, args.job_memory_bytes,
                                       args.active_process_limit)
    report = run(Path(__file__).resolve().parents[2], args.event_base, args.head,
                 args.head_ref, args.candidate_ref, args.output,
                 job_limits=job_limits,
                 bash_binary=args.bash_binary, make_binary=args.make_binary,
                 remote=args.remote)
    print(json.dumps(report, sort_keys=True))


if __name__ == '__main__':
    main()
