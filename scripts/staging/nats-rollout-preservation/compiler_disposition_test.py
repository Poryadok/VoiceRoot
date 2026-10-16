"""Closed precheckpoint compiler rejection; disposable root-owned fixtures only."""
import contextlib
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import deployment_report
import operator_retirement as retirement
from controller import Blocked


class Tests(unittest.TestCase):
    def fixture(self,root,letter='e'):
        installed=root/'installed';installed.mkdir(exist_ok=True)
        for name in ('journal','responses','sources'):(installed/name).mkdir(mode=0o700,exist_ok=True)
        nonce=letter*64;operation=nonce[:12]
        request=dict(action='prepare',nonce=nonce,source_sha='a'*40,report_source_sha='a'*40,
            run_id=123,report_run_id=123,run_attempt=1,mode='images-only',changed_services=['voice','web'])
        row=dict(schema='voice-nats-bridge-request-v1',request=request,request_sha256=retirement.canonical(request),
            phase='STARTED',prepare_stage=dict(name='target-build',status='STARTED'),prepare_error='guard_rejected')
        response=dict(status='BLOCKED',error='rollout_compile_image_inventory_changed')
        report=deployment_report.build(request,None,error=Blocked(response['error']),stage='target-build',binding='b'*64)
        receipt=dict(schema='voice-compiler-inventory-rejection-v1',nonce=nonce,
            disposition='ABANDONED_PRECHECKPOINT',deployment_pass=False,recovery_authorized=False,
            helper_binding='b'*64,files={})
        paths={'journal':installed/'journal'/(nonce+'.json'),'response':installed/'responses'/(nonce+'.json'),
            'report':installed/'responses'/(nonce+'.report.json')}
        for key,value in [('journal',row),('response',response),('report',report)]:
            raw=json.dumps(value,sort_keys=True).encode();paths[key].write_bytes(raw);paths[key].chmod(0o600)
            receipt['files'][key]={'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)}
        old=installed/retirement.PENDING_ABANDONMENT_NAME
        if not old.exists():old.write_bytes(b'{"entries":{}}');old.chmod(0o600)
        receipt['old_receipt_sha256']=hashlib.sha256(old.read_bytes()).hexdigest()
        for key,suffix,name in [('source_tree','','source.txt'),('build_tree','-build','parameters.json')]:
            base=installed/'sources'/(operation+suffix);base.mkdir(mode=0o700)
            raw=b'{}';(base/name).write_bytes(raw);(base/name).chmod(0o600)
            receipt[key]={'files':1,'bytes':len(raw),'sha256':retirement.canonical([
                dict(path=name,kind='file',bytes=len(raw),sha256=hashlib.sha256(raw).hexdigest())])}
        disposition=installed/('compiler-inventory-rejection-'+nonce+'.json')
        disposition.write_text(json.dumps(receipt));disposition.chmod(0o600)
        return installed,paths,receipt,disposition

    def test_separate_receipts_preserve_old_inventory_and_all_bytes(self):
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);installed,paths,_,_=self.fixture(root)
            self.fixture(root,'f')
            unknown=installed/'journal'/('c'*64+'.json');unknown.write_text('{"phase":"STARTED"}');unknown.chmod(0o600)
            before={str(p):p.read_bytes() for p in root.rglob('*') if p.is_file()}
            with patch.object(retirement.guard,'ROOT',root):
                result=retirement.pending_request_accepted(installed,paths['journal'],{'journals':{}})
                self.assertEqual(result['disposition'],'ABANDONED_COMPILER_INVENTORY_REJECTION')
                self.assertEqual(retirement.pending_abandonment_names(installed),{unknown.name})
            self.assertEqual(before,{str(p):p.read_bytes() for p in root.rglob('*') if p.is_file()})

    def test_missing_receipt_does_not_exclude_pending_request(self):
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);installed,paths,_,receipt=self.fixture(root);receipt.unlink()
            with patch.object(retirement.guard,'ROOT',root):
                self.assertEqual(retirement.pending_abandonment_names(installed),{paths['journal'].name})

    def test_drift_and_existing_operation_fail_closed(self):
        for drift in ('journal','response','report','old','source','build','operation','receipt_mode','receipt_symlink'):
            with self.subTest(drift=drift),tempfile.TemporaryDirectory() as td:
                root=Path(td);installed,paths,_,receipt=self.fixture(root)
                if drift in paths:paths[drift].write_bytes(paths[drift].read_bytes()+b' ')
                elif drift=='old':(installed/retirement.PENDING_ABANDONMENT_NAME).write_bytes(b'{}')
                elif drift=='source':(installed/'sources'/('e'*12)/'source.txt').write_bytes(b'changed')
                elif drift=='build':(installed/'sources'/('e'*12+'-build')/'target.json').write_bytes(b'{}')
                elif drift=='operation':(root/('rollout-'+'e'*12)).mkdir()
                elif drift=='receipt_mode':receipt.chmod(0o666)
                else:
                    moved=receipt.with_suffix('.saved');receipt.rename(moved);receipt.symlink_to(moved)
                with patch.object(retirement.guard,'ROOT',root),self.assertRaises((Blocked,OSError)):
                    retirement.pending_request_accepted(installed,paths['journal'],{'journals':{}})

    def test_self_consistent_wrong_failure_or_identity_is_rejected(self):
        for field,value in [('error_code','unknown'),('operation','e'*12),('stage','source-capture'),
                            ('helper_binding','c'*64),('request_status','COMPLETE')]:
            with self.subTest(field=field),tempfile.TemporaryDirectory() as td:
                root=Path(td);installed,paths,receipt,destination=self.fixture(root)
                report=json.loads(paths['report'].read_bytes());report[field]=value
                raw=json.dumps(report).encode();paths['report'].write_bytes(raw)
                receipt['files']['report']={'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)}
                destination.write_text(json.dumps(receipt))
                with patch.object(retirement.guard,'ROOT',root),self.assertRaises(Blocked):
                    retirement.pending_request_accepted(installed,paths['journal'],{'journals':{}})

    def test_root_attestation_is_create_only_and_preserves_original_evidence(self):
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);installed,_,receipt,destination=self.fixture(root)
            for name in ('inbox','processing'):(installed/name).mkdir()
            reviewed=root/'reviewed.json';reviewed.write_bytes(destination.read_bytes());reviewed.chmod(0o600)
            destination.unlink();sha=hashlib.sha256(reviewed.read_bytes()).hexdigest()
            before={str(p):p.read_bytes() for p in root.rglob('*') if p.is_file()}
            with patch.object(retirement.guard,'ROOT',root),patch.object(retirement,'operation_lock',return_value=contextlib.nullcontext()):
                result=retirement.attest_compiler_inventory_rejection(reviewed,sha)
                self.assertEqual(result['disposition'],'ABANDONED_COMPILER_INVENTORY_REJECTION')
                with self.assertRaises(FileExistsError):retirement.attest_compiler_inventory_rejection(reviewed,sha)
            self.assertEqual(destination.read_bytes(),reviewed.read_bytes())
            self.assertEqual(before,{str(p):p.read_bytes() for p in root.rglob('*') if p.is_file() and p!=destination})

    def test_actual_idle_upgrade_rechecks_disposition_and_tree_after_swap(self):
        import installer
        from types import SimpleNamespace
        from operator_retirement_test import RetirementActionTests,FakeKube
        helper=RetirementActionTests()
        for drift in (None,'source','receipt'):
            with self.subTest(drift=drift),tempfile.TemporaryDirectory() as td:
                fixture=helper.fixture(td);kube=FakeKube(fixture[4]);helper.action(fixture,kube)
                root,_,_,_,_,binding,_,_=fixture
                installed,_,_,destination=self.fixture(root)
                (installed/'recovery').mkdir()
                for name in ('recovery-key.pem','recovery-cert.pem'):
                    path=installed/'recovery'/name;path.write_bytes(b'fixture');path.chmod(0o600)
                (installed/'policy.json').write_bytes(b'{}');(installed/'policy.json').chmod(0o600)
                def swap(*args):
                    if drift=='source':(installed/'sources'/('e'*12)/'source.txt').write_bytes(b'changed')
                    if drift=='receipt':destination.write_bytes(destination.read_bytes()+b' ')
                with patch.object(retirement.guard,'ROOT',root),\
                     patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(fixture[3]).hexdigest()),\
                     patch.object(retirement.root_cli,'code_binding',return_value=binding),\
                     patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()),\
                     patch.object(installer,'Kube',return_value=kube),\
                     patch.object(installer.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)),\
                     patch('story_witness.registry_snapshot',return_value={'fixed':'witness'}),\
                     patch.object(installer,'_upgrade_code',side_effect=swap) as upgrade:
                    if drift is None:installer.upgrade_idle_search_bootstrap_v1(root/'candidate')
                    else:
                        with self.assertRaises(Blocked):installer.upgrade_idle_search_bootstrap_v1(root/'candidate')
                    upgrade.assert_called_once()
                    self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)


if __name__=='__main__':unittest.main()
