import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import dev_peer_issuer_metadata as subject

class MetadataTest(unittest.TestCase):
    def test_catalog_and_exact_metadata(self):
        self.assertEqual(len(subject.MEMBERS),9)
        self.assertEqual(subject.parse(b'','voice-role-media-client-tls'),{'state':'absent'})
        self.assertEqual(subject.parse(b'uid\n5\n\ntls.crt\ntls.key\n','voice-role-media-client-tls')['keys'],['tls.crt','tls.key'])
        for raw in (b'uid\n5\n\ntls.crt\n',b'uid\n5\n\ntls.crt\ntls.key\ntls.key\n',b'uid\n5\ndeleting\ntls.crt\ntls.key\n'):
            with self.assertRaises(ValueError): subject.parse(raw,'voice-role-media-client-tls')
        with self.assertRaises(ValueError): subject.parse(b'','unknown')

    def test_fixed_extra_reads_and_drift_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            binary=Path(tmp)/'k3s'; config=Path(tmp)/'config'; binary.write_bytes(b'x'); config.write_bytes(b'y')
            def opened(path,limit):
                fd=os.open(path,os.O_RDONLY); info=os.fstat(fd)
                return fd,(info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns)
            base={name:{'state':'absent'} for name in subject.base.MEMBERS}; calls=[]
            def read(fd,conf,args): calls.append(args); return b''
            with patch.object(subject.base,'K3S',binary),patch.object(subject.base,'KUBECONFIG',config), \
                 patch.object(subject.base,'_open',side_effect=opened),patch.object(subject.base,'_digest',return_value=subject.base.K3S_SHA256), \
                 patch.object(subject.base,'observe',return_value=base),patch.object(subject.base,'_read',side_effect=read):
                result=subject.observe('ns',{}, {}, {})
                self.assertEqual(set(result),set(subject.MEMBERS)); self.assertEqual(len(calls),8)
                calls.clear()
                def drift(fd,conf,args):
                    calls.append(args)
                    return b'uid\n5\n\ntls.crt\ntls.key\n' if len(calls)>4 and 'voice-role-media-client-tls' in args else b''
                with patch.object(subject.base,'_read',side_effect=drift),self.assertRaises(ValueError):
                    subject.observe('ns',{}, {}, {})

if __name__=='__main__': unittest.main()
