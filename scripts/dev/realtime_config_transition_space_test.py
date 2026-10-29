import copy
import base64
import json
import socket
import ssl
import subprocess
import sys
import threading
import time
import unittest
from unittest.mock import patch

import realtime_config_transition_space as space
import realtime_p3_create_only_packet_test as fixture


class TunnelTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        fixture.PacketTest.setUpClass(); cls.addClassCleanup(fixture.PacketTest.doClassCleanups)

    def test_real_tls_exact_peer_and_bounded_http(self):
        sock=socket.socket(); sock.bind(('127.0.0.1',0)); sock.listen(1); sock.settimeout(8); port=sock.getsockname()[1]
        ctx=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        ctx.load_cert_chain(str(fixture.PacketTest.temp.name)+'/space.crt',str(fixture.PacketTest.temp.name)+'/space.key')
        errors=[]
        def server():
            try:
                raw,_=sock.accept()
                with ctx.wrap_socket(raw,server_side=True) as tls:
                    tls.recv(4096); body=b'{"keys":[{"kty":"RSA"}]}'
                    tls.sendall(b'HTTP/1.1 200 OK\r\nContent-Length: '+str(len(body)).encode()+b'\r\nConnection: close\r\n\r\n'+body)
            except Exception as error: errors.append(error)
            finally: sock.close()
        thread=threading.Thread(target=server); thread.start()
        actual_popen=subprocess.Popen; processes=[]
        def start(*args,**kwargs):
            if args[0][0] != 'kubectl': return actual_popen(*args,**kwargs)
            proc=actual_popen([sys.executable,'-c',f'import time;print("Forwarding from 127.0.0.1:{port} -> 8443",flush=True);time.sleep(30)'],**kwargs)
            processes.append(proc); return proc
        try:
            with patch.object(space.subprocess,'Popen',side_effect=start):
                result=space._tunnel('space-0',fixture.PacketTest.media,fixture.PacketTest.space,lambda:None)
            self.assertEqual(result,b'{"keys":[{"kty":"RSA"}]}')
        finally:
            thread.join(timeout=5)
        self.assertFalse(thread.is_alive()); self.assertEqual(errors,[])
        self.assertIsNotNone(processes[0].poll())

    def test_invalid_pod_never_launches(self):
        with patch.object(space.subprocess,'Popen') as process:
            with self.assertRaises(ValueError): space._tunnel('../secret',b'',b'',lambda:None)
        process.assert_not_called()

    def test_drip_feed_exceeds_total_deadline_and_reaps_tunnel(self):
        sock=socket.socket(); sock.bind(('127.0.0.1',0)); sock.listen(1); sock.settimeout(5); port=sock.getsockname()[1]
        ctx=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        ctx.load_cert_chain(str(fixture.PacketTest.temp.name)+'/space.crt',str(fixture.PacketTest.temp.name)+'/space.key')
        def server():
            try:
                raw,_=sock.accept()
                with ctx.wrap_socket(raw,server_side=True) as tls:
                    tls.recv(4096)
                    for byte in b'HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}':
                        tls.sendall(bytes([byte])); time.sleep(.05)
            except OSError: pass
            finally: sock.close()
        thread=threading.Thread(target=server); thread.start()
        actual_popen=subprocess.Popen; processes=[]
        def start(*args,**kwargs):
            if args[0][0] != 'kubectl': return actual_popen(*args,**kwargs)
            proc=actual_popen([sys.executable,'-c',f'import time;print("Forwarding from 127.0.0.1:{port} -> 8443",flush=True);time.sleep(30)'],**kwargs)
            processes.append(proc); return proc
        try:
            with patch.object(space,'TUNNEL_DEADLINE',.5), patch.object(space.subprocess,'Popen',side_effect=start):
                with self.assertRaises(ValueError): space._tunnel('space-0',fixture.PacketTest.media,fixture.PacketTest.space,lambda:None)
        finally: thread.join(timeout=5)
        self.assertFalse(thread.is_alive()); self.assertIsNotNone(processes[0].poll())

    def test_capture_rejects_symmetric_key_before_receipt(self):
        identity={'pod_uid':'pod-uid','pod_rv':'2','deployment_uid':'dep','deployment_rv':'3','container_id':'ctr','restart_count':0}
        refs={name:{'uid':'uid-'+name,'resource_version':'1'} for name in space.public.PUBLIC}
        result={'space_identity':identity,'space_service_spec':{'uid':'svc','resource_version':'1'},'objects':refs}
        pod={'metadata':{'uid':'pod-uid','resourceVersion':'2'},'status':{'podIP':'10.0.0.2'}}
        endpoints={'metadata':{'name':'voice-space','namespace':'voice-dev','uid':'ep','resourceVersion':'1'},
            'subsets':[{'addresses':[{'ip':'10.0.0.2','targetRef':{'kind':'Pod','name':'space-0','uid':'pod-uid'}}],
                        'ports':[{'port':8443}]}]}
        def get(run,guard,kind,name): return copy.deepcopy(pod if kind=='pod' else endpoints if kind=='endpoints' else {})
        runner=lambda args:json.dumps({'metadata':{'uid':'ns'}}).encode()
        with patch.object(space.public,'capture',return_value=result), \
             patch.object(space.common,'kube_workload_snapshot',return_value=lambda:copy.deepcopy(identity)), \
             patch.object(space.public,'_get',side_effect=get),patch.object(space.public,'_mounted'), \
             patch.object(space.public,'_service',return_value=result['space_service_spec']), \
             patch.object(space.public,'_meta',side_effect=lambda run,guard,kind,name:refs[name]), \
             patch.object(space.public,'_value',return_value=b'public-cert'), \
             patch.object(space,'_tunnel',return_value=b'{"keys":[{"kty":"oct","k":"private-sentinel"}]}') as tunnel:
            with self.assertRaises(space.SpaceObservationError) as error:
                space.capture('/source','a'*40,lambda _:None,'space-0','ns',lambda:None,'/dest',runner)
            tunnel.assert_called_once()
            def jwk(kid,n): return {'kty':'RSA','kid':kid,'use':'sig','alg':'RS256',
                'n':base64.urlsafe_b64encode(n).decode().rstrip('='),'e':'AQAB'}
            tunnel.return_value=json.dumps({'keys':[jwk('current.v1',b'\x80'+b'\x01'*255),
                                                     jwk('next',b'\x80'+b'\x02'*255)]}).encode()
            good=space.capture('/source','a'*40,lambda _:None,'space-0','ns',lambda:None,'/dest',runner)
            self.assertEqual(good['scope'],'space-pod-https-through-kubernetes-tunnel')
        self.assertNotIn('private-sentinel',str(error.exception))


if __name__=='__main__': unittest.main()
