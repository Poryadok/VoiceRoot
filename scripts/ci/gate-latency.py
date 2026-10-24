"""Read GitHub Actions metadata for an explicit, reproducible PR-head sample.

Usage: python3 scripts/ci/gate-latency.py sample.json output-directory
sample.json: {"repository":"owner/repo", "samples":[{"run_id":123,"scope":"CI/global"}]}
No workflows are cancelled or rerun. Output includes censored and failed runs.
"""
from datetime import datetime
import json
import math
from pathlib import Path
import statistics
import subprocess
import sys


def stamp(value):
    return datetime.fromisoformat(value.replace('Z', '+00:00')) if value else None


def summarize(run, attempts, scope):
    created = stamp(run['created_at'])
    jobs = [job for attempt in attempts for job in attempt['jobs']
            if job.get('conclusion') != 'skipped']
    starts = [stamp(job['started_at']) for job in jobs if job.get('started_at')]
    gates = [job for job in attempts[-1]['jobs'] if job['name'] == 'ci-gate']
    gate = gates[0] if len(gates) == 1 else None
    end = stamp(gate.get('completed_at')) if gate else None
    terminal = run['status'] == 'completed' and end is not None
    return {'run_id': run['id'], 'url': run['html_url'], 'head_sha': run['head_sha'],
            'scope': scope, 'status': run['status'], 'conclusion': run.get('conclusion'),
            'gate_conclusion': gate.get('conclusion') if gate else None,
            'attempts': run['run_attempt'], 'created_at': run['created_at'],
            'gate_wall_seconds': (end-created).total_seconds() if terminal else None,
            'initial_queue_seconds': (min(starts)-created).total_seconds() if starts else None,
            'execution_envelope_seconds': (end-min(starts)).total_seconds() if terminal and starts else None,
            'attempt_execution_seconds': [sum(
                (stamp(job['completed_at'])-stamp(job['started_at'])).total_seconds()
                for job in attempt['jobs'] if job.get('started_at') and job.get('completed_at')
                and job.get('conclusion') != 'skipped') for attempt in attempts],
            'jobs': [{'name': job['name'], 'url': job['html_url'],
                      'runner_or_dependency_wait_seconds': (
                          stamp(job['started_at'])-stamp(job['created_at'])).total_seconds()
                      if job.get('started_at') and job.get('created_at') else None,
                      'steps': [{'name': step['name'], 'conclusion': step.get('conclusion'),
                                 'seconds': (stamp(step['completed_at'])-stamp(step['started_at'])).total_seconds()
                                 if step.get('started_at') and step.get('completed_at') else None}
                                for step in job.get('steps', [])]}
                     for job in attempts[-1]['jobs'] if job.get('conclusion') != 'skipped']}


def aggregate(rows):
    values = sorted(row['gate_wall_seconds'] for row in rows if row['gate_wall_seconds'] is not None)
    return {'n': len(values), 'mean_seconds': statistics.mean(values) if values else None,
            'p95_seconds': values[math.ceil(.95*len(values))-1] if values else None}


def api(path):
    return json.loads(subprocess.check_output(['gh', 'api', path], text=True, timeout=60))


def main(manifest_path, output):
    manifest = json.loads(Path(manifest_path).read_text(encoding='utf-8'))
    output = Path(output)
    output.mkdir(parents=True, exist_ok=True)
    rows, heads = [], set()
    for sample in manifest['samples']:
        path = 'repos/' + manifest['repository'] + '/actions/runs/' + str(sample['run_id'])
        run = api(path)
        if run['event'] != 'pull_request' or run['head_sha'] in heads:
            raise ValueError('sample must contain distinct pull_request heads')
        heads.add(run['head_sha'])
        attempts = []
        for number in range(1, run['run_attempt']+1):
            jobs, page = [], 1
            while True:
                data = api(path + '/attempts/' + str(number) + '/jobs?per_page=100&page=' + str(page))
                jobs.extend(data['jobs'])
                if len(jobs) >= data['total_count']:
                    break
                page += 1
            attempts.append({'attempt': number, 'jobs': jobs})
        (output / (str(run['id']) + '.json')).write_text(
            json.dumps({'run': run, 'attempts': attempts}, indent=2), encoding='utf-8')
        rows.append(summarize(run, attempts, sample['scope']))
    groups = {'all_terminal': aggregate(rows),
              'successful': aggregate([r for r in rows if r['gate_conclusion'] == 'success'])}
    for scope in sorted({r['scope'] for r in rows}):
        groups[scope] = aggregate([r for r in rows if r['scope'] == scope])
    report = {'method': 'created_at to last-attempt ci-gate completion, including retry gaps; '
              'nearest-rank p95; initial queue vs execution envelope; job waits include dependencies; '
              'attempt execution sums parallel jobs and is not wall-clock; incomplete/missing gates censored',
              'groups': groups, 'samples': rows}
    (output / 'summary.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
    print(json.dumps(groups, indent=2))


if __name__ == '__main__':
    main(*sys.argv[1:])
