"""Create and verify source-bound receipts for selected Go integration jobs."""
import argparse
import hashlib
import json
from pathlib import Path
import sys

from go_integration_receipts import make_receipt, read_events, summarize_events, verify_matrix, verify_no_skips


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    for name in ('produce', 'verify', 'verify-diagnostics'):
        command = sub.add_parser(name)
        command.add_argument('--inventory', required=True)
        command.add_argument('--candidate', required=True)
        command.add_argument('--run', required=True)
        command.add_argument('--attempt', required=True)
    producer = sub.choices['produce']
    producer.add_argument('--service', required=True)
    producer.add_argument('--events', required=True)
    producer.add_argument('--exit-code', required=True, type=int)
    producer.add_argument('--output', required=True)
    producer.add_argument('--reject-skips', action='store_true',
                          help='reject every skipped test in inventory packages')
    verifier = sub.choices['verify']
    verifier.add_argument('--services', required=True, help='JSON array of selected service names')
    verifier.add_argument('--receipt', action='append', default=[])
    verifier.add_argument('--events', action='append', default=[])
    verifier.add_argument('--require-no-skips', action='store_true',
                          help='require strict no-skip evidence for every service')
    diagnostic = sub.choices['verify-diagnostics']
    diagnostic.add_argument('--services', required=True, help='JSON array of selected service names')
    diagnostic.add_argument('--directory', required=True, help='downloaded per-member artifact directories')
    args = parser.parse_args()
    try:
        inventory = json.loads(Path(args.inventory).read_text(encoding='utf-8'))
        identity = {'candidate': args.candidate, 'run': args.run, 'attempt': args.attempt}
        if args.command == 'produce':
            raw = Path(args.events).read_bytes()
            events = read_events(raw.decode('utf-8'))
            receipt = make_receipt(args.service, identity, inventory, events, args.exit_code)
            packages = sorted({item[0] for item in inventory[args.service]}) if args.reject_skips else []
            if packages and verify_no_skips(events, packages):
                raise ValueError('a required package contains skipped tests')
            receipt['no_skips_packages'] = packages
            receipt['schema'] = 'voice-go-integration-receipt-v1'
            receipt['event_sha256'] = hashlib.sha256(raw).hexdigest()
            with Path(args.output).open('x', encoding='utf-8') as output:
                json.dump(receipt, output, sort_keys=True)
                output.write('\n')
            print('Go integration receipt created for ' + args.service)
        elif args.command == 'verify-diagnostics':
            services = json.loads(args.services)
            # Validate names and identity before they become filesystem paths.
            verify_matrix(services, inventory, [], identity)
            if not services:
                raise ValueError('selected integration matrix must not be empty')
            directory = Path(args.directory)
            expected = {'go-integration-' + name + '-' + args.attempt for name in services}
            if not directory.is_dir() or {p.name for p in directory.iterdir()} != expected:
                raise ValueError('downloaded artifact members do not match selected services')
            verified = []
            for service in services:
                folder = directory / ('go-integration-' + service + '-' + args.attempt)
                if folder.is_symlink() or not folder.is_dir():
                    raise ValueError('invalid artifact directory')
                event_path = folder / 'go-integration-events.jsonl'
                summary_path = folder / 'go-integration-summary.json'
                if event_path.is_symlink() or summary_path.is_symlink():
                    raise ValueError('artifact files must not be symlinks')
                raw = event_path.read_bytes()
                events = read_events(raw.decode('utf-8'))
                summary = json.loads(summary_path.read_text(encoding='utf-8'))
                reconstructed = summarize_events(events)
                reconstructed.update(schema='voice-go-integration-diagnostic-v1', service=service,
                                     identity=identity, exit_code=0,
                                     event_sha256=hashlib.sha256(raw).hexdigest())
                if summary != reconstructed or type(summary.get('exit_code')) is not int:
                    raise ValueError('diagnostic summary differs from original events or identity')
                verified.append(make_receipt(service, identity, inventory, events, 0))
            failures = verify_matrix(services, inventory, verified, identity)
            if failures:
                raise ValueError('missing or invalid matrix evidence: ' + ', '.join(failures))
            print('Go integration matrix diagnostics verified for all selected services')
        else:
            services = json.loads(args.services)
            receipts = [json.loads(Path(path).read_text(encoding='utf-8')) for path in args.receipt]
            if len(args.events) != len(receipts):
                raise ValueError('every receipt requires its original event stream')
            failures = verify_matrix(services, inventory, receipts, identity)
            if failures:
                raise ValueError('missing or invalid matrix evidence: ' + ', '.join(failures))
            for receipt, event_path in zip(receipts, args.events):
                raw = Path(event_path).read_bytes()
                if receipt.get('schema') != 'voice-go-integration-receipt-v1':
                    raise ValueError('unsupported receipt schema')
                if receipt.get('event_sha256') != hashlib.sha256(raw).hexdigest():
                    raise ValueError('event stream digest does not match receipt')
                reconstructed = make_receipt(receipt['service'], identity, inventory,
                                             read_events(raw.decode('utf-8')), receipt['exit_code'])
                if reconstructed['events'] != receipt['events']:
                    raise ValueError('receipt does not match its event stream')
                scoped = receipt.get('no_skips_packages')
                expected = sorted({item[0] for item in inventory[receipt['service']]})
                if scoped not in ([], expected):
                    raise ValueError('invalid no-skip scope')
                if args.require_no_skips and scoped != expected:
                    raise ValueError('required no-skip evidence is missing')
                if scoped and verify_no_skips(read_events(raw.decode('utf-8')), scoped):
                    raise ValueError('a required package contains skipped tests')
            print('Go integration matrix evidence accepted')
    except (OSError, ValueError, TypeError, KeyError, AttributeError) as exc:
        print('Go integration matrix evidence rejected: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
