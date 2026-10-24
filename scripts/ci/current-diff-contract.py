"""Bounded validation of the current candidate (never trusts cached results)."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys


class Refused(Exception):
    pass


def git(repo, *args):
    result = subprocess.run(['git', '-C', str(repo), *args], capture_output=True, timeout=90)
    if result.returncode:
        raise Refused('git_command: ' + args[0])
    return result.stdout


def commit(repo, value, label):
    # Do not accept Git revision expressions from event metadata.
    if value != 'HEAD' and not re.fullmatch('[0-9a-fA-F]{40}', value):
        raise Refused(label + '_identity')
    try:
        return git(repo, 'rev-parse', '--verify', value + '^{commit}').decode().strip()
    except Refused as error:
        raise Refused(label + '_object') from error


def entries(repo, revision):
    return dict(row.split(b'\t', 1)[::-1] for row in
                git(repo, 'ls-tree', '-rz', revision).split(b'\0') if row)


def migration(path):
    return path.endswith('.sql') and (
        path.startswith('src/backend/migrations/') or
        path.startswith('src/backend/auth/src/main/resources/db/migration/'))


ACTIVATION = re.compile(
    r'type\s+\w*(?:Coordinator|Bridge|Publisher|Handler|Worker|Runner)\b|'
    r'New\w*(?:Coordinator|Bridge|Publisher|Handler|Worker|Runner)\s*\(|'
    r'Register\w*Lifecycle|LifecycleStore\s*:|go\s+func|'
    r'NewJetStreamPublisher|redis\.New|livekit\.New|time\.NewTicker|'
    r'\.Publish\s*\(|\w*(?:coordinator|worker|runner|bridge|publisher)\w*\.(?:Run|Start)\s*\(|'
    r'\.(?:Handle|HandleFunc)\s*\([^)]*(?:lifecycle|room)', re.S)


def validate(repo, base, target='HEAD', pr_head=None):
    base = commit(repo, base, 'base')
    target = commit(repo, target, 'target')
    try:
        git(repo, 'merge-base', '--is-ancestor', base, target)
    except Refused as error:
        raise Refused('base_ancestry') from error
    if git(repo, 'diff', '--name-only', 'HEAD', '--') or git(repo, 'diff', '--cached', '--name-only'):
        raise Refused('dirty_worktree')
    if pr_head:
        pr_head = commit(repo, pr_head, 'pr_head')
        parents = git(repo, 'rev-list', '--parents', '-n', '1', target).decode().split()[1:]
        if parents != [base, pr_head]:
            raise Refused('merge_parents')
        try:
            expected_tree = git(repo, 'merge-tree', '--write-tree', base, pr_head).decode().splitlines()[0]
        except Refused as error:
            raise Refused('merge_tree_conflict') from error
        actual_tree = git(repo, 'rev-parse', target + '^{tree}').decode().strip()
        if expected_tree != actual_tree:
            raise Refused('merge_tree_mismatch')
    before, after = entries(repo, base), entries(repo, target)
    changed = sorted(path for path in before.keys() | after.keys() if before.get(path) != after.get(path))
    for raw_path in changed:
        path = raw_path.decode('utf-8')
        if migration(path) and raw_path in before:
            raise Refused('immutable_migration: ' + path)
        if not path.startswith('src/backend/voice/') or not path.endswith('.go') or path.endswith('_test.go'):
            continue
        relative = path.removeprefix('src/backend/voice/')
        lifecycle = relative.startswith('internal/roomlifecycle/')
        root_file = '/' not in relative
        if lifecycle and Path(path).name not in (
                'lifecycle_store.go', 'postgres_store.go', 'postgres_decision.go', 'postgres_delivery.go'):
            raise Refused('source_disabled_surface: ' + path)
        if root_file and raw_path not in before and relative not in ('main.go', 'health.go', 'database.go'):
            raise Refused('source_disabled_surface: ' + path)
        if raw_path not in after:
            continue
        if lifecycle:
            content = git(repo, 'show', target + ':' + path).decode('utf-8')
        elif relative in ('main.go', 'health.go', 'database.go'):
            patch = git(repo, 'diff', '--no-ext-diff', '--unified=0', base, target, '--', path).decode('utf-8')
            content = '\n'.join(line[1:] for line in patch.splitlines()
                                if line.startswith('+') and not line.startswith('+++'))
        else:
            continue
        if ACTIVATION.search(content):
            raise Refused('source_disabled_activation: ' + path)
        if (lifecycle or relative == 'database.go') and re.search(
                r'go-redis|nats\.go|internal/(grpcsvc|livekit|s2s|store|voiceevents)', content):
            raise Refused('source_disabled_adapter: ' + path)
    return {'schema': 'voice-current-diff-v1', 'base': base, 'target': target,
            'pr_head': pr_head, 'changed_paths': [p.decode('utf-8') for p in changed],
            'validator_sha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
            'cached': False}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument('--base', required=True)
    parser.add_argument('--target', default='HEAD')
    parser.add_argument('--pr-head')
    args = parser.parse_args()
    try:
        print(json.dumps(validate(args.repo, args.base, args.target, args.pr_head), sort_keys=True))
    except (Refused, subprocess.TimeoutExpired, UnicodeError) as error:
        print('current-diff-contract: ' + str(error), file=sys.stderr)
        sys.exit(1)
