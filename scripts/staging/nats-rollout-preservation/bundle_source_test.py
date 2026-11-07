"""Actual archive dependency closure; fake ELF tests packaging, not execution."""
import ast,hashlib,io,json,os,runpy,sys,tarfile,tempfile,textwrap,unittest
from pathlib import Path
from unittest.mock import patch
import bundle

class CompleteBundleTests(unittest.TestCase):
    def test_captured_archive_has_every_local_runtime_import_and_source_profile(self):
        source=Path(__file__).parent
        local={path.stem:path.name for path in source.glob('*.py') if not path.name.endswith('_test.py')}
        with tempfile.TemporaryDirectory() as folder:
            base=Path(folder);kernel=base/'kernel';kernel.write_bytes(b'\x7fELFpackage-parser-fixture')
            bundle.build(kernel,base/'out',kernel,kernel)
            with tarfile.open(base/'out/rollout-bundle.tar') as archive:
                members={row.name:archive.extractfile(row).read() for row in archive.getmembers()}
            self.assertEqual(members['nats-rollout-preservation/story-current4-probe'],kernel.read_bytes())
            manifest=json.loads(members['capture-manifest.json'])
            self.assertEqual(set(manifest),set(members)-{'capture-manifest.json'})
            import staging_prerequisites
            self.assertTrue(staging_prerequisites.REQUIRED_HELPERS <= set(manifest))
            self.assertNotIn('scripts/deploy/staging_p3_auxiliary_image.py', manifest)
            self.assertEqual(hashlib.sha256(json.dumps(sorted(manifest),separators=(',',':')).encode()).hexdigest(),
                             '9e08bb7df3c0dcdb9f4bf6287895eff08a4451062ca41199b79ca131da7465e4')
            tree=ast.parse((source/'realtime_bootstrap_existing.py').read_bytes())
            assignments=[node for node in tree.body if isinstance(node,ast.Assign) and
                         len(node.targets)==1 and isinstance(node.targets[0],ast.Name) and node.targets[0].id=='SOURCE_HELPERS']
            self.assertEqual(len(assignments),1)
            for relative in ast.literal_eval(assignments[0].value).values():
                self.assertIn(relative,manifest)
                self.assertEqual(manifest[relative],hashlib.sha256((source.parent/relative).read_bytes()).hexdigest())
            missing=set()
            for name,raw in members.items():
                if not name.startswith('nats-rollout-preservation/') or not name.endswith('.py'):continue
                for node in ast.walk(ast.parse(raw)):
                    imported=[item.name.split('.')[0] for item in node.names] if isinstance(node,ast.Import) else [node.module.split('.')[0]] if isinstance(node,ast.ImportFrom) and node.module else []
                    if (isinstance(node,ast.Call) and node.args and isinstance(node.args[0],ast.Constant)
                        and isinstance(node.args[0].value,str) and
                        ((isinstance(node.func,ast.Name) and node.func.id=='__import__') or
                         (isinstance(node.func,ast.Attribute) and isinstance(node.func.value,ast.Name)
                          and node.func.value.id=='importlib' and node.func.attr=='import_module'))):
                        imported.append(node.args[0].value.split('.')[0])
                    for module in imported:
                        if module in local and 'nats-rollout-preservation/'+local[module] not in members:
                            missing.add(local[module])
            self.assertEqual(missing,set())
            profile='nats-rollout-preservation/closed-seal-source-policies.json'
            self.assertEqual(members[profile],(source/'closed-seal-source-policies.json').read_bytes())

