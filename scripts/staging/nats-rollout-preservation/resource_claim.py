"""Cooperative STAGING executor fence; submitted Kubernetes requests are not revocable.

Only trusted installed/captured source chooses the tracker endpoint. Root owns
the durable journal, which stays occupied while the runner applies off-lock.
"""
import copy
from contextlib import contextmanager
from dataclasses import dataclass
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import urllib.parse
import urllib.request
import uuid

ENDPOINT='http://192.168.0.109:30990'
JOURNAL='tt-resource-fence.json'
START_ACTIONS=frozenset(('prepare','prepare-repair','prepare-rollback'))
FIELDS={'project','resource','actor','claim','generation','window','revision','operation_id','commit','images'}


@dataclass(frozen=True,slots=True)
class _CohortNativePermit:
    capability: object
    phase: str
    binding_sha256: str
    lineage_sha256: str
    effect_id: str
    argv: tuple
    body_sha256: str


@dataclass(frozen=True,slots=True)
class InstallationAuthority:
    """Root-composed expected identity, never selected by an observed record."""
    profile: str
    realm_id: str
    generation: str
    account_sha256: str
    bootstrap_actor_sha256: str
    genesis_sha256: str | None

    def __post_init__(self):
        if any(type(value) is not str for value in (self.profile,self.realm_id,self.generation,
                self.account_sha256,self.bootstrap_actor_sha256)):
            raise ValueError('staging_installation_profile_invalid')
        if self.profile=='installed-staging-v1':
            if (self.realm_id,self.generation,self.account_sha256,self.bootstrap_actor_sha256,self.genesis_sha256)!=(
                'voice-installed-staging-r20260930a4','r20260930a4',
                'dd3e1a25fb7abf66f447c02cd6c9418ad9a69fd01cc0acf0b3c9a6243ed0106b',
                '7a5e2eca822ce7682c4c37391abf81e569bf2809d9345c807a67945c63ec50da',None):
                raise ValueError('staging_installation_profile_invalid')
        elif self.profile=='owned-isolated-v1':
            try:valid_uuid=type(self.realm_id) is str and str(uuid.UUID(self.realm_id))==self.realm_id
            except (ValueError,TypeError,AttributeError):valid_uuid=False
            if (not valid_uuid or type(self.generation) is not str
                or not re.fullmatch(r'r[0-9]{8}[a-z0-9]{0,8}',self.generation)
                or any(type(value) is not str or not re.fullmatch('[a-f0-9]{64}',value) or value=='0'*64
                       for value in (self.account_sha256,self.bootstrap_actor_sha256,self.genesis_sha256))):
                raise ValueError('staging_installation_profile_invalid')
        else:raise ValueError('staging_installation_profile_invalid')

    def descriptor(self):
        return dict(schema='voice-installation-authority-v1',profile=self.profile,realm_id=self.realm_id,
            generation=self.generation,account_sha256=self.account_sha256,
            bootstrap_actor_sha256=self.bootstrap_actor_sha256,genesis_sha256=self.genesis_sha256)

    @property
    def sha256(self):
        return hashlib.sha256(json.dumps(self.descriptor(),sort_keys=True,
            separators=(',',':'),allow_nan=False).encode()).hexdigest()


def production_installation():
    return InstallationAuthority('installed-staging-v1','voice-installed-staging-r20260930a4','r20260930a4',
        'dd3e1a25fb7abf66f447c02cd6c9418ad9a69fd01cc0acf0b3c9a6243ed0106b',
        '7a5e2eca822ce7682c4c37391abf81e569bf2809d9345c807a67945c63ec50da',None)


def require_installation(value):
    if value is None:return production_installation()
    if type(value) is not InstallationAuthority:raise ValueError('staging_installation_authority_required')
    value.__post_init__()
    return value


def require_production(value):
    if require_installation(value)!=production_installation():raise RuntimeError('staging_installed_authority_required')


FENCE_FIELDS=frozenset(('schema','context','operation','helper_sha256','target_sha256'))

# Finite outer-record versions. This validates whole records in place; callers
# keep every domain field in their existing digest/parent comparisons.
RECORD_VERSIONS={
    'source_capture':(('voice-staging-source-capture-v1',),('voice-staging-source-capture-v2',)),
    'prerequisites_plan':(('voice-staging-prerequisites-plan-v1','voice-staging-prerequisites-plan-v2'),
                          ('voice-staging-prerequisites-plan-v3','voice-staging-prerequisites-plan-v4')),
    'prerequisites_checkpoint':(('voice-staging-prerequisites-checkpoint-v1',),('voice-staging-prerequisites-checkpoint-v2',)),
    'prerequisites_receipt':(('voice-staging-prerequisites-receipt-v1',),('voice-staging-prerequisites-receipt-v2',)),
    'prerequisites_capture':(('voice-staging-prerequisites-capture-v1',),('voice-staging-prerequisites-capture-v2',)),
    'realtime_plan':(('voice-staging-realtime-bootstrap-v1','voice-staging-realtime-bootstrap-v2'),
                     ('voice-staging-realtime-bootstrap-v3','voice-staging-realtime-bootstrap-v4')),
    'realtime_checkpoint':(('voice-staging-realtime-bootstrap-checkpoint-v1',),('voice-staging-realtime-bootstrap-checkpoint-v2',)),
    'realtime_receipt':(('voice-staging-realtime-bootstrap-receipt-v1',),('voice-staging-realtime-bootstrap-receipt-v2',)),
    'realtime_existing':(('voice-staging-realtime-bootstrap-existing-v1','voice-staging-realtime-bootstrap-existing-v2'),
                         ('voice-staging-realtime-bootstrap-existing-v3','voice-staging-realtime-bootstrap-existing-v4')),
    'copy_checkpoint':(('nats-rollout-copy-v1',),('nats-rollout-copy-v2',)),
}


def record_fields(row,family,fields,*,installation=None):
    """Validate an explicit closed outer version without unwrapping its domain."""
    installation=require_installation(installation)
    if not isinstance(row,dict) or family not in RECORD_VERSIONS:raise RuntimeError('staging_installation_record_invalid')
    legacy,scoped=RECORD_VERSIONS[family];schema=row.get('schema');expected=set(fields)
    if schema in scoped:
        expected.add('installation_sha256')
        if row.get('installation_sha256')!=installation.sha256:raise RuntimeError('staging_installation_binding_changed')
    elif schema in legacy:require_production(installation)
    else:raise RuntimeError('staging_installation_record_invalid')
    if set(row)!=expected:raise ValueError('staging_installation_record_invalid')
    return expected


def validate_source_capture(row,*,installation=None):
    record_fields(row,'source_capture',{'schema','operation','context','helper_sha256','approved_sha256','execution_sha256'},installation=installation)
    return row


def validate_active_release(row,*,installation=None):
    installation=require_installation(installation)
    if not isinstance(row,dict):raise RuntimeError('staging_installation_record_invalid')
    fields={'operation','namespace_uid','pvc_uid','target_sha256'}
    if row.get('schema')=='voice-active-release-v1':
        fields.update(('schema','installation_sha256'))
        if row.get('installation_sha256')!=installation.sha256:raise RuntimeError('staging_installation_binding_changed')
    elif 'schema' not in row:require_production(installation)
    else:raise RuntimeError('staging_installation_record_invalid')
    if 'gis_first_install_transition' in row:fields.add('gis_first_install_transition')
    if 'gis_latest_transition' in row:
        if 'gis_first_install_transition' not in row:raise RuntimeError('staging_installation_record_invalid')
        fields.add('gis_latest_transition')
    if set(row)!=fields:raise RuntimeError('staging_installation_record_invalid')
    if (not isinstance(row['operation'],str) or not re.fullmatch('[a-f0-9]{12}',row['operation'])
        or not isinstance(row['target_sha256'],str) or not re.fullmatch('[a-f0-9]{64}',row['target_sha256'])
        or any(not isinstance(row[key],str) or not row[key].strip() for key in ('namespace_uid','pvc_uid'))):
        raise RuntimeError('staging_installation_record_invalid')
    for key in ('gis_first_install_transition','gis_latest_transition'):
        if key in row and (not isinstance(row[key],dict) or set(row[key])!={'operation','receipt_sha256'}
            or not re.fullmatch('[a-f0-9]{12}',str(row[key]['operation']))
            or not re.fullmatch('[a-f0-9]{64}',str(row[key]['receipt_sha256']))):raise RuntimeError('staging_installation_record_invalid')
    return row


def _validate_fence_identity(row,installation):
    installation=require_installation(installation)
    if not isinstance(row,dict):raise RuntimeError('staging_resource_binding_invalid')
    if row.get('schema')=='voice-staging-resource-fence-v2':
        if set(row)!=FENCE_FIELDS|{'installation_sha256'} or row['installation_sha256']!=installation.sha256:
            raise RuntimeError('staging_installation_binding_changed')
    elif row.get('schema')=='voice-staging-resource-fence-v1':
        require_production(installation)
        if set(row)!=FENCE_FIELDS:raise RuntimeError('staging_resource_binding_invalid')
    else:raise RuntimeError('staging_resource_binding_invalid')
    validate(row.get('context'))
    if (not isinstance(row.get('operation'),str) or not re.fullmatch('[a-f0-9]{12}',row['operation'])
        or not isinstance(row.get('helper_sha256'),str) or not re.fullmatch('[a-f0-9]{64}',row['helper_sha256'])):
        raise RuntimeError('staging_resource_binding_invalid')
    return row


def validate_fence_binding(row,*,installation=None):
    _validate_fence_identity(row,installation)
    if not isinstance(row.get('target_sha256'),str) or not re.fullmatch('[a-f0-9]{64}',row['target_sha256']):
        raise RuntimeError('staging_resource_binding_invalid')
    return row


