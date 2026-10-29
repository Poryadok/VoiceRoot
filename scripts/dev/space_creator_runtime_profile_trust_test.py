import unittest
import space_creator_runtime_profile_trust as trust


def listing(*entries):
    return ('Keystore type: PKCS12\nYour keystore contains %d entries\n' % len(entries) + ''.join(
        'Entry type: %s\nSHA256: %s\n' % (kind, ':'.join([byte]*32)) for kind,byte in entries)).encode()


class TrustTest(unittest.TestCase):
    def test_public_roots_and_media_are_required(self):
        before=listing(('trustedCertEntry','AA'))
        after=listing(('trustedCertEntry','AA'),('trustedCertEntry','BB'))
        result=trust.compare(before,after,'bb'*32)
        self.assertEqual(result['root_count'],1)
        self.assertEqual(result['store_count'],2)
        with self.assertRaises(ValueError): trust.compare(before,after.replace(b'PKCS12',b'JKS'),'bb'*32)
        with self.assertRaises(ValueError): trust.compare(before,b'Keystore type: JKS\n'+after,'bb'*32)
        for bad in (listing(('trustedCertEntry','BB')), listing(('PrivateKeyEntry','AA'),('trustedCertEntry','BB')),
                    listing(('trustedCertEntry','AA'))):
            with self.assertRaises(ValueError): trust.compare(before,bad,'bb'*32)

    def test_inconsistent_keytool_output_fails_closed(self):
        value=listing(('trustedCertEntry','AA'))
        for bad in (value.replace(b'contains 1 entries',b'contains 2 entries'),value+b'PrivateKeyEntry\n',value+value):
            with self.assertRaises(ValueError): trust.parse(bad)
