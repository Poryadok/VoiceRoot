"""Journal one guarded pre-cut User/Notification metadata observation."""
import copy
import re
import secrets

import backend_config_transition_probe as probe
import frontend_runtime_executor
import full_cohort_schema_metadata as metadata
import media_operation_guard
import writer_cut

STEP = 'backend-config-observation'


class ObservationError(RuntimeError): pass


class Observation:
    def __init__(self, journal, fence, client, lock, namespace_uid):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.intent = copy.deepcopy(journal.intent)
        plan = self.intent.get('backend_config_observation')
        if (self.intent.get('resource_context') != fence.context or self.intent.get('namespace_uid') != namespace_uid
                or type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal
                or getattr(fence.ledger,'client',None) is not client or not isinstance(plan,dict)
                or set(plan) != {'source_sha','targets'} or plan['source_sha'] != fence.context.get('commit')
                or not isinstance(plan['targets'],list) or len(plan['targets']) != 2):
            raise ObservationError('observation identity invalid')
        for item, workload in zip(plan['targets'],('notification','user')):
            if (not isinstance(item,dict) or set(item) != {'workload','pod_name','pod_uid','deployment_uid','container_id','restart_count'}
                    or item['workload'] != workload or not isinstance(item['pod_name'],str)
                    or not re.fullmatch('voice-'+workload+r'-[a-z0-9-]+',item['pod_name'])
                    or any(not isinstance(item[k],str) or not item[k] for k in ('pod_uid','deployment_uid','container_id'))
                    or type(item['restart_count']) is not int or item['restart_count'] < 0):
                raise ObservationError('observation target invalid')
        self.plan = copy.deepcopy(plan); self.poisoned = False; self.expected_step = None

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent
                or self.journal.state.get('intent') != self.intent
                or (self.expected_step is not None and self.journal.state['steps'].get(STEP) != self.expected_step)):
            raise ObservationError('observation identity changed')
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        if self.expected_step is not None and self.journal.state['steps'].get(STEP) != self.expected_step:
            raise ObservationError('observation identity changed')

    def run(self):
        pending = False
        try:
            self._guard(); row, revision = self.fence.ledger.read()
            if STEP in self.journal.state['steps'] or 'backend_config_observation' in row: raise ValueError()
            nonce = secrets.token_hex(16)
            intent = {'status':'pending','nonce':nonce,'operation_id':self.fence.context['operation_id'],
                      'source_sha':self.plan['source_sha'],'targets_sha256':writer_cut.digest(self.plan['targets'])}
            pending = True; self.expected_step = copy.deepcopy(intent)
            self.journal.state['steps'][STEP] = copy.deepcopy(intent); self.journal.save()
            self._guard(); current, rev = self.fence.ledger.read()
            if current != row or rev != revision: raise ValueError()
            current['backend_config_observation'] = copy.deepcopy(intent); current['step'] = STEP+'-pending'
            self.fence.ledger.write(rev,current); self._guard()
            current, _ = self.fence.ledger.read()
            if current.get('backend_config_observation') != intent: raise ValueError()
            observations = {}
            for target in self.plan['targets']:
                result = probe.inspect(target['workload'],target['pod_name'],metadata.linux_private_bytes,self._guard)
                identity = result['identity']
                if (result.get('status') != 'observed' or identity['namespace_uid'] != self.namespace_uid
                        or any(identity[k] != target[k] for k in ('pod_name','pod_uid','deployment_uid','container_id','restart_count'))): raise ValueError()
                observations[target['workload']] = result
            self._guard(); current, rev = self.fence.ledger.read()
            if current.get('backend_config_observation') != intent: raise ValueError()
            receipt = {'operation_id':intent['operation_id'],'source_sha':intent['source_sha'],
                       'nonce':nonce,'observations':copy.deepcopy(observations)}
            done = intent | {'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            self.expected_step = copy.deepcopy(done) | {'receipt':copy.deepcopy(receipt)}
            self.journal.state['steps'][STEP] = copy.deepcopy(self.expected_step); self.journal.save()
            current['backend_config_observation'] = copy.deepcopy(done); current['step'] = STEP+'-done'
            self.fence.ledger.write(rev,current); self._guard()
            confirmed, _ = self.fence.ledger.read()
            if confirmed.get('backend_config_observation') != done: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('backend configuration observation uncertain')
                except BaseException: pass
            raise ObservationError('observation uncertain; retain lock and reconcile' if pending else 'observation not admitted') from None
