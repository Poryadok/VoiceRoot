"""Disposable local Docker/keytool fixture; no cluster or release authority."""
import os
import subprocess
import unittest

import space_creator_runtime_profile_trust as trust


@unittest.skipUnless(os.environ.get('CREATOR_DOCKER_INTEGRATION')=='1','explicit disposable Docker fixture only')
class DockerJdkTest(unittest.TestCase):
    def test_real_keytool_readonly_container_and_cacerts_tar(self):
        docker=os.environ['CREATOR_DOCKER_CLI']
        def run(*args):
            return subprocess.run([docker,*args],stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.PIPE,
                                  check=True,timeout=30).stdout
        name='creator-jdk-fixture-'+os.urandom(8).hex()
        cid=run('create','--name',name,'--network','none','--read-only','--cap-drop','ALL',
                '--security-opt','no-new-privileges','--label','voice.test.owner='+name,
                '--entrypoint','/opt/java/openjdk/bin/keytool','eclipse-temurin:25-jre',
                '-list','-v','-keystore','/opt/java/openjdk/lib/security/cacerts','-storepass','changeit',
                '-J-Duser.language=en','-J-Duser.country=US').decode().strip()
        self.assertRegex(cid,r'^[a-f0-9]{64}$')
        def cleanup():
            run('rm','-f','-v',cid)
            self.assertNotIn(cid,run('ps','-a','--no-trunc','--format','{{.ID}}').decode().splitlines())
        self.addCleanup(cleanup)
        roots=trust.parse(run('start','-a',cid)); self.assertGreater(len(roots),0)