def _journal_binding(row,installation):
    """Validate the complete durable shape before extracting its binding."""
    if not isinstance(row,dict):raise RuntimeError('staging_resource_binding_invalid')
    fields=FENCE_FIELDS|({'installation_sha256'} if row.get('schema')=='voice-staging-resource-fence-v2' else set())
    status=row.get('status')
    if status not in ('active','complete','uncertain'):raise RuntimeError('staging_resource_binding_invalid')
    expected=fields|{'status','phase'}
    phase=row.get('phase')
    if phase not in ('prepare','cohort-preservation','prerequisites','prerequisites-captured',
                     'realtime-bootstrap','apply-ready','awaiting-ci','finished'):
        raise RuntimeError('staging_resource_binding_invalid')
    needs_actual=(phase=='awaiting-ci' and status in ('active','uncertain') or
                  phase=='finished' and status=='complete')
    if needs_actual:expected=expected|{'actual'}
    if status=='uncertain':expected=expected|{'reason'}
    if 'cohort_preservation_sha256' in row:
        if not isinstance(row['cohort_preservation_sha256'],str) or not re.fullmatch('[a-f0-9]{64}',row['cohort_preservation_sha256']):
            raise RuntimeError('staging_cohort_lineage_invalid')
        expected=expected|{'cohort_preservation_sha256'}
    elif row.get('phase')=='cohort-preservation':
        raise RuntimeError('staging_cohort_lineage_invalid')
    if set(row)!=expected or not isinstance(row.get('phase'),str):raise RuntimeError('staging_resource_binding_invalid')
    if phase=='finished' and status!='complete':raise RuntimeError('staging_resource_binding_invalid')
    if needs_actual and row.get('actual')!=row.get('context',{}).get('images'):
        raise RuntimeError('staging_resource_binding_invalid')
    if status=='complete' and (row['phase']!='finished' or row.get('actual')!=row.get('context',{}).get('images')):
        raise RuntimeError('staging_resource_binding_invalid')
    if status=='uncertain' and (not isinstance(row['reason'],str) or not row['reason']):
        raise RuntimeError('staging_resource_binding_invalid')
    binding={key:row[key] for key in fields}
    if binding['target_sha256'] is None:
        # Only the initial durable prepare intent can be unbound.
        if status not in ('active','uncertain') or row['phase']!='prepare':raise RuntimeError('staging_resource_binding_invalid')
        _validate_fence_identity(binding,installation)
    else:validate_fence_binding(binding,installation=installation)
    return binding


def _verify_durable_cohort_lineage(row):
    """Reconcile an existing journal against the independently saved P2 intent and plan."""
    import guard
    import staging_cohort_preservation as cohort
    if not isinstance(row,dict):return
    operation=row.get('operation')
    if not isinstance(operation,str) or not re.fullmatch('[a-f0-9]{12}',operation):
        raise RuntimeError('staging_cohort_lineage_invalid')
    request_path=guard.ROOT/'installed'/'cohort-requests'/(operation+'.json')
    checkpoint=guard.ROOT/'installed'/'cohort-preservation'/operation/'checkpoint.json'
    if request_path.is_symlink() or checkpoint.is_symlink():raise RuntimeError('staging_cohort_lineage_invalid')
    has_request=request_path.exists();has_checkpoint=checkpoint.exists();lineage=row.get('cohort_preservation_sha256')
    if not has_request and not has_checkpoint:
        if lineage is not None:raise RuntimeError('staging_cohort_lineage_invalid')
        return
    try:
        if not has_request:raise ValueError()
        request=cohort._read(request_path)
        if (type(request) is not dict or request.get('action')!='prepare'
            or not isinstance(request.get('nonce'),str) or not re.fullmatch('[a-f0-9]{64}',request['nonce'])
            or request['nonce'][:12]!=operation or request.get('resource_context')!=row.get('context')
            or request.get('source_sha')!=row['context']['commit'] or 'token' in request):raise ValueError()
        if row.get('phase')=='prepare' and row.get('target_sha256') is None and lineage is None:
            return
        state=cohort._read(checkpoint);plan=state['plan']
        if (type(plan) is not dict or set(plan)!=cohort.PLAN_FIELDS or plan.get('schema')!=cohort.SCHEMA
            or plan.get('operation')!=operation or plan.get('context')!=row['context']
            or plan.get('helper_sha256')!=row.get('helper_sha256') or digest(plan)!=lineage):raise ValueError()
    except Exception:
        raise RuntimeError('staging_cohort_lineage_invalid') from None


def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


def validate(context):
    if not isinstance(context,dict) or set(context)!=FIELDS:raise ValueError('staging_resource_context_required')
    if (context['project']!='voice' or context['resource']!='STAGING'
        or not isinstance(context['actor'],str) or not context['actor'].strip()):raise ValueError('staging_resource_scope_invalid')
    for key in ('claim','window'):
        if not isinstance(context[key],str) or str(uuid.UUID(context[key]))!=context[key]:raise ValueError('staging_resource_uuid_invalid')
    for key in ('generation','revision'):
        if type(context[key]) is not int or context[key]<1:raise ValueError('staging_resource_counter_invalid')
    if not isinstance(context['operation_id'],str) or not re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,127}',context['operation_id']):
        raise ValueError('staging_resource_operation_invalid')
    if not isinstance(context['commit'],str) or not re.fullmatch('[a-f0-9]{40}',context['commit']):raise ValueError('staging_resource_commit_invalid')
    if not isinstance(context['images'],dict) or not context['images']:raise ValueError('staging_resource_images_invalid')
    for key,image in context['images'].items():
        if (not isinstance(key,str) or not re.fullmatch('voice-[a-z0-9-]+/[a-z0-9-]+',key)
            or not isinstance(image,str) or not re.fullmatch(r'[^\s@]+@sha256:[a-f0-9]{64}',image)):
            raise ValueError('staging_resource_images_invalid')
    return context


class Check:
    def __init__(self,context):self.context=validate(context)

    def request(self,suffix,body=None):
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self,*args):return None
        request=urllib.request.Request(ENDPOINT+'/api/projects/voice/resources/STAGING/'+suffix,
            data=None if body is None else json.dumps(body).encode(),
            headers={} if body is None else {'Content-Type':'application/json'})
        try:
            with urllib.request.build_opener(NoRedirect()).open(request,timeout=10) as response:raw=response.read((2<<20)+1)
            if len(raw)>2<<20:raise ValueError('oversize response')
            return json.loads(raw)
        except (OSError,ValueError) as error:raise RuntimeError('staging_resource_response_unknown_recheck_required') from error

    def __call__(self):
        c=self.context
        row=self.request('check?'+urllib.parse.urlencode(dict(actor=c['actor'],claim=c['claim'],revision=c['revision'],phase='deploy')))
        if row.get('valid') is not True or not isinstance(row.get('resource'),dict):raise RuntimeError('staging_resource_check_invalid')
        return row['resource']

    def uncertain(self,reason):
        self()
        c=self.context
        self.request('actions',dict(kind='release',actor=c['actor'],claim=c['claim'],revision=c['revision'],
            safety='uncertain',reason=reason,pending=c['operation_id'],
            evidence='STAGING executor '+c['operation_id']+'; reconcile root journal '+JOURNAL+' and operation logs',
            next_action='Settle old executor and submitted requests; inspect actual version, root/Kubernetes locks and journal'))


