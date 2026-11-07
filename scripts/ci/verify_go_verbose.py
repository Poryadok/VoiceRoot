"""Reject empty, skipped or incomplete single-test Go verbose proof logs."""
import argparse
from pathlib import Path
import re
import sys


def check(log, name):
    if not isinstance(log, str) or not log.endswith('\n') or not re.fullmatch(r'Test[A-Za-z0-9_]+', name):
        raise ValueError('invalid test name or truncated log')
    lines = log.splitlines()
    run = re.compile(r'^=== RUN\s+' + re.escape(name) + r'\s*$')
    passed = re.compile(r'^--- PASS:\s+' + re.escape(name) + r'\s+\([0-9.]+s\)\s*$')
    rejected = re.compile(r'^\s*--- (?:SKIP|FAIL):\s+')
    runs = [i for i, line in enumerate(lines) if run.fullmatch(line)]
    passes = [i for i, line in enumerate(lines) if passed.fullmatch(line)]
    terminal = [i for i, line in enumerate(lines) if line == 'PASS']
    if (len(runs) != 1 or len(passes) != 1 or len(terminal) != 1 or
            not runs[0] < passes[0] < terminal[0] or any(rejected.match(line) for line in lines)):
        raise ValueError('required Go test did not run and pass exactly once without skips')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--log', required=True)
    parser.add_argument('--test', required=True)
    args = parser.parse_args()
    try:
        check(Path(args.log).read_text(encoding='utf-8-sig'), args.test)
    except (OSError, UnicodeError, ValueError) as exc:
        print('Go proof rejected: ' + str(exc), file=sys.stderr)
        return 1
    print('Go proof accepted: ' + args.test)
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
