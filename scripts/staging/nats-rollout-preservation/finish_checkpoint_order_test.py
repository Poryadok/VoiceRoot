import copy
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import transaction


class FinishCheckpointOrderTests(unittest.TestCase):
    def test_restart_state_is_saved_before_publication_and_save_failure_blocks_it(self):
        for fail in (False, True):
            with self.subTest(fail=fail), tempfile.TemporaryDirectory() as folder:
                receipt = {'target': {'mode': 'images-only'}}
                state = {'target': receipt['target'], 'authorization': receipt,
                         'provenance': {}, 'status': 'WAITING'}
                stage = Mock()
                stage.refence.return_value = {'verified': True}
                engine = Mock()
                engine.restart.return_value = {'verified': True}
                saved = []
                def save(_path, row):
                    if fail and len(saved) == 1:
                        saved.append(copy.deepcopy(row))
                        raise OSError('checkpoint unavailable')
                    saved.append(copy.deepcopy(row))
                def publication(row, *, authority_guard):
                    self.assertEqual(saved[-1], row)
                    self.assertEqual(row['restart'], {'verified': True})
                    authority_guard()
                publish = Mock(side_effect=publication)
                with patch.object(transaction, 'checkpoint_journal'), \
                     patch.object(transaction, 'reconstruct', return_value=stage), \
                     patch.object(transaction, 'revalidate_inputs'), \
                     patch.object(transaction, 'context', return_value={'observed': True}), \
                     patch.object(transaction, 'save', side_effect=save), \
                     patch('expired_finish.bind', return_value=engine):
                    if fail:
                        with self.assertRaisesRegex(OSError, 'checkpoint unavailable'):
                            transaction.finish_only(None, Path(folder), state, receipt, {}, {}, publication=publish)
                        publish.assert_not_called()
                        stage.refence.assert_called_once()
                    else:
                        transaction.finish_only(None, Path(folder), state, receipt, {}, {}, publication=publish)
                        publish.assert_called_once()
                        self.assertEqual(state['status'], 'PASS')


if __name__ == '__main__':
    unittest.main()
