"""Decide whether a verified nightly history result covers a later commit.

Unknown paths and unverified history require a fresh audit. This deliberately
does not turn an incomplete or failed audit into reusable evidence.
"""
import os
import json
import re
import subprocess
import sys
import urllib.error
import urllib.request


SHA = re.compile(r'^[0-9a-f]{40}$')
REPOSITORY = re.compile(r'^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$')


def find_nightly(repository, get):
    if not REPOSITORY.fullmatch(repository):
        return None
    listing = get('/repos/' + repository + '/actions/workflows/ci.yml/runs?event=schedule&branch=master&status=completed&per_page=100')
    runs = listing.get('workflow_runs') if isinstance(listing, dict) else None
    if not isinstance(runs, list):
        return None
    for run in runs:
        if not isinstance(run, dict):
            continue
        run_id, attempt, sha = run.get('id'), run.get('run_attempt'), run.get('head_sha')
        if (type(run_id) is not int or run_id <= 0 or type(attempt) is not int or attempt <= 0
                or not isinstance(sha, str) or not SHA.fullmatch(sha)
                or run.get('event') != 'schedule' or run.get('status') != 'completed'
                or run.get('head_branch') != 'master'
                or not isinstance(run.get('head_repository'), dict)
                or run['head_repository'].get('full_name', '').lower() != repository.lower()):
            continue
        jobs = []
        for page in range(1, 21):
            data = get('/repos/' + repository + '/actions/runs/' + str(run_id)
                       + '/jobs?filter=latest&per_page=100&page=' + str(page))
            if not isinstance(data, dict) or type(data.get('total_count')) is not int or not isinstance(data.get('jobs'), list):
                return None
            jobs.extend(data['jobs'])
            if len(jobs) >= data['total_count']:
                if len(jobs) != data['total_count']:
                    return None
                break
            if not data['jobs']:
                return None
        else:
            return None
        matches = [j for j in jobs if isinstance(j, dict) and j.get('name') == 'voice-history-audit']
        if (successful_audit(jobs) and matches[0].get('run_id') == run_id
                and matches[0].get('run_attempt') == attempt):
            return sha, run_id, attempt
    return None


def successful_audit(jobs):
    if not isinstance(jobs, list) or any(not isinstance(job, dict) for job in jobs):
        return False
    matches = [job for job in jobs if job.get('name') == 'voice-history-audit']
    return (len(matches) == 1 and matches[0].get('status') == 'completed'
            and matches[0].get('conclusion') == 'success')


def requires_audit(repo, baseline, target):
    if not isinstance(baseline, str) or not isinstance(target, str):
        return True
    if not SHA.fullmatch(baseline) or not SHA.fullmatch(target):
        return True
    env = dict(os.environ, GIT_NO_REPLACE_OBJECTS='1')

    def git(*args):
        return subprocess.run(['git', '-C', str(repo), *args], env=env,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              timeout=60, check=True).stdout

    try:
        if git('rev-parse', '--is-shallow-repository').strip() != b'false':
            return True
        for commit in (baseline, target):
            if git('rev-parse', '--verify', commit + '^{commit}').strip().decode('ascii') != commit:
                return True
        git('merge-base', '--is-ancestor', baseline, target)
        commits = git('rev-list', '--topo-order', baseline + '..' + target).splitlines()
        for commit in commits:
            if not SHA.fullmatch(commit.decode('ascii')):
                return True
            # F13 examines merge topology and source contributions, including
            # merges whose resulting tree has no guarded-path difference.
            parents = git('rev-list', '--parents', '-n', '1', commit.decode('ascii')).split()
            if len(parents) > 2:
                return True
            names = git('diff-tree', '--root', '-m', '--no-renames', '--no-commit-id', '--name-only',
                        '-r', '-z', commit.decode('ascii')).split(b'\0')
            for raw in filter(None, names):
                path = raw.decode('utf-8', errors='strict')
                # This narrow exception is for static frontend assets only.
                # Every unknown input and every audit implementation change
                # invalidates the baseline until its independence is proven.
                if not path.startswith('src/frontend/assets/'):
                    return True
        return False
    except (OSError, subprocess.SubprocessError, UnicodeError, ValueError):
        return True


def main():
    repository = os.environ.get('GITHUB_REPOSITORY', '')
    token = os.environ.get('GH_TOKEN', '')
    target = os.environ.get('VOICE_HISTORY_TARGET_SHA', '')
    api = os.environ.get('GITHUB_API_URL', 'https://api.github.com').rstrip('/')
    result = {'run_audit': True, 'reason': 'nightly baseline unavailable or affected'}
    try:
        if not token or not api.startswith('https://'):
            raise ValueError('missing API credentials')

        def get(path):
            request = urllib.request.Request(api + path, headers={
                'Authorization': 'Bearer ' + token,
                'Accept': 'application/vnd.github+json',
                'X-GitHub-Api-Version': '2022-11-28',
            })
            with urllib.request.urlopen(request, timeout=30) as response:
                return json.load(response)

        found = find_nightly(repository, get)
        if found and not requires_audit('.', found[0], target):
            result = {'run_audit': False, 'reason': 'nightly baseline unchanged',
                      'baseline_sha': found[0], 'run_id': found[1], 'attempt': found[2]}
    except (OSError, ValueError, TypeError, AttributeError, urllib.error.URLError, json.JSONDecodeError):
        pass
    print(json.dumps(result, sort_keys=True))
    output = os.environ.get('GITHUB_OUTPUT')
    if output:
        with open(output, 'a', encoding='utf-8') as stream:
            stream.write('run_audit=' + str(result['run_audit']).lower() + '\n')
    return 0


if __name__ == '__main__':
    sys.exit(main())
