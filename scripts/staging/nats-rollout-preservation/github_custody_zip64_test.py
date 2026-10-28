import io
import struct
import unittest
from unittest.mock import patch
import zipfile

import github_custody as custody
import github_custody_test as fixtures


def archive(zip64_size=None, zip64_count=1, zip64_offset=None):
    output = io.BytesIO()
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_STORED) as zipped:
        zipped.writestr('rollout-backup.cms', b'private-ciphertext')
    data = output.getvalue()
    end = len(data) - 22
    _, _, _, _, _, size, offset, _ = struct.unpack('<4s4H2LH', data[end:])
    if zip64_size is None:
        return data
    record = struct.pack('<4sQ2H2L4Q', b'PK\x06\x06', 44, 45, 45, 0, 0,
                         zip64_count, zip64_count, zip64_size,
                         offset if zip64_offset is None else zip64_offset)
    locator = struct.pack('<4sLQL', b'PK\x06\x07', 0, end, 1)
    return data[:end] + record + locator + data[end:]


class Zip64DirectoryTest(unittest.TestCase):
    def test_classic_and_bounded_zip64(self):
        plain = archive()
        directory_size = struct.unpack('<4s4H2LH', plain[-22:])[5]
        for data in (plain, archive(directory_size)):
            with self.subTest(length=len(data)):
                custody._bound_zip_directory(io.BytesIO(data), len(data))

    def test_zip64_override_rejected_before_parser(self):
        for data in (archive(65536), archive(1, zip64_count=2), archive(1, zip64_offset=0)):
            with self.subTest(length=len(data)):
                with self.assertRaises(custody.CustodyError):
                    custody._bound_zip_directory(io.BytesIO(data), len(data))

    def test_readback_rejects_override_before_zipfile_constructor(self):
        fixture = fixtures.CustodyTests('test_valid_full_byte_readback_and_no_redirect_bearer')
        self.addCleanup(lambda: self.assertTrue(fixture.doCleanups()))
        fixture.setUp()
        fixture.zip = archive(65536)
        fixture.metadata['size_in_bytes'] = len(fixture.zip)
        with patch.object(custody.zipfile, 'ZipFile') as parser:
            with self.assertRaises(custody.CustodyError):
                fixture.verify()
            parser.assert_not_called()

    def test_readback_accepts_bounded_zip64(self):
        fixture = fixtures.CustodyTests('test_valid_full_byte_readback_and_no_redirect_bearer')
        self.addCleanup(lambda: self.assertTrue(fixture.doCleanups()))
        fixture.setUp()
        plain = archive()
        directory_size = struct.unpack('<4s4H2LH', plain[-22:])[5]
        fixture.zip = archive(directory_size)
        fixture.metadata['size_in_bytes'] = len(fixture.zip)
        self.assertTrue(fixture.verify()['verified'])


if __name__ == '__main__':
    unittest.main()
