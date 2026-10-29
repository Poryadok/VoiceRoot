import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import root_cli
import staging_cohort_preservation as cohort


class PreservedRootCLITest(unittest.TestCase):
    def test_missing_or_foreign_receipt_is_rejected_before_any_files(self):
        args=dict(resource_session=SimpleNamespace(operation='a'*12),nats_authority=None,actor_authority=None,
                  actor_services=[],renderer_authority=None,gis_source_authority=None,gis_source_root=None,
                  gis_source_capture=None,gis_transition=None,gis_binding=None,realtime_bootstrap_receipt=None,
                  prerequisites_capture=None)
        value={'target':{'cohort_preservation_sha256':'f'*64}}
        with tempfile.TemporaryDirectory(dir='/root') as folder,patch.object(root_cli,'ROOT',Path(folder)):
            with self.assertRaisesRegex(root_cli.Blocked,'cohort_preserved_prepare_invalid'):
                root_cli.prepare_preserved_value(value,Path('/source'),{},'a'*12,preservation_ref=None,**args)
            with patch.object(cohort,'load_qualified',return_value={'receipt':'different'}):
                with self.assertRaisesRegex(root_cli.Blocked,'cohort_preserved_prepare_invalid'):
                    root_cli.prepare_preserved_value(value,Path('/source'),{},'a'*12,preservation_ref={'receipt':'claimed'},**args)
            self.assertEqual(list(Path(folder).iterdir()),[])


if __name__=='__main__':unittest.main()