@unittest.skipUnless(os.name=='posix','ROOT custody readers require isolated Linux')
class RootPackageReaderTests(unittest.TestCase):
    def test_complete_current_map_and_both_checkpoint_readers_keep_their_bounds(self):
        import root_cli,bridge_root
        from controller import Blocked
        self.assertEqual(os.getuid(),0)
        with tempfile.TemporaryDirectory() as folder:
            base=Path(folder);kernel=base/'kernel';kernel.write_bytes(b'\x7fELFpackage-reader-fixture')
            bundle.build(kernel,base/'out',kernel,kernel)
            code=base/'code';code.mkdir(mode=0o700)
            with tarfile.open(base/'out/rollout-bundle.tar') as archive:
                for row in archive.getmembers():
                    path=code/row.name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
                    path.write_bytes(archive.extractfile(row).read());path.chmod(0o400)
            captured=root_cli.code_binding(code)
            self.assertEqual(len(captured),len(bundle.KNOWN)+len(bundle.ROLLOUT)+5)
            self.assertEqual(len(captured),99)
            index=code/'capture-manifest.json';original=index.read_bytes()
            cohort=['nats-rollout-preservation/'+name for name in bundle.ROLLOUT if name.startswith('staging_cohort_preservation')]
            self.assertEqual(len(cohort),5)
            historical={name:value for name,value in captured.items() if name not in cohort}
            index.chmod(0o600);index.write_text(json.dumps(historical));index.chmod(0o400)
            self.assertEqual(root_cli.code_binding(code),historical)
            for count in range(1,5):
                partial=historical|{name:captured[name] for name in cohort[:count]}
                index.chmod(0o600);index.write_text(json.dumps(partial));index.chmod(0o400)
                with self.subTest(count=len(partial)),self.assertRaises(Blocked):root_cli.code_binding(code)
            for failure in ('missing','renamed','extra','digest'):
                changed=dict(captured)
                if failure=='missing':del changed['nats-rollout-preservation/staging_prerequisites.py']
                elif failure=='renamed':changed['nats-rollout-preservation/foreign.py']=changed.pop('nats-rollout-preservation/staging_prerequisites.py')
                elif failure=='extra':changed['extra.py']='a'*64
                else:changed[cohort[0]]='0'*64
                index.chmod(0o600);index.write_text(json.dumps(changed));index.chmod(0o400)
                with self.subTest(failure=failure),self.assertRaises(Blocked):root_cli.code_binding(code)
            index.chmod(0o600);index.write_bytes(original);index.chmod(0o400)
            profile=code/'nats-rollout-preservation/closed-seal-source-policies.json'
            raw=profile.read_bytes();profile.chmod(0o600);profile.write_bytes(raw+b' ');profile.chmod(0o400)
            with self.assertRaises(Blocked):root_cli.code_binding(code)
            profile.chmod(0o600);profile.write_bytes(raw);profile.chmod(0o400)
            manifest=code/'capture-manifest.json';original=manifest.read_bytes()
            manifest.chmod(0o600);manifest.write_text(json.dumps({**captured,'foreign-file':'0'*64}));manifest.chmod(0o400)
            with self.assertRaises(Blocked):root_cli.code_binding(code)
            manifest.chmod(0o600);manifest.write_bytes(original);manifest.chmod(0o400)
            self.assertEqual(root_cli.code_binding(code),captured)
            checkpoint=base/'checkpoint.json';value={'code_capture':captured,'bounded_private_fixture':'x'*(2<<20)}
            checkpoint.write_text(json.dumps(value));checkpoint.chmod(0o600)
            for reader in (root_cli.private_json,bridge_root.private_json):
                self.assertEqual(reader(checkpoint),value)
                other=base/'other.json';other.write_text(json.dumps(value));other.chmod(0o600)
                with self.assertRaises(Blocked):reader(other)


