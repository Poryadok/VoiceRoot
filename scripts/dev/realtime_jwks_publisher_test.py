"""Fenced Realtime JWKS auxiliary publisher contracts."""
import hashlib
import json
import unittest
from unittest import mock

import realtime_jwks_publisher as publisher


def encoded(value):
    raw = json.dumps(value, sort_keys=True, separators=(',', ':')).encode()
    return raw, 'sha256:' + hashlib.sha256(raw).hexdigest()


class Contracts(unittest.TestCase):
    def test_source_render_contains_no_signing_key_and_pins_node(self):
        config, deployment, service = publisher.desired('node-a')
        self.assertEqual(config['metadata']['name'], 'voice-p3-principal-publisher-config')
        self.assertEqual(service['metadata']['name'], 'voice-p3-principal-jwks')
        pod = deployment['spec']['template']['spec']
        self.assertEqual(pod['nodeSelector']['kubernetes.io/hostname'], 'node-a')
        self.assertIs(pod['automountServiceAccountToken'], False)
        self.assertEqual(pod['containers'][0]['image'], publisher.IMAGE)
        self.assertNotIn('voice-realtime-principal-signing', json.dumps(pod))
        self.assertEqual({v['name'] for v in pod['volumes']}, {'public', 'tls', 'config'})

    def test_oci_chain_rejects_changed_config_and_ambiguous_platform(self):
        config, config_sha = encoded({'architecture': 'amd64', 'os': 'linux'})
        child, child_sha = encoded({'schemaVersion': 2, 'layers': [], 'config': {'digest': config_sha,
            'size': len(config), 'mediaType': 'application/vnd.oci.image.config.v1+json'}})
        descriptor = {'digest': child_sha, 'size': len(child), 'mediaType': 'application/vnd.oci.image.manifest.v1+json',
                      'platform': {'os': 'linux', 'architecture': 'amd64'}}
        index, index_sha = encoded({'schemaVersion': 2, 'mediaType': 'application/vnd.oci.image.index.v1+json', 'manifests': [descriptor]})
        with mock.patch.multiple(publisher, INDEX=index_sha, PLATFORM=child_sha, CONFIG=config_sha):
            self.assertEqual(publisher.qualify_oci(index, child, config)['platform'], child_sha)
            with self.assertRaises(publisher.PublisherError): publisher.qualify_oci(index, child, config + b' ')
        ambiguous, ambiguous_sha = encoded({'schemaVersion': 2, 'mediaType': 'application/vnd.oci.image.index.v1+json',
                                            'manifests': [descriptor, descriptor]})
        with mock.patch.multiple(publisher, INDEX=ambiguous_sha, PLATFORM=child_sha, CONFIG=config_sha):
            with self.assertRaises(publisher.PublisherError): publisher.qualify_oci(ambiguous, child, config)

    def test_source_equivalence_is_limited_to_three_canonical_documents(self):
        from pathlib import Path
        raw = (Path(__file__).resolve().parents[2] / 'deploy/staging/services.yaml').read_bytes()
        with mock.patch('realtime_p3_create_only_source._blob', return_value=raw), \
             mock.patch('realtime_p3_create_only_source._disk', return_value=raw):
            publisher.verify_source('/trusted/repo', 'a' * 40)
        modified = raw.replace(b'location = /realtime.json', b'location = /different.json')
        with mock.patch('realtime_p3_create_only_source._blob', side_effect=[raw, modified]), \
             mock.patch('realtime_p3_create_only_source._disk', return_value=modified):
            with self.assertRaises(publisher.PublisherError): publisher.verify_source('/trusted/repo', 'a' * 40)

    def test_admission_rejects_extra_container_or_secret_mount(self):
        import copy
        _, wanted, _ = publisher.desired('node-a')
        publisher.check_admission(wanted, wanted)
        changed = copy.deepcopy(wanted)
        changed['spec']['template']['spec']['containers'].append({'name': 'intruder', 'image': 'untrusted'})
        with self.assertRaises(publisher.PublisherError): publisher.check_admission(changed, wanted)
        changed = copy.deepcopy(wanted)
        changed['spec']['template']['spec']['volumes'].append({'name': 'keys', 'secret': {'secretName': 'voice-realtime-principal-signing'}})
        with self.assertRaises(publisher.PublisherError): publisher.check_admission(changed, wanted)
        changed = copy.deepcopy(wanted); changed['spec']['template']['spec']['automountServiceAccountToken'] = True
        with self.assertRaises(publisher.PublisherError): publisher.check_admission(changed, wanted)
        _, _, service = publisher.desired('node-a')
        wrong = copy.deepcopy(service); wrong['spec']['internalTrafficPolicy'] = 'Local'
        with self.assertRaises(publisher.PublisherError): publisher.check_admission(wrong, service)
        wrong = copy.deepcopy(service); wrong['spec']['clusterIP'] = 'None'
        with self.assertRaises(publisher.PublisherError): publisher.check_admission(wrong, service)

    def test_real_certificate_chain_and_hostname_are_checked_using_only_public_data(self):
        import base64
        import realtime_p3_create_only_packet_test as fixture
        fixture.PacketTest.setUpClass()
        self.addCleanup(fixture.PacketTest.doClassCleanups)
        items = fixture.PacketTest.envelope['objects']['items']
        cert = base64.b64decode(next(x for x in items if x['metadata']['name'] == publisher.TLS_NAME)['data']['tls.crt'])
        self.assertEqual(len(publisher._verify_cert(cert, fixture.PacketTest.principal)), 64)
        with self.assertRaises(Exception): publisher._verify_cert(cert, fixture.PacketTest.media)


if __name__ == '__main__':
    unittest.main()
