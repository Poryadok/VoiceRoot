"""Hosted Linux fixture: genuine owned genesis, verifier, enrollment and broker.

Kube is an in-process external fixture transport, not Kubernetes. All signing,
private-pipe verification, DockerRuntime and NATS authentication are real.
Run only on a disposable hosted VM; never on DEV, STAGING or an installed root.
"""
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import uuid

ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'scripts/staging/nats-known-baseline'))
sys.path.insert(0,str(ROOT/'scripts/staging/nats-rollout-preservation'))
import bootstrap_renewal
import bootstrap_selection
import bootstrap_auth
import actor_auth
import actor_permissions
import actor_verification
import guard
from controller import Blocked
from nats_contract_plan import digest
from nats_owned_enrollment import enroll_owned,RENEWER_SHA256
from resource_claim import InstallationAuthority,production_installation


def sha(raw):return hashlib.sha256(raw).hexdigest()


def failure_diagnostic(error):
    """Only fixed labels, never exception text, paths, requests or credentials."""
    if type(error) is actor_permissions.PermissionError:category='actor_permissions'
    elif isinstance(error,PermissionError):category='os_permission'
    elif isinstance(error,Blocked):category='blocked'
    elif isinstance(error,OSError):category='os_error'
    else:category='other'
    stages={qualify.__code__:'admission',_qualify_in_fixture.__code__:'fixture',
        enroll_owned.__code__:'enrollment',actor_verification.verify.__code__:'actor_verifier',
        actor_permissions.evaluate.__code__:'permission_check',actor_auth.prove.__code__:'actor_auth',
        bootstrap_auth.prove.__code__:'bootstrap_auth',bootstrap_renewal.invoke.__code__:'private_helper',
        bootstrap_selection.select.__code__:'selection'}
    stage='entry';tb=error.__traceback__
    while tb is not None:
        stage=stages.get(tb.tb_frame.f_code,stage);tb=tb.tb_next
    result='stage='+stage+' category='+category
    if type(error) is Blocked:
        # Never render an exception: only this exact source-defined tuple is
        # admitted, and all other values (including extra arguments) collapse.
        reason=('actor_server_authentication_refused' if error.args==
            ('existing_service_actor_server_authentication_refused',) else 'unspecified')
        result+=' reason='+reason
    return result


class FixtureKube:
    """Create-only observations owned solely by this disposable fixture."""
    def __init__(self,marker,old,operator):
        self.marker=copy.deepcopy(marker);self.rows={row['metadata']['name']:copy.deepcopy(row) for row in (old,operator)}
        self.reads=0
    def get(self,kind,name):
        self.reads+=1
        if kind=='configmap' and name=='voice-nats-generation':return copy.deepcopy(self.marker)
        if kind=='secret' and name in self.rows:return copy.deepcopy(self.rows[name])
        raise Blocked('owned_fixture_observation_missing')
    def run(self,args,body=None):
        if len(args)==5 and args[:2]==['get','secrets'] and args[3:]==['-o','json']:
            prefix='--field-selector=metadata.name='
            if not args[2].startswith(prefix):raise Blocked('owned_fixture_command_invalid')
            name=args[2][len(prefix):]
            return {'items':[copy.deepcopy(self.rows[name])] if name in self.rows else []}
        if args==['create','-f','/dev/stdin','-o','json'] and type(body) is dict:
            row=copy.deepcopy(body);name=row['metadata']['name']
            if name in self.rows:raise Blocked('owned_fixture_secret_conflict')
            row['metadata'].update(uid=str(uuid.uuid4()),resourceVersion='1')
            self.rows[name]=row;return copy.deepcopy(row)
        raise Blocked('owned_fixture_command_invalid')


def allocate_fixture(root):
    # Caller reconciles all bound containers before removing this directory.
    return Path(tempfile.mkdtemp(prefix='voice-owned-nats-',dir=root))


def qualify(generator,renewer,fixture_root):
    if sys.platform!='linux' or os.geteuid()!=0 or os.environ.get('RUNNER_ENVIRONMENT')!='github-hosted':
        raise ValueError('disposable_hosted_linux_required')
    if sha(renewer.read_bytes())!=RENEWER_SHA256:raise ValueError('renewal_binary_pin_changed')
    if (not fixture_root.is_absolute() or fixture_root.resolve(strict=True)!=fixture_root
        or fixture_root.stat().st_uid!=0 or fixture_root.stat().st_mode&0o777!=0o700):
        raise ValueError('owned_fixture_root_invalid')
    return _qualify_in_fixture(generator,renewer,fixture_root)


