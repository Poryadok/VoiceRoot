"""Human-root only existing bootstrap renewal; never called by CI requests.

Caller holds the normal operation lock and verifies captured executable bytes.
The separately reviewed isolated auth prover is mandatory before enrollment.
"""
import base64
import hashlib
import os
from pathlib import Path
import stat
from bootstrap_enrollment import EnrollmentKube,enroll
from bootstrap_renewal import invoke
from controller import Blocked

GENERATION='r20260930a4'
ACCOUNT_SHA='dd3e1a25fb7abf66f447c02cd6c9418ad9a69fd01cc0acf0b3c9a6243ed0106b'
ACTOR_SHA='7a5e2eca822ce7682c4c37391abf81e569bf2809d9345c807a67945c63ec50da'
SEED=Path('/var/lib/voice-nats-issuer/app-account-r20260930a4.seed')
def fail():raise Blocked('existing_bootstrap_root_binding_refused')
def hash_bytes(raw):return hashlib.sha256(raw).hexdigest()
def secret_bytes(row,key):
    try:
        raw=base64.b64decode(row['data'][key],validate=True)
        if not 0<len(raw)<=262144:fail()
        return raw
    except Exception:fail()
def protected_seed():
    for parent in (Path('/var'),Path('/var/lib'),SEED.parent):
        s=parent.lstat()
        if parent.resolve(strict=True)!=parent or not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022:fail()
    if SEED.parent.lstat().st_mode&0o077:fail()
    fd=os.open(SEED,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        s=os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or s.st_uid!=0 or s.st_nlink!=1 or s.st_mode&0o077 or s.st_size!=59:fail()
        raw=os.read(fd,60)
        if len(raw)!=59:fail()
        return raw
    finally:os.close(fd)
def enroll_current(kube,code,code_binding,assert_idle,private_read,private_save,prove_auth):
    """Only this explicit human entry may access the pre-enrolled signer seed.

    private_read/save must provide existing root-owned no-follow bounded atomic
    custody. prove_auth must use only an owned network-none empty scratch store.
    """
    assert_idle()
    root=Path('/var/lib/voice-nats-preservation')
    marker=kube.get('configmap','voice-nats-generation')
    if (marker['metadata']['uid']!='cbafb90c-6974-4642-86d6-09b2eef1c843'
        or marker['data'].get('generation')!=GENERATION or marker['data'].get('phase')!='active'
        or 'knownBaselineOperation' in marker['data']):fail()
    old=kube.get('secret','voice-nats-bootstrap-credentials-'+GENERATION)
    if (old['metadata']['uid']!='cd1f24fe-cbf1-40f2-9043-f8affd24127d'
        or old['metadata']['resourceVersion']!='2864382' or old.get('immutable') is not True):fail()
    operator=kube.get('secret','voice-nats-operator-'+GENERATION)
    account=secret_bytes(operator,'account.public').decode().strip()
    if hash_bytes(account.encode())!=ACCOUNT_SHA:fail()
    previous=secret_bytes(old,'bootstrap.creds')
    binding={'generation':GENERATION,'marker_uid':marker['metadata']['uid'],'marker_rv':marker['metadata']['resourceVersion'],
        'old_secret':{k:old['metadata'][k] for k in ('name','uid','resourceVersion')}|{'credential_sha256':hash_bytes(previous)},
        'operator_secret':{k:operator['metadata'][k] for k in ('name','uid','resourceVersion')},
        'actor_sha256':ACTOR_SHA,'account_sha256':ACCOUNT_SHA}
    def unchanged():
        assert_idle()
        if kube.get('configmap','voice-nats-generation')!=marker or kube.get('secret',binding['old_secret']['name'])!=old or kube.get('secret',binding['operator_secret']['name'])!=operator:fail()
    binary=Path(code)/'nats-rollout-preservation'/'bootstrap-renewer'
    relative='nats-rollout-preservation/bootstrap-renewer'
    if relative not in code_binding or hash_bytes(binary.read_bytes())!=code_binding[relative]:fail()
    unchanged()
    # No credentials are returned to a runner, copied to an operation archive,
    # or printed. A durable private draft makes create/crash retries identical.
    draft_path=root/'bootstrap-renewal-draft.json'
    try:draft=private_read(draft_path)
    except FileNotFoundError:draft=None
    request={'schema':'voice-nats-existing-bootstrap-renewal-v1','old_creds':previous.decode(),
        'account_jwt':secret_bytes(operator,'account.jwt').decode().strip(),
        'operator_jwt':secret_bytes(operator,'operator.jwt').decode().strip(),
        'signer_seed':protected_seed().decode(),'actor_sha256':ACTOR_SHA,'account_sha256':ACCOUNT_SHA}
    fresh=invoke(binary,request)
    if draft is None:
        draft={'schema':'voice-nats-bootstrap-renewal-draft-v1','binding':binding,'code_sha256':code_binding[relative],
            'credentials':base64.b64encode(fresh).decode(),'credential_sha256':hash_bytes(fresh)}
        private_save(draft_path,draft)
    if (draft.get('schema')!='voice-nats-bootstrap-renewal-draft-v1' or draft.get('binding')!=binding
        or draft.get('code_sha256')!=code_binding[relative]):fail()
    renewed=base64.b64decode(draft['credentials'],validate=True)
    if hash_bytes(renewed)!=draft.get('credential_sha256'):fail()
    unchanged()
    auth={}
    def authenticated(creds):
        result=prove_auth(creds,operator,unchanged)
        if not isinstance(result,dict) or result.get('verified') is not True:fail()
        auth.update(result);return True
    def commit(receipt):
        receipt=dict(receipt,auth_proof=auth)
        private_save(root/'bootstrap-enrollment.json',receipt)
    return enroll(EnrollmentKube(kube),binding,renewed,unchanged,authenticated,
        commit)
