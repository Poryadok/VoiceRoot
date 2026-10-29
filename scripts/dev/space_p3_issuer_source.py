"""Exact reviewed Space issuer code and fixed runtime source binding."""
from pathlib import Path
import re

import realtime_p3_create_only_source as source

ISSUER_SHA = 'a82249cd96b399424f7bc5e0e9913484db317afc'
RUNTIME_SHA = '7d4221d2355e9f6019beaf7c154b082dc4c5e783'
ISSUER_FILES = ('space_p3_issuer.py','space_p3_issuer_packet.py','space_p3_issuer_export.py',
                'credentials.py','reset_credentials.py','realtime_p3_missing_issuer.py',
                'realtime_p3_root_export.py','realtime_p3_create_only_crypto.py')
RUNTIME_FILES = ('src/backend/space/principal_issuer.go','src/backend/space/main.go')


class SourceError(RuntimeError): pass


def _space_document(raw):
    pieces = re.split(r'^---\s*$',raw.decode('utf-8'),flags=re.M)
    docs = [p for p in pieces if re.search(r'^kind: Deployment\s*$',p,flags=re.M)
            and re.search(r'^  name: voice-space\s*$',p,flags=re.M)]
    if len(docs) != 1: raise ValueError()
    return docs[0]


def verify(repo,candidate_sha):
    try:
        if not Path(repo).is_absolute() or not re.fullmatch(r'[a-f0-9]{40}',candidate_sha): raise ValueError()
        for name in ISSUER_FILES:
            path='scripts/dev/'+name
            current=source._blob(repo,candidate_sha,path)
            if current != source._blob(repo,ISSUER_SHA,path) or source._disk(repo,path) != current: raise ValueError()
        for path in RUNTIME_FILES:
            current=source._blob(repo,candidate_sha,path)
            if current != source._blob(repo,RUNTIME_SHA,path) or source._disk(repo,path) != current: raise ValueError()
        path='deploy/staging/services.yaml'; current=source._blob(repo,candidate_sha,path)
        if source._disk(repo,path) != current or _space_document(current) != _space_document(source._blob(repo,RUNTIME_SHA,path)): raise ValueError()
    except Exception:
        raise SourceError('Space issuer source unavailable') from None
