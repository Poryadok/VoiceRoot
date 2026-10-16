"""Root-bound unchanged actor verification; never a publication/ACK probe."""
import hashlib,json
from pathlib import Path
import actor_permissions
from bootstrap_renewal import invoke
from controller import Blocked
from encrypted_cut import regular

def sha(raw):return hashlib.sha256(raw).hexdigest()
def fail():raise Blocked('existing_service_actor_verification_refused')

def verify(binary,binary_sha,credentials,account_token,operator_token,actor_sha,account_sha,required,authenticate,unchanged):
    """The root producer owns expected identities, source requirements and auth.

    The private verifier output is not accepted as an unbound boolean receipt.
    Expected code/input hashes and a separately performed isolated authentication
    are required; callers may not supply a public actor proof for promotion.
    """
    fd,_=regular(Path(binary),private=False)
    try:
        h=hashlib.sha256()
        while True:
            chunk=__import__('os').read(fd,1<<20)
            if not chunk:break
            h.update(chunk)
    finally:__import__('os').close(fd)
    if h.hexdigest()!=binary_sha:fail()
    unchanged()
    request={'schema':'voice-nats-existing-actor-verification-v1','credentials':credentials.decode(),
        'account_jwt':account_token,'operator_jwt':operator_token,
        'actor_sha256':actor_sha,'account_sha256':account_sha}
    raw=invoke(binary,request,mode='--verify-existing-actor')
    try:receipt=json.loads(raw)
    except (ValueError,UnicodeError):fail()
    expected={'actor_sha256':actor_sha,'account_sha256':account_sha,
        'credential_sha256':sha(credentials),'account_jwt_sha256':sha(account_token.encode()),
        'operator_sha256':sha(operator_token.encode()),'signed_chain_verified':True}
    if not isinstance(receipt,dict) or set(receipt)!=set(expected)|{'actor_jwt_sha256','effective'}:fail()
    if any(receipt[k]!=v for k,v in expected.items()):fail()
    actor_token=credentials.split(b'-----BEGIN NATS USER JWT-----',1)[1].split(b'------END NATS USER JWT------',1)[0].strip()
    # Decorated token parsing is authoritative in the signed Go verifier. This
    # independent token hash check also supports the library's dashed end marker.
    actor_token=actor_token.split(b'-----END NATS USER JWT-----',1)[0].strip()
    if sha(actor_token)!=receipt['actor_jwt_sha256']:fail()
    permission=actor_permissions.evaluate(receipt['effective'],required)
    unchanged()
    auth=authenticate(credentials)
    if (not isinstance(auth,dict) or auth.get('server_authentication_verified') is not True
        or auth.get('credential_sha256')!=sha(credentials)
        or auth.get('account_sha256')!=account_sha):fail()
    unchanged()
    return {'schema':'voice-nats-existing-service-actor-proof-v1',**expected,
        'actor_jwt_sha256':receipt['actor_jwt_sha256'],'verifier_sha256':binary_sha,
        'effective_permissions_sha256':sha(json.dumps(receipt['effective'],sort_keys=True,separators=(',',':')).encode()),
        'required_sha256':sha(json.dumps(required,sort_keys=True,separators=(',',':')).encode()),
        'effective_permission_language_compatible':permission['effective_permission_language_compatible'],
        'server_authentication_verified':True,'signed_effective_permissions_verified':True,
        'broker_grant_compatibility_verified':True,'server_image':auth['server_image'],
        'publication_authorization_exercised':False,'business_delivery_exercised':False}
