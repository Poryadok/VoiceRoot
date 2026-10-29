import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import realtime_p3_public_inputs as capture


class CaptureTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        manifest = self.root / 'deploy/staging/services.yaml'
        manifest.parent.mkdir(parents=True)
        manifest.write_bytes(b'kind: Deployment\nmetadata:\n  name: voice-space\nspec:\n  template:\n    spec:\n      containers:\n        - name: space\n          env:\n            - ' + capture.SOURCE_LINES[0] + b'\n            - ' + capture.SOURCE_LINES[1] + b'\n            - ' + capture.SOURCE_LINES[4] + b'\n            - ' + capture.SOURCE_LINES[5] + b'\n            - ' + capture.SOURCE_LINES[6] + b'\n          volumeMounts:\n            - ' + capture.SOURCE_LINES[2] + b'\n            - ' + capture.SOURCE_LINES[7] + b'\n      volumes:\n        - name: media-server\n          ' + capture.SOURCE_LINES[3] + b'\n        - name: media-signing\n          ' + capture.SOURCE_LINES[8] + b'\n')
        self.manifest_sha = hashlib.sha256(manifest.read_bytes()).hexdigest()
        self.document_sha = self.manifest_sha
        def openssl(*args):
            subprocess.run(['openssl', *map(str, args)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        for name in ('principal', 'media'):
            openssl('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '2', '-subj', '/CN=' + name,
                    '-addext', 'basicConstraints=critical,CA:TRUE', '-keyout', self.root / (name + '.key'), '-out', self.root / (name + '.crt'))
        openssl('req', '-new', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=voice-space.voice-dev.svc',
                '-keyout', self.root / 'leaf.key', '-out', self.root / 'leaf.csr')
        (self.root / 'ext').write_text('basicConstraints=critical,CA:FALSE\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:voice-space.voice-dev.svc\n')
        openssl('x509', '-req', '-in', self.root / 'leaf.csr', '-CA', self.root / 'media.crt', '-CAkey', self.root / 'media.key',
                '-set_serial', '42', '-days', '2', '-extfile', self.root / 'ext', '-out', self.root / 'leaf.crt')
        self.values = {'voice-principal-ca': (self.root / 'principal.crt').read_bytes(),
                       'voice-media-ca': (self.root / 'media.crt').read_bytes(),
                       'voice-space-media-server-tls': (self.root / 'leaf.crt').read_bytes(),
                       'voice-app-config': b'voice-redis:6379'}
        self.pod = {'metadata': {'name': 'voice-space-abc', 'uid': 'pod', 'resourceVersion': '3', 'labels': {'app': 'voice-space'}},
                    'spec': {'containers': [{'name': 'space', 'env': [
                        {'name': 'SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE', 'value': '/etc/voice/media/server/tls.crt'},
                        {'name': 'SPACE_PRINCIPAL_JWKS_TLS_KEY_FILE', 'value': '/etc/voice/media/server/tls.key'},
                        {'name': 'SPACE_PRINCIPAL_JWKS_HTTPS_LISTEN', 'value': ':8443'}],
                        'volumeMounts': [{'name': 'media-server', 'mountPath': '/etc/voice/media/server', 'readOnly': True}]}],
                        'volumes': [{'name': 'media-server', 'secret': {'secretName': 'voice-space-media-server-tls',
                            'items': [{'key': 'tls.crt', 'path': 'tls.crt'}, {'key': 'tls.key', 'path': 'tls.key'}]}}]}}
        self.identity = {'namespace_uid': 'ns', 'pod_uid': 'pod', 'pod_rv': '3', 'deployment_uid': 'deploy',
                         'deployment_rv': '1', 'container_id': 'ctr', 'restart_count': 0}

    def tearDown(self): self.tmp.cleanup()

    def runner(self, args):
        kind, name = args[args.index('get') + 1:args.index('get') + 3]
        if kind == 'namespace': return json.dumps({'metadata': {'name': 'voice-dev', 'uid': 'ns'}}).encode()
        if kind == 'pod': return json.dumps(self.pod).encode()
        if kind == 'service': return json.dumps({'metadata': {'name': name, 'namespace': 'voice-dev', 'uid': 'svc', 'resourceVersion': '2'},
             'spec': {'clusterIP': '10.0.0.1', 'selector': {'app': 'voice-space'}, 'ports': [{'port': 8443, 'protocol': 'TCP'}]}}).encode()
        if name == 'voice-space-principal-signing' and any(x.startswith('go-template=') for x in args):
            return b'signer-uid\n4\n1\n1\n1'
        if kind == 'secret' and any(x.startswith('go-template=') for x in args):
            return ('uid-'+name+'\n1\n1').encode()
        if '-o' in args and args[args.index('-o') + 1].startswith('jsonpath='):
            expression = args[args.index('-o') + 1]
            if '.metadata.uid' in expression: return ('uid-' + name + '\n1').encode()
            value = self.values[name]
            return base64.b64encode(value) if kind == 'secret' else value
        raise AssertionError('unexpected transport')

    def invoke(self, destination, guard=lambda: None, runner=None, workload=None, authority=lambda sha: None, mode='serving'):
        snap = {'source_sha': 'a' * 40, 'files': {'deploy/staging/services.yaml': self.manifest_sha}}
        with patch.object(capture, 'source_snapshot', return_value=snap), patch.object(capture, 'SPACE_DOCUMENT_SHA', self.document_sha), patch.object(capture.common, 'kube_workload_snapshot', return_value=workload or (lambda: dict(self.identity))):
            return capture.capture(self.root, 'a' * 40, authority, 'voice-space-abc', 'ns', guard, destination, runner or self.runner, mode=mode)

    def test_real_chain_and_private_files(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        result = self.invoke(dest)
        self.assertEqual(result['status'], 'observed')
        self.assertFalse(result['admission_qualified'])
        self.assertEqual(result['redis_endpoint'], 'expected')
        self.assertEqual(sorted(p.name for p in dest.iterdir()), ['media-ca.pem', 'principal-ca.pem', 'space-jwks.pem'])
        self.assertTrue(all(p.stat().st_mode & 0o777 == 0o600 for p in dest.iterdir()))
        self.assertNotIn('BEGIN CERTIFICATE', repr(result))

    def test_wrong_mount_fails_without_writing(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        self.pod['spec']['volumes'][0]['secret']['secretName'] = 'other'
        with self.assertRaises(capture.common.Hold): self.invoke(dest)
        self.assertEqual(list(dest.iterdir()), [])

    def test_fixed_space_env_categories_without_values(self):
        for field,prefix in [('SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE','space_cert_env'),
                             ('SPACE_PRINCIPAL_JWKS_TLS_KEY_FILE','space_key_env'),
                             ('SPACE_PRINCIPAL_JWKS_HTTPS_LISTEN','space_listen_env')]:
            for fault in ('absent','duplicate','mismatch'):
                with self.subTest(field=field,fault=fault):
                    original=self.pod['spec']['containers'][0]['env']
                    env=[dict(x) for x in original]; selected=next(x for x in env if x['name']==field)
                    if fault=='absent': env.remove(selected)
                    if fault=='duplicate': env.append(dict(selected))
                    if fault=='mismatch': selected['value']='private-sentinel'
                    self.pod['spec']['containers'][0]['env']=env
                    dest=self.root/(prefix+'-'+fault); dest.mkdir(mode=0o700)
                    with self.assertRaises(capture.common.Hold) as error: self.invoke(dest)
                    self.assertEqual(str(error.exception),prefix+'_'+fault)
                    self.assertNotIn('private-sentinel',str(error.exception)); self.assertEqual(list(dest.iterdir()),[])
                    self.pod['spec']['containers'][0]['env']=original

    def test_issuer_plan_captures_crypto_without_claiming_current_serving(self):
        dest=self.root/'issuer-plan'; dest.mkdir(mode=0o700)
        self.pod['spec']['containers'][0]['env']=[]
        self.pod['spec']['containers'][0]['volumeMounts']=[]
        def runner(args):
            if 'service' in args: raise AssertionError('issuer plan must not claim serving')
            return self.runner(args)
        result=self.invoke(dest,runner=runner,mode='issuer-plan')
        self.assertEqual(result['scope'],'issuer-source-plan-only')
        self.assertIsNone(result['space_service_spec']); self.assertFalse(result['admission_qualified'])
        self.assertEqual(result['current_space_config'],{'space_cert_env':'absent','space_key_env':'absent',
            'space_listen_env':'absent','space_mount':'mismatch'})
        self.assertEqual(result['current_signer']['configuration'],{'signing_dir':'absent','active_kid_ref':'absent','signing_mount':'mismatch'})
        self.assertEqual(result['current_signer']['secret'],{'state':'present','uid':'signer-uid','resource_version':'4',
            'keys':['current.pem','next.pem','active-kid']})
        self.assertIn('current_space_https',result['unobserved'])
        with self.assertRaises(capture.common.Hold):
            other=self.root/'serving'; other.mkdir(mode=0o700); self.invoke(other)

    def test_key_only_shadow_is_not_valid_mount(self):
        self.pod['spec']['containers'][0]['volumeMounts'].append(
            {'name':'foreign','mountPath':'/etc/voice/media/server/tls.key','readOnly':True})
        dest=self.root/'shadow-serving'; dest.mkdir(mode=0o700)
        with self.assertRaisesRegex(capture.common.Hold,'space_mount_invalid'): self.invoke(dest)
        self.assertEqual(list(dest.iterdir()),[])
        planned=self.root/'shadow-plan'; planned.mkdir(mode=0o700)
        result=self.invoke(planned,mode='issuer-plan')
        self.assertEqual(result['current_space_config']['space_mount'],'mismatch')

    def test_signer_metadata_absence_and_drift_are_distinct(self):
        dest=self.root/'signer-absent'; dest.mkdir(mode=0o700)
        def missing(args):
            if 'voice-space-principal-signing' in args: return b''
            return self.runner(args)
        result=self.invoke(dest,runner=missing,mode='issuer-plan')
        self.assertEqual(result['current_signer']['secret']['state'],'absent')
        dest=self.root/'signer-drift'; dest.mkdir(mode=0o700); calls=0
        def changed(args):
            nonlocal calls
            if 'voice-space-principal-signing' in args:
                calls+=1; return b'signer-uid\n4\n1\n1\n1' if calls==1 else b'signer-uid\n5\n1\n1\n1'
            return self.runner(args)
        with self.assertRaisesRegex(capture.common.Hold,'space_identity_changed'):
            self.invoke(dest,runner=changed,mode='issuer-plan')
        self.assertEqual(list(dest.iterdir()),[])

    def test_exact_signer_reference_reports_only_fixed_metadata(self):
        pod=self.pod['spec']; container=pod['containers'][0]
        container['env'].extend([{'name':'SPACE_PRINCIPAL_SIGNING_KEYS_DIR','value':'/etc/voice/media/signing'},
            {'name':'SPACE_PRINCIPAL_ACTIVE_KID','valueFrom':{'secretKeyRef':{
                'name':'voice-space-principal-signing','key':'active-kid'}}}])
        container['volumeMounts'].append({'name':'signer','mountPath':'/etc/voice/media/signing','readOnly':True})
        pod['volumes'].append({'name':'signer','secret':{'secretName':'voice-space-principal-signing',
            'items':[{'key':'current.pem','path':'current.pem'},{'key':'next.pem','path':'next.pem'}]}})
        dest=self.root/'signer-exact'; dest.mkdir(mode=0o700)
        result=self.invoke(dest,mode='issuer-plan')
        self.assertEqual(result['current_signer']['configuration'],{'signing_dir':'exact','active_kid_ref':'exact','signing_mount':'exact'})
        self.assertNotIn('PRIVATE KEY',repr(result))

    def test_service_missing_identity_fails(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        def runner(args):
            result = self.runner(args)
            if 'service' in args:
                value = json.loads(result); value['metadata']['uid'] = ''
                return json.dumps(value).encode()
            return result
        with self.assertRaisesRegex(capture.common.Hold, 'space_service_invalid'): self.invoke(dest, runner=runner)
        self.assertEqual(list(dest.iterdir()), [])

    def test_source_comments_and_sidecar_cannot_claim_space_contract(self):
        manifest = self.root / 'deploy/staging/services.yaml'
        original = manifest.read_bytes()
        for altered in (original.replace(b'            - ' + capture.SOURCE_LINES[0], b'            # - ' + capture.SOURCE_LINES[0]),
                        original.replace(b'        - name: space\n', b'        - name: sidecar\n')):
            manifest.write_bytes(altered)
            self.manifest_sha = hashlib.sha256(altered).hexdigest()
            dest = self.root / ('private' + str(len(list(self.root.glob('private*'))))); dest.mkdir(mode=0o700)
            with self.assertRaisesRegex(capture.common.Hold, 'space_source_contract_unknown'): self.invoke(dest)
            self.assertEqual(list(dest.iterdir()), [])

    def test_guard_loss_propagates(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        with self.assertRaises(capture.common.GuardLost): self.invoke(dest, lambda: False)
        self.assertEqual(list(dest.iterdir()), [])

    def test_private_key_never_written(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        self.values['voice-principal-ca'] = b'-----BEGIN PRIVATE KEY-----\nprivate\n-----END PRIVATE KEY-----\n'
        with self.assertRaises(capture.common.Hold): self.invoke(dest)
        self.assertEqual(list(dest.iterdir()), [])

    def test_relabelled_noncertificate_never_written(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        payload = base64.b64encode(b'private material not an X509 certificate')
        self.values['voice-principal-ca'] = b'-----BEGIN CERTIFICATE-----\n' + payload + b'\n-----END CERTIFICATE-----\n'
        with self.assertRaises(capture.common.Hold): self.invoke(dest)
        self.assertEqual(list(dest.iterdir()), [])

    def test_changed_secret_revision_holds(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        calls = 0
        def runner(args):
            nonlocal calls
            if 'voice-media-ca' in args and any('.metadata.uid' in a for a in args):
                calls += 1
                if calls > 1: return b'uid-voice-media-ca\n2'
            return self.runner(args)
        with self.assertRaisesRegex(capture.common.Hold, 'public_input_changed'): self.invoke(dest, runner=runner)
        self.assertEqual(list(dest.iterdir()), [])

    def test_guard_loss_after_writes_leaves_unqualified_public_files(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        def guard():
            if list(dest.iterdir()): return False
        with self.assertRaises(capture.common.GuardLost): self.invoke(dest, guard)
        self.assertEqual(sorted(p.name for p in dest.iterdir()), ['media-ca.pem', 'principal-ca.pem', 'space-jwks.pem'])

    def test_cleanup_never_unlinks_replaced_file(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        def replace(_paths):
            target = dest / 'principal-ca.pem'
            target.unlink(); target.write_bytes(b'foreign')
            raise ValueError('fixture')
        with patch.object(capture, '_verify', side_effect=replace):
            with self.assertRaises(capture.common.Hold): self.invoke(dest)
        self.assertEqual((dest / 'principal-ca.pem').read_bytes(), b'foreign')
        self.assertEqual(sorted(p.name for p in dest.iterdir()), ['media-ca.pem', 'principal-ca.pem', 'space-jwks.pem'])

    def test_exception_does_not_leak_private_error(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        def runner(args): raise ValueError('do-not-print-private-payload')
        with self.assertRaises(Exception) as error: self.invoke(dest, runner=runner)
        self.assertNotIn('do-not-print-private-payload', str(error.exception))

    def test_unknown_prewrite_failures_report_only_fixed_stage(self):
        faults = [('source_contract','_source_contract','p3_stage_source'),
                  ('pod','_get','p3_stage_pod'),('mount','_mounted','p3_stage_mount'),
                  ('identity','_meta','p3_stage_principal_ca_identity'),('value','_value','p3_stage_public_value'),
                  ('pem','_public_pem','p3_stage_public_pem')]
        for label, symbol, code in faults:
            with self.subTest(stage=label):
                dest = self.root / ('stage-'+label); dest.mkdir(mode=0o700)
                with patch.object(capture,symbol,side_effect=ValueError('private-sentinel')):
                    with self.assertRaises(capture.common.Hold) as error: self.invoke(dest)
                self.assertEqual(str(error.exception),code)
                self.assertNotIn('private-sentinel',str(error.exception))
                self.assertEqual(list(dest.iterdir()),[])

    def test_unknown_wrapped_hold_is_also_sanitized(self):
        dest=self.root/'wrapped'; dest.mkdir(mode=0o700)
        with patch.object(capture,'_mounted',side_effect=capture.common.Hold('private-sentinel')):
            with self.assertRaises(capture.common.Hold) as error: self.invoke(dest)
        self.assertEqual(str(error.exception),'p3_stage_mount')

    def test_signer_metadata_remains_available_when_public_identity_fails(self):
        snap={'source_sha':'a'*40,'files':{'deploy/staging/services.yaml':self.manifest_sha}}
        with patch.object(capture,'source_snapshot',return_value=snap),patch.object(capture,'SPACE_DOCUMENT_SHA',self.document_sha), \
             patch.object(capture.common,'kube_workload_snapshot',return_value=lambda:dict(self.identity)):
            result=capture.observe_signer(self.root,'a'*40,lambda _:None,'voice-space-abc','ns',lambda:None,self.runner)
        self.assertEqual(result['scope'],'signer-metadata-only'); self.assertFalse(result['admission_qualified'])
        self.assertEqual(result['current_signer']['secret']['state'],'present')
        self.assertNotIn('PRIVATE KEY',repr(result))
        dest=self.root/'missing-ca'; dest.mkdir(mode=0o700)
        def runner(args):
            if 'voice-media-ca' in args and any('.metadata.uid' in a for a in args): return b''
            return self.runner(args)
        with self.assertRaisesRegex(capture.common.Hold,'p3_stage_media_ca_identity'):
            self.invoke(dest,runner=runner,mode='issuer-plan')

    def test_public_metadata_distinguishes_absence_and_unknown_transport(self):
        snap={'source_sha':'a'*40,'files':{'deploy/staging/services.yaml':self.manifest_sha}}
        calls=[]
        def absent(args):
            if 'voice-media-ca' in args:
                calls.append(args); return b''
            return self.runner(args)
        with patch.object(capture,'source_snapshot',return_value=snap),patch.object(capture,'SPACE_DOCUMENT_SHA',self.document_sha):
            result=capture.observe_public_metadata(self.root,'a'*40,lambda _:None,'ns',lambda:None,absent)
            self.assertEqual(result['objects']['voice-media-ca']['state'],'absent')
            self.assertFalse(result['objects']['voice-media-ca']['key_present'])
            self.assertFalse(result['admission_qualified']); self.assertTrue(all('--ignore-not-found=true' in a for a in calls))
            def broken(args):
                if 'voice-media-ca' in args: raise OSError('private-sentinel')
                return self.runner(args)
            with self.assertRaises(capture.common.Hold) as error:
                capture.observe_public_metadata(self.root,'a'*40,lambda _:None,'ns',lambda:None,broken)
            self.assertEqual(str(error.exception),'public_metadata_observation_failed')

    def test_restart_and_source_authority_fail_closed(self):
        dest = self.root / 'private'; dest.mkdir(mode=0o700)
        count = 0
        def workload():
            nonlocal count
            count += 1
            return dict(self.identity, restart_count=0 if count == 1 else 1)
        with self.assertRaisesRegex(capture.common.Hold, 'space_identity_changed'):
            self.invoke(dest, workload=workload)
        self.assertEqual(list(dest.iterdir()), [])
        with self.assertRaisesRegex(capture.common.Hold, 'source_authority_failed'):
            self.invoke(dest, authority=lambda sha: False)
        self.assertEqual(list(dest.iterdir()), [])


if __name__ == '__main__': unittest.main()
