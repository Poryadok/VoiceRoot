"""Create-only existing-actor credential enrollment, called under root lock.

Root caller owns signed renewal validation, exact generation binding, isolated
server authentication and atomic private commit. No callable CLI, arbitrary
credential locator, signer access, or live bootstrap Job replay is provided.
"""
import base64
import hashlib
import json
import re

class EnrollmentError(ValueError):pass
def fail():raise EnrollmentError('existing_bootstrap_enrollment_refused')
def digest(value):return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False).encode()).hexdigest()
def credential_hash(row):
    try:
        value=base64.b64decode(row['data']['bootstrap.creds'],validate=True)
        if not 0<len(value)<=262144:fail()
        return hashlib.sha256(value).hexdigest()
    except Exception:fail()
class EnrollmentKube:
    """Existing fixed-namespace Kube adapter; private JSON stays in root."""
    def __init__(self,kube):self.kube=kube
    def optional_secret(self,name):
        if not re.fullmatch(r'voice-nats-bootstrap-(?:credentials-r[0-9]{8}[a-z0-9]{0,8}|contract-[a-f0-9]{24})',name):fail()
        result=self.kube.run(['get','secrets','--field-selector=metadata.name='+name,'-o','json'])
        rows=result.get('items') if isinstance(result,dict) else None
        if not isinstance(rows,list) or len(rows)>1:fail()
        if rows and rows[0].get('metadata',{}).get('name')!=name:fail()
        return rows[0] if rows else None
    def create_secret(self,row):
        if (not re.fullmatch(r'voice-nats-bootstrap-contract-[a-f0-9]{24}',row['metadata']['name'])
            or row['metadata'].get('namespace')!='voice-staging' or row.get('immutable') is not True):fail()
        # create only; admission/result is checked by exact subsequent GET.
        self.kube.run(['create','-f','/dev/stdin','-o','json'],body=row)
def enroll(kube,binding,credentials,assert_binding,prove_auth,commit):
    """A failed auth or process interruption never authorizes the new Secret.

    An existing same-name orphan is adopted only with identical immutable
    bytes/labels and after fresh server proof. The old Secret is never changed.
    assert_binding must recheck the active marker/old chain under the root lock;
    commit must durably atomically publish a private root-owned enrollment.
    """
    if not isinstance(credentials,bytes) or not 0<len(credentials)<=262144:fail()
    assert_binding()
    old=binding['old_secret'];row=kube.optional_secret(old['name'])
    if (not isinstance(row,dict) or row.get('immutable') is not True or row.get('type')!='Opaque'
        or any(row['metadata'].get(k)!=old[k] for k in ('name','uid','resourceVersion'))
        or credential_hash(row)!=old['credential_sha256']):fail()
    sha=hashlib.sha256(credentials).hexdigest()
    name='voice-nats-bootstrap-contract-'+digest({'binding':binding,'credential_sha256':sha})[:24]
    labels={'voice-nats-generation':binding['generation'],'voice-nats-existing-bootstrap-renewal':'true'}
    wanted={'apiVersion':'v1','kind':'Secret','type':'Opaque','immutable':True,
        'metadata':{'name':name,'namespace':'voice-staging','labels':labels},
        'data':{'bootstrap.creds':base64.b64encode(credentials).decode('ascii')}}
    current=kube.optional_secret(name)
    if current is None:
        assert_binding();kube.create_secret(wanted);current=kube.optional_secret(name)
    if (not isinstance(current,dict) or current.get('immutable') is not True or current.get('type')!='Opaque'
        or current['metadata'].get('name')!=name or current['metadata'].get('namespace')!='voice-staging'
        or current['metadata'].get('labels')!=labels or current.get('data')!=wanted['data']
        or not current['metadata'].get('uid') or not current['metadata'].get('resourceVersion')):fail()
    identity={k:current['metadata'][k] for k in ('name','uid','resourceVersion')}
    assert_binding()
    if prove_auth(credentials) is not True:fail()
    # Exact bytes and identity must still be present after authenticated INFO.
    again=kube.optional_secret(name)
    if again!=current:fail()
    assert_binding()
    receipt={'schema':'voice-nats-bootstrap-enrollment-v1','binding':binding,
        'secret':identity,'credential_sha256':sha,'verified':True}
    commit(receipt)
    return receipt
