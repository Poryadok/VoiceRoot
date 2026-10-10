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
            bundle.build(kernel,base/'out',kernel)
            with tarfile.open(base/'out/rollout-bundle.tar') as archive:
                members={row.name:archive.extractfile(row).read() for row in archive.getmembers()}
            manifest=json.loads(members['capture-manifest.json'])
            self.assertEqual(set(manifest),set(members)-{'capture-manifest.json'})
            missing=set()
            for name,raw in members.items():
                if not name.startswith('nats-rollout-preservation/') or not name.endswith('.py'):continue
                for node in ast.walk(ast.parse(raw)):
                    imported=[item.name.split('.')[0] for item in node.names] if isinstance(node,ast.Import) else [node.module.split('.')[0]] if isinstance(node,ast.ImportFrom) and node.module else []
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
            bundle.build(kernel,base/'out',kernel)
            code=base/'code';code.mkdir(mode=0o700)
            with tarfile.open(base/'out/rollout-bundle.tar') as archive:
                for row in archive.getmembers():
                    path=code/row.name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
                    path.write_bytes(archive.extractfile(row).read());path.chmod(0o400)
            captured=root_cli.code_binding(code)
            self.assertEqual(len(captured),len(bundle.KNOWN)+len(bundle.ROLLOUT)+4)
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

@unittest.skipUnless(os.name=='posix' and os.getuid()==0,'isolated Linux ROOT custody fixture')
class WorkflowLauncherTests(unittest.TestCase):
    def members(self,base):
        # CI builds the same qualified member closure. The isolated operator
        # check can supply the exact already-reviewed production archive.
        supplied=os.environ.get('VOICE_TEST_ROLLOUT_BUNDLE')
        if supplied:path=Path(supplied)
        else:
            kernel=base/'kernel';kernel.write_bytes(b'\x7fELFlauncher-parser-fixture')
            bundle.build(kernel,base/'out',kernel);path=base/'out/rollout-bundle.tar'
        with tarfile.open(path) as archive:
            result={row.name:archive.extractfile(row).read() for row in archive.getmembers() if row.isfile()}
        manifest=json.loads(result['capture-manifest.json'])
        self.assertEqual(len(manifest),len(bundle.KNOWN)+len(bundle.ROLLOUT)+4)
        self.assertEqual(set(manifest),set(result)-{'capture-manifest.json'})
        for name,wanted in manifest.items():self.assertEqual(hashlib.sha256(result[name]).hexdigest(),wanted)
        return result

    def launch(self,root):
        workflow=Path(__file__).resolve().parents[3]/'.github/workflows/staging-deploy.yml'
        raw=workflow.read_text(encoding='utf-8')
        start=raw.index('          import hashlib, json, os, pathlib, re, runpy, stat, sys')
        program=ast.parse(textwrap.dedent(raw[start:raw.index('          PY',start)]))
        # Only the external installed-root location is replaced; every actual
        # custody/hash/bound/routing predicate executes unchanged.
        roots=[node for node in ast.walk(program) if isinstance(node,ast.Constant)
               and node.value=='/var/lib/voice-nats-preservation']
        self.assertEqual(len(roots),1);roots[0].value=str(root)
        with patch.dict(os.environ,{'ROLLOUT_ACTION':'--bridge-status','VOICE_NATS_ROLLOUT_OPERATION':'3340764a7d24'},clear=True),\
             patch.object(sys,'argv',[]),patch.object(runpy,'run_path') as run:
            exec(compile(program,str(workflow),'exec'),{})
            run.assert_called_once_with(str(root/'installed/code/nats-rollout-preservation/bridge_client.py'),run_name='__main__')
            self.assertEqual(sys.argv[1:],['status','3340764a7d24'])

    def test_actual_workflow_admits_complete_shipped_bundle_before_bridge_dispatch(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);members=self.members(root);code=root/'installed/code';code.mkdir(parents=True,mode=0o700)
            for name,raw in members.items():
                path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(raw);path.chmod(0o400)
            self.launch(root)

    def test_actual_workflow_keeps_custody_member_and_entrypoint_vetoes(self):
        with tempfile.TemporaryDirectory() as folder:
            base=Path(folder);members=self.members(base)
            for failure in ('hash','digest','absolute','parent','missing-entry','overbound','directory-mode',
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
                    elif failure=='overbound':
                        for index in range(86-len(manifest)):manifest['extra'+str(index)]='a'*64
                    elif failure=='directory-mode':code.chmod(0o720)
                    elif failure=='member-mode':target.chmod(0o420)
                    elif failure=='member-owner':os.chown(target,1000,0)
                    elif failure=='hardlink':os.link(target,root/'another-link')
                    elif failure=='symlink':target.unlink();target.symlink_to(code/'configure-kubectl-ci.sh')
                    elif failure=='oversize':target.chmod(0o600);target.write_bytes(b'x'*((16<<20)+1));target.chmod(0o400)
                    index=code/'capture-manifest.json';index.chmod(0o600);index.write_bytes(json.dumps(manifest).encode());index.chmod(0o400)
                    with self.assertRaises((SystemExit,OSError)):self.launch(root)

if __name__=='__main__':unittest.main()
