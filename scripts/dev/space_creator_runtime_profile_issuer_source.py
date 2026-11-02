"""Source identity for the fixed Creator root issuer bundle."""
from pathlib import Path
import re

import realtime_p3_create_only_source as source

# Exact integrated payload independently reviewed before this source pin.
REVIEWED_SOURCE='7a2d8245868ce65a67dd1e2943302f752e8b1c6c'
FILES=('space_creator_runtime_profile_issuer.py','space_creator_runtime_profile_issuer_packet.py',
       'space_creator_runtime_profile_issuer_export.py','space_p3_issuer_preflight_metadata_root.py',
       'space_p3_issuer_preflight_root.py','space_p3_issuer.py','resource_fencing.py','credentials.py',
       'reset_credentials.py','realtime_p3_missing_issuer.py','realtime_p3_root_export.py')
TEMPLATE='scripts/dev/realtime_p3_root_launch.template.sh'

class SourceError(RuntimeError): pass

def verify(repo,candidate):
    try:
        if (not Path(repo).is_absolute() or not isinstance(candidate,str) or not re.fullmatch(r'[a-f0-9]{40}',candidate) or
            not isinstance(REVIEWED_SOURCE,str) or not re.fullmatch(r'[a-f0-9]{40}',REVIEWED_SOURCE)): raise ValueError()
        for path in ['scripts/dev/'+name for name in FILES]+[TEMPLATE]:
            expected=source._blob(repo,REVIEWED_SOURCE,path); actual=source._blob(repo,candidate,path)
            if expected!=actual or source._disk(repo,path)!=actual: raise ValueError()
    except Exception: raise SourceError('Creator issuer source unavailable') from None
