"""Cooperative TT executor admission, persisted independently of transient locks.

This cannot revoke a request already submitted to Kubernetes. Never recover an
active/uncertain executor without reconciling its processes and API effects.
"""
import copy
import json
import re
import urllib.parse
import urllib.request
import uuid

LEDGER = 'voice-dev-image-update-fence'


def validate_context(context):
    required={'project','resource','actor','claim','generation','window','revision',
              'operation_id','commit','images'}
    if not isinstance(context,dict) or set(context)!=required:
        raise ValueError('exact resource context fields required')
    if (context['project']!='voice' or context['resource']!='DEV'
        or not isinstance(context['actor'],str) or not context['actor'].strip()):
        raise ValueError('Voice DEV resource required')
    for name in ('claim','window'):
        if not isinstance(context[name],str): raise ValueError('canonical UUID required')
        if str(uuid.UUID(context[name]))!=context[name]: raise ValueError('canonical UUID required')
    for name in ('generation','revision'):
        if type(context[name]) is not int or context[name]<1: raise ValueError('positive authority required')
    if not isinstance(context['operation_id'],str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,127}',context['operation_id']):
        raise ValueError('external operation ID required')
    if not isinstance(context['commit'],str) or not re.fullmatch(r'[a-f0-9]{40}',context['commit']):
        raise ValueError('pinned candidate commit required')
    images=context['images']
    if not isinstance(images,dict) or not images: raise ValueError('candidate images required')
    for key,image in images.items():
        if (not isinstance(key,str) or not isinstance(image,str)
            or not re.fullmatch(r'voice-[a-z0-9-]+/[a-z0-9-]+',key)
            or not re.fullmatch(r'[^\s@]+@sha256:[a-f0-9]{64}',image)):
            raise ValueError('workload/container immutable image required')


class ResourceCheck:
    def __init__(self,endpoint,context):
        parsed=urllib.parse.urlsplit(endpoint)
        if parsed.scheme not in ('http','https') or not parsed.netloc or parsed.query or parsed.fragment or parsed.username:
            raise ValueError('explicit TT endpoint required')
        self.endpoint=endpoint.rstrip('/'); self.context=context

    def __call__(self):
        c=self.context
        query=urllib.parse.urlencode(dict(actor=c['actor'],claim=c['claim'],revision=c['revision'],phase='deploy'))
        url=self.endpoint+'/api/projects/voice/resources/DEV/check?'+query
        # Do not follow redirects to a different authority or retry an unknown response.
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self,*args): return None
        try:
            with urllib.request.build_opener(NoRedirect()).open(url,timeout=10) as response:
                raw=response.read((2<<20)+1)
            if len(raw)>2<<20: raise ValueError('resource response too large')
            envelope=json.loads(raw)
            if envelope.get('valid') is not True or not isinstance(envelope.get('resource'),dict):
                raise ValueError('invalid TT check response')
            return envelope['resource']
        except (OSError,ValueError) as exc:
            raise RuntimeError('TT authority unavailable; recheck exact card before continuing') from exc

    def mark_uncertain(self,reason):
        # An unknown response is never retried. Durable executor state is already
        # uncertain; the operator must reread TT/card history before reconciliation.
        self()
        c=self.context
        payload=dict(kind='release',actor=c['actor'],claim=c['claim'],revision=c['revision'],
                     safety='uncertain',reason=reason,pending=c['operation_id'],
                     evidence='DEV executor '+c['operation_id']+'; inspect '+LEDGER+' and retained updater lock',
                     next_action='Reconcile exact executor process, Kubernetes effects, actual version and locks')
        request=urllib.request.Request(self.endpoint+'/api/projects/voice/resources/DEV/actions',
                                       data=json.dumps(payload).encode(),headers={'Content-Type':'application/json'},method='POST')
        try:
            with urllib.request.urlopen(request,timeout=10) as response: response.read(2<<20)
        except OSError as exc:
            raise RuntimeError('TT recovery response unknown; reread card/history before retry') from exc


