"""Execute the actual source downloader with passive bytes and a live-API adapter."""
import io,json,os,subprocess,tarfile,tempfile,textwrap,unittest
from pathlib import Path

SOURCE=Path(__file__).resolve().parents[3]
SHA='a'*40

class WorkflowSourceCacheTests(unittest.TestCase):
    def run_case(self,change=None,missing=False):
        with tempfile.TemporaryDirectory(dir='/home/pmd') as folder:
            base=Path(folder);base.chmod(0o750)
            cache=base/'installed/source-archives';cache.mkdir(parents=True,mode=0o750)
            cache.parent.chmod(0o750)
            runner=base/'runner';workspace=runner/'job';workspace.mkdir(parents=True)
            home=base/'home';home.mkdir();tools=base/'tools';tools.mkdir()
            canonical=base/'canonical';canonical.mkdir()
            (canonical/'plain').write_bytes(b'passive source\n')
            (canonical/'execute').write_bytes(b'#!/bin/sh\nexit 0\n');(canonical/'execute').chmod(0o755)
            def git(*args):return subprocess.check_output(['git','-C',str(canonical),*args],text=True).strip()
            git('init','-q');git('add','--all');tree=git('write-tree')
            commit=json.dumps({'sha':SHA,'commit':{'tree':{'sha':tree}}})
            (tools/'curl').write_text('#!/bin/bash\nset -eu\noutput=\"\"\nwhile (($#)); do\n'
                ' case "$1" in --output) output="$2";shift 2;; *) url="$1";shift;; esac\ndone\n'
                'case "$url" in */commits/'+SHA+') printf %s '+repr(commit)+' > "$output";; '
                '*) echo ARCHIVE_NETWORK_FORBIDDEN >&2;exit 81;; esac\n')
            (tools/'curl').chmod(0o755)
            cached=cache/(SHA+'.tar.gz')
            if not missing:
                with tarfile.open(cached,'w:gz') as archive:
                    for name in ('plain','execute'):
                        if change=='missing' and name=='plain':continue
                        content=(canonical/name).read_bytes()
                        if change=='blob' and name=='plain':content+=b'forged'
                        member=tarfile.TarInfo('VoiceRoot-'+SHA+'/'+name)
                        member.mode=0o775 if name=='execute' else 0o664;member.size=len(content)
                        if change=='mode' and name=='execute':member.mode=0o664
                        if change=='link' and name=='plain':member.type=tarfile.SYMTYPE;member.linkname='execute';member.size=0
                        archive.addfile(member,io.BytesIO(content) if member.isfile() else None)
                cached.chmod(0o440)
                if change=='writable':cached.chmod(0o460)
                if change=='symlink':cached.rename(cache/'original');cached.symlink_to(cache/'original')
            workflow=(SOURCE/'.github/workflows/staging-deploy.yml').read_text()
            anchor=workflow.split('run: &download_staging_source |\n',1)[1].split('\n      - name:',1)[0]
            program=textwrap.dedent(anchor).replace('/var/lib/voice-nats-preservation/installed/source-archives',str(cache))
            environment=dict(os.environ,PATH=str(tools)+':'+os.environ['PATH'],HOME=str(home),
                GITHUB_WORKSPACE=str(workspace),RUNNER_WORKSPACE=str(runner),RUNNER_TEMP=str(base),STAGING_SOURCE_SHA=SHA)
            return subprocess.run(['bash','-c',program],env=environment,text=True,capture_output=True,timeout=30)

    def test_actual_workflow_uses_cache_and_reconstructs_full_git_tree(self):
        result=self.run_case();self.assertEqual(result.returncode,0,result.stderr+result.stdout)
        self.assertIn('STAGING_SOURCE_ARCHIVE=PASS',result.stdout)
        self.assertNotIn('ARCHIVE_NETWORK_FORBIDDEN',result.stderr)

    def test_actual_workflow_invalid_present_cache_never_falls_back(self):
        for change in ('blob','mode','missing','link','writable','symlink'):
            with self.subTest(change=change):
                result=self.run_case(change);self.assertNotEqual(result.returncode,0)
                self.assertNotIn('ARCHIVE_NETWORK_FORBIDDEN',result.stderr)

    def test_absent_cache_uses_existing_network_download(self):
        result=self.run_case(missing=True);self.assertNotEqual(result.returncode,0)
        self.assertIn('ARCHIVE_NETWORK_FORBIDDEN',result.stderr)

if __name__=='__main__':unittest.main()
