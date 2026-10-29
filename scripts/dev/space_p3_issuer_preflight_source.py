"""Exact candidate and reviewed root-helper source binding for preflight."""
from pathlib import Path
import re
import realtime_p3_create_only_source as git_source
import space_p3_issuer_source as issuer_source

# Fixed Git source; promotion still requires the independent security review.
REVIEWED_PREFLIGHT_SHA='dfa43167b2dd953e12a7f767feaf022ecb926629'
PATHS=('scripts/dev/space_p3_issuer_preflight_root.py','scripts/dev/space_p3_issuer_preflight_metadata_root.py',
       'scripts/dev/resource_fencing.py')


def verify(source_root,source_sha):
    if (not Path(source_root).is_absolute() or not re.fullmatch(r'[a-f0-9]{40}',source_sha)
            or not re.fullmatch(r'[a-f0-9]{40}',REVIEWED_PREFLIGHT_SHA)): raise ValueError()
    issuer_source.verify(source_root,source_sha)
    for path in ('scripts/dev/space_p3_issuer_bundle.py','scripts/dev/space_p3_issuer_preflight.py',
                 'scripts/dev/space_p3_issuer_preflight_source.py'):
        if git_source._blob(source_root,source_sha,path)!=git_source._disk(source_root,path): raise ValueError()
    for path in PATHS:
        candidate=git_source._blob(source_root,source_sha,path)
        if candidate!=git_source._blob(source_root,REVIEWED_PREFLIGHT_SHA,path) or candidate!=git_source._disk(source_root,path): raise ValueError()
