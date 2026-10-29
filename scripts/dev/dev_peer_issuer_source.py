"""Candidate source must carry the exact independently reviewed peer issuer."""
from pathlib import Path
import re

import realtime_p3_create_only_source as source

# Set only after the whole fixed issuer/installer payload is reviewed.
REVIEWED_SOURCE='dae4cbf1080108f2909b2f2419ea59f9ef7b2b07'
FILES=('dev_peer_issuer_metadata.py','dev_peer_issuer_preflight.py','dev_peer_issuer_service.py','dev_peer_issuer_authority.py',
       'dev_peer_issuer_client.py','dev_peer_issuer_packet.py','dev_peer_issuer_material.py','dev_peer_issuer_installer.py',
       'dev_realtime_p3_issuer_metadata.py','dev_realtime_p3_issuer_capability.py',
       'dev_realtime_p3_issuer_client.py','dev_realtime_p3_issuer_service.py',
       'realtime_p3_create_only.py','realtime_p3_delegated_receiver.py','realtime_p3_create_only_transport.py','realtime_p3_create_only_socket.py',
       'realtime_p3_create_only_packet.py','realtime_p3_create_only_crypto.py',
       'space_p3_issuer_preflight_metadata_root.py','space_p3_issuer_preflight_root.py',
       'space_creator_runtime_profile_issuer_packet.py','space_p3_issuer.py','realtime_p3_create_only_source.py',
       'resource_fencing.py','credentials.py','reset_credentials.py','realtime_p3_missing_issuer.py',
       'realtime_p3_root_export.py')
ROOT_PATHS=('scripts/dev/canonical_application_package.py','scripts/dev/publish_candidate.py',
            'scripts/staging/nats-rollout-preservation/source_authority.py',
            'scripts/staging/nats-rollout-preservation/github_custody.py')
SELF='scripts/dev/dev_peer_issuer_source.py'

class SourceError(RuntimeError): pass

def verify(repo,candidate):
    try:
        if (not Path(repo).is_absolute() or not isinstance(candidate,str) or not re.fullmatch(r'[a-f0-9]{40}',candidate) or
            not isinstance(REVIEWED_SOURCE,str) or not re.fullmatch(r'[a-f0-9]{40}',REVIEWED_SOURCE)): raise ValueError()
        for path in tuple('scripts/dev/'+name for name in FILES)+ROOT_PATHS:
            expected=source._blob(repo,REVIEWED_SOURCE,path); actual=source._blob(repo,candidate,path)
            if expected!=actual or source._disk(repo,path)!=actual: raise ValueError()
        expected=source._blob(repo,REVIEWED_SOURCE,SELF); actual=source._blob(repo,candidate,SELF)
        if source._disk(repo,SELF)!=actual: raise ValueError()
        if actual.count(("REVIEWED_SOURCE='"+REVIEWED_SOURCE+"'").encode('ascii'))!=1: raise ValueError()
        def normalize(value):
            pattern=rb'(?m)^REVIEWED_SOURCE=(?:None|\x27[a-f0-9]{40}\x27)$'
            result,count=re.subn(pattern,b'REVIEWED_SOURCE=<PIN>',value)
            if count!=1: raise ValueError()
            return result
        if normalize(expected)!=normalize(actual): raise ValueError()
    except Exception: raise SourceError('DEV peer issuer source unavailable') from None
