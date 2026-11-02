"""Fenced, source-derived Chat/Realtime configuration successors."""
import copy
import re
import secrets
import time

import backend_config_transition_deployment as backend
import media_operation_guard
import media_operation_journal
import realtime_config_transition as renderer
import realtime_config_transition_authority as authority
import realtime_config_transition_inputs as inputs
import realtime_config_transition_space_plan_phase as space_plan
import realtime_migration_job
import realtime_p3_create_only
import writer_cut


class DeploymentError(RuntimeError): pass


class Dependencies:
    def __init__(self,cut,journal,session,migration,space_phase,publisher_phase,source_root,token):
        import realtime_jwks_publisher as publisher
        if (type(cut) is not writer_cut.WriterCut or type(journal) is not media_operation_journal.Journal
                or type(session) is not realtime_p3_create_only.Session or type(migration) is not realtime_migration_job.Migration
                or type(publisher_phase) is not publisher.PublisherPhase
                or not isinstance(token,str) or not token): raise ValueError()
        if type(space_phase) is authority.BoundFixed9SpaceAuthority:
            if space_phase.cut is not cut or space_phase.token!=token: raise ValueError()
        elif type(space_phase) is not space_plan.SpacePlanPhase:
            import space_p3_issuer_create as issuer
            if type(space_phase) is not issuer.Session: raise ValueError()
        self.cut,self.journal,self.session,self.migration,self.space_phase,self.publisher_phase=cut,journal,session,migration,space_phase,publisher_phase
        self.source_root,self.token=source_root,token
        self.intent=copy.deepcopy(journal.intent)


def bind_dependencies(cut,journal,session,migration,space_phase,publisher_phase,source_root,token):
    """Attach live fixed producer phases; no callback or caller receipt is authority."""
    if hasattr(journal,'_realtime_config_dependencies'): raise DeploymentError('dependencies already bound')
    try:
        deps=Dependencies(cut,journal,session,migration,space_phase,publisher_phase,source_root,token)
        plan=journal.intent['realtime_config_deployments']
        if (set(plan)!={'source_root','source_sha','cut_invocation'} or plan['source_root']!=source_root
                or plan['source_sha']!=cut.fence.context['commit'] or plan['cut_invocation']!=cut.invocation): raise ValueError()
        journal._realtime_config_dependencies=deps
    except Exception: raise DeploymentError('configuration dependencies unavailable') from None


