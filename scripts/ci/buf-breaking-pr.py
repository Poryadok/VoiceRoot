"""Compare a checked-out PR integration against its immutable event base."""
import argparse
import json
from pathlib import Path
import re
import subprocess
import sys


class Refused(Exception):
    pass


def git(repo, *args):
    result = subprocess.run(['git', '-C', str(repo), *args], capture_output=True,
                            text=True, encoding='utf-8', timeout=30)
    if result.returncode:
        raise Refused('git_command: ' + args[0])
    return result.stdout.strip()


def commit(repo, value, label):
    if not re.fullmatch(r'[0-9a-fA-F]{40}', value):
        raise Refused(label + '_identity')
    try:
        return git(repo, 'rev-parse', '--verify', value + '^{commit}')
    except Refused as error:
        raise Refused(label + '_object') from error


def identities(repo, base, head, merge):
    base = commit(repo, base, 'base')
    head = commit(repo, head, 'head')
    merge = commit(repo, merge, 'merge')
    checkout = git(repo, 'rev-parse', '--verify', 'HEAD^{commit}')
    if checkout != merge:
        raise Refused('checkout_identity')
    parents = git(repo, 'rev-list', '--parents', '-n', '1', merge).split()[1:]
    if len(parents) != 2 or parents[1] != head:
        raise Refused('merge_parents')
    current_base = parents[0]
    try:
        git(repo, 'merge-base', '--is-ancestor', base, current_base)
    except Refused as error:
        raise Refused('base_ancestry') from error
    return {
        'checkout': checkout, 'event_base': base, 'base': current_base,
        'head': head, 'merge': merge,
        'parents': parents,
        'checkout_tree': git(repo, 'rev-parse', merge + '^{tree}'),
        'base_tree': git(repo, 'rev-parse', current_base + '^{tree}'),
        'head_tree': git(repo, 'rev-parse', head + '^{tree}'),
        'checkout_protos_tree': git(repo, 'rev-parse', merge + ':protos'),
        'base_protos_tree': git(repo, 'rev-parse', current_base + ':protos'),
    }


def run_buf(repo, base):
    # Keep the real Buf rules and exit status; never resolve a mutable branch.
    return subprocess.run(
        ['buf', 'breaking', 'protos', '--against', '.git#ref=' + base + ',subdir=protos'],
        cwd=repo, timeout=180).returncode


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, default=Path.cwd())
    parser.add_argument('--base', required=True)
    parser.add_argument('--head', required=True)
    parser.add_argument('--merge', required=True)
    args = parser.parse_args(argv)
    try:
        receipt = identities(args.repo, args.base, args.head, args.merge)
        print(json.dumps(receipt, sort_keys=True), flush=True)
        return run_buf(args.repo, receipt['base'])
    except (Refused, OSError, subprocess.TimeoutExpired) as error:
        print('buf-breaking-pr: ' + str(error), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