@unittest.skipUnless(os.name=='posix' and os.getuid()==0,'isolated Linux public-code ownership fixture')
class PublicCodeReaderTests(unittest.TestCase):
    def test_uid1000_reads_root_captured_code_and_all_workflow_ingress(self):
        import subprocess
        # Dropping this owned child to the real runner identity requires only
        # SETUID/SETGID in the isolated fixture; no FOWNER or live files.
        # The CI TMPDIR ancestor is private; use the fixture's public /tmp
        # ancestor so this reaches O_NOATIME EPERM rather than traversal EACCES.
        with tempfile.TemporaryDirectory(dir='/tmp') as folder:
            base=Path(folder);base.chmod(0o755)
            code=base/'rollout-3340764a7d24/code'
            code.mkdir(parents=True,mode=0o755);code.parent.chmod(0o755)
            (code/'nats-rollout-preservation').mkdir(mode=0o755)
            payload=code/'payload.txt';payload.write_bytes(b'public captured source')
            helper=code/'configure-kubectl-ci.sh';helper.write_bytes(b'#!/bin/bash\nexit 0\n')
            manifest={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in (payload,helper)}
            for p in (payload,helper):p.chmod(0o444)
            (code/'capture-manifest.json').write_text(json.dumps(manifest))
            (code/'capture-manifest.json').chmod(0o444)
            script=r"""
import errno,json,os,sys
from pathlib import Path
from unittest.mock import patch
sys.path[:0]=sys.argv[2:]
import controller,root_cli,workflow_entry
assert os.getuid()==1000
code=Path(sys.argv[1])
# The private/native reader stays root-only and must still refuse this file.
try:controller.file_sha(code/'payload.txt')
except PermissionError as error:assert error.errno==errno.EPERM
else:raise AssertionError('native_noatime_reader_was_weakened')
expected=json.loads((code/'capture-manifest.json').read_text())
assert root_cli.code_binding(code)==expected
workflow_entry.__file__=str(code/'nats-rollout-preservation/workflow_entry.py')
with patch.dict(os.environ,{'VOICE_NATS_ROLLOUT_OPERATION':'3340764a7d24','GITHUB_WORKSPACE':str(code)},clear=False):
    with patch.object(workflow_entry.subprocess,'run') as shell:
        workflow_entry.main(['--configure'])
        shell.assert_called_once_with(['/bin/bash',str(code/'configure-kubectl-ci.sh')],check=True,timeout=120)
    for action,arguments in (('--preflight',['--preflight']),('--apply',[])):
        with patch.object(workflow_entry.runner,'main') as runner:
            workflow_entry.main([action]);runner.assert_called_once_with(arguments)
print('PUBLIC_CODE_RUNNER=PASS')
"""
            source=Path(__file__).resolve().parent
            result=subprocess.run([sys.executable,'-B','-c',script,str(code),str(source),
                str(source.parent/'nats-known-baseline')],user=1000,group=1000,
                capture_output=True,text=True,timeout=30)
            self.assertEqual(result.returncode,0,result.stderr)
            self.assertEqual(result.stdout.strip(),'PUBLIC_CODE_RUNNER=PASS')


    def test_public_code_reader_keeps_hash_custody_and_identity_vetoes(self):
        import root_cli
        from controller import Blocked
        with tempfile.TemporaryDirectory() as folder:
            base=Path(folder)
            for failure in ('hash','write','hardlink','symlink','oversize','read-race'):
                with self.subTest(failure=failure),tempfile.TemporaryDirectory(dir=base) as slot:
                    code=Path(slot);payload=code/'payload.txt'
                    payload.write_bytes(b'captured public source');payload.chmod(0o444)
                    wanted=hashlib.sha256(payload.read_bytes()).hexdigest()
                    (code/'capture-manifest.json').write_text(json.dumps({'payload.txt':wanted}))
                    if failure=='hash':
                        payload.chmod(0o600);payload.write_bytes(b'changed');payload.chmod(0o444)
                    elif failure=='write':payload.chmod(0o664)
                    elif failure=='hardlink':os.link(payload,code/'second-link')
                    elif failure=='symlink':
                        payload.rename(code/'real');payload.symlink_to('real')
                    elif failure=='oversize':
                        payload.chmod(0o600)
                        with payload.open('r+b') as stream:stream.truncate((16<<20)+1)
                        payload.chmod(0o444)
                    if failure=='read-race':
                        original=os.read;changed=False
                        def mutate(fd,size):
                            nonlocal changed
                            result=original(fd,size)
                            # Ignore the preceding manifest read; mutate exactly
                            # while the held payload descriptor is being hashed.
                            if not changed and os.fstat(fd).st_ino==payload.lstat().st_ino:
                                changed=True;payload.chmod(0o600)
                                payload.write_bytes(b'captured public SOURCE changed length')
                                payload.chmod(0o444)
                            return result
                        with patch.object(root_cli.os,'read',side_effect=mutate):
                            with self.assertRaises(Blocked):root_cli.code_binding(code)
                        self.assertTrue(changed)
                    else:
                        with self.assertRaises(Blocked):root_cli.code_binding(code)

