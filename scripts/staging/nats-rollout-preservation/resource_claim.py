"""Cooperative STAGING executor fence; submitted Kubernetes requests are not revocable.

Only trusted installed/captured source chooses the tracker endpoint. Root owns
the durable journal, which stays occupied while the runner applies off-lock.
"""
import copy
from contextlib import contextmanager
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
    def __init__(self,context,operation,helper_sha256,check,journal):
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
        self._cohort_lineage=None

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
        return (isinstance(row,dict) and row.get('schema')=='voice-staging-resource-fence-v1'
            and row.get('context')==self.context and row.get('operation')==self.operation and row.get('helper_sha256')==self.helper)

    def admit(self,*,start,request=None):
        self.authority();previous=self.journal.read()
        if start:
            if previous is not None and previous.get('status')=='complete':
                try:
                    c=validate(previous.get('context'))
                    closed=(set(previous) in ({'schema','context','operation','helper_sha256','status','phase','target_sha256','actual'},
                                              {'schema','context','operation','helper_sha256','status','phase','target_sha256','actual','cohort_preservation_sha256'})
                        and previous['schema']=='voice-staging-resource-fence-v1'
                        and ('cohort_preservation_sha256' not in previous or isinstance(previous['cohort_preservation_sha256'],str)
                             and re.fullmatch('[a-f0-9]{64}',previous['cohort_preservation_sha256']))
                        and previous['phase']=='finished' and previous['actual']==c['images']
                        and re.fullmatch('[a-f0-9]{12}',previous['operation'])
                        and re.fullmatch('[a-f0-9]{64}',previous['helper_sha256'])
                        and re.fullmatch('[a-f0-9]{64}',previous['target_sha256']))
                except (ValueError,TypeError,KeyError):closed=False
                if not closed:raise RuntimeError('staging_closed_executor_reconciliation_required')
            if self.owned(previous) and previous.get('status')=='active':
                # The request journal alone decides completed replay versus
                # interrupted execution; never reset its durable executor intent.
                self.admitted=True;self.expected_target=previous.get('target_sha256');self._cohort_lineage=previous.get('cohort_preservation_sha256');self.authority();return
            if previous is not None and previous.get('status')!='complete' and self.repair_guard is not None:
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
            self.journal.save(dict(schema='voice-staging-resource-fence-v1',context=self.context,
                operation=self.operation,helper_sha256=self.helper,status='active',phase='prepare',target_sha256=None))
        elif not self.owned(previous) or previous.get('status') not in ('active','complete'):
            raise RuntimeError('staging_unfenced_or_uncertain_operation')
        if previous is not None and self.owned(previous):
            self.expected_target=previous.get('target_sha256');self._cohort_lineage=previous.get('cohort_preservation_sha256')
        self.admitted=True
        self.authority()

    def admit_request(self,request):self.admit(start=request['action'] in START_ACTIONS,request=request)

    def check(self,argv=None):
        self.authority();row=self.journal.read()
        if not self.owned(row) or row.get('status')!='active':raise RuntimeError('staging_executor_fence_closed_or_changed')
        if self.expected_target is not None and row.get('target_sha256')!=self.expected_target:
            raise RuntimeError('staging_executor_target_changed')
        if getattr(self,'expected_phase',None) is not None and row.get('phase')!=self.expected_phase:
            raise RuntimeError('staging_executor_phase_changed')
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
            readonly=self._prerequisite_readonly_permit
            if row['phase']=='prerequisites-captured' and readonly is not None and args==readonly:
                self._prerequisite_readonly_permit=None
                return row
            decode=['kubectl','create','--dry-run=client','--validate=false','-f','/dev/stdin','-o','json']
            prefixes=[['/usr/local/bin/k3s','kubectl','--namespace','voice-staging'],
                ['kubectl','--namespace','voice-staging']]
            is_dryrun=any(args==prefix+[verb,'--dry-run=server','-f','/dev/stdin','-o','json']
                for prefix in prefixes for verb in ('apply','create'))
            def safe_get(prefix):
                if args[:len(prefix)+1]!=prefix+['get']:return False
                tail=args[len(prefix)+1:]
                if len(tail)==2 and tail[0]=='--raw':
                    return bool(re.fullmatch(r'/api/v1/namespaces/voice-staging/(?:configmaps|secrets|services)\?fieldSelector=metadata\.name%3D[a-z0-9][a-z0-9.-]{0,252}&limit=2',tail[1])
                        or re.fullmatch(r'/api/v1/namespaces/voice-staging/pods/voice-nats-pvc-candidate-[a-z0-9-]{1,100}:8222/proxy/jsz\?accounts=true&streams=true&consumers=true&config=true&limit=2048',tail[1]))
                if len(tail)==5:
                    return (tail in ([kind,'-l','app=voice-nats-pvc-candidate','-o','json'] for kind in ('pods','replicasets'))
                        or tail==['endpointslices','-l','kubernetes.io/service-name=voice-postgres','-o','json'])
                if len(tail) not in (3,4) or tail[-2]!='-o':return False
                if not re.fullmatch(r'[a-z0-9][a-z0-9.,-]{0,127}',tail[0]):return False
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
            clickhouse_read=prefixes[0]+list(source_plan.CLICKHOUSE_READ_ARGS)
            postgres_reads=[prefixes[0]+source_plan.postgres_read_args(name) for name in ('postgres','gateway_db')]
            identity_read=False
            if args not in postgres_reads and args[:len(prefixes[0])+3]==prefixes[0]+['exec','voice-postgres-0','-c']:
                try:
                    import staging_cohort_preservation_pg
                    identity_args=[getattr(staging_cohort_preservation_pg,name,None)
                        for name in ('IDENTITY_ARGS','SPACE_IDENTITY_ARGS')]
                except ModuleNotFoundError as error:
                    if error.name!='staging_cohort_preservation_pg':raise
                    identity_args=[]
                identity_read=any(isinstance(value,(list,tuple)) and args==prefixes[0]+list(value) for value in identity_args)
            if args!=decode and args!=actor_read and args!=clickhouse_read and args not in postgres_reads and not identity_read and not is_dryrun and not is_get:
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
        if (plan.get('schema')!='voice-staging-prerequisites-plan-v2'
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
              or 'prerequisites_sha256' in plan or plan.get('schema')=='voice-staging-realtime-bootstrap-v2'):
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

    def binding(self):
        self.authority();row=self.journal.read()
        if not self.owned(row) or row.get('status') not in ('active','complete'):
            raise RuntimeError('staging_resource_binding_unavailable')
        if not row.get('target_sha256'):raise RuntimeError('staging_resource_target_unbound')
        return {k:copy.deepcopy(row[k]) for k in ('schema','context','operation','helper_sha256','target_sha256')}

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
    base,state=actions.state(request['operation'])
    binding=historical_binding(state)
    if (state.get('status')!='PROVISIONAL_READY' or state.get('phase')!='POST_RESTART'
        or state.get('fence_status')!='RELEASED'
        or binding!={k:previous.get(k) for k in ('schema','context','operation','helper_sha256','target_sha256')}):
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
    require_helper(actions.binding)
    c=validate(request.get('resource_context'))
    operation=request['nonce'][:12] if request['action'] in START_ACTIONS else request['operation']
    check=Check(c);session=Session(c,operation,digest(actions.binding),check,RootJournal(guard.ROOT))
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
        session.ready()
    if action in ('finish','finish-expired-native','accept-provisional'):
        _,state=actions.state(request['operation'])
        session.complete(state.get('status'),state.get('target',{}).get('images'))


