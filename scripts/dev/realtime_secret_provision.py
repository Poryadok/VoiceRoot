"""Connect a confirmed private Secret creation to the real DEV DB provisioner.

The caller creates a separate private provision journal only after its exact
credential receipt and engine snapshot have been authenticated. This does not
perform a migration or establish application readiness.
"""
import hashlib
import hmac
import copy
import realtime_db_provision as provision
import realtime_secret_phase as secret_phase


class RecoveryRequired(RuntimeError):
    pass


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
