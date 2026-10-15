"""Exact failed request admission preserves history and rejects side effects."""
import contextlib,hashlib,json,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import operator_retirement as r
from controller import Blocked

class Tests(unittest.TestCase):
    def fixture(self,root,stack):
        installed=root/'installed'
        for name in ('journal','responses','sources'):(installed/name).mkdir(parents=True,mode=0o700)
        nonce=r.STORY_PREFENCE_NONCE;sha='b25087c728cdfeb40b9e4f8cb37cb8e441e122fa'
        request={'action':'prepare','nonce':nonce,'source_sha':sha,'run_id':38798032967,'run_attempt':1,
            'report_run_id':38798032967,'report_source_sha':sha,'mode':'full','changed_services':[]}
        row={'schema':'voice-nats-bridge-request-v1','request_sha256':r.canonical(request),'request':request,
            'phase':'STARTED','prepare_stage':{'name':'nats-preflight','status':'STARTED'},'prepare_error':'guard_rejected'}
        raw=json.dumps(row,sort_keys=True,separators=(',',':')).encode()
        response=b'{"error":"exception_Blocked","status":"BLOCKED"}'
        # Actual safe journal/response bytes, not hashes derived from fixture expectations.
        self.assertEqual((hashlib.sha256(raw).hexdigest(),len(raw)),r.STORY_PREFENCE_FILES['journal'])
        self.assertEqual((hashlib.sha256(response).hexdigest(),len(response)),r.STORY_PREFENCE_FILES['responses'])
        files=dict(r.STORY_PREFENCE_FILES)
        report=b'{"request_status":"BLOCKED"}'
        files['reports']=(hashlib.sha256(report).hexdigest(),len(report))
        for folder,name,data in (('journal',nonce+'.json',raw),('responses',nonce+'.json',response),('responses',nonce+'.report.json',report)):
            p=installed/folder/name;p.write_bytes(data);p.chmod(0o600 if folder=='journal' else 0o440)
        source=installed/'sources'/nonce[:12];source.mkdir(mode=0o700)
        (source/'input.txt').write_bytes(b'public source')
        tree={'files':1,'bytes':13,'sha256':r.canonical([{'path':'input.txt','kind':'file','bytes':13,'sha256':hashlib.sha256(b'public source').hexdigest()}])}
        custody=r._retained_prepare_tree(source,tree)['custody_sha256']
        base=root/('rollout-'+r.MEDIA_ABORT_OPERATION);base.mkdir(mode=0o700)
        for name in ('checkpoint.json',r.MEDIA_MONITOR_PREFIX+'-receipt.json'):
            p=base/name;p.write_bytes(b'{}');p.chmod(0o600)
        stack.enter_context(patch.object(r.guard,'ROOT',root))
        stack.enter_context(patch.object(r,'STORY_PREFENCE_FILES',files))
        stack.enter_context(patch.object(r,'STORY_PREFENCE_SOURCE',tree))
        stack.enter_context(patch.object(r,'STORY_PREFENCE_CUSTODY',custody))
        for name in ('STORY_PREFENCE_CHECKPOINT','STORY_PREFENCE_RECEIPT'):
            stack.enter_context(patch.object(r,name,hashlib.sha256(b'{}').hexdigest()))
        verify=stack.enter_context(patch.object(r,'verify_media_abort_terminal',return_value={'verified':True}))
        return installed,installed/'journal'/(nonce+'.json'),source,verify

    def legacy_fixture(self,root,installed,stack):
        from operator_retirement_test import PendingAbandonmentTests
        name,row,response,jraw,rraw,receipt,fence_receipt,fence,state=PendingAbandonmentTests().fixture()
        base=root/('rollout-'+r.FENCE_ABORT_OPERATION);base.mkdir(mode=0o700)
        values={installed/'journal'/name:jraw,installed/'responses'/name:rraw,
            installed/r.PENDING_ABANDONMENT_NAME:json.dumps(receipt,sort_keys=True,separators=(',',':')).encode(),
            base/'checkpoint.json':json.dumps(state).encode(),
            base/'operator-fence-abort-receipt.json':json.dumps(fence_receipt).encode()}
        for path,raw in values.items():path.write_bytes(raw);path.chmod(0o600)
        stack.enter_context(patch.object(r,'verify_fence_abort_terminal',return_value=fence))
        return installed/'journal'/name

    def test_all_journals_preserve_legacy_receipt_and_validate_story_before_exclusion(self):
        for drift in (None,'story','unknown'):
            with self.subTest(drift=drift),tempfile.TemporaryDirectory() as td,contextlib.ExitStack() as stack:
                root=Path(td);installed,path,source,_=self.fixture(root,stack)
                legacy=self.legacy_fixture(root,installed,stack)
                if drift=='story':path.write_bytes(path.read_bytes()+b' ')
                if drift=='unknown':
                    unknown=installed/'journal'/('f'*64+'.json')
                    unknown.write_bytes(json.dumps({'phase':'STARTED','request':{}}).encode());unknown.chmod(0o600)
                before={p:p.read_bytes() for p in root.rglob('*') if p.is_file()}
                if drift is None:
                    self.assertEqual(r.pending_abandonment_names(installed),{legacy.name})
                    for member in sorted((installed/'journal').iterdir()):
                        r.pending_request_accepted(installed,member,{'journals':{}})
                else:
                    with self.assertRaises(Blocked):r.pending_request_accepted(installed,legacy,{'journals':{}})
                self.assertEqual(before,{p:p.read_bytes() for p in before})

    def test_exact_request_is_blocked_abandonment_without_writes(self):
        with tempfile.TemporaryDirectory() as td,contextlib.ExitStack() as stack:
            root=Path(td);installed,path,source,verify=self.fixture(root,stack)
            before={str(p):p.read_bytes() for p in root.rglob('*') if p.is_file()}
            result=r.pending_request_accepted(installed,path,{'journals':{}})
            self.assertEqual((result['disposition'],result['status']),('ABANDONED_PRECHECKPOINT','BLOCKED'))
            verify.assert_called_once()
            self.assertEqual(before,{str(p):p.read_bytes() for p in root.rglob('*') if p.is_file()})

    def test_later_source_sibling_does_not_invalidate_historical_content(self):
        with tempfile.TemporaryDirectory() as td,contextlib.ExitStack() as stack:
            root=Path(td);installed,path,source,_=self.fixture(root,stack)
            before=r.pending_request_accepted(installed,path,{'journals':{}})
            (source.parent/'5e48553f3105').mkdir(mode=0o700)
            after=r.pending_request_accepted(installed,path,{'journals':{}})
            self.assertEqual(before['source']['tree'],after['source']['tree'])
            self.assertNotEqual(before['source']['custody_sha256'],after['source']['custody_sha256'])

    def test_history_source_custody_or_side_effect_drift_is_rejected(self):
        for drift in ('journal','responses','reports','source','custody','link','build','operation','checkpoint','receipt'):
            with self.subTest(drift=drift),tempfile.TemporaryDirectory() as td,contextlib.ExitStack() as stack:
                root=Path(td);installed,path,source,_=self.fixture(root,stack)
                if drift in ('journal','responses','reports'):
                    p=installed/'responses'/(r.STORY_PREFENCE_NONCE+'.report.json') if drift=='reports' else installed/drift/path.name
                    p.chmod(0o600);p.write_bytes(p.read_bytes()+b' ')
                elif drift=='source':(source/'input.txt').write_bytes(b'changed')
                elif drift=='custody':
                    (source/'input.txt').chmod(0o666)
                elif drift=='link':
                    p=source/'input.txt';p.rename(source/'old');p.symlink_to(source/'old')
                elif drift=='build':(installed/'sources'/(r.STORY_PREFENCE_NONCE[:12]+'-build')).mkdir()
                elif drift=='operation':(root/('rollout-'+r.STORY_PREFENCE_NONCE[:12])).mkdir()
                else:
                    name='checkpoint.json' if drift=='checkpoint' else r.MEDIA_MONITOR_PREFIX+'-receipt.json'
                    (root/('rollout-'+r.MEDIA_ABORT_OPERATION)/name).write_bytes(b'{"changed":true}')
                with self.assertRaises(Blocked):r.pending_request_accepted(installed,path,{'journals':{}})

    def test_exact_installed_predecessor_dispatch(self):
        import installer
        with patch.object(installer,'_upgrade_idle_transition') as transition:
            installer.upgrade_idle_story_fixture_v1('candidate')
        self.assertEqual(transition.call_args.args[1],'7f5b901d8429f2d560bb7f71481baabfd90ad2303cfd1b06c8569d94ff87920b')
        self.assertEqual(transition.call_args.args[2],'idle-story-fixture-v1')

    def test_idle_upgrade_rechecks_history_after_swap_and_restores_inbox(self):
        import installer
        from types import SimpleNamespace
        for drift in (None,'reports','source','custody'):
            with self.subTest(drift=drift),tempfile.TemporaryDirectory() as td,contextlib.ExitStack() as stack:
                root=Path(td);installed,path,source,_=self.fixture(root,stack)
                self.legacy_fixture(root,installed,stack)
                for name in ('inbox','processing','recovery'):(installed/name).mkdir(mode=0o700)
                (installed/'inbox').chmod(0o1730)
                for name in ('recovery-key.pem','recovery-cert.pem'):
                    p=installed/'recovery'/name;p.write_bytes(b'fixture');p.chmod(0o600)
                p=installed/'policy.json';p.write_bytes(b'{}');p.chmod(0o600)
                checkpoint=root/('rollout-'+r.MEDIA_ABORT_OPERATION)/'checkpoint.json'
                checkpoint.write_bytes(b'{"operator_retirement":{}}')
                stack.enter_context(patch.object(r,'STORY_PREFENCE_CHECKPOINT',hashlib.sha256(checkpoint.read_bytes()).hexdigest()))
                stack.enter_context(patch.object(r,'idle_state'))
                stack.enter_context(patch.object(r,'verify_terminal',return_value={'journals':{}}))
                stack.enter_context(patch.object(installer.root_cli,'code_binding',return_value={}))
                stack.enter_context(patch.object(installer.root_cli,'rollout_directory_kind',return_value='operation'))
                stack.enter_context(patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()))
                marker={'data':{'phase':'active'}}
                stack.enter_context(patch.object(installer,'Kube',return_value=SimpleNamespace(get=lambda *args:marker)))
                stack.enter_context(patch.object(installer.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)))
                stack.enter_context(patch('story_witness.registry_snapshot',return_value=None))
                def swap(*args):
                    if drift=='source':(source/'input.txt').write_bytes(b'changed')
                    elif drift=='custody':
                        p=source/'input.txt';p.rename(source/'old');p.write_bytes(b'public source');p.chmod(0o644);(source/'old').unlink()
                    elif drift=='reports':
                        p=installed/'responses'/(r.STORY_PREFENCE_NONCE+'.report.json');p.chmod(0o600);p.write_bytes(b'changed')
                upgrade=stack.enter_context(patch.object(installer,'_upgrade_code',side_effect=swap))
                if drift is None:installer.upgrade_idle_story_fixture_v1(root/'candidate')
                else:
                    with self.assertRaises(Blocked):installer.upgrade_idle_story_fixture_v1(root/'candidate')
                upgrade.assert_called_once()
                self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)

if __name__=='__main__':unittest.main()
