"""One fixed GitHub dispatch for an already reserved encrypted cohort upload.

The caller qualifies the selected source and maintains the actual writer drain.
An accepted dispatch is not an uploaded or restored backup. Unknown outcomes
retain the operation lock and require reconciliation, never automatic replay.
"""
import copy
import json
import re
import subprocess


class DispatchError(RuntimeError):
    pass


def _guard(phase, drain):
    if phase._guard() is False or drain() is False:
        raise DispatchError('authority or drain unavailable')


def _transport(command, payload):
    result = subprocess.run(command, input=payload, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=30, check=True)
    return result.stdout


def _response(raw):
    if raw == b'':
        return None
    if not isinstance(raw, bytes) or len(raw) > 4096:
        raise DispatchError('dispatch response invalid')
    def pairs(values):
        out = {}
        for key, value in values:
            if key in out: raise DispatchError('duplicate response field')
            out[key] = value
        return out
    data = json.loads(raw, object_pairs_hook=pairs)
    if (not isinstance(data,dict) or set(data) != {'workflow_run_id','run_url','html_url'} or
            type(data['workflow_run_id']) is not int or not 0 < data['workflow_run_id'] < 10**20):
        raise DispatchError('dispatch response invalid')
    run = str(data['workflow_run_id'])
    if (data['run_url'] != 'https://api.github.com/repos/Poryadok/VoiceRoot/actions/runs/' + run or
            data['html_url'] != 'https://github.com/Poryadok/VoiceRoot/actions/runs/' + run):
        raise DispatchError('dispatch response invalid')
    return run


def submit(phase, planned, drain, source_authority, transport=None):
    """Submit once; fixture transport is injectable and never supplies authority."""
    p = phase.phases
    transport = _transport if transport is None else transport
    pending = False
    try:
        _guard(phase, drain)
        if source_authority(p.fence.context['commit']) is not True:
            raise DispatchError('source authority refused')
        row, revision = p.fence.ledger.read()
        if (not p.fence.owned(row) or row.get('status') != 'active' or
                not p._owned_invocation(row) or row.get('cohort_upload') != planned or
                p.journal.state['steps'].get('cohort-upload') != planned or
                planned.get('status') != 'dispatch-pending' or
                planned.get('source_sha') != p.fence.context['commit'] or
                planned.get('workflow_sha') != p.fence.context['commit'] or
                'cohort-dispatch' in p.journal.state['steps'] or 'cohort_dispatch' in row):
            raise DispatchError('dispatch reservation differs')
        attempt = {'status':'pending', 'operation_id':planned['operation_id'],
                   'challenge':planned['challenge'], 'invocation_nonce':p.invocation,
                   'source_sha':planned['source_sha']}
        pending = True
        p.journal.state['steps']['cohort-dispatch'] = copy.deepcopy(attempt)
        p.journal.save()
        _guard(phase, drain)
        current, actual = p.fence.ledger.read()
        if current != row or actual != revision:
            raise DispatchError('dispatch ledger changed')
        current['cohort_dispatch'] = copy.deepcopy(attempt)
        p.fence.ledger.write(actual, current)
        _guard(phase, drain)
        current, actual = p.fence.ledger.read()
        if (not p.fence.owned(current) or current.get('status') != 'active' or
                not p._owned_invocation(current) or current.get('cohort_upload') != planned or
                current.get('cohort_dispatch') != attempt):
            raise DispatchError('dispatch intent changed')
        payload = json.dumps({'ref':'master', 'return_run_details':True, 'inputs':{'operation':planned['operation_id'],
                             'challenge':planned['challenge']}}, separators=(',', ':')).encode('ascii')
        command = ['gh','api','--method','POST',
                   'repos/Poryadok/VoiceRoot/actions/workflows/dev-cohort-backup-upload.yml/dispatches',
                   '--input','-']
        run_id = _response(transport(command, payload))
        _guard(phase, drain)
        observed, observed_revision = p.fence.ledger.read()
        actual_upload = observed.get('cohort_upload')
        running = dict(planned, status='upload-running')
        if isinstance(actual_upload, dict):
            running['run_id'] = actual_upload.get('run_id')
            running['run_attempt'] = 1
        if (not p.fence.owned(observed) or observed.get('status') != 'active' or
                not p._owned_invocation(observed) or observed.get('cohort_dispatch') != attempt or
                observed.get('cohort_bundle') != row.get('cohort_bundle') or
                observed.get('cohort_members') != row.get('cohort_members') or
                not isinstance(observed_revision, tuple) or len(observed_revision) != 2 or
                any(not isinstance(v, str) or not v for v in observed_revision) or
                (actual_upload != planned and (actual_upload != running or
                 not isinstance(running.get('run_id'), str) or
                 not re.fullmatch(r'[1-9][0-9]{0,19}', running['run_id']) or
                 type(actual_upload.get('run_attempt')) is not int))):
            raise DispatchError('dispatch authority changed')
        # The runner may immediately claim the reservation and read state.json.
        # Do not modify the journal after dispatch; the durable pending record
        # forbids replay and the runner binds the actual run ID in the ledger.
        # run_id is an informational hint. The subsequent consumer must bind
        # the actual run to the owned ledger and authenticated readback.
        return {'status':'accepted-for-dispatch', 'operation_id':planned['operation_id'],
                'challenge':planned['challenge'], 'run_id_hint':run_id}
    except BaseException:
        if pending:
            try: phase.uncertain('dispatch')
            except BaseException: pass
        raise DispatchError('dispatch refused or uncertain; reconcile if pending') from None
