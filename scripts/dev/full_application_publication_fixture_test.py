"""Integration against an explicitly supplied immutable clean test checkout."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


@unittest.skipUnless(os.environ.get('VOICE_PUBLICATION_SOURCE') and os.environ.get('VOICE_PUBLICATION_SHA'),
                     'supply an immutable test-only Git checkout')
class PublicationTest(unittest.TestCase):
    def test_real_canonical_capture_and_native_reader(self):
        source=Path(os.environ['VOICE_PUBLICATION_SOURCE']).resolve(); sha=os.environ['VOICE_PUBLICATION_SHA']
        helper=Path(__file__).with_name('full_application_publication_fixture.py').resolve()
        native=os.environ.get('VOICE_PUBLICATION_NATIVE','')
        code='''
import importlib.util,os,sys,tempfile
from pathlib import Path
source=Path(sys.argv[1]); sha=sys.argv[2]; helper=sys.argv[3]; native=sys.argv[4]
sys.path.insert(0,str(source/'scripts/dev'))
sys.path.insert(0,str(source/'scripts/staging/nats-rollout-preservation'))
spec=importlib.util.spec_from_file_location('publication_fixture_external',helper)
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
import canonical_application_package as package
import space_native_tool_identity as tool
with module.PublicationFixture(source,sha,native or None) as external:
    with tempfile.TemporaryDirectory() as tmp:
        receipt=package.capture(external.token,sha,external.run_id,external.attempt,Path(tmp)/'package.json')
        assert receipt['source_sha']==sha and receipt['verifier_sha']==sha
        assert receipt['request']==external.request and receipt['images']==external.images
        assert receipt['runtime_qualified'] is False and receipt['deployment_performed'] is False
        if native:
            diagnostics=[]
            def trace(frame,event,arg):
                if event=='exception' and frame.f_code.co_filename.endswith(('space_native_tool_identity.py','full_application_publication_fixture.py','source_authority.py')):
                    diagnostics.append((frame.f_code.co_name,frame.f_lineno,arg[0].__name__,str(arg[1])))
                return trace
            sys.settrace(trace)
            try: proof=tool.authenticated_publication(external.token,external.actor,receipt,source)
            except Exception as error: raise AssertionError('native fixture failed: '+repr(diagnostics[-10:])) from error
            finally: sys.settrace(None)
            assert proof['publication']['source_sha']==sha
            assert proof['publication']['runtime_qualified'] is False
        assert any('/git/trees/' in request for request in external.requests)
        # Producer is the external working revision; source/CI verifiers remain
        # imported from the immutable candidate checkout. Kubernetes is a local
        # synthetic boundary and no phase or verifier is replaced.
        sys.path.insert(0,str(Path(helper).parent))
        import copy,json
        from unittest.mock import patch
        import frontend_runtime_executor as executor
        import media_backup_phase_test as base_fixture
        import media_operation_journal as journal
        import update_images
        import full_cohort_package_phase as phase
        base=base_fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        try:
            base.setUp()
            mapped={'voice-'+name+'/'+container:external.images[name] for name,container in update_images.SERVICES.items()}
            base.context['commit']=sha; base.context['images']=copy.deepcopy(mapped)
            base.fence.context=copy.deepcopy(base.context); base.ledger.row.update(base.context)
            base.card['candidate'].update(commit=sha,images=copy.deepcopy(mapped))
            base.lock['data']['resource_context']=json.dumps(base.context,sort_keys=True,separators=(',',':'))
            root=Path(tmp)/'operations'; root.mkdir(mode=0o700); operation=root/base.context['operation_id']; operation.mkdir(mode=0o700)
            intent={'resource_context':copy.deepcopy(base.context),'namespace_uid':'ns','full_cohort_package':{
                'run_id':external.run_id,'run_attempt':external.attempt,
                'nodes':[{'name':'fixture-node','uid':'node-uid','hostname':'fixture-node','architecture':'amd64'}]}}
            with journal.Journal(operation/'phase-journal',intent) as j:
                client=executor.KubectlTransport(j)
                def kube(*args,**kwargs):
                    if args[1]=='namespace':return {'metadata':{'name':'voice-dev','uid':'ns','labels':base_fixture.update_images.LABELS}}
                    if args[1]=='node':return {'metadata':{'name':'fixture-node','uid':'node-uid','labels':{'kubernetes.io/os':'linux',
                        'kubernetes.io/arch':'amd64','kubernetes.io/hostname':'fixture-node'}},'spec':{},
                        'status':{'nodeInfo':{'operatingSystem':'linux','architecture':'amd64'},'conditions':[{'type':'Ready','status':'True'}]}}
                    return copy.deepcopy(base.lock)
                client.call=kube; base.ledger.client=client
                with patch.object(phase,'ROOT',root):
                    instance=phase.PackagePhase(j,base.fence,client,base.lock,'ns')
                    qualified=instance.run(external.token,external.actor)
                    assert qualified['images']==mapped and len(qualified['oci_provenance'])==23
                    assert all(row['requested_kind']=='manifest' and row['source_tag_kind']=='index' for row in qualified['oci_provenance'].values())
                    assert instance.verify_current(external.token)==qualified
        finally:
            if not base.doCleanups(): raise AssertionError('fixture cleanup failed')
'''
        result=subprocess.run([sys.executable,'-B','-c',code,str(source),sha,str(helper),native],
                              cwd=str(source),capture_output=True,text=True,timeout=600)
        self.assertEqual(result.returncode,0,result.stderr)


if __name__=='__main__': unittest.main()
