"""Actual Creator issuer/socket/receiver; Kubernetes and TT are synthetic."""
import copy
import hashlib
import os
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import patch

import canonical_application_package as canonical
import media_backup_phase_test as fixture
import media_operation_journal as journals
import realtime_jwks_publisher_phase_test as shared
import reset_credentials as reset
import resource_fencing
import space_creator_runtime_profile_create as create
import space_creator_runtime_profile_issuer as issuer
import space_creator_runtime_profile_issuer_export as exporter
import space_creator_runtime_profile_issuer_packet as packet
import space_p3_issuer_preflight_metadata_root as root_metadata
import space_p3_issuer_preflight_test as preflight_fixture


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated root Linux fixture')
class CreateJointTest(unittest.TestCase):
    def test_real_preflight_root_issuer_export_and_receiver(self):
        case=preflight_fixture.PreflightTest('test_fenced_root_response_and_live_ref_drift')
        def cleanup():
            if not case.doCleanups(): raise RuntimeError('nested preflight cleanup failed')
        self.addCleanup(cleanup); case.setUp()
        with tempfile.TemporaryDirectory(dir='/root',prefix='creator-receiver-joint-') as parent:
            root=Path(parent); archive=root/'archive'; archive.mkdir(mode=0o700); operations=root/'operations'; operations.mkdir(mode=0o700)
            key,cert=reset.ResetMaterial(root).ca('fixture'); (archive/'media-ca.crt').write_bytes(cert.read_bytes())
            (archive/'media-ca.key').write_bytes(key.read_bytes()); (archive/'media-ca.key').chmod(0o600)
            media=cert.read_bytes(); case.ca['der_sha256']=hashlib.sha256(packet._openssl(['x509','-in','{0}','-outform','DER'],[media])).hexdigest()
            case.cut.verify_drained=lambda: {'scope':'synthetic-drain'}
            original=case._completed(); suffix=case.base.context['operation_id'][-32:]; operation_dir=operations/suffix
            journal=journals.Journal(root/'receiver-journal',{'resource_context':case.base.context,'namespace_uid':'ns'}); self.addCleanup(journal.close)
            client=fixture.Client(case.base.lock); client.journal=journal
            fence=resource_fencing.Fence(case.base.context,lambda:copy.deepcopy(case.base.card),shared.View(case.base.ledger,client))
            phase=create.Session(journal,fence,client,case.base.lock,'ns',case.package,case.phase,canonical.ROOT,'token')
            objects={}; errors=[]
            def get(name):return copy.deepcopy(objects.get(name))
            def apply(obj,dry):
                value=copy.deepcopy(obj)
                if not dry:
                    name=value['metadata']['name']; value['metadata'].update(uid='uid-'+name,resourceVersion='1')
                    objects[name]=copy.deepcopy(value); case.creator[name]={'state':'present','uid':'uid-'+name,'resourceVersion':'1','keys':sorted(root_metadata.MEMBERS[name])}
                return value
            def directory(value):
                self.assertEqual(value,suffix); operation_dir.mkdir(mode=0o700); return os.open(operation_dir,os.O_RDONLY|os.O_DIRECTORY)
            sock=operation_dir/'packet.sock'
            with patch.object(create,'ROOT',operations),patch.object(create,'_directory',side_effect=directory), \
                 patch.object(create.source,'verify'),patch.object(create,'_get',side_effect=get),patch.object(create,'_create',side_effect=apply), \
                 patch.object(create.preflight.public,'_value',return_value=media),patch.object(exporter,'ROOT',root), \
                 patch.object(exporter,'_socket_path',return_value=sock),patch.object(exporter.existing,'_pmd_uid',return_value=0), \
                 patch.object(issuer.ca_custody,'ARCHIVE',archive),patch.object(root_metadata,'observe',side_effect=lambda *args:copy.deepcopy(case.creator)):
                ready=phase.prepare(); self.assertEqual(ready['missing'],sorted(root_metadata.MEMBERS))
                def serve():
                    try:phase.serve_preflight()
                    except BaseException as error:errors.append(type(error).__name__)
                worker=threading.Thread(target=serve); worker.start()
                try:
                    with patch.dict(os.environ,{'PATH':'/usr/bin:/bin','LANG':'C','LC_ALL':'C'},clear=True):
                        staged=issuer.stage(case.base.context['operation_id'],case.base.context['commit'],os.getpid(),exporter.existing._start_ticks(os.getpid()))
                finally:worker.join(timeout=30)
                self.assertFalse(worker.is_alive()); self.assertFalse(errors); self.assertEqual(staged['object_names'],ready['missing'])
                def export():
                    try:exporter.export(case.base.context['operation_id'],case.base.context['commit'],'ns',
                                        create.writer_cut.digest(original),os.getpid(),exporter.existing._start_ticks(os.getpid()))
                    except BaseException as error:errors.append(type(error).__name__)
                worker=threading.Thread(target=export); worker.start()
                try:result=phase.finish()
                finally:worker.join(timeout=30)
                self.assertFalse(worker.is_alive()); self.assertFalse(errors)
                self.assertEqual(set(result['objects']),set(root_metadata.MEMBERS))
                self.assertEqual(create.verify_completed(phase,'token'),result)
                self.assertNotIn('PRIVATE KEY',str(result))