def _qualify_in_fixture(generator,renewer,fixture_root):
    """Caller retains the directory for external container reconciliation."""
    base=allocate_fixture(fixture_root)
    private=base/'genesis';private.mkdir(mode=0o700)
    result=subprocess.run([str(generator),str(private),str(ROOT/'deploy/nats/acl-intent.yaml')],
        stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
    if result.returncode:raise ValueError('owned_genesis_failed')
    files={path.name:path.read_bytes() for path in private.iterdir()}
    genesis=json.loads(files['genesis.json'])
    installation=InstallationAuthority('owned-isolated-v1',genesis['realm_id'],genesis['generation'],
        sha(genesis['account'].encode()),sha(genesis['users']['bootstrap'].encode()),sha(files['genesis.json']))
    # Freeze authority before publishing any observed fixture object.
    gen=installation.generation
    def secret(name,values):
        return {'type':'Opaque','immutable':True,'metadata':{'name':name,'uid':str(uuid.uuid4()),
            'resourceVersion':'1','namespace':'voice-staging'},
            'data':{key:base64.b64encode(value).decode() for key,value in values.items()}}
    old=secret('voice-nats-bootstrap-credentials-'+gen,{'bootstrap.creds':files['bootstrap.creds']})
    operator=secret('voice-nats-operator-'+gen,{key:files[key] for key in
        ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')})
    marker={'metadata':{'uid':str(uuid.uuid4()),'resourceVersion':'1'},'data':{'generation':gen,'phase':'active'}}
    kube=FixtureKube(marker,old,operator)
    identity=lambda row:{key:row['metadata'][key] for key in ('name','uid','resourceVersion')}
    binding={'generation':gen,'marker_uid':marker['metadata']['uid'],'marker_rv':'1',
        'old_secret':dict(identity(old),credential_sha256=sha(files['bootstrap.creds'])),
        'operator_secret':identity(operator),'actor_sha256':installation.bootstrap_actor_sha256,
        'account_sha256':installation.account_sha256}
    frozen={name:sha(raw) for name,raw in files.items()}
    def unchanged():
        if ({p.name:sha(p.read_bytes()) for p in private.iterdir()}!=frozen
            or kube.get('secret',old['metadata']['name'])!=old
            or kube.get('secret',operator['metadata']['name'])!=operator
            or kube.get('configmap','voice-nats-generation')!=marker):
            raise Blocked('owned_genesis_changed')
    old_guard=guard.ROOT;guard.ROOT=base
    try:
        acl=(ROOT/'deploy/nats/acl-intent.yaml').read_bytes()
        first=enroll_owned(kube,binding,files['bootstrap.creds'],operator,installation,renewer,acl,unchanged)
        credentials=files['bootstrap.creds'].decode()
        lines=credentials.splitlines(keepends=True)
        indexes=[i for i,line in enumerate(lines) if line.count('.')==2]
        if len(indexes)!=1:raise ValueError('owned_credential_shape_invalid')
        index=indexes[0];parts=lines[index].split('.');signature=parts[2]
        parts[2]=('A' if signature[0]!='A' else 'B')+signature[1:];lines[index]='.'.join(parts)
        bad={'schema':'voice-nats-existing-actor-verification-v1','credentials':''.join(lines),
            'account_jwt':files['account.jwt'].decode(),'operator_jwt':files['operator.jwt'].decode(),
            'actor_sha256':installation.bootstrap_actor_sha256,'account_sha256':installation.account_sha256}
        try:bootstrap_renewal.invoke(renewer,bad,mode='--verify-existing-actor')
        except bootstrap_renewal.RenewalError:pass
        else:raise ValueError('invalid_signature_accepted')
        renewal={'schema':'voice-nats-existing-bootstrap-renewal-v1','old_creds':files['bootstrap.creds'].decode(),
            'account_jwt':files['account.jwt'].decode(),'operator_jwt':files['operator.jwt'].decode(),
            'signer_seed':files['fixture-account.seed'].decode(),'actor_sha256':installation.bootstrap_actor_sha256,
            'account_sha256':installation.account_sha256}
        unchanged();renewed=bootstrap_renewal.invoke(renewer,renewal);unchanged()
        second=enroll_owned(kube,dict(binding,previous_enrollment_sha256=digest(first)),renewed,
            operator,installation,renewer,acl,unchanged,previous=first)
        if second.get('previous_enrollment')!=first or second.get('installation_sha256')!=installation.sha256:
            raise ValueError('owned_enrollment_chain_invalid')
        reads=kube.reads
        try:bootstrap_selection.select(kube,second,gen,installation=production_installation())
        except Blocked:pass
        else:raise ValueError('production_accepted_owned_enrollment')
        if kube.reads!=reads:raise ValueError('foreign_domain_read_kube')
    finally:
        guard.ROOT=old_guard
    print('owned_nats_enrollment=PASS verifier=PASS signature_refusal=PASS broker_auth=PASS production_refusal=PASS')


if __name__=='__main__':
    try:
        if len(sys.argv)!=4:raise ValueError('qualified_paths_required')
        qualify(Path(sys.argv[1]),Path(sys.argv[2]),Path(sys.argv[3]))
    except Exception as error:
        print('owned_nats_enrollment=FAIL '+failure_diagnostic(error),file=sys.stderr)
        sys.exit(1)
