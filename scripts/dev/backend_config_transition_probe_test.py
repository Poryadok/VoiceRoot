import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import backend_config_transition_probe as probe


class ProbeTest(unittest.TestCase):
    def setUp(self):
        self.snapshot = {'namespace':'voice-dev','namespace_uid':'ns','deployment':'voice-notification',
            'deployment_uid':'deploy','deployment_rv':'2','pod_name':'voice-notification-123',
            'pod_uid':'pod','pod_rv':'3','container_name':'notification','container_id':'containerd://id',
            'restart_count':0,'envFrom':[{'configMapRef':{'name':'config'}},{'secretRef':{'name':'secrets'}}],
            'env':[{'name':'VOICE_RELEASE_PROFILE','value':'development'},
                   {'name':'FCM_CREDENTIALS_JSON','valueFrom':{'secretKeyRef':{'name':'secrets','key':'FCM_CREDENTIALS_JSON'}}},
                   {'name':'NOTIFICATION_RECORD_PUSHES','value':'false'}]}
        self.keys = {'config':['VOICE_RELEASE_PROFILE','APNS_KEY_ID'],
                     'secrets':['APNS_KEY_ID','FCM_CREDENTIALS_JSON']}
        self.calls = []; self.fault = None; self.snapshots = 0

    def snap(self):
        self.snapshots += 1; result = copy.deepcopy(self.snapshot)
        if self.fault == 'container_drift' and self.snapshots > 1: result['container_id'] = 'other'
        return result

    def run_bytes(self,args):
        self.calls.append(args)
        if 'get' in args:
            if args[args.index('get')+1] == 'deployment':
                env = copy.deepcopy(self.snapshot['env'])
                if self.fault == 'deployment_env': env.append({'name':'FCM_PROJECT_ID','value':'changed'})
                return json.dumps({'apiVersion':'apps/v1','kind':'Deployment',
                    'metadata':{'name':'voice-notification','namespace':'voice-dev','uid':'deploy','resourceVersion':'2'},
                    'spec':{'template':{'spec':{'containers':[{'name':'notification',
                        'env':env,'envFrom':self.snapshot['envFrom']}]}}}}).encode()
            name = args[args.index('get')+2]; rv = '9'
            if self.fault == 'ref_drift' and sum('get' in call and 'deployment' not in call for call in self.calls) > 2: rv = '10'
            return ('uid-'+name+'\n'+rv+'\n'+'\n'.join(self.keys[name])+'\n').encode()
        field = args[-1]
        result = {'VOICE_RELEASE_PROFILE':'development','FCM_CREDENTIALS_JSON':'present',
                  'APNS_KEY_ID':'present','NOTIFICATION_RECORD_PUSHES':'false'}.get(field,'absent')
        if self.fault == 'secret_return' and field == 'FCM_CREDENTIALS_JSON': result = 'private-credential'
        if self.fault == 'unexpected_env' and field == 'DATABASE_URL': result = 'present'
        if self.fault == 'empty_debug' and field == 'NOTIFICATION_DEBUG_HTTP_ENABLED': result = 'empty'
        return result.encode()

    def inspect(self):
        with patch.object(probe.metadata,'kube_workload_snapshot',return_value=self.snap):
            return probe.inspect('notification','voice-notification-123',self.run_bytes,lambda:None)

    def test_fixed_enum_and_reference_precedence_no_secret_output(self):
        value = self.inspect(); fields = value['fields']
        self.assertEqual(fields['VOICE_RELEASE_PROFILE'],{'state':'development','origin':{'source':'literal'}})
        self.assertEqual(fields['FCM_CREDENTIALS_JSON']['origin']['source'],'explicit')
        self.assertEqual(fields['APNS_KEY_ID']['origin']['name'],'secrets')
        self.assertEqual(fields['APNS_KEY_ID']['origin']['source'],'envFrom')
        self.assertEqual([x['name'] for x in value['env_sources']],['config','secrets'])
        self.assertEqual(value['env_sources'][0]['fields'],['APNS_KEY_ID','VOICE_RELEASE_PROFILE'])
        self.assertNotIn('private-credential',json.dumps(value)); self.assertEqual(value['status'],'observed')

    def test_empty_envfrom_is_still_bound(self):
        self.keys['config'] = []
        value = self.inspect()
        self.assertEqual(value['env_sources'][0]['fields'],[])
        self.assertEqual(value['env_sources'][0]['resource_version'],'9')

    def test_identity_ref_and_runtime_drift_hold_without_private_diagnostics(self):
        for fault in ('container_drift','ref_drift','secret_return','unexpected_env','deployment_env'):
            with self.subTest(fault=fault):
                self.setUp(); self.fault = fault
                with self.assertRaises(probe.ProbeError) as error: self.inspect()
                self.assertNotIn('private-credential',str(error.exception))
                if fault == 'secret_return': self.assertIn('runtime-FCM_CREDENTIALS_JSON',str(error.exception))
                if fault == 'unexpected_env': self.assertIn('origin-DATABASE_URL',str(error.exception))
                if fault in ('container_drift','ref_drift'): self.assertIn('identity-drift',str(error.exception))
                if fault == 'deployment_env': self.assertIn('deployment-template',str(error.exception))

    def test_duplicate_and_optional_sources_hold(self):
        self.snapshot['env'].append(copy.deepcopy(self.snapshot['env'][0]))
        with self.assertRaisesRegex(probe.ProbeError,'explicit-env-duplicate'): self.inspect()
        self.setUp(); self.snapshot['envFrom'][0]['configMapRef']['optional'] = True
        with self.assertRaises(probe.ProbeError): self.inspect()

    def test_explicit_ref_failures_report_only_fixed_reason_and_field(self):
        for fault in ('unsupported-ref','optional','missing-key'):
            with self.subTest(fault=fault):
                self.setUp(); entry = self.snapshot['env'][1]
                if fault == 'unsupported-ref': entry['valueFrom'] = {'fieldRef':{'fieldPath':'metadata.name'}}
                if fault == 'optional': entry['valueFrom']['secretKeyRef']['optional'] = True
                if fault == 'missing-key': entry['valueFrom']['secretKeyRef']['key'] = 'private-name'
                with self.assertRaises(probe.ProbeError) as error: self.inspect()
                self.assertIn('explicit-env-'+fault+'-FCM_CREDENTIALS_JSON',str(error.exception))
                self.assertNotIn('private-name',str(error.exception))

    def test_empty_literal_env_is_observed_as_empty(self):
        self.snapshot['env'].append({'name':'NOTIFICATION_DEBUG_HTTP_ENABLED'})
        self.fault = 'empty_debug'
        self.assertEqual(self.inspect()['fields']['NOTIFICATION_DEBUG_HTTP_ENABLED'],
                         {'state':'empty','origin':{'source':'literal'}})

    def test_guard_loss_prevents_observation(self):
        with patch.object(probe.metadata,'kube_workload_snapshot',return_value=self.snap):
            with self.assertRaises(probe.ProbeError):
                probe.inspect('notification','voice-notification-123',self.run_bytes,lambda:False)
        self.assertEqual(self.calls,[])

    def test_real_shell_categories_trim_and_never_emit_values(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); executable = root/'exe'; executable.symlink_to('/service'); environment = root/'environ'
            script = probe.SCRIPT.replace('/proc/1/exe',str(executable)).replace('/proc/1/environ',str(environment))
            for field,value,wanted in [('APNS_PRODUCTION',b' false ',b'false'),
                    ('APNS_PRODUCTION',b'\n false \r\n',b'false'),
                    ('APNS_PRODUCTION',b'fal\nse',b'other'),
                    ('APNS_PRODUCTION',b'FALSE',b'other'),
                    ('APNS_PRODUCTION',b'\xc2\xa0false\xc2\xa0',b'unsupported'),
                    ('NOTIFICATION_RECORD_PUSHES',b' TrUe ',b'true'),
                    ('NOTIFICATION_RECORD_PUSHES',b'\n TrUe \r\n',b'true'),
                    ('FCM_CREDENTIALS_JSON',b'private-credential',b'present')]:
                environment.write_bytes(field.encode()+b'='+value+b'\0')
                result = subprocess.run(['bash','-ec',script,'--',field],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=5)
                with self.subTest(field=field,value=value):
                    self.assertEqual(result.returncode,0); self.assertEqual(result.stdout,wanted)
                    self.assertNotIn(b'private-credential',result.stdout+result.stderr)


if __name__ == '__main__': unittest.main()