class Session:
    def __init__(self,context,operation,helper_sha256,check,journal,*,installation=None):
        self._installation=require_installation(installation)
        validate(context)
        if not re.fullmatch('[a-f0-9]{12}',operation) or not re.fullmatch('[a-f0-9]{64}',helper_sha256):
            raise ValueError('staging_executor_identity_invalid')
        self.context=copy.deepcopy(context);self.operation=operation;self.helper=helper_sha256
        self.check_card=check;self.journal=journal
        self.expected_target=None
        self.admitted=False
        self.repair_guard=None
        self._prerequisite_permit=None
        self._prerequisite_readonly_permit=None
        self._cohort_permit=None
        self._cohort_auth_permit=None
        self._cohort_native_capability=None
        self._cohort_native_permit=None
        self._cohort_lineage=None
        self._prerequisite_readonly_busy=False
        self._prerequisite_readonly_capture=None

    @property
    def installation(self):return self._installation

    def authority(self):
        row=self.check_card();c=self.context
        expected=dict(code='STAGING',environment='stg',state='held',phase='deploy',owner=c['actor'],
            claim=c['claim'],generation=c['generation'],window=c['window'],revision=c['revision'])
        if any(row.get(k)!=v for k,v in expected.items()):raise RuntimeError('staging_resource_authority_changed')
        candidate=row.get('candidate',{})
        if candidate.get('state')!='known' or candidate.get('commit')!=c['commit'] or candidate.get('images')!=c['images']:
            raise RuntimeError('staging_resource_candidate_changed')
        return row

    def owned(self,row):
        try:_journal_binding(row,self.installation)
        except (ValueError,RuntimeError,KeyError,TypeError):return False
        return (row.get('context')==self.context and row.get('operation')==self.operation and row.get('helper_sha256')==self.helper)

    def admit(self,*,start,request=None):
        self.authority();previous=self.journal.read()
        if previous is not None:_verify_durable_cohort_lineage(previous)
        if start:
            if previous is not None and previous.get('status')=='complete':
                try:
                    c=validate(previous.get('context'))
                    _journal_binding(previous,self.installation)
                    closed=(previous['phase']=='finished' and previous['actual']==c['images']
                        and re.fullmatch('[a-f0-9]{12}',previous['operation'])
                        and re.fullmatch('[a-f0-9]{64}',previous['helper_sha256'])
                        and re.fullmatch('[a-f0-9]{64}',previous['target_sha256']))
                except (ValueError,TypeError,KeyError,RuntimeError):closed=False
                if not closed:raise RuntimeError('staging_closed_executor_reconciliation_required')
            if self.owned(previous) and previous.get('status')=='active':
                # The request journal alone decides completed replay versus
                # interrupted execution; never reset its durable executor intent.
                self.admitted=True;self.expected_target=previous.get('target_sha256');self._cohort_lineage=previous.get('cohort_preservation_sha256');self.authority();return
            if previous is not None and previous.get('status')!='complete' and self.repair_guard is not None:
                require_production(self.installation)
                _journal_binding(previous,self.installation)
                old=previous.get('context',{})
                if (request is None or request.get('action')!='prepare-repair'
                    or previous.get('status')!='active' or previous.get('phase')!='awaiting-ci'
                    or previous.get('actual')!=old.get('images') or not previous.get('actual')
                    or old.get('generation',0)>=self.context['generation']
                    or old.get('claim')==self.context['claim']
                    or old.get('operation_id')==self.context['operation_id']
                    or previous.get('operation')==self.operation):
                    raise RuntimeError('staging_executor_reconciliation_required')
                proof=self.repair_guard(request,previous)
                self.authority()
                self.journal.supersede(previous,proof,self.context,self.operation,self.helper)
                # Durable history precedes replacement; re-read protected parent
                # and high-water journal under the same root operation flock.
                if self.journal.read()!=previous or self.repair_guard(request,previous)!=proof:
                    raise RuntimeError('staging_repair_parent_changed')
            elif previous is not None and (previous.get('status')!='complete'
                or previous.get('context',{}).get('generation',0)>=self.context['generation']
                or previous.get('operation')==self.operation):raise RuntimeError('staging_executor_reconciliation_required')
            self.journal.save(dict(schema='voice-staging-resource-fence-v2',installation_sha256=self.installation.sha256,context=self.context,
                operation=self.operation,helper_sha256=self.helper,status='active',phase='prepare',target_sha256=None))
        elif not self.owned(previous) or previous.get('status') not in ('active','complete'):
            raise RuntimeError('staging_unfenced_or_uncertain_operation')
        if previous is not None and self.owned(previous):
            self.expected_target=previous.get('target_sha256');self._cohort_lineage=previous.get('cohort_preservation_sha256')
        self.admitted=True
        self.authority()

    def admit_request(self,request):self.admit(start=request['action'] in START_ACTIONS,request=request)

    def check(self,argv=None,*,launch=None):
        readonly=None
        if argv is not None and self._prerequisite_readonly_permit is not None:
            readonly=self._prerequisite_readonly_permit
            self._prerequisite_readonly_permit=None
        self.authority();row=self.journal.read()
        if (isinstance(row,dict) and self._cohort_lineage is not None
            and row.get('cohort_preservation_sha256')!=self._cohort_lineage):
            raise RuntimeError('staging_cohort_lineage_invalid')
        if not self.owned(row) or row.get('status')!='active':raise RuntimeError('staging_executor_fence_closed_or_changed')
        if self.expected_target is not None and row.get('target_sha256')!=self.expected_target:
            raise RuntimeError('staging_executor_target_changed')
        if getattr(self,'expected_phase',None) is not None and row.get('phase')!=self.expected_phase:
            raise RuntimeError('staging_executor_phase_changed')
        if readonly is not None and (not argv or row.get('phase')!='prerequisites-captured'
            or self._cohort_auth_permit is not None):
            raise RuntimeError('staging_prerequisite_readonly_launch_changed')
        cohort_hash=row.get('cohort_preservation_sha256')
        if ((cohort_hash is not None and (not isinstance(cohort_hash,str) or not re.fullmatch(r'[a-f0-9]{64}',cohort_hash)))
            or row.get('phase')=='cohort-preservation' and cohort_hash is None
            or self._cohort_lineage is not None and cohort_hash!=self._cohort_lineage):
            raise RuntimeError('staging_cohort_lineage_invalid')
        if argv and self._cohort_auth_permit is not None:
            permit=self._cohort_auth_permit
            if (row.get('phase') not in ('cohort-preservation','prerequisites','prerequisites-captured','realtime-bootstrap','prepare')
                or not row.get('target_sha256') or type(permit) is not dict or set(permit)!={'effect_id','argv','body_sha256'}
                or not isinstance(permit['effect_id'],str) or not permit['effect_id']
                or not isinstance(permit['argv'],list) or list(argv)!=permit['argv']
                or not isinstance(permit['body_sha256'],str) or not re.fullmatch(r'[a-f0-9]{64}',permit['body_sha256'])):
                raise RuntimeError('staging_cohort_auth_permit_invalid')
            self._cohort_auth_permit=None
            return row
        if argv and self._cohort_native_capability is not None and Path(str(argv[0])).name=='docker':
            permit=self._cohort_native_permit
            if (type(permit) is not _CohortNativePermit or permit.capability is not self._cohort_native_capability
                or row.get('phase') not in ('prerequisites-captured','realtime-bootstrap','prepare')
                or permit.phase!=row.get('phase') or permit.lineage_sha256!=cohort_hash
                or permit.binding_sha256!=digest(_journal_binding(row,self.installation))
                or not re.fullmatch(r'observe-native-[a-f0-9]{16}',permit.effect_id)
                or type(permit.argv) is not tuple or tuple(argv)!=permit.argv
                or not re.fullmatch(r'[a-f0-9]{64}',permit.body_sha256)):
                raise RuntimeError('staging_cohort_native_permit_invalid')
            self._cohort_native_permit=None
            return row
        if argv and row.get('phase') in ('cohort-preservation','prerequisites','prerequisites-captured'):
            args=list(argv);permit=self._prerequisite_permit
            cohort=self._cohort_permit
            if (row['phase']=='cohort-preservation' and type(cohort) is dict
                and set(cohort)=={'effect_id','argv','body_sha256'}
                and isinstance(cohort['effect_id'],str) and cohort['effect_id']
                and isinstance(cohort['argv'],list) and args==cohort['argv']
                and isinstance(cohort['body_sha256'],str) and re.fullmatch(r'[a-f0-9]{64}',cohort['body_sha256'])):
                self._cohort_permit=None
                return row
            if (row['phase']=='prerequisites' and permit is not None and args==permit):
                self._prerequisite_permit=None
                return row
            if readonly is not None:
                if row['phase']!='prerequisites-captured' or launch!=readonly or tuple(args)!=readonly[0]:
                    raise RuntimeError('staging_prerequisite_readonly_launch_changed')
                self.readonly_current()
                return row
            decode=['kubectl','create','--dry-run=client','--validate=false','-f','/dev/stdin','-o','json']
            prefixes=[['/usr/local/bin/k3s','kubectl','--namespace','voice-staging'],
                ['kubectl','--namespace','voice-staging']]
            is_dryrun=any(args==prefix+[verb,'--dry-run=server','-f','/dev/stdin','-o','json']
                for prefix in prefixes for verb in ('apply','create'))
            def safe_get(prefix):
                if args[:len(prefix)+1]!=prefix+['get']:return False
                tail=args[len(prefix)+1:]
                if row['phase']=='cohort-preservation' and not row.get('target_sha256'):return False
                if len(tail)==2 and tail[0]=='--raw':
                    return bool(re.fullmatch(r'/api/v1/namespaces/voice-staging/(?:configmaps|secrets|services)\?fieldSelector=metadata\.name%3D[a-z0-9][a-z0-9.-]{0,252}&limit=2',tail[1])
                        or re.fullmatch(r'/api/v1/namespaces/voice-staging/pods/voice-nats-pvc-candidate-[a-z0-9-]{1,100}:8222/proxy/jsz\?accounts=true&streams=true&consumers=true&config=true&limit=2048',tail[1]))
                if len(tail)==5:
                    if (tail==['endpointslices','-l','kubernetes.io/service-name=voice-postgres','-o','json']
                        and (row['phase']=='cohort-preservation' or row['phase']=='prerequisites-captured'
                             and row.get('target_sha256') is not None and prefix==prefixes[0])):return True
                    return tail in ([kind,'-l','app=voice-nats-pvc-candidate','-o','json'] for kind in ('pods','replicasets'))
                if len(tail) not in (3,4) or tail[-2]!='-o':return False
                if row['phase']=='cohort-preservation' and len(tail)==3:
                    from runtime_stage import IMAGE_TEMPLATE
                    from stage_runtime import FENCE_TEMPLATE
                    return tail in ([kind,'-o','json'] for kind in ('pods','hpa,jobs,deployments')) or tail in (
                        ['pods,replicasets','-o','go-template='+IMAGE_TEMPLATE],
                        ['pods,replicasets','-o','go-template='+FENCE_TEMPLATE])
                canonical_kinds=(('Secret','ConfigMap','Deployment','StatefulSet','Service',
                    'PersistentVolumeClaim','Job','Pod') if row['phase']=='cohort-preservation' and row.get('target_sha256')
                    else ('Secret','ConfigMap','Ingress','Deployment','StatefulSet','Service','NetworkPolicy',
                          'PersistentVolumeClaim','Job','Pod') if row['phase']=='prerequisites-captured' else ())
                # _recheck_stores replays this fixed named projection while a
                # qualified cohort is in prerequisites. Keep lists, arbitrary
                # names, other kinds and the bare kubectl route out of this seam.
                store_reads={('Secret','voice-app-secrets'),('Secret','voice-minio-credentials'),
                    ('ConfigMap','voice-app-config'),('ConfigMap','voice-clickhouse-init'),
                    ('Deployment','voice-file'),('Deployment','voice-user'),('Deployment','voice-redis'),
                    ('StatefulSet','voice-minio'),('StatefulSet','voice-clickhouse'),
                    ('PersistentVolumeClaim','data-voice-minio-0'),('PersistentVolumeClaim','chdata-voice-clickhouse-0'),
                    ('Service','voice-minio'),('Service','voice-redis'),('Service','voice-clickhouse'),
                    ('Job','voice-minio-create-avatars-bucket'),('Job','voice-minio-create-files-bucket'),
                    ('Pod','voice-clickhouse-0')}
                prerequisite_store=(row['phase']=='prerequisites' and cohort_hash is not None
                    and row.get('target_sha256') is not None and prefix==prefixes[0]
                    and len(tail)==4 and tail[-1]=='json' and (tail[0],tail[1]) in store_reads)
                canonical=(len(tail)==4 and tail[-1]=='json' and tail[0] in canonical_kinds) or prerequisite_store
                postgres_named=(row['phase']=='cohort-preservation' and len(tail)==4 and tail[-1]=='json'
                    and ((tail[0],tail[1]) in (('statefulset','voice-postgres'),
                        ('persistentvolumeclaim','pgdata-voice-postgres-0'),('endpoints','voice-postgres'))
                        or tail[0]=='persistentvolume'))
                if (row['phase']=='cohort-preservation' and not canonical
                    and not postgres_named
                    and tail[0] not in ('namespace','configmap','deployment','pvc','pv','node','pod',
                                       'replicaset','service','secret')):return False
                if not canonical and not re.fullmatch(r'[a-z0-9][a-z0-9.,-]{0,127}',tail[0]):return False
                if len(tail)==4 and not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}',tail[1]):return False
                output=tail[-1]
                if output in ('json',"jsonpath-as-json={.metadata['uid','resourceVersion']}"):return True
                from runtime_stage import IMAGE_TEMPLATE
                from stage_runtime import FENCE_TEMPLATE
                return output in ('go-template='+IMAGE_TEMPLATE,'go-template='+FENCE_TEMPLATE)
            is_get=any(safe_get(prefix) for prefix in prefixes)
            import actor_root
            actor_read=prefixes[0]+list(actor_root.STORY_SCHEMA_ARGS)
            import source_plan
            clickhouse_reads=[prefix+list(source_plan.CLICKHOUSE_READ_ARGS) for prefix in prefixes]
            postgres_reads=[prefix+source_plan.postgres_read_args(name)
                for prefix in prefixes for name in ('postgres','gateway_db')]
            identity_read=False
            if args not in postgres_reads and args[:len(prefixes[0])+3]==prefixes[0]+['exec','voice-postgres-0','-c']:
                try:
                    import staging_cohort_preservation_pg
                    identity_args=getattr(staging_cohort_preservation_pg,'IDENTITY_ARGS',None)
                except ModuleNotFoundError as error:
                    if error.name!='staging_cohort_preservation_pg':raise
                    identity_args=None
                identity_read=identity_args is not None and args==prefixes[0]+list(identity_args)
            if args!=decode and args!=actor_read and args not in clickhouse_reads and args not in postgres_reads and not identity_read and not is_dryrun and not is_get:
                raise RuntimeError('staging_prerequisite_command_forbidden')
        # Read-only preflight may use exec for typed SQL observations. Existing
        # stage guards own those queries. No workload fence/apply/cleanup may
        # run until its fresh actual baseline and complete target are bound.
        if argv and row.get('target_sha256') is None and Path(str(argv[0])).name=='kubectl':
            verbs={'patch','scale','apply','create','delete','replace','run','set'}
            source_decode=list(argv[1:])==['create','--dry-run=client','--validate=false','-f','/dev/stdin','-o','json']
            if any(str(v) in verbs for v in argv[1:]) and '--dry-run=server' not in argv and not source_decode:
                raise RuntimeError('staging_resource_target_unbound')
        return row

    def readonly_current(self):
        import staging_prerequisites
        require_session(self,installation=self.installation)
        captured=self._prerequisite_readonly_capture
        row=self.check()
        if (not self._prerequisite_readonly_busy or captured is None
            or row.get('phase')!='prerequisites-captured'
            or staging_prerequisites.load_capture(self)!=captured
            or self.check()!=row or row.get('target_sha256')!=digest(captured)):
            raise RuntimeError('staging_prerequisite_readonly_capture_changed')
        require_session(self,installation=self.installation)
        if self.check()!=row:raise RuntimeError('staging_prerequisite_readonly_capture_changed')

    def bind_target(self,commit,images,actual,target_sha256):
        row=self.check();card=self.authority();observed=card.get('observed',{})
        if row.get('phase') in ('cohort-preservation','realtime-bootstrap','prerequisites','prerequisites-captured'):raise RuntimeError('staging_realtime_bootstrap_advance_required')
        if (commit!=self.context['commit'] or images!=self.context['images']
            or observed.get('state')!='known' or observed.get('claim')!=self.context['claim']
            or observed.get('window')!=self.context['window'] or observed.get('operation')!=card.get('operation',0)-1
            or observed.get('images')!=actual or not re.fullmatch('[a-f0-9]{64}',target_sha256)):
            raise RuntimeError('staging_actual_baseline_or_candidate_changed')
        if row.get('target_sha256') is not None:
            if row['target_sha256']!=target_sha256:raise RuntimeError('staging_resource_target_changed')
            self.expected_target=target_sha256
            return
        row['target_sha256']=target_sha256;self.journal.save(row);self.expected_target=target_sha256

    def bind_cohort_preservation(self,plan,actual_before):
        import staging_cohort_preservation
        row=self.check();card=self.authority();observed=card.get('observed',{})
        staging_cohort_preservation.validate_plan(plan,self)
        if (row.get('phase')!='prepare' or row.get('target_sha256') is not None
            or plan.get('actual_before')!=actual_before
            or plan.get('candidate')!={'commit':self.context['commit'],'images':self.context['images']}
            or observed.get('state')!='known' or observed.get('claim')!=self.context['claim']
            or observed.get('window')!=self.context['window']
            or observed.get('operation')!=card.get('operation',0)-1 or observed.get('images')!=actual_before):
            raise RuntimeError('staging_cohort_baseline_changed')
        if self.check()!=row or self.authority()!=card:raise RuntimeError('staging_cohort_baseline_changed')
        row.update(phase='cohort-preservation',target_sha256=digest(plan),cohort_preservation_sha256=digest(plan))
        self.journal.save(row);self.expected_target=row['target_sha256'];self._cohort_lineage=row['cohort_preservation_sha256']
        return self.binding()

    def bind_prerequisites(self,plan,actual_before):
        import staging_prerequisites
        import staging_cohort_preservation
        row=self.check();card=self.authority();observed=card.get('observed',{})
        staging_prerequisites.validate_plan(plan,self)
        if row.get('phase')!='cohort-preservation':raise RuntimeError('staging_cohort_preservation_required')
        receipt=staging_cohort_preservation.load_qualified(self,actual=actual_before)
        if (plan.get('schema')!='voice-staging-prerequisites-plan-v4'
            or plan.get('preservation_sha256')!=digest(receipt)
            or plan.get('actual_before')!=actual_before or observed.get('state')!='known'
            or observed.get('claim')!=self.context['claim'] or observed.get('window')!=self.context['window']
            or observed.get('operation')!=card.get('operation',0)-1 or observed.get('images')!=actual_before):
            raise RuntimeError('staging_prerequisites_baseline_changed')
        if self.check()!=row or self.authority()!=card:raise RuntimeError('staging_prerequisites_baseline_changed')
        row.update(phase='prerequisites',target_sha256=digest(plan))
        self.journal.save(row);self.expected_target=row['target_sha256']
        return self.binding()

    def capture_prerequisites(self,stage):
        import staging_prerequisites
        row=self.check();card=self.authority()
        if row.get('phase')!='prerequisites':raise RuntimeError('staging_prerequisites_phase_invalid')
        captured=staging_prerequisites.capture(self,stage)
        if not isinstance(captured,dict) or staging_prerequisites.load_capture(self)!=captured:
            raise RuntimeError('staging_prerequisites_capture_changed')
        if self.check()!=row or self.authority()!=card:raise RuntimeError('staging_prerequisites_capture_changed')
        row.update(phase='prerequisites-captured',target_sha256=digest(captured))
        self.journal.save(row);self.expected_target=row['target_sha256']
        return self.binding()

    def bind_realtime_bootstrap(self,plan,actual):
        import realtime_bootstrap
        row=self.check();card=self.authority();observed=card.get('observed',{})
        realtime_bootstrap.validate_plan(plan,self)
        parent=None
        if row.get('phase')=='prerequisites-captured':
            import staging_prerequisites
            parent=staging_prerequisites.load_capture(self,actual=actual)
            if plan.get('prerequisites_sha256')!=digest(parent):raise RuntimeError('staging_prerequisites_capture_changed')
        elif (row.get('phase')!='prepare' or row.get('target_sha256') is not None
              or 'prerequisites_sha256' in plan or plan.get('schema') in ('voice-staging-realtime-bootstrap-v2','voice-staging-realtime-bootstrap-v4')):
            raise RuntimeError('staging_realtime_bootstrap_baseline_changed')
        if (plan.get('actual_images')!=actual or observed.get('state')!='known'
            or observed.get('claim')!=self.context['claim'] or observed.get('window')!=self.context['window']
            or observed.get('operation')!=card.get('operation',0)-1
            or observed.get('images')!=(parent['actual_before'] if parent is not None else actual)):
            raise RuntimeError('staging_realtime_bootstrap_baseline_changed')
        if self.check()!=row or self.authority()!=card:
            raise RuntimeError('staging_realtime_bootstrap_baseline_changed')
        row.update(phase='realtime-bootstrap',target_sha256=digest(plan))
        self.journal.save(row);self.expected_target=row['target_sha256']
        return self.binding()

    def _validate_cohort_target(self,row,target):
        if row.get('cohort_preservation_sha256') is None:
            if 'cohort_preservation_sha256' in target:raise RuntimeError('staging_cohort_target_changed')
            return
        import staging_cohort_preservation
        receipt=staging_cohort_preservation.load_qualified(self)
        if target.get('cohort_preservation_sha256')!=digest(receipt):
            raise RuntimeError('staging_cohort_target_changed')

    def advance_realtime_bootstrap(self,target,actual):
        import realtime_bootstrap
        if not isinstance(target,dict) or target.get('installation_sha256')!=self.installation.sha256:
            raise RuntimeError('staging_installation_binding_changed')
        row=self.check();card=self.authority();observed=card.get('observed',{})
        if row.get('phase')!='realtime-bootstrap':raise RuntimeError('staging_realtime_bootstrap_phase_invalid')
        receipt=realtime_bootstrap.load_completed(self,actual=actual)
        plan=realtime_bootstrap.load_plan(self)
        if not isinstance(target,dict) or not isinstance(plan,dict):raise RuntimeError('staging_realtime_bootstrap_target_changed')
        parent=None
        if 'prerequisites_sha256' in plan:
            import staging_prerequisites
            parent=staging_prerequisites.load_capture(self,actual=actual)
            if target.get('prerequisites_sha256')!=plan['prerequisites_sha256'] or plan['prerequisites_sha256']!=digest(parent):
                raise RuntimeError('staging_prerequisites_capture_changed')
        elif 'prerequisites_sha256' in target:raise RuntimeError('staging_prerequisites_capture_changed')
        self._validate_cohort_target(row,target)
        if (not isinstance(target,dict) or target.get('tag')!=self.context['commit']
            or target.get('images')!=self.context['images']
            or target.get('realtime_bootstrap_sha256')!=digest(receipt)
            or observed.get('state')!='known' or observed.get('claim')!=self.context['claim']
            or observed.get('window')!=self.context['window']
            or observed.get('operation')!=card.get('operation',0)-1
            or observed.get('images')!=(parent['actual_before'] if parent is not None else actual)):
            raise RuntimeError('staging_realtime_bootstrap_target_changed')
        if self.check()!=row or self.authority()!=card:
            raise RuntimeError('staging_realtime_bootstrap_target_changed')
        row.update(phase='prepare',target_sha256=digest(target))
        self.journal.save(row);self.expected_target=row['target_sha256']
        return self.binding()

    def advance_prerequisites_target(self,target,actual):
        import staging_prerequisites,realtime_bootstrap_existing
        if not isinstance(target,dict) or target.get('installation_sha256')!=self.installation.sha256:
            raise RuntimeError('staging_installation_binding_changed')
        row=self.check();card=self.authority();observed=card.get('observed',{})
        if row.get('phase')!='prerequisites-captured':raise RuntimeError('staging_prerequisites_phase_invalid')
        parent=staging_prerequisites.load_capture(self,actual=actual)
        receipt=realtime_bootstrap_existing.load_existing(self,actual=actual)
        if isinstance(target,dict):self._validate_cohort_target(row,target)
        if (not isinstance(target,dict) or target.get('tag')!=self.context['commit']
            or not isinstance(receipt,dict) or receipt.get('prerequisites_sha256')!=digest(parent)
            or target.get('images')!=self.context['images'] or target.get('prerequisites_sha256')!=digest(parent)
            or target.get('realtime_bootstrap_sha256')!=digest(receipt) or observed.get('state')!='known'
            or observed.get('claim')!=self.context['claim'] or observed.get('window')!=self.context['window']
            or observed.get('operation')!=card.get('operation',0)-1 or observed.get('images')!=parent['actual_before']):
            raise RuntimeError('staging_prerequisites_target_changed')
        if self.check()!=row or self.authority()!=card:raise RuntimeError('staging_prerequisites_target_changed')
        row.update(phase='prepare',target_sha256=digest(target))
        self.journal.save(row);self.expected_target=row['target_sha256']
        return self.binding()

    def capture_binding(self):
        """Project the actual current capture intent; never bind or guess a target."""
        if not self.admitted:raise RuntimeError('staging_resource_session_not_admitted')
        row=self.check()
        return copy.deepcopy(_journal_binding(row,self.installation))

    def binding(self):
        self.authority();row=self.journal.read()
        if not self.owned(row) or row.get('status') not in ('active','complete'):
            raise RuntimeError('staging_resource_binding_unavailable')
        if not row.get('target_sha256'):raise RuntimeError('staging_resource_target_unbound')
        return copy.deepcopy(_journal_binding(row,self.installation))

    def require_binding(self,binding):
        if binding!=self.binding():raise RuntimeError('staging_resource_checkpoint_binding_changed')

    def ready(self):
        self.authority();previous=self.journal.read()
        # Cached authorization is historical after finish/provisional restart;
        # it must neither reopen apply nor create uncertainty in a safe window.
        if self.owned(previous) and (previous.get('status')=='complete' or previous.get('phase')=='awaiting-ci'):return
        row=self.check()
        if row.get('phase') in ('cohort-preservation','realtime-bootstrap','prerequisites','prerequisites-captured'):raise RuntimeError('staging_realtime_bootstrap_advance_required')
        if not row.get('target_sha256'):raise RuntimeError('staging_resource_target_unbound')
        row['phase']='apply-ready';self.journal.save(row)

    def complete(self,status,images):
        self.authority();existing=self.journal.read()
        if self.owned(existing) and existing.get('status')=='complete':
            if status!='PASS' or images!=self.context['images'] or existing.get('actual')!=images:
                raise RuntimeError('staging_actual_finish_required')
            return
        row=self.check()
        if status not in ('PASS','PROVISIONAL_READY') or images!=self.context['images'] or row.get('phase') not in ('apply-ready','awaiting-ci'):
            raise RuntimeError('staging_actual_finish_required')
        if status=='PROVISIONAL_READY':
            row.update(phase='awaiting-ci',actual=copy.deepcopy(images));self.journal.save(row);return
        row.update(status='complete',phase='finished',actual=copy.deepcopy(images));self.journal.save(row)

    def uncertain(self,reason):
        row=self.journal.read()
        if self.owned(row) and row.get('status')=='active':
            row.update(status='uncertain',reason=reason);self.journal.save(row)
            return True
        return False


