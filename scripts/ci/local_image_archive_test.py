"""Reject ambiguous or corrupt private-daemon image archives."""
import hashlib
import io
import json
import tarfile
import unittest

import local_image_archive


def fixture(*, tag='alpine:3.17', second=False, bad_layer=False, duplicate=False, unsafe=False,
            duplicate_json=False, foreign_repository=False, trailing=False, extra_file=False):
    layer = b'layer bytes'
    diff = hashlib.sha256(layer).hexdigest()
    config = json.dumps({'os': 'linux', 'architecture': 'amd64',
                         'rootfs': {'type': 'layers', 'diff_ids': ['sha256:' + diff]}},
                        separators=(',', ':')).encode()
    image_id = 'sha256:' + hashlib.sha256(config).hexdigest()
    config_name = image_id.removeprefix('sha256:') + '.json'
    layer_name = 'a' * 64 + '/layer.tar'
    manifest = [{'Config': config_name, 'RepoTags': [tag], 'Layers': [layer_name]}]
    if second:
        manifest.append(dict(manifest[0]))
    data = io.BytesIO()
    with tarfile.open(fileobj=data, mode='w') as tar:
        def add(name, value):
            entry = tarfile.TarInfo(name)
            entry.size = len(value)
            tar.addfile(entry, io.BytesIO(value))
        add(config_name, config)
        add(layer_name, b'wrong bytes' if bad_layer else layer)
        encoded = json.dumps(manifest).encode()
        if duplicate_json:
            encoded = encoded.replace(b'"Config":', b'"Config":"foreign.json","Config":', 1)
        add('manifest.json', encoded)
        if duplicate:
            add('manifest.json', json.dumps(manifest).encode())
        if unsafe:
            add('../outside', b'bad')
        if foreign_repository:
            add('repositories', json.dumps({'alpine': {'3.17': 'a' * 64, 'foreign': 'a' * 64}}).encode())
        if extra_file:
            add('unrecognized', b'extra')
    raw = data.getvalue()
    if trailing:
        raw += b'nonzero trailing bytes'
    return raw, hashlib.sha256(raw).hexdigest(), image_id


