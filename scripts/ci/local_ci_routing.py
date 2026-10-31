"""Run a safe, source-bound subset of CI gate tests as incomplete diagnostics.

The four router cases below avoid shell fixtures that can invoke Docker or Go.
This does not run all router tests, CI script shards, or historical replay.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import sys

import local_process_supervision
import local_source_snapshot


GATE_CASES = (
    'test_f13_selected_result_cannot_be_skipped', 'test_changed_live_flutter_suite_requires_compose_proof',
    'test_independent_go_jobs_stop_on_cancellation_and_pkg_failure_still_blocks_gate',
    'test_changed_t055_path_selects_live_handoff_and_gate', 'test_actual_selection_matches_gate_across_filters_and_events',
    'test_selected_failure_cancel_skip_or_missing_blocks_gate', 'test_docs_only_global_ci_still_checks_selected_jobs',
    'test_unselected_and_plain_docs_only_skips_remain_valid',
)
ROUTER_CASES = ('test_docs_exclusions_and_mixed_changes', 'test_leaf_ci_keeps_script_checks_without_product_fanout',
                'test_ci_only_changes_cannot_skip_script_matrix', 'test_current_diff_runs_for_all_prs_and_is_required')


def _hash(path):
    digest = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def run(root, source, output):
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('routing output must be new and outside source')
    output.mkdir(parents=True)
    # materialize checks the original identity both before and after copying.
    snapshot = local_source_snapshot.materialize(root, source, output / 'source')
    executable = Path(sys.executable).resolve(strict=True)
    binary_hash = _hash(executable)
    env = {key: value for key, value in os.environ.items()
           if not key.upper().startswith(('PYTHON', 'VOICE_', 'GIT_'))
           and key.upper() not in ('BASH_ENV', 'ENV', 'SHELLOPTS', 'CDPATH')}
    private_temp = output / 'tmp'
    private_temp.mkdir()
    env.update(PYTHONDONTWRITEBYTECODE='1', TEMP=str(private_temp), TMP=str(private_temp), TMPDIR=str(private_temp))
    commands = []
    for label, path, suite, cases in (
        ('gate', '.github/ci/selected-acceptance-gate_test.py', 'SelectedAcceptanceGateTests', GATE_CASES),
        ('router', 'scripts/ci/fast-gate-routing_test.py', 'FastGateRoutingTests', ROUTER_CASES),
    ):
        command = [str(executable), '-B', str(snapshot / path), *(suite + '.' + case for case in cases)]
        out, err = output / (label + '.stdout'), output / (label + '.stderr')
        with out.open('wb') as stdout, err.open('wb') as stderr:
            completed = local_process_supervision.run_diagnostic(command, timeout=180, cwd=snapshot,
                                                               env=env, stdout=stdout, stderr=stderr, check=False)
        raw = err.read_bytes()
        matches = re.findall(rb'(?m)^Ran (\d+) tests? in [0-9.]+s\r?$', raw)
        if (completed.returncode != 0 or len(matches) != 1 or int(matches[0]) != len(cases) or
                not re.search(rb'(?m)^OK\r?$', raw) or re.search(rb'(?i)skipped|FAILED', raw)):
            raise ValueError(label + ' routing test inventory failed; inspect retained logs')
        local_source_snapshot.verify_materialized(root, source, snapshot)
        if local_source_snapshot.capture(root, source['head']) != source or _hash(executable) != binary_hash:
            raise ValueError('source changed during routing diagnostic')
        commands.append({'name': label, 'argv': command, 'exit_code': completed.returncode,
                         'tests': len(cases), 'stdout': out.name, 'stderr': err.name,
                         'stdout_sha256': _hash(out), 'stderr_sha256': _hash(err)})
    result = {'schema': 'voice-local-ci-routing-diagnostic-v1', 'status': 'incomplete', 'source': source,
              'python': {'path': str(executable), 'sha256': binary_hash}, 'commands': commands,
              'omitted': ['remaining router cases including shell Docker/Go fixtures', 'other CI-script shards',
                          'historical replay', 'dependency and shell provenance']}
    (output / 'diagnostic.json').write_text(json.dumps(result, indent=2, sort_keys=True), encoding='utf-8')
    return result
