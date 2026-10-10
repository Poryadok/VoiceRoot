"""Actual archive dependency closure; fake ELF tests packaging, not execution."""
import ast,io,json,os,tarfile,tempfile,unittest
from pathlib import Path
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

if __name__=='__main__':unittest.main()