def oci_fixture(*, bad_tag=False, bad_size=False, bad_media=False, duplicate_config=False,
                extra_blob=False, bad_layer_sources=False, pax=False, untagged=False,
                multiple=False, untagged_annotation=False, missing_repotags=False,
                multiple_index=False, untagged_repositories=False, missing_index=False):
    layer = b'layer bytes'
    layer_id = hashlib.sha256(layer).hexdigest()
    config = json.dumps({'os': 'linux', 'architecture': 'amd64',
                         'rootfs': {'type': 'layers', 'diff_ids': ['sha256:' + layer_id]}},
                        separators=(',', ':')).encode()
    if duplicate_config:
        config = config.replace(b'"os":', b'"os":"windows","os":', 1)
    config_id = hashlib.sha256(config).hexdigest()
    config_name, layer_name = 'blobs/sha256/' + config_id, 'blobs/sha256/' + layer_id
    layer_descriptor = {'mediaType': 'application/vnd.oci.image.layer.v1.tar',
                        'digest': 'sha256:' + layer_id, 'size': len(layer)}
    oci = {'schemaVersion': 2, 'mediaType': 'application/vnd.oci.image.manifest.v1+json',
           'config': {'mediaType': 'application/vnd.oci.image.config.v1+json',
                      'digest': 'sha256:' + config_id, 'size': len(config)},
           'layers': [dict(layer_descriptor)]}
    if bad_media:
        oci['layers'][0]['mediaType'] = 'application/octet-stream'
    oci_raw = json.dumps(oci, separators=(',', ':')).encode()
    oci_id = hashlib.sha256(oci_raw).hexdigest()
    index = {'schemaVersion': 2, 'mediaType': 'application/vnd.oci.image.index.v1+json',
             'manifests': [{'mediaType': 'application/vnd.oci.image.manifest.v1+json',
                            'digest': 'sha256:' + oci_id, 'size': len(oci_raw) + (1 if bad_size else 0),
                            'annotations': {'io.containerd.image.name': 'docker.io/library/' + ('foreign' if bad_tag else 'alpine') + ':3.17',
                                            'org.opencontainers.image.ref.name': '3.17'}}]}
    legacy = [{'Config': config_name, 'RepoTags': ['alpine:3.17'], 'Layers': [layer_name],
               'LayerSources': {'sha256:' + layer_id: layer_descriptor}}]
    if untagged:
        legacy[0]['RepoTags'] = None
        del index['manifests'][0]['annotations']
        if untagged_annotation:
            index['manifests'][0]['annotations'] = {'org.opencontainers.image.ref.name': 'unexpected'}
    if multiple:
        legacy.append(dict(legacy[0]))
    if missing_repotags:
        del legacy[0]['RepoTags']
    if multiple_index:
        index['manifests'].append(dict(index['manifests'][0]))
    if bad_layer_sources:
        legacy[0]['LayerSources']['sha256:' + layer_id] = dict(layer_descriptor, size=len(layer) + 1)
    entries = {config_name: config, layer_name: layer, 'blobs/sha256/' + oci_id: oci_raw,
               'manifest.json': json.dumps(legacy).encode(), 'index.json': json.dumps(index).encode(),
               'oci-layout': b'{"imageLayoutVersion":"1.0.0"}',
               'repositories': json.dumps({'alpine': {'3.17': layer_id}}).encode()}
    if untagged and not untagged_repositories:
        del entries['repositories']
    if missing_index:
        del entries['index.json']
    if extra_blob:
        extra = b'extra data'
        entries['blobs/sha256/' + hashlib.sha256(extra).hexdigest()] = extra
    data = io.BytesIO()
    with tarfile.open(fileobj=data, mode='w') as tar:
        for name, value in entries.items():
            item = tarfile.TarInfo(name)
            item.size = len(value)
            if pax and name == config_name:
                item.pax_headers = {'comment': 'not copied into canonical output'}
            tar.addfile(item, io.BytesIO(value))
    raw = data.getvalue()
    return raw, hashlib.sha256(raw).hexdigest(), 'sha256:' + config_id


