"""Fixed Chat lifecycle Service CAS and post-resume receipt verifier."""
import copy
import re
import secrets

import backend_config_transition_deployment as backend
import media_operation_guard
import media_operation_journal
import realtime_config_transition as renderer
import writer_cut

STEP='backend-config-service-voice-chat'


class ChatServiceError(RuntimeError): pass


def _source(cut,journal):
    intent=journal.intent; plan=intent.get('realtime_chat_service')
    if (type(cut) is not writer_cut.WriterCut or type(journal) is not media_operation_journal.Journal
            or journal is cut.journal or journal.poisoned or journal.state.get('intent')!=intent
            or intent.get('resource_context')!=cut.fence.context or intent.get('namespace_uid')!=cut.namespace_uid
            or not isinstance(plan,dict) or set(plan)!={'source','source_sha','cut_invocation'}
            or plan['source_sha']!=cut.fence.context['commit'] or plan['cut_invocation']!=cut.invocation): raise ValueError()
    matches=[x for x in cut.admission['controllers'] if x.get('kind')=='Deployment' and x.get('metadata',{}).get('name')=='voice-chat']
    if len(matches)!=1: raise ValueError()
    source=copy.deepcopy(plan['source']); patch,target=renderer.render_chat_service(source,matches[0]['spec']['template']['metadata']['labels'])
    details={'operation_id':cut.fence.context['operation_id'],'invocation_nonce':cut.invocation,
             'source':source,'target_spec':target,'patch':patch}
    return source,target,patch,details


def _check(cut,journal):
    cut._guard(); row,_=cut.fence.ledger.read()
    if not cut._owned(row) or row.get('backend_config_effects',{})!=backend._effect_map(cut,journal): raise ValueError()


def apply_chat_service(cut,journal):
    """Apply one source-planned CAS while the writer cut is held."""
    pending=False
    try:
        cut.verify_drained(); source,target,patch,details=_source(cut,journal); _check(cut,journal)
        if STEP in journal.state['steps']: raise ValueError()
        actual=cut.client.call('get','service','voice-chat','-o','json'); meta=source['metadata']; current=actual['metadata']
        if (actual.get('apiVersion')!='v1' or actual.get('kind')!='Service' or actual.get('spec')!=source['spec']
                or current.get('deletionTimestamp') or any(current.get(k)!=meta.get(k) for k in ('name','namespace','uid','resourceVersion','generation'))
                or ('generation' in current)!=('generation' in meta)
                or ('generation' in meta and (type(meta['generation']) is not int or meta['generation']<0))): raise ValueError()
        record={'status':'pending','nonce':secrets.token_hex(16),'details_sha256':writer_cut.digest(details)}
        row,rev=cut.fence.ledger.read(); pending=True
        expected=record|{'details':copy.deepcopy(details)}; journal.state['steps'][STEP]=copy.deepcopy(expected); journal.save()
        now,version=cut.fence.ledger.read()
        if now!=row or version!=rev or row.get('backend_config_effects',{})!=backend._effect_map(cut,journal,exclude=STEP): raise ValueError()
        now.setdefault('backend_config_effects',{})[STEP]=copy.deepcopy(record); now['step']=STEP
        cut.fence.ledger.write(rev,now); cut.verify_drained(); _check(cut,journal)
        if journal.state['steps'].get(STEP)!=expected: raise ValueError()
        response=cut._patch('Service','voice-chat',patch); actual=cut.client.call('get','service','voice-chat','-o','json')
        for obj in (response,actual):
            m=obj['metadata']
            if (obj.get('apiVersion')!='v1' or obj.get('kind')!='Service' or obj.get('spec')!=target
                    or m.get('name')!='voice-chat' or m.get('namespace')!='voice-dev' or m.get('uid')!=meta['uid']
                    or m.get('deletionTimestamp') or not isinstance(m.get('resourceVersion'),str) or not m['resourceVersion']
                    or ('generation' in m)!=('generation' in meta) or m.get('generation')!=meta.get('generation')): raise ValueError()
        receipt={'uid':meta['uid'],'spec_sha256':writer_cut.digest(target),'responseResourceVersion':response['metadata']['resourceVersion'],
                 'resourceVersion':actual['metadata']['resourceVersion']}
        if 'generation' in meta: receipt['generation']=meta['generation']
        cut.verify_drained(); _check(cut,journal); row,rev=cut.fence.ledger.read()
        if row['backend_config_effects'].get(STEP)!=record or journal.state['steps'].get(STEP)!=expected: raise ValueError()
        done=record|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
        local=expected|{'status':'done','receipt':copy.deepcopy(receipt)}; journal.state['steps'][STEP]=copy.deepcopy(local); journal.save()
        now,version=cut.fence.ledger.read()
        if now!=row or version!=rev: raise ValueError()
        now['backend_config_effects'][STEP]=done; now['step']=STEP+'-done'; cut.fence.ledger.write(rev,now)
        _check(cut,journal)
        if journal.state['steps'].get(STEP)!=local: raise ValueError()
        return copy.deepcopy(receipt)
    except BaseException:
        if pending:
            cut.poisoned=True
            try: cut.fence.uncertain('Chat Service configuration uncertain')
            except BaseException: pass
        raise ChatServiceError('Chat Service configuration unavailable') from None


def verify_chat_service(cut,journal):
    """Reopen protected Chat Service CAS and compare the actual current Service."""
    try:
        source,target,patch,details=_source(cut,journal); _check(cut,journal)
        row,rev=cut.fence.ledger.read(); entry=copy.deepcopy(journal.state['steps'][STEP]); receipt=entry['receipt']
        if (set(entry)!={'status','nonce','details_sha256','details','receipt'} or entry['status']!='done'
                or not isinstance(entry['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',entry['nonce'])
                or entry['details']!=details or entry['details_sha256']!=writer_cut.digest(details)
                or row['backend_config_effects'].get(STEP)!={'status':'done','nonce':entry['nonce'],
                    'details_sha256':entry['details_sha256'],'receipt_sha256':writer_cut.digest(receipt)}): raise ValueError()
        actual=cut.client.call('get','service','voice-chat','-o','json'); meta=actual['metadata']; original=source['metadata']
        required={'uid','spec_sha256','responseResourceVersion','resourceVersion'} | ({'generation'} if 'generation' in original else set())
        if (set(receipt)!=required or actual.get('apiVersion')!='v1' or actual.get('kind')!='Service' or actual.get('spec')!=target
                or meta.get('name')!='voice-chat' or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
                or meta.get('uid')!=original['uid'] or meta.get('resourceVersion')!=receipt['resourceVersion']
                or ('generation' in meta)!=('generation' in original) or meta.get('generation')!=original.get('generation')
                or receipt.get('uid')!=original['uid'] or receipt.get('spec_sha256')!=writer_cut.digest(target)
                or any(not isinstance(receipt[k],str) or not receipt[k] for k in ('responseResourceVersion','resourceVersion'))): raise ValueError()
        _check(cut,journal); after,end=cut.fence.ledger.read()
        if after!=row or end!=rev or journal.state['steps'].get(STEP)!=entry: raise ValueError()
        return {'source':source,'target_spec':target,'receipt':copy.deepcopy(receipt),'ledger_record':copy.deepcopy(row['backend_config_effects'][STEP])}
    except Exception: raise ChatServiceError('Chat Service successor unavailable') from None
