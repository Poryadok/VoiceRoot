"""Fixed DEV Space installer dispatch within the cooperative writer cut.

This module accepts only the concrete source-owned Space phase. Session
attribution is bounded operational evidence, not an atomic SQL revocation gate.
"""
import copy

import space_fresh_install_phase as fresh
import writer_cut
import writer_cut_pg_probe as pg

BIND='writer-cut-space-bind'
FINISH='writer-cut-space-finish'


def _pair(cut,phase):
    if (type(cut) is not writer_cut.WriterCut or type(phase) is not fresh.SpaceFreshInstallPhase or
        phase.cut is not cut or phase.fence.context!=cut.fence.context or phase.lock!=cut.lock or
        phase.namespace_uid!=cut.namespace_uid or not cut.started or not cut.drained or
        cut.migration is not None or cut.provision_successor is not None):raise ValueError()
    cut._guard();phase._guard()
    left,rev=cut.fence.ledger.read();right,other=phase.fence.ledger.read()
    if (left!=right or rev!=other or not cut._owned(left)):raise ValueError()
    return copy.deepcopy(left),rev


def _refs(cut,phase):
    return (cut.journal,cut.fence,cut.client,cut.fence.ledger,phase,phase.journal,phase.fence,phase.client,phase.fence.ledger)


def verify_binding(cut,require_finished=False):
    """Recheck private binding and exact durable CUT effects; no PG/CUT recursion."""
    private=cut._space_private;binding=cut.space_fresh;phase=private['phase']
    if (binding is not private['binding'] or set(binding)!={'phase','token','finished','receipt_sha256'} or
        binding['phase'] is not phase or binding['token']!=private['token'] or
        binding['finished'] is not private['finished'] or binding['receipt_sha256']!=private['receipt_sha256'] or
        (require_finished and not binding['finished']) or
        any(a is not b for a,b in zip(_refs(cut,phase),private['refs'])) or
        cut.fence.context!=private['context'] or cut.lock!=private['lock'] or
        cut.admission!=private['admission']):raise ValueError()
    # Later typed Realtime provisioning is permitted only for historical use;
    # active Space windows always precede that producer.
    if binding['finished']:
        cut._guard();phase._guard()
        row,rev=cut.fence.ledger.read();other,other_rev=phase.fence.ledger.read()
        if row!=other or rev!=other_rev or not cut._owned(row):raise ValueError()
    else:row,rev=_pair(cut,phase)
    if {k for k in cut.journal.state['steps'] if k in (BIND,FINISH)} != set(private['records']):raise ValueError()
    for name,expected in private['records'].items():
        if (cut.journal.state['steps'].get(name)!=expected or
            row.get('writer_cut_effects',{}).get(name)!=
                {k:expected[k] for k in ('status','nonce','details_sha256')}|
                {'receipt_sha256':writer_cut.digest(expected['receipt'])}):raise ValueError()
    if require_finished and FINISH not in private['records']:raise ValueError()
    source=phase._read();details=source[0];descriptor=private['records'][BIND]['details']
    if (source[1]!=row or source[2]!=rev or source[3]['marker']['nonce']!=descriptor['phase_nonce'] or
        details['old_oid']!=descriptor['old_oid'] or fresh._digest(details['catalog'])!=descriptor['original_catalog_sha256'] or
        fresh._digest(details['tool'])!=descriptor['tool_sha256'] or
        (binding['finished'] and (details['state']!='done' or source[3]['marker']['details_sha256']!=binding['receipt_sha256'])) or
        any(cut.journal.state['steps'].get(k)!=v for k,v in private['records'].items())):raise ValueError()
    return binding


def _boundary(cut):
    verify_binding(cut)
    cut._guards();cut._recheck_config_successors()
    if not cut._retired():raise ValueError()
    verify_binding(cut)


def _scope(catalog,engine,start):
    oids=copy.deepcopy(catalog['all_databases'])
    if 'postgres' not in oids:raise ValueError()
    return {'engine':copy.deepcopy(engine),'server_start':start,'databases':sorted(oids),'database_oids':oids}