def historical_binding(state):
    """Read a protected parent as data; never reuse its expired TT authority."""
    binding=state.get('resource_fence');target=state.get('target',{});helper=state.get('code_capture',{})
    if (not isinstance(binding,dict) or set(binding)!={'schema','context','operation','helper_sha256','target_sha256'}
        or binding.get('schema')!='voice-staging-resource-fence-v1'
        or binding.get('operation')!=state.get('operation') or binding.get('target_sha256')!=digest(target)
        or not isinstance(helper,dict) or 'nats-rollout-preservation/resource_claim.py' not in helper
        or binding.get('helper_sha256')!=digest(helper)):
        raise RuntimeError('staging_historical_unfenced_checkpoint_refused')
    c=validate(binding.get('context'))
    if c['commit']!=target.get('tag') or c['images']!=target.get('images'):
        raise RuntimeError('staging_historical_candidate_changed')
    return binding


@contextmanager
def runner_scope(receipt,root,helper_binding):
    """Runner has no journal write authority; active state alone blocks takeover."""
    require_helper(helper_binding)
    binding=receipt.get('resource_fence')
    if not isinstance(binding,dict) or set(binding)!={'schema','context','operation','helper_sha256','target_sha256'}:
        raise RuntimeError('staging_resource_legacy_receipt_refused')
    if (binding['operation']!=receipt.get('operation') or binding['target_sha256']!=digest(receipt.get('target'))
        or binding['helper_sha256']!=digest(helper_binding)):
        raise RuntimeError('staging_resource_receipt_helper_or_target_changed')
    c=validate(binding['context']);check=Check(c)
    import commands
    parent=getattr(commands.authority_guard,'__self__',None)
    if isinstance(parent,Session):
        # runner -> apply is one executor, with one failure-notification owner.
        parent.require_binding(binding)
        if parent.check().get('phase')!='apply-ready':raise RuntimeError('staging_resource_apply_not_authorized')
        yield parent
        return
    session=Session(c,binding['operation'],binding['helper_sha256'],check,RootJournal(root))
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
