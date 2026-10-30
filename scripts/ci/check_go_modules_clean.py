"""Reject changes to committed Go module inputs in a clean CI checkout."""
import json
from pathlib import Path
import re
import subprocess
import sys


def check(root, services):
    root = Path(root).resolve()
    if (not isinstance(services, list) or not services or
            any(not isinstance(name, str) or not re.fullmatch(r'[a-z][a-z0-9]*', name)
                or name == 'pkg' for name in services) or len(services) != len(set(services))):
        raise ValueError('invalid Go service inventory')
    names = ['pkg', *sorted(services)]
    modules = ['src/backend/' + name + '/go.mod' for name in names]
    paths = [path for name in names for path in
             ('src/backend/' + name + '/go.mod', 'src/backend/' + name + '/go.sum')]
    tracked = subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z', '--', *modules])
    if set(tracked.decode('utf-8').split('\0')) - {''} != set(modules):
        raise ValueError('a required go.mod is not tracked')
    status = subprocess.check_output(['git', '-C', str(root), 'status', '--porcelain=v1', '-z',
                                      '--untracked-files=all', '--ignored', '--', *paths])
    if status:
        raise ValueError('Go module files differ from the CI checkout: ' +
                         status.decode('utf-8', errors='replace').replace('\0', ', ').rstrip(', '))


def main():
    root = Path(__file__).resolve().parents[2]
    try:
        inventory = json.loads((root / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        if not isinstance(inventory, dict):
            raise ValueError('invalid integration inventory')
        check(root, list(inventory))
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print('Go module source check failed: ' + str(error), file=sys.stderr)
        return 1
    print('Go module inputs match the CI checkout.')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
