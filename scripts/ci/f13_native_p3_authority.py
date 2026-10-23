"""Exact pinned Native source dispatch; versions supplied by callers grant nothing."""
import argparse
import json
from pathlib import Path
import sys

# F54 also loads this file by absolute importlib spec from extracted fixtures.
sys.path.insert(0, str(Path(__file__).resolve().parent))
import f13_native_p3_source as legacy
import f13_native_p3_successor as successor

Rejected = legacy.Rejected
RECORD_PATH = legacy.RECORD_PATH
entry, rows, load_record = legacy.entry, legacy.rows, legacy.load_record


def qualify(*args):
    try:
        return legacy.qualify(*args)
    except Rejected as old_error:
        try:
            return successor.qualify(*args)
        except Rejected as new_error:
            raise Rejected('v1=' + str(old_error) + ';v2=' + str(new_error)) from new_error


def verified_record(repo, binding):
    """Choose only a fixed tuple, and independently reconstruct its full record."""
    key = tuple(binding.get(k) for k in ('version', 'content_base', 'snapshot', 'record', 'record_sha256'))
    old_key = ('NATIVE-P3-SOURCE-v1', legacy.PINNED_BASE, legacy.PINNED_SOURCE,
               legacy.PINNED_RECORD_COMMIT, legacy.PINNED_RECORD_SHA256)
    if key == old_key:
        return legacy, legacy.load_record(repo)
    record = successor.load_record(repo)
    new_key = (successor.VERSION, record['base'], record['source'], successor.PINNED_RECORD_COMMIT,
               successor.PINNED_RECORD_SHA256)
    legacy.require(key == new_key, 'native_unknown_authority_tuple')
    return successor, record


def audit_map(repo, binding, map_path, history_path):
    verifier, record = verified_record(repo, binding)
    legacy.require(binding.get('paths') == [r['path'] for r in record['projection']],
                   'native_complete_projection')
    return verifier.audit_map(repo, binding, map_path, history_path)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ('repo', 'base', 'source', 'target', 'event', 'ref', 'event-sha', 'before', 'base-ref'):
        parser.add_argument('--' + key, required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--map')
    parser.add_argument('--history')
    args = parser.parse_args()
    try:
        binding = qualify(args.repo, args.base, args.source, args.target, args.event, args.ref,
                          args.event_sha, args.before, args.base_ref)
        legacy.require(not args.map or args.history, 'missing_history')
        result = audit_map(args.repo, binding, args.map, args.history) if args.map else binding
        Path(args.output).write_text(json.dumps(result, sort_keys=True) + '\n')
        if not args.map:
            print('\n'.join(binding['paths']))
    except (Rejected, ValueError, KeyError, TypeError, OSError) as exc:
        print('NATIVE-P3 source rejected: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
