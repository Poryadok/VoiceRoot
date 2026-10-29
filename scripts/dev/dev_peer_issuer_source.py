"""Candidate source must carry the exact independently reviewed peer issuer."""
from pathlib import Path
import re

import realtime_p3_create_only_source as source

# Set only after the whole fixed issuer/installer payload is reviewed.
REVIEWED_SOURCE=None
FILES=('dev_peer_issuer_metadata.py','dev_peer_issuer_preflight.py','dev_peer_issuer_service.py','dev_peer_issuer_authority.py',
       'dev_peer_issuer_client.py','dev_peer_issuer_packet.py','dev_peer_issuer_material.py','dev_peer_issuer_installer.py',
       'space_p3_issuer_preflight_metadata_root.py','space_p3_issuer_preflight_root.py',
       'space_creator_runtime_profile_issuer_packet.py','space_p3_issuer.py','realtime_p3_create_only_source.py',
       'resource_fencing.py','credentials.py','reset_credentials.py','realtime_p3_missing_issuer.py',
       'realtime_p3_root_export.py')

class SourceError(RuntimeError): pass

def verify(repo,candidate):
    try:
        if (not Path(repo).is_absolute() or not isinstance(candidate,str) or not re.fullmatch(r'[a-f0-9]{40}',candidate) or
            not isinstance(REVIEWED_SOURCE,str) or not re.fullmatch(r'[a-f0-9]{40}',REVIEWED_SOURCE)): raise ValueError()
        for name in FILES:
            path='scripts/dev/'+name
            expected=source._blob(repo,REVIEWED_SOURCE,path); actual=source._blob(repo,candidate,path)
            if expected!=actual or source._disk(repo,path)!=actual: raise ValueError()
    except Exception: raise SourceError('DEV peer issuer source unavailable') from None
