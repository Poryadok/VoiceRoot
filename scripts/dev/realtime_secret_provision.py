"""Connect a confirmed private Secret creation to the real DEV DB provisioner.

The caller creates a separate private provision journal only after its exact
credential receipt and engine snapshot have been authenticated. This does not
perform a migration or establish application readiness.
"""
import hashlib
import hmac
import copy
import re
import realtime_db_provision as provision
import realtime_secret_phase as secret_phase
import frontend_runtime_executor
import media_operation_journal
import resource_fencing


class RecoveryRequired(RuntimeError):
    pass


def run_owned(secret, journal, client, fence, lock, namespace_uid, password):
    """Provision with a separately journal-bound worker retained for the cut."""
    attempted = False
    try:
        if (type(secret) is not secret_phase.SecretPhase or type(journal) is not media_operation_journal.Journal or
                type(client) is not frontend_runtime_executor.KubectlTransport or
                type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
                client.journal is not journal or fence.ledger.client is not client or
                fence.context != secret.fence.context or lock != secret.lock or namespace_uid != secret.namespace_uid or
                not isinstance(password,bytes)):
            raise RecoveryRequired('provision worker identity differs')
        intent=copy.deepcopy(journal.intent)
        expected_record=None
        def credential_guard():
            nonlocal expected_record
            secret._guard()
            left, left_revision = secret.fence.ledger.read()
            right, right_revision = fence.ledger.read()
            receipt = intent.get('realtime_credential', {})
            digest = hashlib.sha256(password).hexdigest()
            local = secret.journal.state['steps'].get('realtime-secret')
            record = left.get('realtime_secret', {})
            if (journal.intent != intent or journal.state.get('intent') != intent or
                    intent.get('resource_context') != fence.context or intent.get('namespace_uid') != namespace_uid or
                    left != right or left_revision != right_revision or not fence.owned(left) or
                    left.get('status') != 'active' or set(receipt) != {'name','uid','resourceVersion','password_sha256'} or
                    not hmac.compare_digest(receipt['password_sha256'], digest) or
                    set(record) != {'status','operation_id','nonce','name','receipt'} or
                    record.get('operation_id') != fence.context['operation_id'] or record.get('name') != receipt['name'] or
                    not isinstance(record.get('nonce'),str) or not re.fullmatch(r'[0-9a-f]{32}',record['nonce']) or
                    local != record | {'password_sha256':digest} or record.get('status') != 'done' or
                    record.get('receipt') != {k:receipt[k] for k in ('name','uid','resourceVersion')} or
                    (expected_record is not None and record != expected_record)):
                raise RecoveryRequired('credential or ledger authority changed')
            if expected_record is None: expected_record=copy.deepcopy(record)
        credential_guard()
        pg=provision.KubePostgres(credential_guard)
        worker=provision.Provision(journal,fence,client,lock,namespace_uid,pg,password)
        worker._guard(); worker._secret(); credential_guard()
        attempted = True
        receipt = worker.run()
        credential_guard()
        checked = provision.verify_completed(worker)
        if checked['database_oid'] != receipt['database_oid'] or checked['role_oid'] != receipt['role_oid']:
            raise RecoveryRequired('provision completion changed')
        credential_guard()
        return worker, receipt
    except BaseException:
        if attempted:
            try: fence.uncertain('Realtime provision bridge outcome uncertain')
            except BaseException: pass
        raise RecoveryRequired('Realtime provision not complete; retain lock and reconcile') from None


def run(secret, journal, password):
    try:
        if type(secret) is not secret_phase.SecretPhase or not isinstance(password,bytes):
            raise RecoveryRequired('fixed Secret phase required')
        secret._guard()
        local = secret.journal.state['steps']['realtime-secret']
        ledger, revision = secret.fence.ledger.read()
        receipt = journal.intent['realtime_credential']
        public = {k:receipt[k] for k in ('name','uid','resourceVersion')}
        digest = hashlib.sha256(password).hexdigest()
        if (set(receipt)!={'name','uid','resourceVersion','password_sha256'} or
                not hmac.compare_digest(receipt['password_sha256'],digest) or
                local.get('password_sha256')!=digest or
                local != ledger.get('realtime_secret',{}) | {'password_sha256':digest} or
                local.get('status')!='done' or local.get('receipt')!=public or
                not secret.fence.owned(ledger) or ledger.get('status')!='active' or
                not secret_phase.valid_revision(revision) or
                journal.intent.get('resource_context')!=secret.fence.context or
                journal.intent.get('namespace_uid')!=secret.namespace_uid):
            raise RecoveryRequired('credential receipt not owned')
        expected_record = copy.deepcopy(ledger['realtime_secret'])
        def credential_guard():
            secret._guard()
            current, current_revision = secret.fence.ledger.read()
            if (not secret.fence.owned(current) or current.get('status')!='active' or
                    current.get('realtime_secret')!=expected_record or
                    secret.journal.state['steps'].get('realtime-secret') != expected_record | {'password_sha256':digest} or
                    not secret_phase.valid_revision(current_revision)):
                raise RecoveryRequired('credential authority changed')
        credential_guard()
        pg = provision.KubePostgres(credential_guard)
        if pg.snapshot()!=journal.intent.get('postgres'):
            raise RecoveryRequired('engine identity changed')
        worker = provision.Provision(journal,secret.fence,secret.client,secret.lock,secret.namespace_uid,pg,password)
        worker._secret()
        credential_guard()
        return worker.run()
    except BaseException:
        raise RecoveryRequired('Realtime provision not complete; retain lock and reconcile') from None
