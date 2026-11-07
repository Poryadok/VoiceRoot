"""Non-authoritative complete Git DAG inventory for source review.

This inventory does not establish review, runtime, or release authority.
"""
import base64
import hashlib
import json

import f13_native_p3_source as legacy


def raw_history(repo, base, source):
    """Reproduce the review's full-history, per-parent raw Git evidence."""
    base, source = map(legacy.oid, (base, source))
    legacy.require(legacy.git(repo, 'rev-parse', '--is-shallow-repository').strip() == b'false',
                   'source_dag_shallow')
    legacy.ancestor(repo, base, source)
    return legacy.git(repo, 'log', '--full-history', '-m', '--raw', '--no-abbrev',
                      '--no-renames', '--format=commit %H %T %P', base + '..' + source)


def inventory(repo, base, source):
    base, source = map(legacy.oid, (base, source))
    git, require = legacy.git, legacy.require
    require(git(repo, 'rev-parse', '--is-shallow-repository').strip() == b'false',
            'source_dag_shallow')
    legacy.ancestor(repo, base, source)
    commits = []
    lines = git(repo, 'rev-list', '--topo-order', '--reverse', '--parents',
                source, '^' + base).decode('ascii').splitlines()
    for line in lines:
        commit, *parents = line.split()
        legacy.oid(commit)
        for parent in parents:
            legacy.oid(parent)
        edges = []
        for parent in parents or [None]:
            if parent is None:
                raw = git(repo, 'diff-tree', '--root', '--no-commit-id', '-r',
                          '--raw', '-z', '--no-abbrev', '--no-renames', commit)
            else:
                raw = git(repo, 'diff', '--raw', '-z', '--no-abbrev',
                          '--no-renames', parent, commit)
            edges.append(dict(parent=parent, raw_sha256=hashlib.sha256(raw).hexdigest(),
                              raw_base64=base64.b64encode(raw).decode('ascii')))
        tree = git(repo, 'rev-parse', commit + '^{tree}').decode('ascii').strip()
        commits.append(dict(commit=commit, parents=parents, tree=legacy.oid(tree), edges=edges))
    return dict(schema='F13-SOURCE-DAG-INVENTORY-v1', status='OPEN', authority=False,
                base=base, source=source, commits=commits)


def verify(repo, record, base, source):
    """Recompute all parent edges; equality alone grants no authority."""
    canonical = lambda value: json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False)
    legacy.require(canonical(record) == canonical(inventory(repo, base, source)), 'source_dag_mismatch')
