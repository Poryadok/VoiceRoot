"""Validate Flutter JSON test events; not a source-bound execution receipt."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import sys


def read_events(raw):
    if not isinstance(raw, bytes):
        raise ValueError('event stream must be bytes')
    try:
        encoding = 'utf-16' if raw.startswith((b'\xff\xfe', b'\xfe\xff')) else 'utf-8-sig'
        text = raw.decode(encoding)
        if not text.endswith('\n'):
            raise ValueError('event stream is truncated')
        def unique(pairs):
            value = {}
            for key, item in pairs:
                if key in value:
                    raise ValueError('duplicate JSON key')
                value[key] = item
            return value
        rows = [json.loads(line, object_pairs_hook=unique) for line in text.splitlines() if line.strip()]
    except (UnicodeError, json.JSONDecodeError) as error:
        raise ValueError('invalid Flutter event stream') from error
    if not rows or any(not isinstance(row, dict) for row in rows):
        raise ValueError('invalid Flutter event rows')
    return rows


def _path(path):
    if not isinstance(path, str) or not os.path.isabs(path):
        raise ValueError('suite paths must be absolute')
    return os.path.normcase(os.path.normpath(path))


def summarize(events, expected_suites, reject_skips=True):
    if (not isinstance(events, list) or not events or any(not isinstance(event, dict) for event in events)
            or not isinstance(expected_suites, list)
            or not expected_suites or type(reject_skips) is not bool):
        raise ValueError('invalid event summary inputs')
    expected = [_path(path) for path in expected_suites]
    if len(expected) != len(set(expected)):
        raise ValueError('duplicate expected suite')
    if events[0].get('type') != 'start' or events[-1].get('type') != 'done':
        raise ValueError('missing start or terminal event')
    suites, starts, finishes = {}, {}, {}
    all_suites = None
    done = 0
    for index, event in enumerate(events):
        if not isinstance(event, dict):
            raise ValueError('invalid event')
        kind = event.get('type')
        if kind == 'start':
            if index != 0:
                raise ValueError('duplicate start')
        elif kind == 'suite':
            suite = event.get('suite')
            if not isinstance(suite, dict) or type(suite.get('id')) is not int or suite['id'] in suites:
                raise ValueError('invalid or duplicate suite')
            suites[suite['id']] = _path(suite.get('path'))
        elif kind == 'allSuites':
            if all_suites is not None or type(event.get('count')) is not int:
                raise ValueError('invalid suite count')
            all_suites = event['count']
        elif kind == 'testStart':
            test = event.get('test')
            if (not isinstance(test, dict) or type(test.get('id')) is not int or test['id'] in starts
                    or type(test.get('suiteID')) is not int or test['suiteID'] not in suites
                    or not isinstance(test.get('name'), str) or not test['name']
                    or not isinstance(test.get('metadata'), dict)
                    or type(test['metadata'].get('skip')) is not bool):
                raise ValueError('invalid test start')
            starts[test['id']] = test
        elif kind == 'testDone':
            test_id = event.get('testID')
            if (type(test_id) is not int or test_id not in starts or test_id in finishes
                    or event.get('result') != 'success' or type(event.get('skipped')) is not bool
                    or type(event.get('hidden')) is not bool):
                raise ValueError('missing, duplicate or unsuccessful test result')
            if starts[test_id]['metadata']['skip'] and not event['skipped']:
                raise ValueError('declared skip did not match result')
            finishes[test_id] = event
        elif kind == 'done':
            done += 1
            if index != len(events) - 1 or event.get('success') is not True:
                raise ValueError('test runner did not finish successfully')
        elif kind == 'error':
            raise ValueError('Flutter test runner reported an error')
        elif kind not in ('group', 'print', 'debug'):
            raise ValueError('unsupported Flutter event type')
    if (done != 1 or all_suites != len(suites) or sorted(suites.values()) != sorted(expected)
            or len(suites) != len(set(suites.values())) or set(starts) != set(finishes)):
        raise ValueError('incomplete suites or test results')
    visible, hidden, skipped = [], 0, []
    for test_id, test in starts.items():
        result = finishes[test_id]
        item = {'suite': suites[test['suiteID']], 'name': test['name']}
        if result['skipped']:
            skipped.append(item)
        if result['hidden']:
            hidden += 1
        else:
            visible.append((item, result['skipped']))
    names = [(item['suite'], item['name']) for item, _ in visible]
    if ({item['suite'] for item, _ in visible} != set(expected)
            or len(names) != len(set(names)) or (reject_skips and skipped)):
        raise ValueError('empty, duplicate or skipped required tests')
    return {'tests': len(visible), 'passed': sum(not skip for _, skip in visible),
            'hidden': hidden, 'skipped': skipped, 'suites': sorted(suites.values())}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--events', required=True)
    parser.add_argument('--suite', action='append', required=True, help='absolute expected test file')
    parser.add_argument('--allow-skips', action='store_true')
    args = parser.parse_args()
    try:
        raw = Path(args.events).read_bytes()
        result = summarize(read_events(raw), args.suite, reject_skips=not args.allow_skips)
        result.update(schema='voice-flutter-event-summary-v1', event_sha256=hashlib.sha256(raw).hexdigest())
    except (OSError, ValueError) as error:
        print('Flutter event stream rejected: ' + str(error), file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
