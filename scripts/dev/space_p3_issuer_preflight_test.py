import copy
import hashlib
import json
import os
from pathlib import Path
import socket
import threading
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixture
import media_operation_journal as journal
import space_p3_issuer_preflight as subject
import space_p3_issuer_preflight_root as root_helper


class Package:
    def verify_current(self,token):
        if token!='token': raise ValueError()


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated root Linux fixture')
class PreflightTest(unittest.TestCase):
    def setUp(self):
        self.base=fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        self.client=fixture.Client(self.base.lock)
        intent={'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns',
                'space_issuer_preflight':{'source_root':'/source','source_sha':self.base.context['commit']}}
        self.journal=journal.Journal(Path(self.base.temp.name)/'preflight-journal',intent); self.addCleanup(self.journal.close)
        self.client.journal=self.journal; self.base.ledger.client=self.client
        self.cut=SimpleNamespace(fence=self.base.fence,lock=self.base.lock,namespace_uid='ns',_guard=lambda:None,journal=self.base.journal)
        self.package=Package(); self.package.fence=self.base.fence; self.package.lock=self.base.lock; self.package.namespace_uid='ns'
        self.ca={'uid':'ca','resourceVersion':'1','der_sha256':'a'*64}
        self.creator={name:{'state':'absent'} for name in subject.metadata.MEMBERS}
        self.directory=Path(self.base.temp.name)/'socket'; self.directory.mkdir(mode=0o700)
        suffix=self.base.context['operation_id'][-32:]; (self.directory/suffix).mkdir(mode=0o700)
        self.patches=[patch.object(subject.frontend_runtime_executor,'KubectlTransport',fixture.Client),
            patch.object(subject.full_cohort_package_phase,'PackagePhase',Package),patch.object(subject.writer_cut,'WriterCut',SimpleNamespace),
            patch.object(subject.media_operation_guard,'verify'),patch.object(subject.source,'verify'),patch.object(subject,'SOCKET_ROOT',self.directory),
            patch.object(subject.IssuerPreflightPhase,'_ca',side_effect=lambda:copy.deepcopy(self.ca)),
            patch.object(subject.IssuerPreflightPhase,'_creator',side_effect=lambda:copy.deepcopy(self.creator))]
        for p in self.patches: p.start(); self.addCleanup(p.stop)
        self.phase=subject.IssuerPreflightPhase(self.journal,self.base.fence,self.client,self.base.lock,'ns',self.package,self.cut,'/source','token')
        self.addCleanup(self.phase.close)

    def cleanup(self,case):
        if not case.doCleanups(): raise RuntimeError('nested cleanup failed')

    def deliver(self,record,foreign=False):
        response={'schema':'voice-space-media-ca-custody-v1','operation_id':record['operation_id'],
                  'source_sha':record['source_sha'],'namespace_uid':'ns','nonce':record['nonce'],
                  'context_sha256':subject.writer_cut.digest(self.base.context),
                  'lock':{k:self.base.lock['metadata'][k] for k in ('uid','resourceVersion')},
                  'ca':copy.deepcopy(record['ca']),'creator_secrets':copy.deepcopy(record['creator_secrets']),
                  'scope':'root-custody-available-not-issued'}
        if foreign: response['nonce']='f'*32
        channel=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
        with channel:
            channel.connect(str(self.directory/self.phase.suffix/'custody.sock')); channel.recv(65536); channel.send(json.dumps(response).encode())
        return response

    def test_fenced_root_response_and_live_ref_drift(self):
        record=self.phase.prepare(); response=[]
        thread=threading.Thread(target=lambda:response.append(self.deliver(record))); thread.start()
        try: result=self.phase.finish()
        finally: thread.join(timeout=10)
        self.assertFalse(thread.is_alive()); self.assertEqual(result,response[0]); self.assertEqual(subject.verify_current(self.phase,'token'),response[0])
        self.creator[next(iter(self.creator))]={'state':'present','uid':'foreign','resourceVersion':'2','keys':[]}
        with self.assertRaises(subject.PreflightError): subject.verify_current(self.phase,'token')

    def test_foreign_nonce_is_uncertain_without_replay(self):
        record=self.phase.prepare(); thread=threading.Thread(target=lambda:self.deliver(record,True)); thread.start()
        try:
            with self.assertRaises(subject.PreflightError): self.phase.finish()
        finally: thread.join(timeout=10)
        self.assertEqual(self.base.ledger.row['status'],'uncertain')
        with self.assertRaises(subject.PreflightError): self.phase.prepare()

    def test_actual_root_keycheck_handoff_to_protected_phase(self):
        scratch=tempfile.TemporaryDirectory(dir='/root',prefix='space-preflight-joint-'); self.addCleanup(scratch.cleanup)
        archive=Path(scratch.name)/'ca'; archive.mkdir(mode=0o700)
        subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','2','-subj','/CN=fixture',
            '-addext','basicConstraints=critical,CA:TRUE','-keyout',str(archive/'media-ca.key'),
            '-out',str(archive/'media-ca.crt')],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        (archive/'media-ca.key').chmod(0o600)
        der=subprocess.run(['openssl','x509','-in',str(archive/'media-ca.crt'),'-outform','DER'],check=True,capture_output=True).stdout
        self.ca['der_sha256']=hashlib.sha256(der).hexdigest(); record=self.phase.prepare(); errors=[]
        def send():
            try:
                root_helper.check(record['operation_id'],record['pid'],int(record['start_ticks']))
            except Exception as error: errors.append(error)
        with patch.object(root_helper.issuer,'ARCHIVE',archive),patch.object(root_helper,'_socket',return_value=self.directory/self.phase.suffix/'custody.sock'), \
             patch.object(root_helper.root_export,'_pmd_uid',return_value=0),patch.object(root_helper.metadata,'observe',return_value=copy.deepcopy(self.creator)):
            thread=threading.Thread(target=send); thread.start()
            try: result=self.phase.finish()
            finally: thread.join(timeout=10)
            self.assertFalse(thread.is_alive()); self.assertEqual(errors,[])
        self.assertEqual(result['ca'],self.ca); self.assertEqual(subject.verify_current(self.phase,'token'),result)


if __name__=='__main__': unittest.main()
