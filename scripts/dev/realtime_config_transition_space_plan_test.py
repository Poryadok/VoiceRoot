import base64
import copy
import json
import socket
import subprocess
import sys
import threading
import unittest
from unittest.mock import patch

import realtime_config_transition_space_plan as plan


def jwks():
    def item(kid,value): return {'kty':'RSA','kid':kid,'use':'sig','alg':'RS256',
        'n':base64.urlsafe_b64encode(b'\x80'+bytes([value])*255).decode().rstrip('='),'e':'AQAB'}
    return json.dumps({'keys':[item('current',1),item('next',2)]}).encode()


class PlanTest(unittest.TestCase):
    def test_fixed_public_key_shape(self):
        plan._parse(jwks())
        for value in (b'{"keys":[{"kty":"oct","k":"private-sentinel"}]}',
                      b'{"keys":[],"keys":[]}'):
            with self.assertRaises(ValueError): plan._parse(value)

    def test_actual_http_tunnel_bounded_and_reaped(self):
        server=socket.socket(); server.bind(('127.0.0.1',0)); server.listen(1); server.settimeout(5); port=server.getsockname()[1]
        errors=[]
        def serve():
            try:
                conn,_=server.accept()
                with conn:
                    conn.recv(4096); body=jwks()
                    conn.sendall(b'HTTP/1.1 200 OK\r\nContent-Length: '+str(len(body)).encode()+b'\r\nConnection: close\r\n\r\n'+body)
            except Exception as error: errors.append(error)
            finally: server.close()
        thread=threading.Thread(target=serve); thread.start(); processes=[]; actual=subprocess.Popen
        def launch(*args,**kwargs):
            process=actual([sys.executable,'-c',f'import time;print("Forwarding from 127.0.0.1:{port} -> 8080",flush=True);time.sleep(30)'],**kwargs)
            processes.append(process); return process
        try:
            with patch.object(plan.subprocess,'Popen',side_effect=launch):
                self.assertEqual(plan._http('voice-space-abc',lambda:None),jwks())
        finally: thread.join(timeout=6)
        self.assertFalse(thread.is_alive()); self.assertEqual(errors,[]); self.assertIsNotNone(processes[0].poll())

    def test_capture_requires_exact_signer_and_rechecks_identity(self):
        identity={'pod_uid':'pod','pod_rv':'1','deployment_uid':'dep','deployment_rv':'2','container_id':'ctr','restart_count':0}
        secret={'state':'present','uid':'signer','resource_version':'4','keys':['current.pem','next.pem','active-kid']}
        refs={name:{'uid':'uid-'+name,'resource_version':'1'} for name in list(plan.public.PUBLIC)+['voice-app-config']}
        observed={'current_signer':{'configuration':{'signing_dir':'exact','active_kid_ref':'exact','signing_mount':'exact'},
                                    'secret':secret},'space_identity':identity,'objects':refs,'public_der_sha256':{}}
        runner=lambda args:json.dumps({'metadata':{'uid':'ns'}}).encode()
        with patch.object(plan.public,'capture',return_value=copy.deepcopy(observed)), \
             patch.object(plan.common,'kube_workload_snapshot',return_value=lambda:copy.deepcopy(identity)), \
             patch.object(plan.public,'_signer_identity',return_value=secret), \
             patch.object(plan.public,'_meta',side_effect=lambda run,guard,kind,name:refs[name]),patch.object(plan,'_http',return_value=jwks()):
            result=plan.capture('/source','a'*40,lambda _:None,'voice-space-abc','ns',lambda:None,'/dest',runner)
            self.assertEqual(result['scope'],'space-current-http-pod-public-keys-not-https-readiness')
        observed['current_signer']['configuration']['signing_mount']='mismatch'
        with patch.object(plan.public,'capture',return_value=observed),patch.object(plan,'_http') as network:
            with self.assertRaises(plan.PlanError): plan.capture('/source','a'*40,lambda _:None,'voice-space-abc','ns',lambda:None,'/dest',runner)
            network.assert_not_called()

    def test_public_ref_change_during_http_holds(self):
        identity={'pod_uid':'pod','pod_rv':'1','deployment_uid':'dep','deployment_rv':'2','container_id':'ctr','restart_count':0}
        secret={'state':'present','uid':'signer','resource_version':'4','keys':['current.pem','next.pem','active-kid']}
        refs={name:{'uid':'uid-'+name,'resource_version':'1'} for name in list(plan.public.PUBLIC)+['voice-app-config']}
        observed={'current_signer':{'configuration':{'signing_dir':'exact','active_kid_ref':'exact','signing_mount':'exact'},
            'secret':secret},'space_identity':identity,'objects':copy.deepcopy(refs),'public_der_sha256':{}}
        def http(*args): refs['voice-media-ca']['resource_version']='2'; return jwks()
        runner=lambda args:json.dumps({'metadata':{'uid':'ns'}}).encode()
        with patch.object(plan.public,'capture',return_value=observed), \
             patch.object(plan.common,'kube_workload_snapshot',return_value=lambda:copy.deepcopy(identity)), \
             patch.object(plan.public,'_signer_identity',return_value=secret), \
             patch.object(plan.public,'_meta',side_effect=lambda run,guard,kind,name:refs[name]),patch.object(plan,'_http',side_effect=http):
            with self.assertRaises(plan.PlanError): plan.capture('/source','a'*40,lambda _:None,'voice-space-abc','ns',lambda:None,'/dest',runner)


if __name__=='__main__': unittest.main()