def _plan(cut,journal,service):
    deps=journal._realtime_config_dependencies; intent=journal.intent
    if (type(deps) is not Dependencies or deps.cut is not cut or deps.journal is not journal or journal.poisoned
            or intent!=deps.intent or journal.state.get('intent')!=intent or intent.get('resource_context')!=cut.fence.context
            or intent.get('namespace_uid')!=cut.namespace_uid or service not in ('chat','realtime','space')): raise ValueError()
    plan=intent['backend_config_deployments']; names=plan['services']
    if (not isinstance(names,list) or not names or names!=sorted(set(names)) or service not in names
            or any(x not in ('chat','notification','realtime','space','user') for x in names)
            or set(plan['sources'])!=set(names) or plan['source_sha']!=cut.fence.context['commit']
            or plan['cut_invocation']!=cut.invocation): raise ValueError()
    bound=cut.journal.state['steps']['writer-cut-config-plan']; row,_=cut.fence.ledger.read()
    details={'intent':intent,'steps':{},'effects':{},'services':[]}
    receipt={'operation_id':cut.fence.context['operation_id'],'cut_invocation':cut.invocation,
             'services':names,'config_intent_sha256':writer_cut.digest(intent)}
    if (set(bound)!={'status','nonce','details_sha256','details','receipt'} or bound['status']!='done'
            or not isinstance(bound['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',bound['nonce'])
            or bound['details']!=details or bound['receipt']!=receipt or bound['details_sha256']!=writer_cut.digest(details)
            or row['writer_cut_effects'].get('writer-cut-config-plan')!={'status':'done','nonce':bound['nonce'],
                'details_sha256':bound['details_sha256'],'receipt_sha256':writer_cut.digest(receipt)}): raise ValueError()
    source=copy.deepcopy(plan['sources'][service]); meta=source['metadata']; spec=source['spec']
    originals=[x for x in cut.admission['controllers'] if x.get('kind')=='Deployment' and x.get('metadata',{}).get('name')=='voice-'+service]
    drained=cut.journal.state['steps']['writer-drain-deployment-voice-'+service]
    if (len(originals)!=1 or drained.get('status')!='done' or meta.get('uid')!=originals[0]['metadata']['uid']
            or meta.get('uid')!=drained['receipt']['uid'] or meta.get('generation')!=drained['receipt']['generation']
            or spec!=drained['details']['target_spec'] or type(spec.get('replicas')) is not int or spec['replicas']!=0
            or row['writer_cut_effects']['writer-drain-deployment-voice-'+service].get('receipt_sha256')!=writer_cut.digest(drained['receipt'])): raise ValueError()
    evidence=authority.collect(cut,journal,deps.session,deps.migration,deps.space_phase,deps.publisher_phase,deps.source_root,deps.token)
    patch,target=(renderer.render_realtime(source,evidence,cut.fence.context['operation_id']) if service=='realtime'
                  else renderer.render_chat(source,evidence['assets']['voice-app-config']['keys']) if service=='chat'
                  else renderer.render_space(source,evidence['assets']['voice-app-config']['keys']))
    return source,target,patch


def verify_deployment_successor(cut,journal,service):
    """Reconstruct protected source-owned config effect without a zero-Pod check."""
    try:
        cut._guard(); source,target,patch=_plan(cut,journal,service)
        step='backend-config-deployment-voice-'+service; entry=copy.deepcopy(journal.state['steps'][step]); row,rev=cut.fence.ledger.read()
        details={'operation_id':cut.fence.context['operation_id'],'invocation_nonce':cut.invocation,'source':source,'target_spec':target,'patch':patch}
        receipt=entry['receipt']; expected={'status':'done','nonce':entry['nonce'],'details_sha256':writer_cut.digest(details),
                                            'receipt_sha256':writer_cut.digest(receipt)}
        if (set(entry)!={'status','nonce','details_sha256','details','receipt'} or entry['status']!='done'
                or not isinstance(entry['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',entry['nonce'])
                or entry['details']!=details or entry['details_sha256']!=expected['details_sha256']
                or row.get('backend_config_effects',{}).get(step)!=expected
                or set(receipt)!={'uid','spec_sha256','responseResourceVersion','resourceVersion','generation'}
                or receipt['uid']!=source['metadata']['uid'] or receipt['spec_sha256']!=writer_cut.digest(target)
                or type(receipt['generation']) is not int or receipt['generation']!=source['metadata']['generation']+1
                or any(not isinstance(receipt[k],str) or not receipt[k] for k in ('responseResourceVersion','resourceVersion'))): raise ValueError()
        cut._guard(); after,end=cut.fence.ledger.read()
        if after!=row or end!=rev or journal.state['steps'].get(step)!=entry: raise ValueError()
        return {'source':source,'target_spec':target,'receipt':copy.deepcopy(receipt),'ledger_record':copy.deepcopy(expected)}
    except Exception: raise DeploymentError('P3 configuration successor unavailable') from None


def read_realtime_route(cut,journal):
    """Read the protected Realtime DB route after resume, without a drain check."""
    try:
        cut._guard(); before,revision=cut.fence.ledger.read()
        deps=journal._realtime_config_dependencies; step='backend-config-deployment-voice-realtime'
        snapshot=(copy.deepcopy(journal.intent),copy.deepcopy(journal.state.get('intent')),
                  copy.deepcopy(journal.state['steps'].get(step)),copy.deepcopy(deps.migration.intent),
                  copy.deepcopy(deps.migration.journal.state),copy.deepcopy(deps.session.intent),copy.deepcopy(deps.session.journal.state))
        proof=verify_deployment_successor(cut,journal,'realtime')
        source=inputs.observe(cut,deps.session,deps.migration,deps.source_root)
        execution=source['migration']['execution']; credential=deps.migration.intent['realtime_migration']['credential']
        containers=[c for c in proof['target_spec']['template']['spec']['containers'] if c.get('name')=='realtime']
        if len(containers)!=1: raise ValueError()
        entries=[e for e in containers[0].get('env',[]) if e.get('name')=='DATABASE_URL']
        if (len(entries)!=1 or entries[0]!={'name':'DATABASE_URL','valueFrom':{'secretKeyRef':{'name':credential['name'],'key':'DATABASE_URL'}}}
                or execution.get('database')!='realtime_db' or execution.get('role')!='realtime'
                or type(execution.get('database_oid')) is not int or execution['database_oid']<=0): raise ValueError()
        cut._guard(); after,end=cut.fence.ledger.read()
        if (after!=before or end!=revision or snapshot!=(journal.intent,journal.state.get('intent'),journal.state['steps'].get(step),
                deps.migration.intent,deps.migration.journal.state,deps.session.intent,deps.session.journal.state)): raise ValueError()
        return {'database':'realtime_db','role':'realtime','database_oid':execution['database_oid'],
                'deployment_uid':proof['receipt']['uid'],'target_spec_sha256':proof['receipt']['spec_sha256'],
                'scope':'realtime-config-migration-route'}
    except Exception: raise DeploymentError('Realtime route unavailable') from None


def apply(cut,journal,service):
    """Apply one fixed successor, then immediately have WriterCut accept it."""
    pending=False
    try:
        cut.verify_drained(); source,target,patch=_plan(cut,journal,service)
        names=journal.intent['backend_config_deployments']['services']
        previous=cut.config_snapshot['services'] if cut.config_journal is journal and isinstance(cut.config_snapshot,dict) else None
        if not isinstance(previous,list) or previous!=names[:len(previous)] or len(previous)>=len(names) or names[len(previous)]!=service: raise ValueError()
        step='backend-config-deployment-voice-'+service
        if step in journal.state['steps']: raise ValueError()
        actual=cut.client.call('get','deployment','voice-'+service,'-o','json'); meta=source['metadata']; am=actual['metadata']
        if actual.get('spec')!=source['spec'] or any(am.get(k)!=meta.get(k) for k in ('uid','resourceVersion','generation','name','namespace')) or am.get('deletionTimestamp'): raise ValueError()
        details={'operation_id':cut.fence.context['operation_id'],'invocation_nonce':cut.invocation,'source':source,'target_spec':target,'patch':patch}
        record={'status':'pending','nonce':secrets.token_hex(16),'details_sha256':writer_cut.digest(details)}
        row,rev=cut.fence.ledger.read()
        if not cut._owned(row) or row.get('backend_config_effects',{})!=backend._effect_map(cut,journal): raise ValueError()
        pending=True; local=record|{'details':copy.deepcopy(details)}; journal.state['steps'][step]=copy.deepcopy(local); journal.save()
        now,version=cut.fence.ledger.read()
        if now!=row or version!=rev: raise ValueError()
        now.setdefault('backend_config_effects',{})[step]=copy.deepcopy(record); now['step']=step
        cut.fence.ledger.write(rev,now); cut.verify_drained()
        if journal.state['steps'].get(step)!=local: raise ValueError()
        current,_=cut.fence.ledger.read()
        if current.get('backend_config_effects',{})!=backend._effect_map(cut,journal): raise ValueError()
        if _plan(cut,journal,service)!=(source,target,patch): raise ValueError()
        cut._guard(); current,_=cut.fence.ledger.read()
        if current.get('backend_config_effects',{})!=backend._effect_map(cut,journal) or journal.state['steps'].get(step)!=local: raise ValueError()
        response=cut._patch('Deployment','voice-'+service,patch)
        def check(obj):
            m=obj['metadata']
            if (obj.get('apiVersion')!='apps/v1' or obj.get('kind')!='Deployment' or obj.get('spec')!=target
                    or m.get('uid')!=meta['uid'] or m.get('name')!=meta['name'] or m.get('namespace')!='voice-dev'
                    or m.get('deletionTimestamp') or type(m.get('generation')) is not int or m['generation']!=meta['generation']+1
                    or not isinstance(m.get('resourceVersion'),str) or not m['resourceVersion']): raise ValueError()
        check(response); deadline=time.monotonic()+60
        while True:
            media_operation_guard.verify(cut.fence,cut.client,cut.lock,cut.namespace_uid); cut._guards()
            if not cut._pods_drained(): raise ValueError()
            current,_=cut.fence.ledger.read()
            if current.get('backend_config_effects',{})!=backend._effect_map(cut,journal) or journal.state['steps'].get(step)!=local: raise ValueError()
            observed=cut.client.call('get','deployment','voice-'+service,'-o','json'); check(observed)
            status=observed.get('status',{}); generation=status.get('observedGeneration')
            if type(generation) is int and generation>=meta['generation']+1 and status.get('replicas',0)==0 and status.get('readyReplicas',0)==0: break
            if time.monotonic()>=deadline: raise ValueError()
            time.sleep(min(1,max(0,deadline-time.monotonic())))
        if _plan(cut,journal,service)!=(source,target,patch): raise ValueError()
        current,rev=cut.fence.ledger.read()
        if current.get('backend_config_effects',{})!=backend._effect_map(cut,journal) or journal.state['steps'].get(step)!=local: raise ValueError()
        receipt={'uid':meta['uid'],'spec_sha256':writer_cut.digest(target),'responseResourceVersion':response['metadata']['resourceVersion'],
                 'resourceVersion':observed['metadata']['resourceVersion'],'generation':observed['metadata']['generation']}
        done=record|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}; complete=local|{'status':'done','receipt':copy.deepcopy(receipt)}
        journal.state['steps'][step]=copy.deepcopy(complete); journal.save()
        now,version=cut.fence.ledger.read()
        if now!=current or version!=rev: raise ValueError()
        now['backend_config_effects'][step]=done; now['step']=step+'-done'; cut.fence.ledger.write(rev,now)
        media_operation_guard.verify(cut.fence,cut.client,cut.lock,cut.namespace_uid)
        after,_=cut.fence.ledger.read()
        if after.get('backend_config_effects',{})!=backend._effect_map(cut,journal) or journal.state['steps'].get(step)!=complete: raise ValueError()
        cut.accept_config_successor(journal,service); cut.verify_drained(); return copy.deepcopy(receipt)
    except BaseException:
        if pending:
            cut.poisoned=True
            try: cut.fence.uncertain('P3 Deployment configuration uncertain')
            except BaseException: pass
        raise DeploymentError('P3 Deployment configuration unavailable') from None
