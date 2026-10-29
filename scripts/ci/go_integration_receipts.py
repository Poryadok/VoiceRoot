"""Verify explicit required integration cases in Go's JSON event stream."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys


def read_events(text):
    if not text or not text.endswith('\n'):
        raise ValueError('empty or truncated Go test event stream')
    events = []
    for line in text.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ValueError('invalid Go test event') from exc
        if not isinstance(event, dict) or not isinstance(event.get('Action'), str):
            raise ValueError('invalid Go test event shape')
        events.append(event)
    return events


def summarize_events(events):
    counts = {'pass': 0, 'skip': 0, 'fail': 0}
    skipped = []
    packages = {}
    for event in events:
        action = event.get('Action')
        package = event.get('Package')
        test = event.get('Test')
        if action not in counts or not isinstance(package, str):
            continue
        if isinstance(test, str):
            counts[action] += 1
            if action == 'skip':
                skipped.append([package, test])
        elif test is None:
            if package in packages:
                raise ValueError('duplicate package terminal event')
            packages[package] = action
    return {'counts': counts, 'skipped': sorted(skipped), 'packages': dict(sorted(packages.items()))}


def verify_required(events, required, exit_code):
    if (not isinstance(required, (list, tuple)) or not required or
            any(not isinstance(item, (list, tuple)) or len(item) != 2 or
                not all(isinstance(x, str) and x for x in item) for item in required)):
        raise ValueError('required cases must be a nonempty unique package/test inventory')
    required = [tuple(item) for item in required]
    if len(set(required)) != len(required):
        raise ValueError('duplicate required case')
    observed = {key: [] for key in required}
    packages = {package: [] for package, _ in required}
    for event in events:
        key = (event.get('Package'), event.get('Test'))
        action = event.get('Action')
        if key in observed and action in ('run', 'pass', 'skip', 'fail'):
            observed[key].append(action)
        if key[1] is None and key[0] in packages and action in ('pass', 'skip', 'fail'):
            packages[key[0]].append(action)
    return [key for key in required if exit_code != 0 or observed[key] != ['run', 'pass']
            or packages[key[0]] != ['pass']]


def verify_no_skips(events, packages):
    if (not isinstance(packages, (list, tuple)) or not packages or
            any(not isinstance(package, str) or not package for package in packages) or
            len(packages) != len(set(packages))):
        raise ValueError('skip scope must contain unique package names')
    failures = []
    for package in packages:
        scoped = [event for event in events if event.get('Package') == package]
        terminal = [event.get('Action') for event in scoped if event.get('Test') is None
                    and event.get('Action') in ('pass', 'fail', 'skip')]
        if terminal != ['pass']:
            failures.append(package + ': missing or failed package')
        failures.extend(package + ': ' + str(event.get('Test') or 'package')
                        for event in scoped if event.get('Action') == 'skip')
    return failures


def verify_matrix(services, inventory, receipts, identity):
    if (not isinstance(identity, dict) or set(identity) != {'candidate', 'run', 'attempt'} or
            not isinstance(identity['candidate'], str) or not re.fullmatch(r'[0-9a-f]{40}', identity['candidate']) or
            identity['candidate'] == '0' * 40 or
            any(not isinstance(identity[key], str) or not re.fullmatch(r'[1-9][0-9]*', identity[key])
                for key in ('run', 'attempt')) or
            not isinstance(services, list) or any(not isinstance(s, str) or
                                                  not re.fullmatch(r'[a-z][a-z0-9]*', s)
                                                  for s in services) or
            len(services) != len(set(services)) or not isinstance(inventory, dict) or
            not isinstance(receipts, list)):
        raise ValueError('invalid integration matrix inputs')
    by_service = {}
    for receipt in receipts:
        if not isinstance(receipt, dict) or receipt.get('service') not in services:
            return ['unexpected receipt']
        name = receipt['service']
        if name in by_service:
            return ['duplicate receipt: ' + name]
        by_service[name] = receipt
    failures = []
    for name in services:
        receipt = by_service.get(name)
        try:
            required = inventory.get(name, [])
            prefix = 'voice/backend/' + name
            if any(not isinstance(item, (list, tuple)) or len(item) != 2 or
                   not isinstance(item[0], str) or
                   not (item[0] == prefix or item[0].startswith(prefix + '/')) for item in required):
                raise ValueError('inventory package outside selected service')
            if (not isinstance(receipt, dict) or receipt.get('identity') != identity or
                    type(receipt.get('exit_code')) is not int or
                    not isinstance(receipt.get('events'), list) or
                    verify_required(receipt['events'], required, receipt['exit_code'])):
                failures.append(name)
        except (ValueError, TypeError, AttributeError):
            failures.append(name)
    return failures


def make_receipt(service, identity, inventory, events, exit_code):
    receipt = {'service': service, 'identity': identity, 'exit_code': exit_code, 'events': events}
    if verify_matrix([service], inventory, [receipt], identity):
        raise ValueError('required integration cases did not complete successfully')
    required = {tuple(item) for item in inventory[service]}
    packages = {package for package, _ in required}
    # Logs remain separate; carry only evidence needed to recheck selected cases.
    receipt['events'] = [
        {key: event[key] for key in ('Package', 'Test', 'Action') if key in event}
        for event in events
        if (event.get('Action') in ('run', 'pass', 'skip', 'fail') and
            ((event.get('Package'), event.get('Test')) in required or
             (event.get('Test') is None and event.get('Package') in packages and
              event.get('Action') in ('pass', 'skip', 'fail'))))
    ]
    return receipt


def main():
    parser = argparse.ArgumentParser(description='Require one exact Go integration test result')
    parser.add_argument('--events', required=True)
    parser.add_argument('--package')
    parser.add_argument('--test')
    parser.add_argument('--exit-code', required=True, type=int)
    parser.add_argument('--summary')
    parser.add_argument('--service')
    parser.add_argument('--candidate')
    parser.add_argument('--run')
    parser.add_argument('--attempt')
    args = parser.parse_args()
    try:
        raw = Path(args.events).read_bytes()
        events = read_events(raw.decode('utf-8'))
        if args.summary:
            identity = {'candidate': args.candidate, 'run': args.run, 'attempt': args.attempt}
            # Reuse matrix input validation; this produces a diagnostic, not a PASS receipt.
            verify_matrix([args.service], {}, [], identity)
            summary = summarize_events(events)
            summary.update(schema='voice-go-integration-diagnostic-v1', service=args.service,
                           identity=identity, exit_code=args.exit_code,
                           event_sha256=hashlib.sha256(raw).hexdigest())
            Path(args.summary).write_text(json.dumps(summary, sort_keys=True, indent=2) + '\n', encoding='utf-8')
            print(json.dumps(summary, sort_keys=True))
            return 0 if args.exit_code == 0 else 1
        if not args.package or not args.test:
            raise ValueError('package and test are required for case verification')
        failures = verify_required(events, [(args.package, args.test)], args.exit_code)
        if failures:
            raise ValueError('required integration case did not complete successfully')
    except (OSError, ValueError) as exc:
        print('Go integration evidence rejected: ' + str(exc), file=sys.stderr)
        return 1
    print('Go integration evidence accepted for ' + args.package + '/' + args.test)
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