def _identity(details,pod=None):
    return {'old_oid':details['old_oid'],'new_oid':details['new_oid'],'role':details['installer_role'],
        'pod_uid':pod['uid'] if pod else None,'pod_ip':pod['pod_ip'] if pod else None,
        'run_id':details['run_id'],'job_kind':details['job_kind']}


def _zero(receipt,scope,allow_owned):
    if (type(receipt) is not dict or set(receipt)!={'scope','active_sessions','active_transactions','attributed_sessions'} or
        receipt['scope']!=scope or type(receipt['active_sessions']) is not int or receipt['active_sessions']!=0 or
        type(receipt['active_transactions']) is not int or receipt['active_transactions']!=0 or
        not isinstance(receipt['attributed_sessions'],list) or
        (not allow_owned and receipt['attributed_sessions'])):raise ValueError()


def _same(cut,before):
    phase=verify_binding(cut)['phase']
    phase._stable(before)
    verify_binding(cut)
    # No callback after the final comparison.
    if (cut.fence.ledger.read()!=(before[1],before[2]) or
        phase.journal.state['steps'].get(fresh.STEP)!=before[3] or
        any(cut.journal.state['steps'].get(k)!=v for k,v in cut._space_private['records'].items())):raise ValueError()
    phase._identity();cut._identity()
    private=cut._space_private;binding=cut.space_fresh
    if (binding is not private['binding'] or set(binding)!={'phase','token','finished','receipt_sha256'} or
        binding.get('phase') is not phase or
        binding.get('token')!=private['token'] or binding.get('finished') is not private['finished'] or
        binding.get('receipt_sha256')!=private['receipt_sha256'] or
        any(a is not b for a,b in zip(_refs(cut,phase),private['refs'])) or
        cut.fence.context!=private['context'] or cut.lock!=private['lock'] or cut.admission!=private['admission'] or
        phase.journal.state['steps'].get(fresh.STEP)!=before[3] or
        any(cut.journal.state['steps'].get(k)!=v for k,v in private['records'].items())):raise ValueError()


def bind(cut,phase,token):
    try:
        if getattr(cut,'space_fresh',None) is not None or hasattr(cut,'_space_private') or token!=phase.token:raise ValueError()
        _pair(cut,phase);cut.verify_drained()
        before=phase._read();details=before[0]
        if details['state']!='drop_pending' or details['resources'] or details['new_oid'] is not None:raise ValueError()
        fresh.verify_catalog_successor(phase,cut,token);phase._stable(before)
        scope=cut.admission['session_scope'];original=details['catalog']
        if (scope!=_scope(original,original['engine'],original['server_start']) or
            scope['database_oids'].get('space_db')!=details['old_oid']):raise ValueError()
        descriptor={'operation_id':phase.context['operation_id'],'source_sha':phase.context['commit'],
            'namespace_uid':phase.namespace_uid,'invocation':cut.invocation,'old_oid':details['old_oid'],
            'phase_nonce':before[3]['marker']['nonce'],'original_catalog_sha256':fresh._digest(original),
            'tool_sha256':fresh._digest(details['tool'])}
        record=cut._reserve(BIND,descriptor)
        cut._complete(BIND,record,descriptor)
        binding={'phase':phase,'token':token,'finished':False,'receipt_sha256':None}
        cut.space_fresh=binding
        cut._space_private={'phase':phase,'token':token,'binding':binding,'finished':False,'receipt_sha256':None,
            'refs':_refs(cut,phase),'context':copy.deepcopy(cut.fence.context),'lock':copy.deepcopy(cut.lock),
            'admission':copy.deepcopy(cut.admission),'records':{BIND:copy.deepcopy(cut.journal.state['steps'][BIND])}}
        # CUT reservation changed the shared row, but cannot change the source reservation.
        if phase._read()[3]!=before[3]:raise ValueError()
        pending(cut)
        return copy.deepcopy(descriptor)
    except BaseException:cut._uncertain()


