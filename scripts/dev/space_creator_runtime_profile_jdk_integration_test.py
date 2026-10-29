"""Disposable local Docker/keytool fixture; no cluster or release authority."""
import os
import io
import hashlib
import json
import subprocess
import tarfile
import unittest

import space_creator_runtime_profile_trust as trust
import space_creator_runtime_profile_target_trust as target_trust


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

    def test_keytool_accepts_private_store_via_stdin(self):
        docker=os.environ['CREATOR_DOCKER_CLI']
        def run(*args,input=None):
            return subprocess.run([docker,*args],input=input,stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=True,timeout=30).stdout
        name='creator-stdin-fixture-'+os.urandom(8).hex()
        original=run('create','--name',name,'--network','none','--entrypoint','/bin/true','eclipse-temurin:25-jre').decode().strip()
        self.assertRegex(original,r'^[a-f0-9]{64}$'); self.addCleanup(lambda:run('rm','-f','-v',original))
        raw=run('cp','-L',original+':/opt/java/openjdk/lib/security/cacerts','-')
        with tarfile.open(fileobj=io.BytesIO(raw),mode='r:') as archive:
            entries=archive.getmembers(); self.assertEqual(len(entries),1); self.assertTrue(entries[0].isfile())
            data=archive.extractfile(entries[0]).read(1048577); self.assertLessEqual(len(data),1048576)
        script='umask 077; cat > /scratch/store; exec /opt/java/openjdk/bin/keytool -list -v -keystore /scratch/store -storepass changeit -J-Duser.language=en -J-Duser.country=US'
        target=run('create','-i','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges',
                   '--tmpfs','/scratch:rw,noexec,nosuid,nodev,size=2m,mode=0700,uid=65532,gid=65532','--user','65532:65532',
                   '--entrypoint','/bin/sh','eclipse-temurin:25-jre','-ec',script).decode().strip()
        self.assertRegex(target,r'^[a-f0-9]{64}$'); self.addCleanup(lambda:run('rm','-f','-v',target))
        info=json.loads(run('inspect',target))[0]
        self.assertTrue(info['Config']['OpenStdin'])
        self.assertEqual(info['HostConfig']['Tmpfs'],{'/scratch':'rw,noexec,nosuid,nodev,size=2m,mode=0700,uid=65532,gid=65532'})
        self.assertEqual(info['HostConfig']['SecurityOpt'],['no-new-privileges'])
        self.assertEqual(info['HostConfig']['CapDrop'],['ALL']); self.assertTrue(info['HostConfig']['ReadonlyRootfs'])
        self.assertFalse(info['Mounts'])
        try: listing=run('start','-ai',target,input=data)
        except subprocess.CalledProcessError as error:
            raise AssertionError((error.stdout+error.stderr).decode('utf-8','replace')[:512]) from None
        self.assertGreater(len(trust.parse(listing)),0)

    def test_real_pkcs12_roots_and_media_ca_via_tmpfs(self):
        docker=os.environ['CREATOR_DOCKER_CLI']
        def run(*args,input=None):
            return subprocess.run([docker,*args],input=input,stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=True,timeout=45).stdout
        generator=('cd /tmp; openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=Fixture '
                   '-keyout ca.key -out ca.crt >/dev/null 2>&1; '
                   'keytool -importkeystore -noprompt -srckeystore /opt/java/openjdk/lib/security/cacerts -srcstorepass changeit '
                   '-destkeystore store.p12 -deststoretype PKCS12 -deststorepass changeit >/dev/null 2>&1; '
                   'keytool -importcert -noprompt -alias fixture -file ca.crt -keystore store.p12 -storepass changeit >/dev/null 2>&1; '
                   'keytool -importkeystore -noprompt -srckeystore store.p12 -srcstorepass changeit -destkeystore store.jks -deststoretype JKS -deststorepass changeit >/dev/null 2>&1; '
                   'openssl x509 -in ca.crt -outform DER -out ca.der; '
                   'keytool -list -v -keystore /opt/java/openjdk/lib/security/cacerts -storepass changeit -J-Duser.language=en -J-Duser.country=US > roots.txt; '
                   'tar -cf - store.p12 store.jks ca.der roots.txt')
        cid=run('create','--network','none','--cap-drop','ALL','--entrypoint','/bin/sh','eclipse-temurin:25-jre','-ec',generator).decode().strip()
        self.assertRegex(cid,r'^[a-f0-9]{64}$'); self.addCleanup(lambda:run('rm','-f','-v',cid))
        raw=run('start','-a',cid)
        with tarfile.open(fileobj=io.BytesIO(raw),mode='r:') as archive:
            members=archive.getmembers(); self.assertEqual({m.name for m in members},{'store.p12','store.jks','ca.der','roots.txt'})
            files={m.name:archive.extractfile(m).read(1048577) for m in members if m.isfile()}
            self.assertEqual(len(files),4); self.assertTrue(all(len(v)<=1048576 for v in files.values()))
        script=target_trust.SCRIPT
        target=run('create','-i','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges',
                   '--tmpfs','/scratch:rw,noexec,nosuid,nodev,size=2m,mode=0700,uid=65532,gid=65532','--user','65532:65532',
                   '--entrypoint','/bin/sh','eclipse-temurin:25-jre','-ec',script).decode().strip()
        self.assertRegex(target,r'^[a-f0-9]{64}$'); self.addCleanup(lambda:run('rm','-f','-v',target))
        listing=target_trust._input([docker,'start','-ai',target],files['store.p12'])
        result=trust.compare_roots(trust.parse(files['roots.txt']),listing,hashlib.sha256(files['ca.der']).hexdigest())
        self.assertEqual(result['store_count'],result['root_count']+1)
        bad=run('create','-i','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges',
                '--tmpfs','/scratch:rw,noexec,nosuid,nodev,size=2m,mode=0700,uid=65532,gid=65532','--user','65532:65532',
                '--entrypoint','/bin/sh','eclipse-temurin:25-jre','-ec',script).decode().strip()
        self.assertRegex(bad,r'^[a-f0-9]{64}$'); self.addCleanup(lambda:run('rm','-f','-v',bad))
        with self.assertRaises((ValueError,subprocess.CalledProcessError)):
            target_trust._input([docker,'start','-ai',bad],files['store.jks'])