@unittest.skipUnless(os.name=='posix' and os.getuid()==0,'isolated Linux ROOT custody fixture')
class WorkflowLauncherTests(unittest.TestCase):
    def members(self,base):
        # CI builds the same qualified member closure. The isolated operator
        # check can supply the exact already-reviewed production archive.
        supplied=os.environ.get('VOICE_TEST_ROLLOUT_BUNDLE')
        if supplied:path=Path(supplied)
        else:
            kernel=base/'kernel';kernel.write_bytes(b'\x7fELFlauncher-parser-fixture')
            bundle.build(kernel,base/'out',kernel,kernel);path=base/'out/rollout-bundle.tar'
        with tarfile.open(path) as archive:
            result={row.name:archive.extractfile(row).read() for row in archive.getmembers() if row.isfile()}
        manifest=json.loads(result['capture-manifest.json'])
        self.assertEqual(len(manifest),len(bundle.KNOWN)+len(bundle.ROLLOUT)+5)
        self.assertEqual(set(manifest),set(result)-{'capture-manifest.json'})
        for name,wanted in manifest.items():self.assertEqual(hashlib.sha256(result[name]).hexdigest(),wanted)
        return result

    def launch(self,root,route='workflow'):
        workflow=Path(__file__).resolve().parents[3]/'.github/workflows/staging-deploy.yml'
        raw=workflow.read_text(encoding='utf-8')
        start=raw.index('          import hashlib, json, os, pathlib, re, runpy, stat, sys')
        program=ast.parse(textwrap.dedent(raw[start:raw.index('          PY',start)]))
        if route=='render':
            workflow=Path(__file__).resolve().parents[3]/'scripts/staging/render-and-apply.sh'
            raw=workflow.read_text(encoding='utf-8')
            start=raw.index('import hashlib, json, os, pathlib, re, runpy, stat, sys')
            program=ast.parse(raw[start:raw.index('\nPY',start)])
        # Only the external installed-root location is replaced; every actual
        # custody/hash/bound/routing predicate executes unchanged.
        roots=[node for node in ast.walk(program) if isinstance(node,ast.Constant)
               and node.value=='/var/lib/voice-nats-preservation']
        self.assertEqual(len(roots),1);roots[0].value=str(root)
        env={'ROLLOUT_ACTION':'--bridge-status','VOICE_NATS_ROLLOUT_OPERATION':'3340764a7d24',
             'VOICE_NATS_PRESERVATION_RECEIPT':str(root/'rollout-3340764a7d24/apply-authorization.json')}
        with patch.dict(os.environ,env,clear=True),\
             patch.object(sys,'argv',[]),patch.object(runpy,'run_path') as run:
            exec(compile(program,str(workflow),'exec'),{})
            entry=('rollout-3340764a7d24/code/nats-rollout-preservation/runner.py' if route=='render'
                   else 'installed/code/nats-rollout-preservation/bridge_client.py')
            run.assert_called_once_with(str(root/entry),run_name='__main__')
            self.assertEqual(sys.argv[1:],[] if route=='render' else ['status','3340764a7d24'])

    def test_render_ingress_accepts_only_complete_current_capture(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);members=self.members(root)
            for failure in ('none','historical','partial1','partial2','partial3','partial4','missing','renamed','extra','digest'):
                with self.subTest(failure=failure),tempfile.TemporaryDirectory(dir=root) as slot:
                    base=Path(slot);code=base/'rollout-3340764a7d24/code';code.mkdir(parents=True,mode=0o700)
                    for name,raw in members.items():
                        path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
                        path.write_bytes(raw);path.chmod(0o400)
                    manifest=json.loads(members['capture-manifest.json'])
                    helper='nats-rollout-preservation/staging_prerequisites.py'
                    cohort=['nats-rollout-preservation/'+name for name in bundle.ROLLOUT if name.startswith('staging_cohort_preservation')]
                    if failure=='historical' or failure.startswith('partial'):
                        count=int(failure[-1]) if failure.startswith('partial') else 0
                        for name in cohort[count:]:del manifest[name]
                    if failure=='missing':del manifest[helper]
                    elif failure=='renamed':manifest['foreign.py']=manifest.pop(helper)
                    elif failure=='extra':manifest['extra.py']='a'*64
                    elif failure=='digest':manifest[cohort[0]]='0'*64
                    index=code/'capture-manifest.json';index.chmod(0o600);index.write_text(json.dumps(manifest));index.chmod(0o400)
                    if failure in ('none','historical'):self.launch(base,'render')
                    else:
                        with self.assertRaises((SystemExit,OSError)):self.launch(base,'render')

    def test_actual_workflow_admits_complete_shipped_bundle_before_bridge_dispatch(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);members=self.members(root);code=root/'installed/code';code.mkdir(parents=True,mode=0o700)
            for name,raw in members.items():
                path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(raw);path.chmod(0o400)
            self.launch(root)

    def test_p2_exact_capture_is_admitted_by_both_ingress_readers(self):
        import root_cli
        binaries=Path('/fixture/qualified-binaries')
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder)
            bundle.build(binaries/'kernel',root/'out',binaries/'bootstrap-renewer',binaries/'story-current4-probe',
                         story_target8=binaries/'story-restored8-probe')
            with tarfile.open(root/'out/rollout-bundle.tar') as archive:
                members={row.name:archive.extractfile(row).read() for row in archive.getmembers() if row.isfile()}
            for code in (root/'installed/code',root/'rollout-3340764a7d24/code'):
                code.mkdir(parents=True,mode=0o700)
                for name,raw in members.items():
                    path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
                    path.write_bytes(raw);path.chmod(0o400)
                self.assertEqual(len(root_cli.code_binding(code)),100)
            self.launch(root)
            self.launch(root,'render')
            code=root/'installed/code';index=code/'capture-manifest.json'
            manifest=json.loads(index.read_text());manifest['unreviewed.py']=manifest.pop('nats-rollout-preservation/story-restored8-probe')
            index.chmod(0o600);index.write_text(json.dumps(manifest));index.chmod(0o400)
            with self.assertRaises(SystemExit):self.launch(root)
            index=root/'rollout-3340764a7d24/code/capture-manifest.json'
            index.chmod(0o600);index.write_text(json.dumps(manifest));index.chmod(0o400)
            with self.assertRaises(SystemExit):self.launch(root,'render')
            invalid=root/'invalid-probe';invalid.write_bytes(b'\x7fELFwrong')
            with self.assertRaisesRegex(ValueError,'Qualified restored8 Story probe required'):
                bundle.build(binaries/'kernel',root/'invalid',binaries/'bootstrap-renewer',binaries/'story-current4-probe',
                             story_target8=invalid)

    def test_workflow_historical_and_partial_successor_inventories(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);members=self.members(root);code=root/'installed/code';code.mkdir(parents=True,mode=0o700)
            for name,raw in members.items():
                path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(raw);path.chmod(0o400)
            full=json.loads(members['capture-manifest.json'])
            cohort=['nats-rollout-preservation/'+name for name in bundle.ROLLOUT if name.startswith('staging_cohort_preservation')]
            historical={name:value for name,value in full.items() if name not in cohort}
            index=code/'capture-manifest.json'
            for count in range(5):
                manifest=historical|{name:full[name] for name in cohort[:count]}
                index.chmod(0o600);index.write_text(json.dumps(manifest));index.chmod(0o400)
                if count==0:self.launch(root)
                else:
                    with self.subTest(count=len(manifest)),self.assertRaises(SystemExit):self.launch(root)

    def test_workflow_accepts_87_and_89_and_rejects_90_members(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);code=root/'installed/code';code.mkdir(parents=True,mode=0o700)
            manifest={}
            names=['nats-rollout-preservation/bridge_client.py','nats-rollout-preservation/resource_claim.py']+['module%d.py'%n for n in range(92)]
            for name in names:
                path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
                raw=('fixture '+name).encode();path.write_bytes(raw);path.chmod(0o400)
                manifest[name]=hashlib.sha256(raw).hexdigest()
            last=[(name,manifest.pop(name)) for name in ('module85.py','module86.py','module87.py','module88.py','module89.py','module90.py','module91.py')]
            index=code/'capture-manifest.json';index.write_text(json.dumps(manifest));index.chmod(0o400)
            self.launch(root)
            manifest.update(last[:2]);index.chmod(0o600);index.write_text(json.dumps(manifest));index.chmod(0o400)
            self.launch(root)
            manifest.update(last[2:3]);index.chmod(0o600);index.write_text(json.dumps(manifest));index.chmod(0o400)
            with self.assertRaises(SystemExit):self.launch(root)

    def test_actual_workflow_keeps_custody_member_and_entrypoint_vetoes(self):
        with tempfile.TemporaryDirectory() as folder:
            base=Path(folder);members=self.members(base)
            for failure in ('hash','digest','absolute','parent','missing-entry','missing-helper','renamed-helper','overbound','directory-mode',
                            'member-mode','member-owner','hardlink','symlink','oversize'):
                with self.subTest(failure=failure),tempfile.TemporaryDirectory(dir=base) as slot:
                    root=Path(slot);code=root/'installed/code';code.mkdir(parents=True,mode=0o700)
                    for name,raw in members.items():
                        path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(raw);path.chmod(0o400)
                    target=code/'nats-rollout-preservation/bridge_client.py';manifest=json.loads(members['capture-manifest.json'])
                    if failure=='hash':target.chmod(0o600);target.write_bytes(b'changed');target.chmod(0o400)
                    elif failure=='digest':manifest[str(target.relative_to(code))]='invalid'
                    elif failure=='absolute':manifest['/outside']='a'*64
                    elif failure=='parent':manifest['../outside']='a'*64
                    elif failure=='missing-entry':del manifest[str(target.relative_to(code))]
                    elif failure=='missing-helper':del manifest['nats-rollout-preservation/staging_prerequisites.py']
                    elif failure=='renamed-helper':manifest['nats-rollout-preservation/foreign.py']=manifest.pop('nats-rollout-preservation/staging_prerequisites.py')
                    elif failure=='overbound':manifest['extra.py']='a'*64
                    elif failure=='directory-mode':code.chmod(0o720)
                    elif failure=='member-mode':target.chmod(0o420)
                    elif failure=='member-owner':os.chown(target,1000,0)
                    elif failure=='hardlink':os.link(target,root/'another-link')
                    elif failure=='symlink':target.unlink();target.symlink_to(code/'configure-kubectl-ci.sh')
                    elif failure=='oversize':target.chmod(0o600);target.write_bytes(b'x'*((16<<20)+1));target.chmod(0o400)
                    index=code/'capture-manifest.json';index.chmod(0o600);index.write_bytes(json.dumps(manifest).encode());index.chmod(0o400)
                    with self.assertRaises((SystemExit,OSError)):self.launch(root)

if __name__=='__main__':unittest.main()
