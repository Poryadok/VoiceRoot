"""Decode and validate one E2E catalog selection before emitting any test names."""
from pathlib import Path
import json
import os
import re
import sys

try:
    import yaml
except ImportError:
    print('ERROR: PyYAML required; run python3 -m pip install -r scripts/ci/requirements-e2e.txt', file=sys.stderr)
    raise SystemExit(1)


LIST_SECTIONS = {
    'smoke_gateway', 'smoke_flutter', 'full_flutter', 'restart_proof_gateway',
    'a1_multi_account_gateway', 'a1_flutter_profile_handoff',
}


class UniqueLoader(yaml.SafeLoader):
    def construct_mapping(self, node, deep=False):
        self.flatten_mapping(node)
        result = {}
        for key_node, value_node in node.value:
            key = self.construct_object(key_node, deep=deep)
            if key in result:
                raise ValueError('duplicate YAML mapping key')
            result[key] = self.construct_object(value_node, deep=deep)
        return result


def selection(path, section):
    if section not in LIST_SECTIONS | {'full_gateway_run'}:
        raise ValueError('unknown section')
    document = yaml.load(path.read_text(encoding='utf-8'), Loader=UniqueLoader)
    if type(document) is not dict or section not in document:
        raise ValueError('missing section')
    value = document[section]
    if section == 'full_gateway_run':
        if type(value) is not str or not value.strip() or any(c in value for c in '\r\n\0'):
            raise ValueError('full_gateway_run must be a nonempty single-line string')
        return [value]
    if type(value) is not list or not value:
        raise ValueError('section must be a nonempty list')
    pattern = r'Test[A-Za-z0-9_]+' if section.endswith('gateway') else r'test/[A-Za-z0-9_./-]+\.dart'
    if any(type(item) is not str or re.fullmatch(pattern, item) is None for item in value):
        raise ValueError('section contains an invalid test name')
    if len(set(value)) != len(value):
        raise ValueError('section contains duplicate test names')
    return value


def affected_flutter(path, changed_paths, root=None):
    """Return changed catalogued suites in the catalog's deterministic order."""
    if type(changed_paths) is not list or any(type(value) is not str for value in changed_paths):
        raise ValueError('changed paths must be a list of strings')
    changed = set(changed_paths)
    root = Path(root) if root is not None else Path(__file__).resolve().parents[2]
    full = selection(path, 'full_flutter')
    separately_proved = selection(path, 'a1_flutter_profile_handoff')
    known = {'src/frontend/' + suite for suite in full + separately_proved}
    unknown = [name for name in changed if name.startswith('src/frontend/test/')
               and 'live' in name and name.endswith('test.dart') and name not in known
               and ((root / name).exists() or (root / name).is_symlink())]
    if unknown:
        raise ValueError('changed live Flutter suite is absent from the E2E catalog')
    return [suite for suite in full
            if 'src/frontend/' + suite in changed]


def main():
    if len(sys.argv) != 3:
        print('ERROR: expected manifest path and section', file=sys.stderr)
        return 1
    try:
        if sys.argv[2] == 'affected_flutter':
            names = affected_flutter(Path(sys.argv[1]), json.loads(os.environ['VOICE_AFFECTED_PATHS_JSON']))
        else:
            names = selection(Path(sys.argv[1]), sys.argv[2])
    except (OSError, UnicodeError, ValueError, TypeError, KeyError, RecursionError, yaml.YAMLError) as error:
        print(f'ERROR: invalid E2E manifest selection ({type(error).__name__})', file=sys.stderr)
        return 1
    # Output is atomic with respect to validation: no usable prefix on failure.
    if names:
        sys.stdout.write('\n'.join(names) + '\n')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