class ImageArchiveTests(unittest.TestCase):
    def test_untagged_oci_requires_one_unannotated_image_and_exact_digests(self):
        raw, digest, image_id = oci_fixture(untagged=True, extra_blob=True)
        output = io.BytesIO()
        result = local_image_archive.sanitize_archive(io.BytesIO(raw), output, digest, image_id, None)
        self.assertIsNone(result['tag'])
        self.assertEqual(local_image_archive.verify_archive(io.BytesIO(output.getvalue()),
            result['archive_sha256'], image_id, None)['image_id'], image_id)
        for options in ({'untagged': True, 'multiple': True},
                        {'untagged': True, 'untagged_annotation': True},
                        {'untagged': True, 'missing_repotags': True},
                        {'untagged': True, 'multiple_index': True},
                        {'untagged': True, 'untagged_repositories': True},
                        {'untagged': True, 'missing_index': True},
                        {'untagged': True, 'bad_layer_sources': True},
                        {'untagged': True, 'bad_size': True}):
            raw, digest, image_id = oci_fixture(**options)
            with self.subTest(options=options), self.assertRaises(ValueError):
                local_image_archive.sanitize_archive(io.BytesIO(raw), io.BytesIO(), digest, image_id, None)
        raw, digest, image_id = oci_fixture(untagged=True)
        for bad_digest, bad_id in (('0'*64, image_id), (digest, 'sha256:'+'f'*64)):
            with self.assertRaises(ValueError):
                local_image_archive.sanitize_archive(io.BytesIO(raw), io.BytesIO(), bad_digest, bad_id, None)
        with self.assertRaises(ValueError):
            local_image_archive.sanitize_archive(io.BytesIO(raw), io.BytesIO(), digest, image_id, 'alpine:3.17')
        tagged, tagged_digest, tagged_id = oci_fixture()
        with self.assertRaises(ValueError):
            local_image_archive.sanitize_archive(io.BytesIO(tagged), io.BytesIO(), tagged_digest, tagged_id, None)

    def test_sanitizer_removes_unreferenced_blobs_and_tar_extensions(self):
        raw, digest, image_id = oci_fixture(extra_blob=True, pax=True)
        output = io.BytesIO()
        result = local_image_archive.sanitize_archive(io.BytesIO(raw), output, digest, image_id, 'alpine:3.17')
        clean = output.getvalue()
        self.assertEqual(hashlib.sha256(clean).hexdigest(), result['archive_sha256'])
        self.assertEqual(local_image_archive.verify_archive(io.BytesIO(clean), result['archive_sha256'],
                                                            image_id, 'alpine:3.17')['image_id'], image_id)
        with tarfile.open(fileobj=io.BytesIO(clean), mode='r:') as archive:
            members = archive.getmembers()
            self.assertEqual(len(members), 7)
            self.assertTrue(all(not item.pax_headers for item in members))

    def test_sanitizer_rejects_input_changed_between_passes(self):
        raw, digest, image_id = oci_fixture()
        altered = bytearray(raw)
        altered[600] ^= 1
        class ChangingStream(io.BytesIO):
            def __init__(self):
                super().__init__(raw)
                self.rewinds = 0
            def seek(self, offset, whence=0):
                if offset == 0 and whence == 0:
                    self.rewinds += 1
                    if self.rewinds == 2:
                        super().seek(0)
                        super().write(altered)
                return super().seek(offset, whence)
        with self.assertRaises(ValueError):
            local_image_archive.sanitize_archive(ChangingStream(), io.BytesIO(), digest, image_id, 'alpine:3.17')
    def test_oci_index_and_descriptors_match_the_legacy_manifest(self):
        raw, digest, image_id = oci_fixture()
        self.assertEqual(local_image_archive.verify_archive(io.BytesIO(raw), digest, image_id, 'alpine:3.17')['image_id'], image_id)
        for options in ({'bad_tag': True}, {'bad_size': True}, {'bad_media': True},
                        {'duplicate_config': True}, {'bad_layer_sources': True}):
            with self.subTest(options=options):
                raw, digest, image_id = oci_fixture(**options)
                with self.assertRaises(ValueError):
                    local_image_archive.verify_archive(io.BytesIO(raw), digest, image_id, 'alpine:3.17')
    def test_accepts_one_expected_image_and_hashes_complete_stream(self):
        raw, digest, image_id = fixture()
        result = local_image_archive.verify_archive(io.BytesIO(raw), digest, image_id, 'alpine:3.17')
        self.assertEqual(result['image_id'], image_id)
        self.assertEqual(result['archive_sha256'], digest)
        self.assertEqual(result['tag'], 'alpine:3.17')

    def test_rejects_ambiguous_corrupt_or_mismatched_archive(self):
        for options in ({'second': True}, {'bad_layer': True}, {'duplicate': True},
                        {'unsafe': True}, {'tag': 'foreign:latest'}, {'duplicate_json': True},
                        {'foreign_repository': True}, {'trailing': True}, {'extra_file': True}):
            with self.subTest(options=options):
                raw, digest, image_id = fixture(**options)
                with self.assertRaises(ValueError):
                    local_image_archive.verify_archive(io.BytesIO(raw), digest, image_id, 'alpine:3.17')
        raw, digest, image_id = fixture()
        for wrong_digest, wrong_id in [('0' * 64, image_id), (digest, 'sha256:' + 'f' * 64)]:
            with self.assertRaises(ValueError):
                local_image_archive.verify_archive(io.BytesIO(raw), wrong_digest, wrong_id, 'alpine:3.17')


if __name__ == '__main__':
    unittest.main()
