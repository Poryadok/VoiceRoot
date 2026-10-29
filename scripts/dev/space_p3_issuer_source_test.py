import unittest
from unittest.mock import patch

import space_p3_issuer_source as source


class SourceTest(unittest.TestCase):
    def test_exact_issuer_and_runtime_blobs_and_space_document(self):
        document=b'kind: Deployment\nmetadata:\n  name: voice-space\nspec: {}\n---\nkind: Service\nmetadata:\n  name: other\n'
        def blob(repo,commit,path):
            return document if path=='deploy/staging/services.yaml' else ('data:'+path).encode()
        with patch.object(source.source,'_blob',side_effect=blob), \
             patch.object(source.source,'_disk',side_effect=lambda repo,path:blob(repo,'candidate',path)):
            source.verify('/source','a'*40)
            def changed(repo,path): return b'changed' if path.endswith('space_p3_issuer.py') else blob(repo,'candidate',path)
            with patch.object(source.source,'_disk',side_effect=changed):
                with self.assertRaises(source.SourceError): source.verify('/source','a'*40)
            with self.assertRaises(source.SourceError): source.verify('relative','a'*40)
        with self.assertRaises(ValueError): source._space_document(document+document)


if __name__ == '__main__': unittest.main()
