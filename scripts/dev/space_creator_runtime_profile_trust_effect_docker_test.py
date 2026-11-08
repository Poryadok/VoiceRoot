"""Offline real-keytool check; fixture image is not a P1 qualification."""

import hashlib
import base64
import copy
import json
import os
import re
import subprocess
import unittest
from unittest import mock
import uuid

import space_creator_runtime_profile_target_trust as target
import space_creator_runtime_profile_trust as trust
import space_creator_runtime_profile_trust_effect as effect
import space_creator_runtime_profile_trust_effect_test as fixture
import space_creator_runtime_profile_jdk as jdk
import dev_peer_issuer_preflight as issuer
import writer_cut


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
        jks_script = ('umask 077; cat > /scratch/in; '
                      '/opt/java/openjdk/bin/keytool -importkeystore -noprompt '
                      '-srckeystore /scratch/in -srcstoretype PKCS12 -srcstorepass changeit '
                      '-destkeystore /scratch/out -deststoretype JKS -deststorepass changeit >/dev/null 2>&1; '
                      'cat /scratch/out')
        cid = create(*options, jks_script)
        jks = target._input([docker, 'start', '-ai', cid], store)
        cid = create(*options, target.SCRIPT)
        with self.assertRaises(ValueError):
            target._input([docker, 'start', '-ai', cid], jks)

        # Actual effect/tool/guard/journals; issuer, package and Kubernetes remain synthetic.
        phase_fixture = fixture.EffectStateTest('test_missing_create_has_private_keyed_receipt_and_detects_drift')
        phase_fixture.setUp(); self.addCleanup(phase_fixture.doCleanups)
        image_info = json.loads(run('image', 'inspect', image))[0]
        phase_fixture.roots.update(image=image_info['RepoDigests'][0], config_digest=image_info['Id'],
                                   root_fingerprints=sorted(roots))
        phase_fixture.source['ca']['der_sha256'] = ca_sha
        for phase, module, receipt in ((phase_fixture.phase.jdk, jdk, phase_fixture.roots),
                                       (phase_fixture.phase.preflight, issuer, phase_fixture.source)):
            phase.record = {'status': 'done', 'receipt_sha256': writer_cut.digest(receipt)}
            step = module.STEP if module is jdk else phase.step
            ledger = module.LEDGER if module is jdk else phase.ledger
            phase.journal.state['steps'][step] = copy.deepcopy(phase.record) | {'receipt': copy.deepcopy(receipt)}
            phase.journal.save(); phase_fixture.base.ledger.row[ledger] = copy.deepcopy(phase.record)

        def kube(args, payload=None):
            if args[:3] == ['get', 'secret', 'voice-media-ca']:
                return b'ca\n1\n\n' + base64.b64encode(ca)
            return phase_fixture.kube(args, payload)

        actual_tool = effect.TrustEffectPhase._tool.__get__(phase_fixture.phase)
        with mock.patch.object(effect.private, '_run', side_effect=kube), \
             mock.patch.object(effect.target.material, '_cert_der', return_value=der), \
             mock.patch.object(phase_fixture.phase, '_tool', actual_tool):
            plan = phase_fixture.phase.prepare('token')
            self.assertEqual(plan['mode'], 'create'); self.assertEqual(phase_fixture.writes, [])
            receipt = phase_fixture.phase.run('token')
            self.assertEqual(receipt['mode'], 'create')
            self.assertEqual(effect.verify_current(phase_fixture.phase, 'token'), receipt)
            self.assertEqual(len(phase_fixture.writes), 1)


if __name__ == '__main__':
    unittest.main()
