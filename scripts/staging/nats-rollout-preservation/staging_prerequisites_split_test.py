"""The pre-cut prerequisite planner persists its immutable source-bound inputs."""
import json
import unittest
from unittest.mock import patch
import commands
import compiler
import stage_runtime
import staging_prerequisites as parent
import staging_prerequisites_test


class Planned(Exception):pass


class PreparedInputsTests(unittest.TestCase):
    def test_actual_planner_returns_prepared_binding_before_any_kube_write(self):
        prepare_inputs=parent.prepare_inputs
        def prepare(actions,request,approved,execution,workspace):
            calls=[];original=commands.capture.side_effect
            def capture(argv,**kwargs):
                if any(x in argv for x in ('create','patch','apply','delete','scale')) and '--dry-run=server' not in argv and '--dry-run=client' not in argv:
                    calls.append(argv)
                return original(argv,**kwargs)
            with patch.object(commands.capture,'side_effect',capture), \
                 patch.object(compiler.capture,'side_effect',capture), \
                 patch.object(stage_runtime.capture,'side_effect',capture):
                result=prepare_inputs(actions,request,approved,execution,workspace)
            record=json.loads((parent._base(actions.tt_session)/'prepared.json').read_bytes())
            self.assertEqual(result['inputs_sha256'],parent.digest(record))
            self.assertEqual(record['request'],{'mode':request['mode'],'changed_services':request['changed_services'],
                                                'gis_first_install':request.get('gis_first_install') is True})
            self.assertEqual(actions.tt_session.check()['phase'],'prepare')
            self.assertEqual(calls,[])
            raise Planned()
        fixture=staging_prerequisites_test.PrepareCompositionTests()
        with patch.object(parent,'prepare_inputs',side_effect=prepare),self.assertRaises(Planned):
            fixture._cold_prepare()


if __name__=='__main__':unittest.main()
