import json
import unittest
from unittest.mock import patch

import bootstrap
import environment as dev


class FreshNamespaceTests(unittest.TestCase):
    def test_namespace_creation_is_exclusive_and_conflict_is_not_adopted(self):
        namespace = dev.foundation('pmdebook')[0]
        with patch.object(bootstrap, 'run', side_effect=RuntimeError('AlreadyExists')) as run:
            with self.assertRaises(RuntimeError):
                bootstrap.create_namespace(namespace)
        args, kwargs = run.call_args
        self.assertEqual(args, ('kubectl', 'create', '-f', '-'))
        self.assertEqual(json.loads(kwargs['body']), namespace)
        self.assertEqual(run.call_count, 1)

    def test_foreign_namespace_is_rejected_before_write(self):
        with patch.object(bootstrap, 'run') as run:
            with self.assertRaises(ValueError):
                bootstrap.create_namespace({'metadata': {'name': 'voice-staging'}})
        run.assert_not_called()
