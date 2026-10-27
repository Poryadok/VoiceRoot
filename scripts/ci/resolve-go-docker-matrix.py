"""Select PR image builds from direct changes and Docker COPY source consumers.

S2S test dependencies do not imply image dependencies. Shared packages, generated
code, module manifests and unclassified global wiring still select every image.
Read COPY inputs from the actual Dockerfiles rather than duplicating their edges.
"""
import json
import os
from pathlib import Path
import shlex

ROOT = Path(__file__).resolve().parents[2]


def select(filters, force_full=False):
    services = sorted(f.parent.name for f in (ROOT / 'src/backend').glob('*/Dockerfile')
                      if (f.parent / 'go.mod').is_file())
    if force_full or any(filters.get(k) == 'true' for k in
                         ('global', 'ci_global', 'protos', 'pkg', 'go_build_shared')):
        return services
    changed = {s for s in services if filters.get('svc_' + s) == 'true'}
    selected = set(changed)
    for service in services:
        dockerfile = ROOT / 'src/backend' / service / 'Dockerfile'
        for line in dockerfile.read_text(encoding='utf-8').splitlines():
            line = line.strip()
            if line.upper().startswith('ADD '):
                return services
            if not line.upper().startswith('COPY '):
                continue
            if '--from=' in line:
                continue
            # Unknown COPY grammar must widen the build, never silently drop it.
            if '\\' in line or '[' in line or '$' in line or '--' in line:
                return services
            sources = shlex.split(line)[1:-1]
            for source in sources:
                parts = source.removeprefix('src/backend/').split('/')
                if parts[0] in ('.', '..') or parts[0].startswith('$'):
                    return services
                # Generated pb and module manifests have the broad shared filter.
                if 'pb' not in parts and parts[0] in changed:
                    selected.add(service)
    return [s for s in services if s in selected]


if __name__ == '__main__':
    filters = json.loads(os.environ.get('FILTER_JSON') or '{}')
    if not filters:
        filters = {k.removeprefix('FILTER_'): v for k, v in os.environ.items()
                   if k.startswith('FILTER_')}
    print(json.dumps(select(filters, os.environ.get('FORCE_FULL') == 'true')))