def pending(cut):
    try:
        binding=verify_binding(cut);phase=binding['phase'];before=phase._read();details=before[0]
        if binding['finished'] or details['state'] not in ('drop_pending','drop_done','job_pending'):raise ValueError()
        fresh.verify_catalog_successor(phase,cut,binding['token'])
        actual=copy.deepcopy(details['catalog'])
        if details['state']!='drop_pending':
            for key in ('all_databases','databases','excluded','properties'):actual[key].pop('space_db',None)
        scope=_scope(actual,details['catalog']['engine'],details['catalog']['server_start'])
        _boundary(cut)
        receipt=pg.inspect_space(scope,_identity(details),cut._guard);_zero(receipt,scope,False)
        fresh.verify_catalog_successor(phase,cut,binding['token'])
        _boundary(cut);_same(cut,before)
        return copy.deepcopy(receipt)
    except BaseException:cut._uncertain()


def running(cut):
    try:
        binding=verify_binding(cut);phase=binding['phase'];before=phase._read();details=before[0]
        if binding['finished'] or details['state'] not in ('job_owned','created','done'):raise ValueError()
        proof=fresh.verify_writer_successor(phase,binding['token']);pod=proof['owned_pod']
        scope=_scope(proof['catalog_actual'],proof['engine'],proof['server_start'])
        identity=_identity(details,pod)
        # Before the created event, current OID is temporary observer input only.
        identity['new_oid']=proof['catalog_actual']['all_databases'].get('space_db')
        _boundary(cut)
        receipt=pg.inspect_space(scope,identity,cut._guard)
        _zero(receipt,scope,pod is not None and proof['record']['state']=='owned' and details['state']!='done')
        if fresh.verify_writer_successor(phase,binding['token'])!=proof:raise ValueError()
        _boundary(cut);_same(cut,before)
        return copy.deepcopy(receipt)
    except BaseException:cut._uncertain()


def guard_jobs(cut,actual):
    """Remove only independently re-read exact owned source Jobs from inventory."""
    binding=verify_binding(cut);phase=binding['phase'];before=phase._read();details=before[0]
    rows=copy.deepcopy(actual);uids=[x['metadata']['uid'] for x in rows]
    if len(uids)!=len(set(uids)):raise ValueError()
    for key in ('provision','verify'):
        record=details['resources'].get(key)
        if not record:continue
        if record['state'] in ('owned','removing'):
            observed=phase._get('job',record['authored']['metadata']['name'])
            fresh.jobs.job(record['authored'],observed)
            if observed['metadata']['uid']!=record['uid']:raise ValueError()
            selected=[x for x in rows if x['metadata']['uid']==record['uid']]
            if selected!=writer_cut.inventory([observed]):raise ValueError()
            rows=[x for x in rows if x['metadata']['uid']!=record['uid']]
        elif record['state']=='removed':
            if phase._get('job',record['authored']['metadata']['name']):raise ValueError()
        elif record['state']!='pending':raise ValueError()
    if rows!=cut.admission['guards']['jobs']:raise ValueError()
    _same(cut,before)
    return rows


def finish(cut):
    try:
        binding=verify_binding(cut);phase=binding['phase']
        if binding['finished']:raise ValueError()
        running(cut)
        current=fresh.verify_completed(phase,binding['token'])
        proof=fresh.verify_historical(phase,binding['token'])
        if current['receipt_sha256']!=proof['receipt_sha256']:raise ValueError()
        descriptor={'operation_id':proof['operation_id'],'source_sha':proof['source_sha'],
            'namespace_uid':proof['namespace_uid'],'invocation':cut.invocation,
            'old_oid':proof['old_oid'],'new_oid':proof['new_oid'],
            'receipt_sha256':proof['receipt_sha256'],'catalog_sha256':fresh._digest(proof['catalog'])}
        record=cut._reserve(FINISH,descriptor)
        if fresh.verify_historical(phase,binding['token'])!=proof:raise ValueError()
        cut._complete(FINISH,record,descriptor)
        private=cut._space_private
        private['records'][FINISH]=copy.deepcopy(cut.journal.state['steps'][FINISH])
        running(cut)
        before=phase._read()
        if fresh.verify_historical(phase,binding['token'])!=proof:raise ValueError()
        _same(cut,before)
        binding['finished']=private['finished']=True
        binding['receipt_sha256']=private['receipt_sha256']=proof['receipt_sha256']
        return copy.deepcopy(descriptor)
    except BaseException:cut._uncertain()
