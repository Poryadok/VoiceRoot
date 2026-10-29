"""Fenced, one-shot Realtime lifecycle Service configuration transition."""
import copy
import re
import secrets

import realtime_p3_adoption as planner
import writer_cut

STEP = 'backend-config-service-voice-realtime'


class ConfigError(RuntimeError): pass


class ServiceTransition:
    def __init__(self, journal, fence, client, lock, namespace_uid, cut):
        self.journal, self.fence, self.client, self.cut = journal, fence, client, cut
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.intent = copy.deepcopy(journal.intent)
        if (type(cut) is not writer_cut.WriterCut or cut.journal is not journal or cut.fence is not fence
                or cut.client is not client or cut.lock != lock or cut.namespace_uid != namespace_uid
                or self.intent.get('resource_context') != fence.context
                or not isinstance(self.intent.get('backend_config_service'),dict)):
            raise ConfigError('service transition authority invalid')

    def _effects(self, exclude=None):
        effects = {}
        for name, entry in self.journal.state['steps'].items():
            if not name.startswith('backend-config-') or name == exclude: continue
            if (name != STEP or not isinstance(entry,dict) or entry.get('status') not in ('pending','done')
                    or not isinstance(entry.get('nonce'),str) or not re.fullmatch(r'[0-9a-f]{32}',entry['nonce'])
                    or entry.get('details_sha256') != writer_cut.digest(entry.get('details'))): raise ValueError()
            record = {k:entry[k] for k in ('status','nonce','details_sha256')}
            if entry['status'] == 'done': record['receipt_sha256'] = writer_cut.digest(entry['receipt'])
            effects[name] = record
        return effects

    def _guard(self):
        if self.journal.poisoned or self.journal.intent != self.intent or self.journal.state.get('intent') != self.intent:
            raise ValueError()
        self.cut.verify_drained()
        row, _ = self.fence.ledger.read()
        if not self.cut._owned(row) or row.get('backend_config_effects',{}) != self._effects(): raise ValueError()

    def _get(self):
        return self.client.call('get','service','voice-realtime','-o','json')

    def _patch(self, patch):
        return self.cut._patch('Service','voice-realtime',patch)

    def _uncertain(self):
        self.cut.poisoned = True
        try: self.fence.uncertain('backend configuration outcome uncertain')
        except BaseException: pass

    def apply_realtime_service(self):
        pending = False
        try:
            self._guard()
            originals = [item for item in self.cut.admission['controllers']
                         if item['kind'] == 'Deployment' and item['metadata']['name'] == 'voice-realtime']
            if len(originals) != 1 or STEP in self.journal.state['steps']: raise ValueError()
            source = self._get(); expected = self.intent['backend_config_service']
            meta = source['metadata']; expected_meta = expected['metadata']
            if (source.get('apiVersion') != expected.get('apiVersion') or source.get('kind') != expected.get('kind')
                    or source.get('spec') != expected.get('spec') or meta.get('deletionTimestamp')
                    or any(meta.get(k) != expected_meta.get(k) for k in ('name','namespace','uid','resourceVersion','generation'))
                    or ('generation' in meta and (type(meta['generation']) is not int or meta['generation'] < 0))
                    or ('generation' in meta) != ('generation' in expected_meta)): raise ValueError()
            patch, target = planner.plan_service(source, originals[0]['spec']['template']['metadata']['labels'])
            details = {'operation_id':self.fence.context['operation_id'],'invocation_nonce':self.cut.invocation,
                       'source':copy.deepcopy(source),'target_spec':copy.deepcopy(target),'patch':copy.deepcopy(patch)}
            record = {'status':'pending','nonce':secrets.token_hex(16),'details_sha256':writer_cut.digest(details)}
            pending = True
            self.journal.state['steps'][STEP] = record | {'details':details}; self.journal.save()
            self.cut.verify_drained(); row, revision = self.fence.ledger.read()
            if (not self.cut._owned(row) or row.get('backend_config_effects',{}) != self._effects(exclude=STEP)
                    or STEP in row.get('backend_config_effects',{})): raise ValueError()
            row.setdefault('backend_config_effects',{})[STEP] = copy.deepcopy(record); row['step'] = STEP
            self.fence.ledger.write(revision,row); self._guard()
            response = self._patch(patch); actual = self._get()
            for obj in (response,actual):
                m = obj['metadata']
                if (obj.get('apiVersion') != 'v1' or obj.get('kind') != 'Service' or m.get('name') != 'voice-realtime'
                        or m.get('namespace') != 'voice-dev' or m.get('uid') != meta['uid'] or m.get('deletionTimestamp')
                        or not isinstance(m.get('resourceVersion'),str) or not m['resourceVersion']
                        or ('generation' in m) != ('generation' in meta)
                        or m.get('generation') != meta.get('generation')
                        or obj.get('spec') != target): raise ValueError()
            receipt = {'uid':meta['uid'],'spec_sha256':writer_cut.digest(target),
                       'responseResourceVersion':response['metadata']['resourceVersion'],
                       'resourceVersion':actual['metadata']['resourceVersion']}
            if 'generation' in meta: receipt['generation'] = meta['generation']
            self._guard(); row, revision = self.fence.ledger.read()
            if not self.cut._owned(row) or row.get('backend_config_effects',{}).get(STEP) != record: raise ValueError()
            self.journal.state['steps'][STEP].update(status='done',receipt=copy.deepcopy(receipt)); self.journal.save()
            row['backend_config_effects'][STEP] = record | {'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            row['step'] = STEP+'-done'; self.fence.ledger.write(revision,row); self._guard()
            return receipt
        except BaseException:
            if pending: self._uncertain()
            raise ConfigError('service configuration uncertain; retain lock and reconcile' if pending else 'service configuration not admitted') from None