def trusted_json(path,*,missing=False,limit=65536):
    if limit not in (65536, 2 << 20):raise ValueError('staging_resource_limit_invalid')
    path=Path(path)
    for parent in (*path.parents[::-1],):
        row=parent.lstat()
        if not stat.S_ISDIR(row.st_mode) or row.st_uid!=0 or row.st_mode&0o022:raise RuntimeError('staging_resource_parent_untrusted')
    try:fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    except FileNotFoundError:
        if missing:return None
        raise
    try:
        row=os.fstat(fd)
        if not stat.S_ISREG(row.st_mode) or row.st_uid!=0 or row.st_mode&0o022 or row.st_nlink!=1 or not 1<=row.st_size<=limit:
            raise RuntimeError('staging_resource_journal_untrusted')
        raw=os.read(fd,row.st_size+1)
        if len(raw)!=row.st_size:raise RuntimeError('staging_resource_journal_changed')
        return json.loads(raw)
    finally:os.close(fd)


class RootJournal:
    def __init__(self,root):self.path=Path(root)/'installed'/JOURNAL
    def read(self):return trusted_json(self.path,missing=True)
    def save(self,row):
        if sys.platform!='linux' or os.geteuid()!=0:raise RuntimeError('staging_resource_root_writer_required')
        # Existing root save provides exclusive scratch creation, fsync, atomic
        # replacement and directory fsync under the existing operation flock.
        from root_main import save
        import grp
        trusted_json(self.path,missing=True)
        save(self.path,row)
        os.chown(self.path,0,grp.getgrnam('pmd').gr_gid);self.path.chmod(0o440)
        fd=os.open(self.path,os.O_RDONLY|os.O_NOFOLLOW)
        try:os.fsync(fd)
        finally:os.close(fd)

    def supersede(self,previous,proof,context,operation,helper):
        if sys.platform!='linux' or os.geteuid()!=0:raise RuntimeError('staging_resource_root_writer_required')
        from root_main import save
        directory=self.path.parent/'tt-resource-supersessions'
        if not directory.exists():
            directory.mkdir(mode=0o700)
            fd=os.open(directory.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
            try:os.fsync(fd)
            finally:os.close(fd)
        path=directory/(operation+'.json')
        record=dict(schema='voice-staging-resource-supersession-v1',reason='verified-provisional-repair',
            previous=previous,parent=proof,context=context,operation=operation,helper_sha256=helper)
        existing=trusted_json(path,missing=True)
        if existing is not None:
            if existing!=record:raise RuntimeError('staging_repair_history_changed')
            return
        save(path,record)


def repair_proof(actions,request,previous):
    installation=require_installation(actions.installation)
    require_production(installation)
    base,state=actions.state(request['operation'])
    binding=historical_binding(state,installation=installation)
    if (state.get('status')!='PROVISIONAL_READY' or state.get('phase')!='POST_RESTART'
        or state.get('fence_status')!='RELEASED'
        or binding!=_journal_binding(previous,installation)):
        raise RuntimeError('staging_repair_parent_not_finished_provisional')
    from operator_retirement import private_bytes
    raw=private_bytes(base/'checkpoint.json',128<<20)
    if json.loads(raw)!=state:raise RuntimeError('staging_repair_parent_changed')
    return dict(operation=state['operation'],checkpoint_sha256=hashlib.sha256(raw).hexdigest())


@contextmanager
def command_scope(check):
    import commands
    previous=commands.authority_guard
    commands.authority_guard=check
    try:yield
    finally:commands.authority_guard=previous


def require_helper(binding):
    required=('nats-rollout-preservation/resource_claim.py','nats-rollout-preservation/runner.py',
        'nats-rollout-preservation/apply.py','nats-rollout-preservation/guard.py',
        'nats-rollout-preservation/bridge.py','nats-rollout-preservation/bridge_root.py',
        'nats-known-baseline/commands.py')
    if not isinstance(binding,dict) or not all(k in binding for k in required):
        raise RuntimeError('staging_resource_helper_upgrade_required')
    import importlib
    if any(getattr(importlib.import_module(name),'TT_FENCE_VERSION',None)!=1 for name in ('runner','apply','guard','commands')):
        raise RuntimeError('staging_resource_helper_upgrade_required')


@contextmanager
def root_scope(actions,request):
    """Actual bridge drain holds operation_lock across this entire scope."""
    import guard
    if request['action']=='status':yield None;return
    from bridge_root import Actions
    if type(actions) is not Actions:raise RuntimeError('staging_resource_actions_required')
    installation=require_installation(actions.installation)
    if request['action'] not in ('prepare','prepare-rollback','authorize','finish'):
        require_production(installation)
    require_helper(actions.binding)
    c=validate(request.get('resource_context'))
    operation=request['nonce'][:12] if request['action'] in START_ACTIONS else request['operation']
    check=Check(c);session=Session(c,operation,digest(actions.binding),check,RootJournal(guard.ROOT),installation=installation)
    if session.installation!=actions.installation:raise RuntimeError('staging_installation_binding_changed')
    if request['action']=='prepare-repair':
        session.repair_guard=lambda req,previous:repair_proof(actions,req,previous)
    actions.tt_session=session
    try:
        with command_scope(session.check):yield session
    except BaseException:
        notify=False
        try:notify=session.uncertain('staging_execution_interrupted_or_partial')
        except Exception:
            notify=session.admitted
            print('STAGING_RESOURCE_JOURNAL_RECHECK_REQUIRED',file=sys.stderr)
        if notify:
            try:check.uncertain('staging_execution_interrupted_or_partial')
            except Exception:print('STAGING_RESOURCE_RECOVERY_RECHECK_REQUIRED',file=sys.stderr)
        raise
    finally:actions.tt_session=None


def finish_request(session,actions,request,result):
    action=request['action']
    if action in ('authorize','authorize-preserved-upload','authorize-expired-native','authorize-expired-finish'):
        row=session.journal.read()
        if row.get("cohort_preservation_sha256") is not None:
            import staging_cohort_preservation as cohort
            expected=cohort.final_authorization_result(session,request)
            if expected!=result:raise RuntimeError("staging_cohort_authorization_result_changed")
            if row.get("phase")=="prepare":session.ready()
        else:session.ready()
    if action in ('finish','finish-expired-native','accept-provisional'):
        _,state=actions.state(request['operation'])
        session.complete(state.get('status'),state.get('target',{}).get('images'))


def _final_target_inventory(target,installation):
    """Pure scoped normalized-target shape; compiler capture has its own shape."""
    fields={'registry','tag','mode','changed_services','source_hashes','template_hashes',
        'input_template_hashes','images','manifest_sha256','migration_sha256','nonnats_sha256',
        'installation_sha256'}
    references={'cohort_preservation_sha256','prerequisites_sha256','realtime_bootstrap_sha256','gis_first_install_sha256',
        'gis_first_install_normalized_sha256','gis_existing_normalized_sha256'}
    def invalid():raise RuntimeError('staging_final_target_invalid')
    if type(target) is not dict:invalid()
    selected=set(target)&references
    first={'gis_first_install_sha256','gis_first_install_normalized_sha256'}
    if (set(target)!=fields|selected or target.get('installation_sha256')!=installation.sha256
        or bool(selected&first) and not first<=selected
        or first<=selected and 'gis_existing_normalized_sha256' in selected
        or 'prerequisites_sha256' in selected and 'realtime_bootstrap_sha256' not in selected
        or 'cohort_preservation_sha256' in selected and 'prerequisites_sha256' not in selected):invalid()
    if (not isinstance(target['registry'],str) or not target['registry']
        or target['mode'] not in ('full','app-only','images-only')
        or type(target['changed_services']) is not list
        or any(not isinstance(name,str) for name in target['changed_services'])
        or target['changed_services']!=sorted(set(target['changed_services']))
        or type(target['source_hashes']) is not dict or not 1<=len(target['source_hashes'])<=2048
        or type(target['template_hashes']) is not dict or not target['template_hashes']
        or type(target['input_template_hashes']) is not dict
        or set(target['input_template_hashes'])!=set(target['template_hashes'])
        or type(target['images']) is not dict or not target['images']):invalid()
    if (not isinstance(target['tag'],str) or not re.fullmatch('[a-f0-9]{40}',target['tag'])
        or any(not isinstance(value,str) or not re.fullmatch('[a-f0-9]{64}',value)
            for value in [target[name] for name in
                {'manifest_sha256','migration_sha256','nonnats_sha256','installation_sha256'}|selected]
                +list(target['source_hashes'].values())+list(target['template_hashes'].values())
                +list(target['input_template_hashes'].values()))):invalid()
    return target


def historical_binding(state,*,installation=None):
    """Read a protected parent as data; never reuse its expired TT authority."""
    installation=require_installation(installation)
    binding=state.get('resource_fence');target=state.get('target',{});helper=state.get('code_capture',{})
    validate_fence_binding(binding,installation=installation)
    if binding['schema']=='voice-staging-resource-fence-v2':
        if (state.get('schema')!='nats-rollout-root-v2' or state.get('installation_sha256')!=installation.sha256
            or target.get('installation_sha256')!=installation.sha256):raise RuntimeError('staging_installation_binding_changed')
        _final_target_inventory(target,installation)
    elif state.get('schema')!='nats-rollout-root-v1' or 'installation_sha256' in state or 'installation_sha256' in target:
        raise RuntimeError('staging_installation_binding_changed')
    if (binding.get('operation')!=state.get('operation') or binding.get('target_sha256')!=digest(target)
        or not isinstance(helper,dict) or 'nats-rollout-preservation/resource_claim.py' not in helper
        or binding.get('helper_sha256')!=digest(helper)):
        raise RuntimeError('staging_historical_unfenced_checkpoint_refused')
    c=validate(binding.get('context'))
    if c['commit']!=target.get('tag') or c['images']!=target.get('images'):
        raise RuntimeError('staging_historical_candidate_changed')
    return binding


def _capture_json(path,limit):
    """Bounded protected data, with complete reads and stable inode custody."""
    import encrypted_cut
    import guard
    path=Path(path)
    fingerprint=lambda s:(s.st_dev,s.st_ino,s.st_mode,s.st_uid,s.st_gid,
        s.st_nlink,s.st_size,s.st_mtime_ns,s.st_ctime_ns)
    fd,before=encrypted_cut.regular(path,private=True)
    try:
        if not 1<=before.st_size<=limit:raise RuntimeError('staging_capture_size_invalid')
        chunks=[];remaining=before.st_size
        while remaining:
            chunk=os.read(fd,min(remaining,1<<20))
            if not chunk:raise RuntimeError('staging_capture_file_changed')
            chunks.append(chunk);remaining-=len(chunk)
        if (os.read(fd,1) or fingerprint(os.fstat(fd))!=fingerprint(before)
            or fingerprint(path.lstat())!=fingerprint(before)):
            raise RuntimeError('staging_capture_file_changed')
        return json.loads(b''.join(chunks),object_pairs_hook=guard.object_pairs,
            parse_constant=lambda _:(_ for _ in ()).throw(ValueError('staging_capture_json_invalid')))
    finally:os.close(fd)


def _capture_source(state,binding,installation):
    import guard
    import gis_existing_update
    import staging_prerequisites
    operation=binding['operation'];context=binding['context'];helper=binding['helper_sha256']
    installed=guard.ROOT/'installed';directory=installed/'source-authority'/operation
    approved=_capture_json(directory/'approved.json',32<<20)
    execution=_capture_json(directory/'execution.json',2<<20)
    custody=_capture_json(directory/'code-custody.json',2<<20)
    captured=_capture_json(installed/'source-authority'/(operation+'.json'),65536)
    validate_source_capture(captured,installation=installation)
    expected=dict(schema='voice-staging-source-capture-v2',installation_sha256=installation.sha256,
        operation=operation,context=context,helper_sha256=helper,
        approved_sha256=digest(approved),execution_sha256=digest(execution))
    if captured!=expected:raise RuntimeError('staging_capture_source_changed')
    ordinary={'schema','repository','source_sha','workflow_id','run_id','run_attempt','tree_sha',
        'source_files','images','lock_artifact_id','lock_sha256','image_provenance','verified'}
    fields=set(approved)
    if fields not in (ordinary,ordinary|{'provisional_staging'},ordinary|{'source_equivalence'}):
        raise RuntimeError('staging_capture_source_variant_changed')
    if fields!=ordinary:require_production(installation)
    gis_existing_update._validate_source_authority_fields(approved,{'tag':context['commit'],'images':context['images']},fields)
    if 'provisional_staging' in approved:
        import provisional
        if (approved['provisional_staging'] is not True or state['input_target']['mode']!='images-only'
            or state.get('provisional')!=provisional.admission(approved,execution,state['code_capture'])):
            raise RuntimeError('staging_capture_source_variant_changed')
    elif 'provisional' in state:
        raise RuntimeError('staging_capture_source_variant_changed')
    if 'source_equivalence' in approved:
        equivalence=approved['source_equivalence']
        paths={'src/backend','.github/workflows/ci.yml','scripts/ci/backend-docker-context.txt','scripts/ci/staging-image-catalog.json'}
        if (type(equivalence) is not dict or set(equivalence)!={'current_master_sha_before','current_master_sha_after','equivalent_inputs','source_is_ancestor'}
            or equivalence['source_is_ancestor'] is not True
            or not isinstance(equivalence['current_master_sha_before'],str)
            or not re.fullmatch('[a-f0-9]{40}',equivalence['current_master_sha_before'])
            or equivalence['current_master_sha_after']!=equivalence['current_master_sha_before']
            or equivalence['current_master_sha_before']==approved['source_sha']
            or type(equivalence['equivalent_inputs']) is not dict or set(equivalence['equivalent_inputs'])!=paths
            or any(type(row) is not dict or set(row)!={'git_object_sha1'}
                or not isinstance(row['git_object_sha1'],str) or not re.fullmatch('[a-f0-9]{40}',row['git_object_sha1'])
                for row in equivalence['equivalent_inputs'].values())):
            raise RuntimeError('staging_capture_source_variant_changed')
    if 'source_authority' in state:
        saved=state['source_authority']
        if (type(saved) is not dict or set(saved)!=fields-{'source_files'}
            or any(saved[key]!=approved[key] for key in saved) or state.get('execution_authority')!=execution):
            raise RuntimeError('staging_capture_source_variant_changed')
    elif 'execution_authority' in state or 'provisional_staging' in approved:
        raise RuntimeError('staging_capture_source_variant_changed')
    import source_authority
    if (type(execution) is not dict or set(execution)!={'dispatcher_run_id','run_attempt','head_sha','event','workflow_id','path','repository'}
        or execution['head_sha']!=context['commit'] or execution['repository']!=source_authority.REPO
        or any(type(execution[key]) is not int or execution[key]<1 for key in ('dispatcher_run_id','run_attempt'))
        or (execution['workflow_id'],execution['path'],execution['event']) not in (
            (source_authority.WORKFLOW,'.github/workflows/ci.yml','push'),
            (263689731,'.github/workflows/staging-deploy.yml','workflow_dispatch'))):
        raise RuntimeError('staging_capture_execution_changed')
    source=installed/'sources'/operation
    import encrypted_cut
    encrypted_cut.directory(source)
    staging_prerequisites._verify_helper_source_data(custody,helper,approved,source)
    if custody['manifest']!=state['code_capture']:raise RuntimeError('staging_capture_helper_changed')
    return approved,execution,source


def _capture_children(state,binding,installation,approved,execution):
    """Independently load real protected children, including the first partial save."""
    import guard
    import realtime_bootstrap as c1
    import realtime_bootstrap_existing as c2
    import staging_prerequisites
    target=state['input_target'];operation=binding['operation'];context=binding['context'];helper=binding['helper_sha256']
    directory=guard.ROOT/'installed'/'realtime-bootstrap'/operation
    parent=None;receipt=None;phase='prepare';wanted=None;actual=None;namespace=None
    if 'prerequisites_sha256' in target:
        parent=staging_prerequisites.load_capture_data(operation,context,helper,installation=installation)
        if target['prerequisites_sha256']!=digest(parent):raise RuntimeError('staging_capture_parent_changed')
        if state.get('prerequisites')!=parent:raise RuntimeError('staging_capture_parent_changed')
    elif 'prerequisites' in state:raise RuntimeError('staging_capture_parent_changed')
    if 'realtime_bootstrap_sha256' in target:
        # These fixed producers are mutually exclusive; a root field is never
        # a selector and may not yet be attached at the first prerequisite save.
        first=(directory/'checkpoint.json').exists();second=(directory/'existing.json').exists()
        if first==second:raise RuntimeError('staging_capture_child_ambiguous')
        if first:
            plan,receipt,_,initial,_,authority,child_stage,_=c1._completed_data(operation,context,helper,installation=installation)
            if initial!=binding:raise RuntimeError('staging_capture_child_binding_changed')
            phase='realtime-bootstrap';wanted=digest(plan);actual=plan['actual_images'];namespace=plan['namespace_uid']
            if plan.get('prerequisites_sha256')!=(digest(parent) if parent is not None else None):
                raise RuntimeError('staging_capture_parent_changed')
        else:
            record,authority,child_stage,loaded_parent,_,_,_,_,_=c2._existing_data(operation,context,helper,installation=installation)
            receipt=record;actual=record['actual_images'];namespace=record['namespace_uid']
            if loaded_parent!=parent:raise RuntimeError('staging_capture_parent_changed')
            if parent is not None:phase='prerequisites-captured';wanted=digest(parent)
        if authority!={'approved':approved,'execution':execution} or digest(receipt)!=target['realtime_bootstrap_sha256']:
            raise RuntimeError('staging_capture_child_changed')
        if 'realtime_bootstrap' in state and state['realtime_bootstrap']!=receipt:
            raise RuntimeError('staging_capture_child_changed')
        if child_stage.get('marker',{}).get('data',{}).get('generation')!=installation.generation:
            raise RuntimeError('staging_capture_generation_changed')
    elif ('realtime_bootstrap' in state or parent is not None
        or (directory/'checkpoint.json').exists() or (directory/'existing.json').exists()
        or (guard.ROOT/'installed'/'prerequisites'/operation/'capture.json').exists()):
        raise RuntimeError('staging_capture_child_missing')
    if binding['target_sha256']!=wanted:raise RuntimeError('staging_capture_child_binding_changed')
    if parent is not None and (parent['actual_after']!=actual or parent['namespace_uid']!=namespace):
        raise RuntimeError('staging_capture_baseline_changed')
    if parent is not None and parent['stage']['expected'].get('generation')!=installation.generation:
        raise RuntimeError('staging_capture_generation_changed')
    if 'context' in state and actual is not None:
        saved=state['context']
        if saved.get('expected',{}).get('generation')!=installation.generation:
            raise RuntimeError('staging_capture_generation_changed')
        if (saved.get('old_images')!=actual or saved.get('expected',{}).get('namespace_uid')!=namespace):
            raise RuntimeError('staging_capture_baseline_changed')
    return phase,parent


def _capture_compiler(state,base,approved,source,installation):
    import root_cli
    import source_authority
    compiled=_capture_json(base/'compiler-input.json',32<<20)
    if (type(compiled) is not dict or not {'target','manifests','migrations','nonnats','contract'}<=set(compiled)
        or set(compiled)-{'target','manifests','migrations','nonnats','contract','gis_first_install'}
        or state.get('input_target')!=compiled['target'] or state.get('contract')!=compiled['contract']):
        raise RuntimeError('staging_capture_compiler_changed')
    target=compiled['target'];context=state['resource_fence']['context']
    fields={'registry','tag','mode','changed_services','source_hashes','template_hashes','images',
        'manifest_sha256','migration_sha256','nonnats_sha256','installation_sha256'}
    for key in ('cohort_preservation_sha256','prerequisites_sha256','realtime_bootstrap_sha256','gis_first_install_sha256'):
        if key in target:fields.add(key)
    if (set(target)!=fields or 'cohort_preservation_sha256' in target and not {'prerequisites_sha256','realtime_bootstrap_sha256'}<=set(target)
        or target['registry']!=source_authority.REGISTRY
        or target['tag']!=context['commit'] or target['images']!=context['images']
        or target['mode'] not in ('full','app-only','images-only')
        or target['installation_sha256']!=state['installation_sha256']
        or target['changed_services']!=sorted(set(target['changed_services']))
        or type(target['source_hashes']) is not dict or not 1<=len(target['source_hashes'])<=2048
        or type(target['template_hashes']) is not dict or not target['template_hashes']
        or target['manifest_sha256']!=digest(compiled['manifests'])
        or target['migration_sha256']!=digest(compiled['migrations'])
        or target['nonnats_sha256']!=digest(compiled['nonnats'])):
        raise RuntimeError('staging_capture_compiler_changed')
    for key in fields-{'registry','tag','mode','changed_services','source_hashes','template_hashes','images'}:
        if not isinstance(target[key],str) or not re.fullmatch('[a-f0-9]{64}',target[key]):
            raise RuntimeError('staging_capture_compiler_changed')
    documents=compiled['manifests'];templates={};images={};identities=set()
    if type(documents) is not list or len(documents)>128:raise RuntimeError('staging_capture_compiler_templates_changed')
    if 'gis_first_install' in compiled:
        first=compiled['gis_first_install']
        if (type(first) is not dict or set(first)!={'schema','objects'}
            or first['schema']!='voice-gis-first-install-source-v1'
            or target.get('gis_first_install_sha256')!=digest(first)):
            raise RuntimeError('staging_capture_compiler_templates_changed')
        documents=documents+first['objects']
    for row in documents:
        kind=row.get('kind');metadata=row.get('metadata',{});name=metadata.get('name')
        if (metadata.get('namespace')!='voice-staging' or (kind,name) in identities
            or kind not in ('Deployment','Service','NetworkPolicy','ConfigMap','Ingress')
            or not isinstance(name,str) or not re.fullmatch('voice-[a-z0-9-]{1,100}',name) or name.startswith('voice-nats')):
            raise RuntimeError('staging_capture_compiler_templates_changed')
        identities.add((kind,name))
        if kind=='Deployment':
            template=row['spec']['template'];templates[name]=digest(template)
            for container in template['spec'].get('initContainers',[])+template['spec']['containers']:
                key=name+'/'+container['name']
                if key in images:raise RuntimeError('staging_capture_compiler_templates_changed')
                images[key]=container['image']
    if state.get('renderer_authority') is not None:
        import renderer_root
        projected=renderer_root.bind_target(dict(target,template_hashes=copy.deepcopy(templates),images=copy.deepcopy(images)),state['renderer_authority'])
        templates=projected['template_hashes'];images=projected['images']
    if templates!=target['template_hashes'] or images!=target['images']:
        raise RuntimeError('staging_capture_compiler_templates_changed')
    for relative,wanted in target['source_hashes'].items():
        path=Path(relative)
        import encrypted_cut
        if (not isinstance(relative,str) or path.is_absolute() or '..' in path.parts
            or not relative.startswith(('scripts/','deploy/','docker/clickhouse/init/',
                'src/backend/migrations/','src/backend/auth/src/main/resources/db/migration/'))
            or approved['source_files'].get(relative)!=wanted):
            raise RuntimeError('staging_capture_source_changed')
        encrypted_cut.directory((source/path).parent)
        if (source/path).lstat().st_size>2<<20 or root_cli.public_code_sha(source/path)!=wanted:
            raise RuntimeError('staging_capture_source_changed')
    if state.get('target')!=target:_capture_normalization(state,base,compiled,installation)
    return compiled


def _capture_normalization(state,base,compiled,installation):
    import normalize
    from apply import paused_documents
    record=_capture_json(base/'normalization.json',128<<20)
    fields={'operation','compiler_input_sha256','input_target','input_manifests',
        'snapshots','old_images','expected','normalized_documents','target'}
    first='gis_first_install' in record
    if first:fields.update(('gis_first_install','gis_first_install_input'))
    existing='gis_existing_normalized_sha256' in record.get('target',{})
    if existing:fields.add('gis_existing')
    if (set(record)!=fields or record['operation']!=state['operation']
        or record['compiler_input_sha256']!=digest(compiled) or record['target']!=state['target']):
        raise RuntimeError('staging_capture_normalization_changed')
    if record.get('expected',{}).get('generation')!=installation.generation:
        raise RuntimeError('staging_capture_generation_changed')
    namespace=record['expected'].get('namespace_uid')
    if not isinstance(namespace,str) or not namespace:
        raise RuntimeError('staging_capture_baseline_changed')
    source=compiled['target'];target=record['input_target'];documents=record['input_manifests']
    renderer={}
    if state.get('renderer_authority') is not None:
        import renderer_root
        renderer=renderer_root.validate_target_data(source,state['renderer_authority'],record['snapshots'],record['expected'])
    if source['mode']=='images-only':
        expected_documents,expected_target=normalize.image_only_documents_data(record['snapshots'],record['old_images'],
            record['expected'],compiled['manifests'],source,renderer)
    else:expected_documents,expected_target=compiled['manifests'],source
    if documents!=expected_documents or target!=expected_target:
        raise RuntimeError('staging_capture_image_transformation_changed')
    actual=record['target']
    extra={'input_template_hashes'}
    if first:extra.add('gis_first_install_normalized_sha256')
    if 'gis_existing_normalized_sha256' in actual:extra.add('gis_existing_normalized_sha256')
    if (set(actual)!=set(target)|extra or actual['input_template_hashes']!=target['template_hashes']
        or any(actual[key]!=value for key,value in target.items() if key!='template_hashes')):
        raise RuntimeError('staging_capture_normalized_inventory_changed')
    rows=record['normalized_documents'];templates={}
    if type(rows) is not list or len(rows)>128:raise RuntimeError('staging_capture_normalized_inventory_changed')
    if first:
        import gis_first_install as gis
        plan=record['gis_first_install']
        if (documents!=[] or 'gis_first_install' not in compiled
            or state.get('gis_first_install')!=plan or rows!=plan['objects']
            or actual['gis_first_install_normalized_sha256']!=gis.normalized_binding(plan)):
            raise RuntimeError('staging_capture_gis_normalization_changed')
        try:
            initial=record['gis_first_install_input']
            original=gis.bind_operation(compiled['gis_first_install'],state['operation'],source)
            if any(initial.get(key)!=value for key,value in original.items()):raise ValueError()
            # The original DB receipt is produced only after preservation and
            # final fencing. An early normalized source plan does not have it.
            if 'prerequisite_receipt' in initial:
                gis.validate_prerequisite_receipt(initial['prerequisite_receipt'],initial)
                if initial['prerequisite_receipt']['namespace_uid']!=record['expected']['namespace_uid']:
                    raise ValueError()
            wanted,updated=normalize.first_install_normalized_data(documents,target,record['gis_first_install_input'],
                copy.deepcopy(rows),state['operation'],renderer)
        except (KeyError,TypeError,ValueError,RuntimeError,normalize.Blocked,gis.Blocked) as error:
            raise RuntimeError('staging_capture_gis_normalization_changed') from error
        if wanted!=actual or updated!=plan:
            raise RuntimeError('staging_capture_gis_normalization_changed')
        names={(o['kind'],o['metadata']['name']) for o in rows}
        if names!=set(gis.IDENTITIES) or len(rows)!=2:raise RuntimeError('staging_capture_gis_normalization_changed')
    else:
        requested=paused_documents(documents,{'target':target,'pvc':{'name':record['expected']['source_claim']}})
        names={(o['kind'],o['metadata']['name']) for o in requested}
        if len(names)!=len(requested):raise RuntimeError('staging_capture_normalized_inventory_changed')
    seen=set()
    for row in rows:
        key=(row.get('kind'),row.get('metadata',{}).get('name'))
        if key not in names or key in seen:raise RuntimeError('staging_capture_normalized_inventory_changed')
        seen.add(key)
        if row['kind']=='Deployment':
            name=key[1]
            if (row['spec'].get('replicas')!=0 or not first and
                row['metadata'].get('uid')!=record['snapshots'][name]['metadata']['uid']):
                raise RuntimeError('staging_capture_normalized_identity_changed')
            template=row['spec']['template'];containers=template['spec'].get('initContainers',[])+template['spec']['containers']
            if any(target['images'].get(name+'/'+c['name'])!=c['image'] for c in containers):
                raise RuntimeError('staging_capture_normalized_image_changed')
            templates[name]=digest(template)
    if renderer:
        from stage_runtime import HUB
        templates[HUB]=target['template_hashes'][HUB]
    if seen!=names or templates!=actual['template_hashes']:
        raise RuntimeError('staging_capture_normalized_inventory_changed')
    if existing:
        import gis_existing_update as gis_update
        import gis_first_install as gis
        raw=record['gis_existing']
        try:
            if (first or type(raw) is not dict or set(raw)!={'binding','deployment','service','selected'}
                or type(raw['selected']) is not bool):raise ValueError()
            binding=raw['binding'];deployment=raw['deployment'];service=raw['service']
            gis_update.validate(binding)
            deps=[row for row in rows if gis._identity(row)==gis.DEPLOYMENT]
            services=[row for row in rows if gis._identity(row)==gis.SERVICE]
            original=record['snapshots']['voice-gameintegration']
            if (len(deps)>1 or len(services)>1 or raw['selected']!=bool(deps)
                or deployment!=(deps[0] if deps else original)
                or services and service!=services[0]
                or not services and gis.semantic(service)!=binding['service']
                or original['metadata']['uid']!=binding['deployment_uid']
                or gis.semantic(original)!=binding['old']
                or record['expected']['namespace_uid']!=binding['namespace_uid']):raise ValueError()
            pair={'deployment_mode':'selected-paused' if raw['selected'] else 'preserved-active',
                'deployment_uid':deployment['metadata']['uid'],'service_uid':service['metadata']['uid'],
                'deployment':gis.semantic(deployment),'service':gis.semantic(service)}
            bound=gis_update.bind_normalized_target(binding,deployment,service,raw['selected'],target['mode'],
                actual['gis_existing_normalized_sha256'])
            if state.get('gis_binding')!=bound or state.get('gis_normalized_pair')!=pair:raise ValueError()
        except (KeyError,TypeError,ValueError,gis.Blocked) as error:
            raise RuntimeError('staging_capture_gis_normalization_changed') from error
    if 'context' in state:
        saved=state['context']
        if any(saved.get(key)!=record[key] for key in ('snapshots','old_images','expected')):
            raise RuntimeError('staging_capture_baseline_changed')


def _capture_state_data(state,installation):
    import guard
    required={'schema','installation_sha256','operation','phase','resource_fence','status','fence_status',
        'events','target','code_capture','contract','input_target'}
    optional={'service_actor_authority','renderer_authority','service_actor_services','prerequisites','realtime_bootstrap',
        'gis_first_install','gis_transition','gis_source_authority','provisional','repair_parent','source_authority','execution_authority',
        'gis_expected_active','gis_binding','gis_normalized_pair','gis_service_account','migrations','migration_secret_metadata',
        'space_preflight','nonnats','nonnats_binding','context','native_capacity','combined_space_capacity','provenance',
        'bootstrap_enrollment','nats_contract','nats_contract_binding','nats_target_scripts','nats_contract_expires_at',
        'kernel_sha256','error'}
    if (type(state) is not dict or not required<=set(state) or set(state)-required-optional
        or state['schema']!='nats-rollout-root-v2' or state['installation_sha256']!=installation.sha256
        or state['phase']!='READ_ONLY_CAPTURE' or type(state['events']) is not list):
        raise RuntimeError('staging_capture_fields_invalid')
    binding=state.get('resource_fence')
    _validate_fence_identity(binding,installation)
    if binding['target_sha256'] is not None:validate_fence_binding(binding,installation=installation)
    if (binding['operation']!=state.get('operation') or binding['helper_sha256']!=digest(state.get('code_capture'))
        or not isinstance(state.get('code_capture'),dict)
        or 'nats-rollout-preservation/resource_claim.py' not in state['code_capture']
        or state.get('status') not in ('RUNNING','BLOCKED') or state.get('fence_status')!='UNKNOWN'):
        raise RuntimeError('staging_capture_binding_changed')
    base=guard.ROOT/('rollout-'+binding['operation'])
    # The caller's dictionary alone is never protected root custody.
    if _capture_json(base/'checkpoint.json',128<<20)!=state:
        raise RuntimeError('staging_capture_checkpoint_changed')
    approved,execution,source=_capture_source(state,binding,installation)
    _capture_compiler(state,base,approved,source,installation)
    phase,_=_capture_children(state,binding,installation,approved,execution)
    return phase


def require_capture_state(session,state):
    """Continue only the same current admitted executor and exact saved snapshot."""
    installation=require_installation(session.installation if type(session) is Session else None)
    session=require_session(session,installation=installation)
    if state.get('phase')!='READ_ONLY_CAPTURE':raise RuntimeError('staging_capture_phase_invalid')
    phase=_capture_state_data(state,installation)
    if (session.operation!=state['operation'] or session.capture_binding()!=state['resource_fence']
        or session.check().get('phase')!=phase):
        raise RuntimeError('staging_capture_current_binding_changed')
    return state


def validate_state_binding(state,*,installation=None):
    """Dispatch protected root records without deriving their expected domain."""
    installation=require_installation(installation)
    if not isinstance(state,dict):raise RuntimeError('staging_installation_binding_changed')
    if state.get('schema')=='nats-rollout-root-v2':
        if (state.get('installation_sha256')!=installation.sha256
            or state.get('resource_fence',{}).get('schema')!='voice-staging-resource-fence-v2'):
            raise RuntimeError('staging_installation_binding_changed')
        if state.get('phase')=='READ_ONLY_CAPTURE':_capture_state_data(state,installation)
        else:historical_binding(state,installation=installation)
    elif state.get('schema')=='nats-rollout-root-v1':
        require_production(installation)
        if ('installation_sha256' in state or 'installation_sha256' in state.get('target',{})
            or state.get('resource_fence',{}).get('schema')=='voice-staging-resource-fence-v2'):
            raise RuntimeError('staging_installation_binding_changed')
    else:raise RuntimeError('staging_installation_binding_changed')
    return state


def require_session(session=None,*,installation=None,binding=None):
    """Use the existing bound command guard, never a separate Session registry."""
    import commands
    installation=require_installation(installation)
    parent=getattr(commands.authority_guard,'__self__',None)
    if session is None:session=parent
    if type(session) is not Session or not session.admitted:
        raise RuntimeError('staging_resource_session_required')
    if session.installation!=installation:raise RuntimeError('staging_installation_binding_changed')
    if parent is not session or commands.authority_guard!=session.check:
        raise RuntimeError('staging_resource_session_guard_required')
    if binding is not None:session.require_binding(binding)
    session.check()
    return session


@contextmanager
def runner_scope(receipt,root,helper_binding):
    """Runner has no journal write authority; active state alone blocks takeover."""
    require_helper(helper_binding)
    import commands
    parent=getattr(commands.authority_guard,'__self__',None)
    if type(parent) is Session and commands.authority_guard==parent.check:
        installation=parent.installation
    else:
        parent=None
        installation=production_installation()
    binding=receipt.get('resource_fence')
    validate_fence_binding(binding,installation=installation)
    if binding['schema']=='voice-staging-resource-fence-v2':
        if (receipt.get('schema')!='nats-rollout-preservation-v2'
            or receipt.get('installation_sha256')!=installation.sha256
            or receipt.get('target',{}).get('installation_sha256')!=installation.sha256):
            raise RuntimeError('staging_installation_binding_changed')
    elif 'installation_sha256' in receipt or 'installation_sha256' in receipt.get('target',{}):
        raise RuntimeError('staging_installation_binding_changed')
    if (binding['operation']!=receipt.get('operation') or binding['target_sha256']!=digest(receipt.get('target'))
        or binding['helper_sha256']!=digest(helper_binding)):
        raise RuntimeError('staging_resource_receipt_helper_or_target_changed')
    c=validate(binding['context']);check=Check(c)
    if parent is not None:
        # runner -> apply is one executor, with one failure-notification owner.
        if not parent.admitted:raise RuntimeError('staging_resource_session_not_admitted')
        parent.require_binding(binding)
        if parent.check().get('phase')!='apply-ready':raise RuntimeError('staging_resource_apply_not_authorized')
        yield parent
        return
    session=Session(c,binding['operation'],binding['helper_sha256'],check,RootJournal(root))
    session.admit(start=False)
    session.require_binding(binding)
    session.expected_target=binding['target_sha256'];session.expected_phase='apply-ready'
    if session.check().get('phase')!='apply-ready':raise RuntimeError('staging_resource_apply_not_authorized')
    try:
        with command_scope(session.check):yield session
    except BaseException:
        # The root-owned journal remains active even if this notification or
        # the runner disappears. Never retry an unknown POST or close locally.
        try:check.uncertain('captured_runner_interrupted_or_partial_apply')
        except Exception:print('STAGING_RESOURCE_RECOVERY_RECHECK_REQUIRED',file=sys.stderr)
        raise
