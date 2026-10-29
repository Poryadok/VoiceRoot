import copy
import hashlib
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import realtime_config_transition_inputs as inputs


class Ledger:
    def __init__(self): self.row = {}; self.revision = '1'
    def read(self): return copy.deepcopy(self.row), self.revision


class Session:
    def _guard(self): pass
    def _check_roots(self): pass


class Migration:
    def _guard(self): pass


class InputsTest(unittest.TestCase):
    def setUp(self):
        self.context = {'operation_id':'media-c1-'+'a'*32,'commit':'b'*40}
        self.ledger = Ledger(); self.fence = SimpleNamespace(context=self.context,ledger=self.ledger)
        self.cut = SimpleNamespace(fence=self.fence,namespace_uid='ns',lock={'uid':'lock'},_guard=lambda:None)
        self.session = Session(); self.session.fence=self.fence; self.session.namespace_uid='ns'; self.session.lock=self.cut.lock
        self.session.completed=True; self.session.effects={}; self.actual={}
        for kind,name in inputs.packet.MEMBERS:
            entry={'status':'done','kind':kind,'name':name,'operation_id':self.context['operation_id'],
                   'keys':sorted(inputs.packet.MEMBERS[kind,name]),'uid':name+'-uid','resourceVersion':'1'}
            self.session.effects[kind.lower()+'-'+name] = entry
            self.actual[kind,name]={'apiVersion':'v1','kind':kind,'metadata':{'name':name,'namespace':'voice-dev',
                'uid':entry['uid'],'resourceVersion':'1'}}
        self.session.roots={(name,key):{'uid':'uid-'+name,'resourceVersion':'1','certificate':b'cert'}
                            for name,key in inputs.creation.PUBLIC}
        public_sources={name:{'uid':'uid-'+name,'resourceVersion':'1','der_sha256':hashlib.sha256(b'cert').hexdigest()}
                        for name,key in inputs.creation.PUBLIC}
        receipt={'operation_id':self.context['operation_id'],'candidate_source_sha':self.context['commit'],
                 'namespace_uid':'ns','scope':'p3-create-only','objects':copy.deepcopy(self.session.effects),
                 'public_sources':public_sources}
        self.session.handoff={'status':'done','receipt':receipt}
        self.session.journal=SimpleNamespace(state={'steps':{'realtime-p3-handoff':copy.deepcopy(self.session.handoff)}})
        self.ledger.row['realtime_p3_handoff']=copy.deepcopy(self.session.handoff)
        self.migration=Migration(); self.migration.fence=self.fence; self.migration.namespace='ns'
        self.migration.lock=self.cut.lock; self.migration.root=Path('/source'); self.migration.invocation_status='done'
        self.proof={'job':{'state':'done'},'record':{'terminal_sha256':'a'*64}}
        self.patches=[patch.object(inputs.creation,'Session',Session),patch.object(inputs.realtime_migration_job,'Migration',Migration),
                      patch.object(inputs.transport,'get',side_effect=lambda k,n:copy.deepcopy(self.actual[k,n])),
                      patch.object(inputs.crypto,'der',side_effect=lambda value:value),
                      patch.object(inputs.realtime_migration_job,'verify_writer_successor',side_effect=lambda *a:copy.deepcopy(self.proof))]
        for item in self.patches: item.start(); self.addCleanup(item.stop)

    def test_exact_completed_phases(self):
        result=inputs.observe(self.cut,self.session,self.migration,'/source')
        self.assertEqual(len(result['objects']),7); self.assertEqual(result['scope'],'p3-create-and-migration-only')

    def test_incomplete_foreign_or_drifting_inputs_hold(self):
        for fault in ('session','migration','object','ledger','context'):
            with self.subTest(fault=fault):
                session=copy.copy(self.session); migration=copy.copy(self.migration)
                if fault == 'session': session.completed=False
                if fault == 'migration': migration.invocation_status='pending'
                if fault == 'object': self.actual['Secret','voice-realtime-p3-tls']['metadata']['resourceVersion']='2'
                if fault == 'ledger': self.ledger.row['realtime_p3_handoff']={'status':'foreign'}
                if fault == 'context': migration.fence=SimpleNamespace(context={'operation_id':'foreign'},ledger=self.ledger)
                with self.assertRaises(inputs.InputError): inputs.observe(self.cut,session,migration,'/source')
                self.actual['Secret','voice-realtime-p3-tls']['metadata']['resourceVersion']='1'
                self.ledger.row['realtime_p3_handoff']=copy.deepcopy(self.session.handoff)

    def test_object_drift_during_migration_verification_holds(self):
        def mutate(*args):
            self.actual['Secret','voice-realtime-p3-tls']['metadata']['resourceVersion']='2'
            return copy.deepcopy(self.proof)
        with patch.object(inputs.realtime_migration_job,'verify_writer_successor',side_effect=mutate):
            with self.assertRaises(inputs.InputError): inputs.observe(self.cut,self.session,self.migration,'/source')

    def test_root_snapshot_drift_during_migration_verification_holds(self):
        def mutate(*args):
            self.session.roots[inputs.creation.PUBLIC[0]]['resourceVersion']='2'
            return copy.deepcopy(self.proof)
        with patch.object(inputs.realtime_migration_job,'verify_writer_successor',side_effect=mutate):
            with self.assertRaises(inputs.InputError): inputs.observe(self.cut,self.session,self.migration,'/source')


if __name__ == '__main__': unittest.main()
