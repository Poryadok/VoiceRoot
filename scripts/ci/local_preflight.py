"""Run implemented local diagnostics from the affected plan, never issuing PASS.

This is an incomplete preflight foundation. Unimplemented checks and unbound
inputs prevent PR handoff; GitHub's merge-candidate checks remain mandatory.
"""
import argparse
import json
from pathlib import Path

import affected_check_plan
import local_flutter_execution
import local_source_snapshot


def run(root, base, head, output, flutter, head_ref=''):
    root = Path(root).resolve()
    output = Path(output).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('output must be a new directory outside the source tree')
    plan = affected_check_plan.build_local_plan(root, base, head, head_ref)
    output.mkdir(parents=True, exist_ok=False)
    report = {'schema': 'voice-local-preflight-diagnostic-v1', 'status': 'incomplete',
              'plan': plan, 'checks': {name: {'status': 'not_run'} for name in plan['checks']}}
    try:
        flutter_checks = {'flutter', 'flutter-unit-tests'} & set(plan['checks'])
        if flutter_checks:
            diagnostic = local_flutter_execution.run(root, head, output / 'flutter', flutter)
            report['flutter_report'] = 'flutter/diagnostic.json'
            if diagnostic.get('source') != plan['source']:
                raise ValueError('Flutter diagnostic source does not match the plan')
            if diagnostic.get('status') != 'incomplete':
                raise ValueError('Flutter diagnostic failed or has an unsupported status')
            if not diagnostic.get('full_inventory') or not isinstance(diagnostic.get('tests'), dict):
                raise ValueError('Flutter diagnostic did not execute the full inventory')
            tests = diagnostic['tests']
            if type(tests.get('passed')) is not int or tests['passed'] < 1 or not isinstance(tests.get('skipped'), list):
                raise ValueError('Flutter diagnostic test counts are missing or invalid')
            if tests['skipped']:
                raise ValueError('Flutter diagnostic skipped required tests')
            for name in flutter_checks:
                report['checks'][name] = {'status': 'diagnostic', 'report': 'flutter/diagnostic.json'}
        if local_source_snapshot.capture(root, head) != plan['source']:
            raise ValueError('source changed during local preflight')
    except (OSError, ValueError, KeyError, TypeError) as error:
        report['status'] = 'failed'
        report['error'] = str(error)
    (output / 'preflight.json').write_text(json.dumps(report, indent=2, sort_keys=True), encoding='utf-8')
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', required=True)
    parser.add_argument('--head', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--flutter', default='flutter')
    parser.add_argument('--head-ref', required=True, help='PR head branch for branch-specific checks')
    args = parser.parse_args()
    report = run(Path(__file__).resolve().parents[2], args.base, args.head, args.output, args.flutter, args.head_ref)
    print(json.dumps(report, sort_keys=True))
    return 1 if report['status'] == 'failed' else 2


if __name__ == '__main__':
    raise SystemExit(main())
