"""Offline real-keytool check; fixture image is not a P1 qualification."""

import hashlib
import json
import os
import re
import subprocess
import unittest
import uuid

import space_creator_runtime_profile_target_trust as target
import space_creator_runtime_profile_trust as trust
import space_creator_runtime_profile_trust_effect as effect


class RealKeytoolTest(unittest.TestCase):
    def test_generated_store_has_only_expected_public_roots(self):
        docker = os.environ['CREATOR_DOCKER_CLI']
        image = 'eclipse-temurin:25-jre'

        def run(*args):
            return subprocess.run([docker, *args], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                                  timeout=90, check=True).stdout

        def create(*args):
            owner = 'voice-trust-test-' + uuid.uuid4().hex
            cid = run('create', '--name', owner, '--network', 'none', '--read-only', '--cap-drop', 'ALL',
                      '--security-opt', 'no-new-privileges', '--log-driver', 'none',
                      '--label', 'voice.test.owner=' + owner, *args).decode('ascii').strip()
            self.assertRegex(cid, r'^[a-f0-9]{64}$')
            detail = json.loads(run('inspect', cid))
            self.assertEqual(detail[0]['Id'], cid)
            self.assertEqual(detail[0]['Config']['Labels']['voice.test.owner'], owner)
            self.addCleanup(lambda exact=cid: run('rm', '-f', '-v', exact))
            return cid

        ca_script = ('umask 077; openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=Fixture '
                     '-keyout /tmp/key -out /tmp/ca >/dev/null 2>&1; cat /tmp/ca')
        cid = create('--tmpfs', '/tmp:rw,noexec,nosuid,nodev,size=2m,mode=0700',
                     '--entrypoint', '/bin/sh', image, '-ec', ca_script)
        ca = run('start', '-a', cid)
        der = subprocess.run(['openssl', 'x509', '-outform', 'DER'], input=ca, stdout=subprocess.PIPE,
                             stderr=subprocess.DEVNULL, timeout=10, check=True).stdout
        ca_sha = hashlib.sha256(der).hexdigest()

        cid = create('--entrypoint', '/opt/java/openjdk/bin/keytool', image, '-list', '-v',
                     '-keystore', '/opt/java/openjdk/lib/security/cacerts', '-storepass', 'changeit',
                     '-J-Duser.language=en', '-J-Duser.country=US')
        roots = trust.parse(run('start', '-a', cid))
        options = ['-i', '--tmpfs', '/scratch:' + target.TMPFS, '--user', '65532:65532',
                   '--entrypoint', '/bin/sh', image, '-ec']
        cid = create(*options, effect.GENERATE)
        store = target._input([docker, 'start', '-ai', cid], ca)
        self.assertTrue(store); self.assertLessEqual(len(store), 1048576)
        cid = create(*options, target.SCRIPT)
        listing = target._input([docker, 'start', '-ai', cid], store)
        facts = trust.compare_roots(roots, listing, ca_sha)
        self.assertEqual(facts['store_count'], facts['root_count'] + 1)
        self.assertEqual(effect.classify_listing(listing, roots, ca_sha), 'preserve')


if __name__ == '__main__':
    unittest.main()
