"""Git object and disk drift are checked without mocking the source reader."""
import subprocess
import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch

import space_p3_issuer_preflight_source as subject


class SourceTest(unittest.TestCase):
    def test_fixed_root_source_rejects_candidate_drift(self):
        with tempfile.TemporaryDirectory(prefix='preflight-source-') as tmp:
            root=Path(tmp); subprocess.run(['git','init','-q',str(root)],check=True)
            subprocess.run(['git','-C',str(root),'config','user.email','fixture@example.invalid'],check=True)
            subprocess.run(['git','-C',str(root),'config','user.name','fixture'],check=True)
            files=list(subject.PATHS)+['scripts/dev/space_p3_issuer_bundle.py','scripts/dev/space_p3_issuer_preflight.py',
                                       'scripts/dev/space_p3_issuer_preflight_source.py']
            for name in files:
                path=root/name; path.parent.mkdir(parents=True,exist_ok=True); path.write_text('source '+name+'\n')
            subprocess.run(['git','-C',str(root),'add','scripts'],check=True)
            subprocess.run(['git','-C',str(root),'commit','-qm','fixture'],check=True)
            old=subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD'],text=True).strip()
            with patch.object(subject,'REVIEWED_PREFLIGHT_SHA',old),patch.object(subject.issuer_source,'verify'):
                subject.verify(str(root),old)
                (root/subject.PATHS[0]).write_text('different root helper\n')
                subprocess.run(['git','-C',str(root),'add','scripts'],check=True)
                subprocess.run(['git','-C',str(root),'commit','-qm','drift'],check=True)
                new=subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD'],text=True).strip()
                with self.assertRaises(ValueError): subject.verify(str(root),new)


if __name__=='__main__': unittest.main()