class KubeLedger:
    """Persistent high-water mark. Never delete it when releasing updater lock."""
    def __init__(self,client): self.client=client; self.document=None

    def read(self):
        row=self.client.call('get','configmap',LEDGER,'--ignore-not-found=true','-o','json')
        self.document=row
        if not row: return None,None
        meta=row['metadata']
        if meta['name']!=LEDGER or meta['namespace']!='voice-dev' or not meta.get('uid'):
            raise RuntimeError('fence identity invalid')
        return json.loads(row['data']['executor']), (meta['uid'],meta['resourceVersion'])

    def write(self,previous,row):
        data={'executor':json.dumps(row,sort_keys=True,separators=(',',':'))}
        if previous is None:
            result=self.client.call('create','-f','-','-o','json',body={
                'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':LEDGER,'namespace':'voice-dev'},'data':data})
        else:
            uid,rv=previous
            result=self.client.call('patch','configmap',LEDGER,'--type=json','--patch-file=-','-o','json',body=[
                {'op':'test','path':'/metadata/uid','value':uid},
                {'op':'test','path':'/metadata/resourceVersion','value':rv},
                {'op':'replace','path':'/data','value':data}])
        if result.get('data')!=data or not result.get('metadata',{}).get('uid'):
            raise RuntimeError('fence persistence outcome uncertain')


