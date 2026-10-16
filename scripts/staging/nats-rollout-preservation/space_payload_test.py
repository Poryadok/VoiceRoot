"""Cipher membership checks use private synthetic files, never a live DB."""
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import encrypted_cut
import space_migration

class Tests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.base=Path(self.temp.name);raw=b'PGDMP synthetic retained snapshot'
        before={'database':'space_db','version':15,'dirty':False,'allow_guests_true':2,
            'column_default':'true','column_type':'boolean'}
        self.receipt={'operation':'a'*12,'source_sha':'c'*40,'migration_plan_sha256':'e'*64,
            'requirement':space_migration.requirement(before,24),'exporter_closed':True,
            'backup':{'schema':'voice-space-backup-v1','before':before,'snapshot':'00000003-00000008-1',
                'dump_sha256':hashlib.sha256(raw).hexdigest(),'dump_bytes':len(raw),
                'restored':True,'offnode_verified':False}}
        (self.base/'space-before.dump').write_bytes(raw);self.manifest()
        # Production FD custody is covered by the existing root crypto fixture.
        # This seam substitutes only owner checks for the unprivileged unit run.
        def regular(path,private=False):
            fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
            return fd,os.fstat(fd)
        self.p=patch.object(encrypted_cut,'regular',side_effect=regular);self.p.start();self.addCleanup(self.p.stop)
    def manifest(self):
        (self.base/'space-before-manifest.json').write_text(json.dumps(self.receipt))
    def test_restored_closed_dump_binds_both_exact_cipher_members(self):
        names,hashes=encrypted_cut.space_members(self.base,self.receipt)
        self.assertEqual(names,('space-before.dump','space-before-manifest.json'))
        self.assertEqual(hashes,{n:hashlib.sha256((self.base/n).read_bytes()).hexdigest() for n in names})
        self.assertFalse(self.receipt['backup']['offnode_verified'])
    def test_open_exporter_or_unrestored_dump_cannot_be_exported(self):
        for path in ('exporter_closed','restored'):
            row=copy.deepcopy(self.receipt)
            if path=='restored':row['backup'][path]=False
            else:row[path]=False
            with self.assertRaises((encrypted_cut.CryptoError,space_migration.SpaceError)):
                encrypted_cut.space_members(self.base,row)
    def test_dump_or_manifest_drift_rejected_before_encryption(self):
        original=(self.base/'space-before.dump').read_bytes()
        (self.base/'space-before.dump').write_bytes(original+b'x')
        with self.assertRaises(encrypted_cut.CryptoError):encrypted_cut.space_members(self.base,self.receipt)
        (self.base/'space-before.dump').write_bytes(original)
        row=copy.deepcopy(self.receipt);row['backup']['before']['allow_guests_true']=9
        (self.base/'space-before-manifest.json').write_text(json.dumps(row))
        with self.assertRaises(encrypted_cut.CryptoError):encrypted_cut.space_members(self.base,self.receipt)

    def test_only_matching_full_cipher_readback_promotes_offnode_evidence(self):
        _,hashes=encrypted_cut.space_members(self.base,self.receipt)
        binding={'operation':'a'*12,'challenge':'b'*32,'run_id':7,'head_sha':'c'*40,
            'cipher_sha256':'d'*64,'cipher_bytes':100}
        state={'operation':'a'*12,'space_backup':self.receipt,'cipher_space_members':hashes,
            'target':{'tag':'c'*40,'migration_sha256':'e'*64},
            'cipher_binding':binding,'custody':{**binding,'schema':'voice-nats-custody-v1',
                'destination':'github:Poryadok/VoiceRoot','verified':True}}
        self.assertTrue(encrypted_cut.authorize_space(self.base,state)['offnode_verified'])
        self.assertFalse(self.receipt['backup']['offnode_verified'])
        for key in ('verified','cipher_sha256','operation','destination'):
            bad=copy.deepcopy(state);bad['custody'][key]=False
            with self.assertRaises(encrypted_cut.CryptoError):encrypted_cut.authorize_space(self.base,bad)
        bad=copy.deepcopy(state);bad['cipher_space_members']['space-before.dump']='0'*64
        with self.assertRaises(encrypted_cut.CryptoError):encrypted_cut.authorize_space(self.base,bad)
        for key in ('tag','migration_sha256'):
            bad=copy.deepcopy(state);bad['target'][key]='0'*64
            with self.assertRaisesRegex(encrypted_cut.CryptoError,'source_binding'):encrypted_cut.authorize_space(self.base,bad)

if __name__=='__main__':unittest.main()
