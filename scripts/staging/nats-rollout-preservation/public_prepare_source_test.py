"""Public source/receipt integration with real Session and producer entry.

Only HTTP, tracker HTTP and the first Kubernetes command are substituted.
This is source-entry failure isolation, not a complete cut/qualification proof.
The mandatory fixture binaries are provisioned by the Linux test harness.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import tarfile

import bridge
import bridge_root
import bundle
import commands
import guard
import resource_claim
import resource_claim_test as tracker
import source_authority
import source_authority_test as source_fixture
import staging_prerequisites
import stage_runtime


class PublicSourceReceiptTests(unittest.TestCase):
    def test_original_four_cases_and_postwrite_loss_use_actual_public_full_and_app_entry(self):
        for mode in ("full","app-only"):
            for fault in ("dispatcher","source","session","none","postwrite-session"):
                with self.subTest(mode=mode,fault=fault),tempfile.TemporaryDirectory(dir="/root") as directory:
                    self.exercise(Path(directory),mode,fault)

    def exercise(self,root,mode,fault):
        binaries=Path("/fixture/qualified-binaries")
        for name,sha in staging_prerequisites.P2_BINARY_PINS.items():
            path=binaries/Path(name).name
            self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(),sha)
        http=source_fixture.SourceTests("test_valid_fixed_ci_actual_canonical_tree_and_images")
        http.setUp();self.addCleanup(http.doCleanups)
        http.run.update(status="in_progress",conclusion=None)
        installed=root/"installed";installed.mkdir(mode=0o700)
        (installed/"sources").mkdir(mode=0o700)
        code=installed/"code";code.mkdir(mode=0o750)
        bundle.build(binaries/"kernel",root/"package",binaries/"bootstrap-renewer",binaries/"story-current4-probe",
                     story_target8=binaries/"story-restored8-probe")
        with tarfile.open(root/"package"/"rollout-bundle.tar") as archive:
            for member in archive.getmembers():
                path=code/member.name
                path.parent.mkdir(mode=0o750,parents=True,exist_ok=True)
                path.write_bytes(archive.extractfile(member).read());path.chmod(0o440)
        actions=bridge_root.Actions(code)
        for name in actions.binding:
            if name not in staging_prerequisites.P2_BINARY_PINS:
                http.files["scripts/staging/"+name]=(code/name).read_bytes()
        context=tracker.context();context["commit"]=source_fixture.SHA
        context["images"]={"voice-user/user":source_authority.REGISTRY+"/user@"+http.manifest_digest}
        current=tracker.card(context)
        request={"action":"prepare","nonce":"a"*64,"source_sha":context["commit"],"run_id":123,
            "mode":mode,"changed_services":[],"token":"private-token","resource_context":context}
        requests=root/"requests";requests.mkdir(mode=0o700)
        receipt=installed/"source-authority"/("a"*12+".json")
        calls=[];source_calls=[]
        def network(url,headers,limit,deadline,target=None):
            source_calls.append(url)
            if fault=="dispatcher" and "/actions/runs/123" in url:
                raise RuntimeError("fixture_dispatcher_transport")
            if fault=="source" and "/tarball/" in url:
                raise OSError("fixture_source_transport")
            result=http.fetch(url,headers,limit,deadline,target)
            if fault=="session" and url.endswith("/git/ref/heads/master") and http.master_calls>=2:
                current["claim"]="revoked"
            return result
        def tracker_http(_check,suffix,body=None):
            card=copy.deepcopy(current)
            if fault=="postwrite-session" and receipt.exists():card["claim"]="revoked"
            return {"valid":True,"resource":card}
        def terminal(argv,**kwargs):
            commands.authority_guard(argv)
            calls.append(list(argv))
            raise RuntimeError("fixture_producer_kubernetes_boundary")
        with patch.object(guard,"ROOT",root),patch.object(bridge_root,"INSTALLED",installed),\
             patch.object(source_authority,"_fetch",side_effect=network),\
             patch.object(resource_claim.Check,"request",autospec=True,side_effect=tracker_http),\
             patch.object(commands,"capture",side_effect=terminal),\
             patch.object(stage_runtime,"capture",side_effect=terminal):
            with self.assertRaises(Exception) as failure:
                with actions.resource_scope(request) as session:
                    bridge.dispatch(requests,request,actions.execute,actions.recover,
                        execute_observed=actions.execute_observed,admit=session.admit_request,
                        validate_cached=actions.validate_cached)
        if fault=="dispatcher":self.assertIn("fixture_dispatcher_transport",str(failure.exception))
        elif fault=="source":self.assertEqual(getattr(failure.exception,"phase",None),"source-archive")
        elif fault in ("session","postwrite-session"):self.assertIn("staging_resource_authority_changed",str(failure.exception))
        if fault=="none":
            self.assertIn("fixture_producer_kubernetes_boundary",str(failure.exception))
            self.assertEqual(len(calls),1)
            self.assertTrue((installed/"prerequisites"/("a"*12)/"code-custody.json").is_file())
        else:
            self.assertEqual(calls,[])
            self.assertFalse((installed/"prerequisites"/("a"*12)).exists())
        if fault in ("none","postwrite-session"):
            row=json.loads(receipt.read_text())
            self.assertEqual(set(row),{"schema","installation_sha256","operation","context","helper_sha256","approved_sha256","execution_sha256"})
            self.assertEqual(row["schema"],"voice-staging-source-capture-v2")
            self.assertEqual(row["installation_sha256"],actions.installation.sha256)
            self.assertEqual(row["context"],context)
            self.assertEqual(row["helper_sha256"],resource_claim.digest(actions.binding))
            self.assertEqual(receipt.stat().st_mode&0o777,0o600)
            if fault=="none":
                approved=json.loads((installed/"prerequisites"/("a"*12)/"approved.json").read_text())
                execution=json.loads((installed/"prerequisites"/("a"*12)/"execution.json").read_text())
                self.assertEqual(row["approved_sha256"],resource_claim.digest(approved))
                self.assertEqual(row["execution_sha256"],resource_claim.digest(execution))
        else:self.assertFalse(receipt.exists())
        material=[requests/(request["nonce"]+".json"),installed/"cohort-requests"/("a"*12+".json")]
        if receipt.exists():material.append(receipt)
        for path in material:self.assertNotIn("private-token",path.read_text())


if __name__=="__main__":unittest.main()