class Fence:
    def __init__(self,context,check,ledger,on_uncertain=None):
        validate_context(context)
        self.context=copy.deepcopy(context); self.check=check; self.ledger=ledger; self.card=None
        self.on_uncertain=on_uncertain

    def authority(self):
        row=self.check(); c=self.context
        expected=dict(code=c['resource'],environment='dev',owner=c['actor'],claim=c['claim'],
                      generation=c['generation'],window=c['window'],revision=c['revision'],state='held',phase='deploy')
        if any(row.get(k)!=v for k,v in expected.items()): raise RuntimeError('stale resource authority')
        candidate=row.get('candidate',{})
        if candidate.get('state')!='known' or candidate.get('commit')!=c['commit'] or candidate.get('images')!=c['images']:
            raise RuntimeError('candidate changed or not exactly pinned')
        self.card=row
        return row

    def validate_candidate(self,images):
        if images!=self.context['images']: raise ValueError('requested images differ from TT candidate')

    def owned(self,row):
        return row is not None and all(row.get(k)==self.context[k] for k in self.context)

    def begin(self):
        self.authority()
        previous,revision=self.ledger.read()
        if previous is not None:
            if (previous.get('project')!=self.context['project'] or previous.get('resource')!=self.context['resource']
                or previous.get('status')!='complete' or previous.get('generation',0)>=self.context['generation']
                or previous.get('operation_id')==self.context['operation_id']):
                raise RuntimeError('executor not reconciled or generation/operation already closed')
        self.ledger.write(revision,self.context|dict(status='active',step='admitted'))
        # A takeover during persistence cannot authorize the following workload write.
        self.authority()

    def begin_with_predecessor(self,client,lock,namespace_uid,journal):
        """Capture a completed C1 row in the same CAS that starts C2."""
        import full_cohort_predecessor_phase as predecessor
        if (not re.fullmatch(r'media-c2-[a-f0-9]{32}',self.context['operation_id']) or
                type(self.check) is not ResourceCheck or
                getattr(self.on_uncertain,'__self__',None) is not self.check or
                getattr(self.on_uncertain,'__func__',None) is not ResourceCheck.mark_uncertain):
            raise RuntimeError('predecessor capture requires the C2 operation')
        self.authority()
        previous,revision=self.ledger.read()
        descriptor=copy.deepcopy(predecessor.capture(self,client,lock,namespace_uid,journal,previous,revision))
        local=copy.deepcopy(journal.state['steps']['full-cohort-predecessor'])
        step='predecessor-admission'
        if step in journal.state['steps']:raise RuntimeError('predecessor admission already attempted')
        pending={'status':'pending','descriptor':descriptor,'old_revision':list(revision)}
        journal.state['steps'][step]=copy.deepcopy(pending);journal.save()
        attempted=False
        try:
            attempted=True
            self.ledger.write(revision,self.context | dict(status='active',step='admitted',predecessor=descriptor))
            self.authority()
            row,new_revision=self.ledger.read()
            if (not self.owned(row) or row.get('status')!='active' or row.get('step')!='admitted' or
                    row.get('predecessor')!=descriptor or not isinstance(new_revision,tuple) or
                    len(new_revision)!=2 or new_revision[0]!=revision[0] or
                    journal.dirfd is None or journal.lockfd is None or
                    journal.state['steps'].get('full-cohort-predecessor')!=local):raise RuntimeError('predecessor admission changed')
            predecessor.load(self,client,lock,namespace_uid,journal)
            if (self.ledger.read()!=(row,new_revision) or journal.state['steps'].get(step)!=pending or
                    journal.state['steps'].get('full-cohort-predecessor')!=local or
                    journal.dirfd is None or journal.lockfd is None):raise RuntimeError('predecessor admission changed')
            done={'status':'done','descriptor':descriptor,'old_revision':list(revision),'new_revision':list(new_revision)}
            journal.state['steps'][step]=copy.deepcopy(done);journal.save()
            self.authority()
            if (self.ledger.read()!=(row,new_revision) or journal.state['steps'].get(step)!=done or
                    journal.state['steps'].get('full-cohort-predecessor')!=local or
                    journal.dirfd is None or journal.lockfd is None):raise RuntimeError('predecessor admission changed')
        except BaseException:
            if attempted:
                try: self.admission_uncertain('C2 predecessor begin outcome uncertain')
                except BaseException: pass
            raise

    def begin_image_only(self,client,lock,namespace_uid,journal):
        """Preserve a qualified prior row atomically before initial C1 admission."""
        import copy
        import full_cohort_image_only_baseline as baseline
        if (not re.fullmatch(r'media-c1-[a-f0-9]{32}',self.context['operation_id']) or
                type(self.check) is not ResourceCheck or
                getattr(self.on_uncertain,'__self__',None) is not self.check or
                getattr(self.on_uncertain,'__func__',None) is not ResourceCheck.mark_uncertain):
            raise RuntimeError('image-only capture requires the C1 operation')
        self.authority()
        previous,revision=self.ledger.read()
        descriptor=copy.deepcopy(baseline.capture(self,client,lock,namespace_uid,journal,previous,revision))
        local=copy.deepcopy(journal.state['steps'][baseline.STEP])
        step='image-only-admission'
        if step in journal.state['steps']: raise RuntimeError('image-only admission already attempted')
        pending={'status':'pending','descriptor':descriptor,'old_revision':list(revision)}
        journal.state['steps'][step]=copy.deepcopy(pending);journal.save()
        attempted=False
        try:
            attempted=True
            self.ledger.write(revision,self.context | dict(status='active',step='admitted',image_only_baseline=descriptor))
            self.authority()
            row,new_revision=self.ledger.read()
            if (not self.owned(row) or row.get('status')!='active' or row.get('step')!='admitted' or
                    row.get('image_only_baseline')!=descriptor or not isinstance(new_revision,tuple) or
                    len(new_revision)!=2 or new_revision[0]!=revision[0] or
                    journal.dirfd is None or journal.lockfd is None or journal.state['steps'].get(baseline.STEP)!=local):
                raise RuntimeError('image-only admission changed')
            done={'status':'done','descriptor':descriptor,'old_revision':list(revision),'new_revision':list(new_revision)}
            journal.state['steps'][step]=copy.deepcopy(done);journal.save()
            self.authority()
            if (self.ledger.read()!=(row,new_revision) or journal.state['steps'].get(step)!=done or
                    journal.state['steps'].get(baseline.STEP)!=local or journal.dirfd is None or journal.lockfd is None):
                raise RuntimeError('image-only admission changed')
        except BaseException:
            if attempted:
                try:self.admission_uncertain('image-only begin outcome uncertain')
                except BaseException:pass
            raise

    def before_step(self,step):
        self.authority()
        row,revision=self.ledger.read()
        if not self.owned(row) or row.get('status')!='active': raise RuntimeError('executor fence changed or closed')
        row['step']=step; self.ledger.write(revision,row)
        self.authority()

    def validate_baseline(self,actual,ancestor):
        row=self.authority(); observed=row.get('observed',{})
        c=self.context
        if (observed.get('state')!='known' or observed.get('claim')!=c['claim'] or observed.get('window')!=c['window']
            or observed.get('operation')!=row.get('operation',0)-1 or observed.get('images')!=actual
            or not re.fullmatch(r'[a-f0-9]{40}',observed.get('commit',''))
            or not ancestor(observed['commit'],c['commit'])):
            raise RuntimeError('fresh actual baseline and non-rollback candidate qualification required')

    def complete(self,actual):
        self.before_step('verified-rollout')
        if actual!=self.context['images']: raise RuntimeError('actual version differs from pinned candidate')
        row,revision=self.ledger.read()
        if not self.owned(row) or row.get('status')!='active': raise RuntimeError('executor changed')
        row.update(status='complete',actual=actual); self.ledger.write(revision,row)

    def uncertain(self,reason):
        row,revision=self.ledger.read()
        if not self.owned(row) or row.get('status')!='active': raise RuntimeError('cannot settle another executor')
        row.update(status='uncertain',reason=reason); self.ledger.write(revision,row)
        if self.on_uncertain is not None: self.on_uncertain(reason)

    def admission_uncertain(self,reason):
        if self.on_uncertain is not None: self.on_uncertain(reason)
