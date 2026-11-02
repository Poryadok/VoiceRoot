"""Final readiness requires real protected authorization, never a result flag."""
import copy
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import commands
import guard
import resource_claim
import resource_claim_test as fixture


class CompletionAdmissionTests(unittest.TestCase):
    def test_legacy_final_reader_rejects_resealed_extraneous_installation(self):
        import datetime as dt
        import staging_cohort_preservation as cohort
        c=fixture.context();journal=fixture.MemoryJournal()
        session=resource_claim.Session(c,'a'*12,'b'*64,lambda:fixture.card(c),journal);session.admit(start=True)
        target={'fixture':'legacy'};journal.row.update(schema='voice-staging-resource-fence-v1',target_sha256=cohort.digest(target))
        del journal.row['installation_sha256'];binding=session.binding()
        request={'action':'authorize','operation':session.operation,'artifact_id':17}
        intent={'authorize_request':request};lineage={'fixture':'lineage'}
        record={'schema':'voice-staging-cohort-authorizing-v1','context':c,'helper_sha256':session.helper,
                'target_sha256':cohort.digest(target),'request_sha256':cohort.digest(request),
                'continuation_sha256':cohort.digest(intent),'lineage':lineage,'invariant':{},'events_before':[]}
        authorization={'schema':'nats-rollout-preservation-v1','operation':session.operation,'target':target,
                       'resource_fence':binding,'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat()}
        state={'operation':session.operation,'phase':'PAUSED_APPLY','status':'WAITING','target':target,
               'authorization':authorization,'events':[]}
        def read(path,**kwargs):
            if path.name=='checkpoint.json':return state
            if path.name=='authorizing.json':return record
            if path.name=='continuation.json':return intent
            return {'schema':'voice-staging-cohort-authorization-complete-v1','operation':session.operation,
                'authorizing_sha256':cohort.digest(record),'checkpoint_sha256':cohort.digest(state),
                'authorization_sha256':cohort.digest(authorization),'request_sha256':cohort.digest(request),
                'target_sha256':cohort.digest(target),'active_contract_sha256':None}
        with patch.object(cohort,'_require_session'),patch.object(cohort,'_final_target',return_value=target), \
             patch.object(cohort,'_read',side_effect=read),patch.object(cohort,'_lineage_snapshot',return_value=lineage), \
             patch.object(guard,'protected_receipt',side_effect=lambda *a,**k:authorization):
            self.assertEqual(cohort.final_authorization_result(session,request)['status'],'WAITING')
            authorization['installation_sha256']=session.installation.sha256
            with self.assertRaisesRegex(cohort.Blocked,'cohort_final_completion_changed'):
                cohort.final_authorization_result(session,request)

    def test_legacy_completion_rejects_extraneous_installation(self):
        import staging_cohort_preservation as cohort
        c=fixture.context();journal=fixture.MemoryJournal()
        session=resource_claim.Session(c,'a'*12,'b'*64,lambda:fixture.card(c),journal);session.admit(start=True)
        journal.row.update(schema='voice-staging-resource-fence-v1',target_sha256='c'*64)
        del journal.row['installation_sha256']
        binding=session.binding();target={'fixture':'legacy'}
        authorization={'schema':'nats-rollout-preservation-v1','target':target,'resource_fence':binding}
        state={'phase':'PAUSED_APPLY','status':'WAITING','target':target,'authorization':authorization}
        record={'request_sha256':'d'*64,'target_sha256':'c'*64}
        handle=cohort.QualifiedFinalAdmission()
        with patch.object(cohort,'_issued',return_value=record),patch.object(cohort,'_write') as write, \
             patch.object(guard,'protected_receipt',side_effect=lambda *a,**k:authorization):
            authorization['installation_sha256']=session.installation.sha256
            with self.assertRaisesRegex(cohort.Blocked,'cohort_final_authorization_incomplete'):
                cohort.complete_authorization(handle,session,Path('/unused'),state)
            write.assert_not_called()
            del authorization['installation_sha256']
            cohort.complete_authorization(handle,session,Path('/unused'),state)
            write.assert_called_once()

    def test_fabricated_authorize_result_cannot_make_real_session_ready(self):
        with tempfile.TemporaryDirectory(dir="/root") as directory:
            root=Path(directory);(root/"installed").mkdir(mode=0o700)
            context=fixture.context();current=fixture.card(context)
            journal=resource_claim.RootJournal(root)
            session=resource_claim.Session(context,"a"*12,"b"*64,lambda:copy.deepcopy(current),journal)
            session.admit(start=True)
            row=journal.read();row.update(target_sha256="c"*64,cohort_preservation_sha256="d"*64)
            journal.save(row);session.expected_target="c"*64;session._cohort_lineage="d"*64
            request={"action":"authorize","operation":session.operation,"artifact_id":17,"nonce":"e"*64,
                "resource_context":context}
            with patch.object(guard,"ROOT",root),resource_claim.command_scope(session.check):
                with self.assertRaises(Exception):
                    resource_claim.finish_request(session,object(),request,
                        {"operation":session.operation,"phase":"PAUSED_APPLY","status":"WAITING",
                         "authorization":str(root/("rollout-"+session.operation)/"apply-authorization.json")})
            self.assertEqual(journal.read()["phase"],"prepare")



class ProducerBundleTests(unittest.TestCase):
    def test_shipped_archive_contains_the_actual_preservation_executor_family(self):
        import bundle
        import hashlib
        import json
        import tarfile
        family=("staging_cohort_preservation.py","staging_cohort_preservation_runtime.py",
            "staging_cohort_preservation_pg.py","staging_cohort_preservation_auxiliary.py",
            "staging_cohort_preservation_auth.py")
        with tempfile.TemporaryDirectory(dir="/root") as directory:
            root=Path(directory);binary=root/"parser-fixture";binary.write_bytes(bytes([127])+b"ELFpackage-parser-only")
            bundle.build(binary,root/"out",binary,binary)
            with tarfile.open(root/"out"/"rollout-bundle.tar") as archive:
                members={item.name:archive.extractfile(item).read() for item in archive.getmembers()}
            manifest=json.loads(members["capture-manifest.json"])
            for name in family:
                relative="nats-rollout-preservation/"+name
                self.assertTrue(relative in manifest,relative)
                self.assertEqual(members[relative],Path(__file__).with_name(name).read_bytes())
                self.assertEqual(manifest[relative],hashlib.sha256(members[relative]).hexdigest())


class EarlyStatusTests(unittest.TestCase):
    def test_public_status_reads_cohort_checkpoint_without_a_final_target(self):
        import bridge_root
        import root_cli
        import root_main
        import staging_cohort_preservation as cohort
        with tempfile.TemporaryDirectory(dir="/root") as directory:
            root=Path(directory);base=root/("rollout-"+"a"*12);base.mkdir(mode=0o700)
            actions=object.__new__(bridge_root.Actions);actions.binding={"fixture":"b"*64}
            plan={name:"c"*64 for name in cohort.PLAN_FIELDS}
            context=fixture.context()
            plan.update(schema=cohort.SCHEMA,operation="a"*12,context=context,
                helper_sha256=resource_claim.digest(actions.binding),namespace_uid="fixture-ns",
                actual_before={"voice-user/user":"old"},candidate={"commit":context["commit"],"images":context["images"]})
            root_main.save(base/"checkpoint.json",{"schema":"voice-staging-cohort-preservation-checkpoint-v1",
                "operation":"a"*12,"phase":"AWAITING_OFF_NODE","status":"AWAITING_OFF_NODE",
                "fence_status":"VERIFIED","plan":plan})
            with patch.object(guard,"ROOT",root),patch.object(root_cli,"ROOT",root):
                self.assertEqual(actions.execute({"action":"status","operation":"a"*12}),
                    {"operation":"a"*12,"phase":"AWAITING_OFF_NODE","status":"AWAITING_OFF_NODE","fence_status":"VERIFIED"})


class ReplayAndForgeryTests(unittest.TestCase):
    def real_session(self,root):
        context=fixture.context();current=fixture.card(context)
        journal=resource_claim.RootJournal(root)
        session=resource_claim.Session(context,"a"*12,"b"*64,lambda:copy.deepcopy(current),journal)
        session.admit(start=True)
        return session,current,journal

    def test_class_constructor_new_and_copy_cannot_authorize_migrations(self):
        import migrations
        import staging_cohort_preservation as cohort
        from types import SimpleNamespace
        from controller import Blocked
        with tempfile.TemporaryDirectory(dir="/root") as directory:
            root=Path(directory);(root/"installed").mkdir(mode=0o700)
            session,current,journal=self.real_session(root)
            handles=(cohort.QualifiedFinalAdmission(),
                object.__new__(cohort.QualifiedFinalAdmission),
                copy.copy(cohort.QualifiedFinalAdmission()))
            with patch.object(guard,"ROOT",root),resource_claim.command_scope(session.check),\
                 patch.object(migrations,"preflight",wraps=migrations.preflight) as preflight:
                for mode in ("full","app-only"):
                    for handle in handles:
                        with self.subTest(mode=mode,handle=handle),self.assertRaisesRegex(Blocked,"not_issued"):
                            migrations.execute(object(),SimpleNamespace(operation=session.operation),[],mode,None,None,
                                resource_session=session,qualified_admission=handle,
                                authorization_base=root/("rollout-"+session.operation),authorization_state={})
                preflight.assert_not_called()

    def test_token_free_recovery_and_complete_cache_reject_all_unproved_phase_rows_read_only(self):
        import bridge_root
        import root_main
        from controller import Blocked
        cases=(
            ("capture-started","prepare","active","prepare"),
            ("producer-awaiting","cohort-preservation","active","prepare"),
            ("qualified-no-continuation","cohort-preservation","active","authorize"),
            ("successor-incomplete","prerequisites","active","authorize"),
            ("final-target-no-intent","prepare","active","authorize"),
            ("authorizing-incomplete","prepare","active","authorize"),
            ("auth-file-no-completion","prepare","active","authorize"),
            ("apply-ready-unproved","apply-ready","active","authorize"),
            ("terminal-superseded","finished","complete","authorize"),
            ("revoked-authority","prepare","active","authorize"),
        )
        for name,phase,status,action in cases:
            with self.subTest(row=name),tempfile.TemporaryDirectory(dir="/root") as directory:
                root=Path(directory);(root/"installed").mkdir(mode=0o700)
                session,current,journal=self.real_session(root)
                row=journal.read();row.update(phase=phase,status=status,target_sha256="c"*64,cohort_preservation_sha256="d"*64)
                journal.save(row);session.expected_target="c"*64;session._cohort_lineage="d"*64
                base=root/("rollout-"+session.operation);base.mkdir(mode=0o700)
                state={"schema":"nats-rollout-root-v1","operation":session.operation,"phase":"PAUSED_APPLY",
                    "status":"WAITING","target":{"mode":"full"},"authorization":{"verified":True}}
                root_main.save(base/"checkpoint.json",state)
                if name=="authorizing-incomplete":root_main.save(base/"authorizing.json",{"status":"PENDING"})
                if name=="auth-file-no-completion":root_main.save(base/"apply-authorization.json",state["authorization"])
                if name=="revoked-authority":current["claim"]="changed"
                request={"action":action,"nonce":"a"*64,"operation":session.operation,"artifact_id":17,
                    "resource_context":session.context,"mode":"full","source_sha":session.context["commit"],"changed_services":[]}
                actions=object.__new__(bridge_root.Actions);actions.tt_session=session;actions.binding={}
                def snapshot():return {str(p.relative_to(root)):p.read_bytes() for p in root.rglob("*") if p.is_file()}
                before=snapshot()
                with patch.object(guard,"ROOT",root),resource_claim.command_scope(session.check),\
                     patch.object(bridge_root.source_authority,"_fetch",side_effect=AssertionError("recovery transport effect")):
                    with self.assertRaises((Blocked,RuntimeError,OSError,KeyError,ValueError)):
                        actions.recover(request)
                    with self.assertRaises((Blocked,RuntimeError,OSError,KeyError,ValueError)):
                        actions.validate_cached(request,{"status":"WAITING","authorization":"forged"})
                self.assertEqual(snapshot(),before)




class FinalPreservationLinkTests(unittest.TestCase):
    """Isolated issuance crosslink units; trusted-loader doubles are not P2 proof."""
    def test_issuance_requires_the_protected_cut_provenance_contract_and_event_snapshot(self):
        import staging_cohort_preservation as cohort
        import realtime_bootstrap
        from controller import Blocked
        for fault in (None,'cut','provenance','contract','events','cohort_preservation','custody'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                session=resource_claim.Session(fixture.context(),'a'*12,'b'*64,
                    lambda:fixture.card(fixture.context()),resource_claim.RootJournal(root))
                session.admit(start=True)
                qualified={'protected':'qualified'};capture={'protected':'prerequisites'}
                rt={'schema':'voice-staging-realtime-bootstrap-receipt-v1','protected':'realtime'}
                preserved={'cut':{'protected':'native-cut'},'provenance':{'protected':'source-provenance'},
                    'contract':{'protected':'original-contract'},'events':[{'kind':'protected-paused-event'}],
                    'custody':{'protected':'offnode-custody'}}
                target={'tag':session.context['commit'],'cohort_preservation_sha256':cohort.digest(qualified),
                    'prerequisites_sha256':cohort.digest(capture),'realtime_bootstrap_sha256':cohort.digest(rt)}
                state={**copy.deepcopy(preserved),'target':target,'phase':'AWAITING_OFF_NODE','status':'WAITING',
                    'realtime_bootstrap':rt,'prerequisites':capture,'cohort_preservation':qualified}
                if fault is not None:state[fault]={'foreign':'snapshot'}
                with patch.object(guard,'ROOT',root):
                    base=cohort._base(session);cohort.private._private_directory(base.parent);cohort.private._private_directory(base)
                    with patch.object(cohort,'_final_target',return_value=target),\
                        patch.object(cohort,'require_continuation',return_value={'qualified_sha256':cohort.digest(qualified)}),\
                        patch.object(cohort,'load_qualified',return_value=qualified),\
                        patch.object(cohort,'load_prepared',return_value={'approved':{'source_sha':target['tag']}}),\
                        patch.object(cohort.private,'load_capture',return_value=capture),\
                        patch.object(realtime_bootstrap,'load_completed',return_value=rt),\
                        patch.object(cohort,'load_rollout_continuation',return_value=copy.deepcopy(preserved)),\
                        patch.object(cohort,'_offnode_records',return_value=preserved['custody']),\
                        patch.object(cohort,'_lineage_snapshot',return_value={'protected':'immutable-lineage'}):
                        if fault is None:
                            self.assertIs(type(cohort.issue_final_admission(session,root/'runtime',state,{})),cohort.QualifiedFinalAdmission)
                        else:
                            with self.assertRaisesRegex(Blocked,'cohort_final_lineage_changed'):
                                cohort.issue_final_admission(session,root/'runtime',state,{})
                            self.assertFalse((base/'authorizing.json').exists())


class OwnedNatsCompletionTests(unittest.TestCase):
    """Event pairing unit coverage; not full producer/admission acceptance."""
    def exercise(self, root, *, noop=False, executed=False, recovered=False, extra=(), mutate=None):
        from types import SimpleNamespace
        import staging_cohort_preservation as cohort
        import nats_contract_custody as custody
        import nats_contract_custody_test as transport
        import nats_active_contract
        kube, previous_stage, state, rows = transport.fixture()
        state["operation"] = "a" * 12
        session = resource_claim.Session(fixture.context(), state["operation"], "b" * 64,
            lambda: fixture.card(fixture.context()), resource_claim.RootJournal(root))
        stage = SimpleNamespace(expected=previous_stage.expected, kube=kube,
            verify_final_storage=lambda: None)
        migration_id = state["nats_contract"]["migration_id"]
        state["nats_contract"]["sources"] = {name: "c" * 64 for name in nats_active_contract.SOURCE_PATHS}
        state["nats_contract"]["actions"] = []
        if executed:
            import nats_contract_plan_test as plan_fixture
            state["nats_contract"] = plan_fixture.Tests().plan()
        state["nats_migration"]["applied"] = []
        state["nats_migration"]["proof"] = {
            "schema": nats_active_contract.VERSIONS[migration_id][0], "migration_id": migration_id,
            "before_sha256": "d" * 64, "after_sha256": "e" * 64,
            "messages": 1, "old_consumers_preserved": 1, "created_consumers": 0}
        if noop:
            for entry in state["contract"]["scripts"]:
                target = state["nats_target_scripts"][entry["part"]]
                row = rows["voice-nats-" + entry["part"] + "-bootstrap"]
                row["data"]["bootstrap.sh"] = target["script"]
                entry.update(sha256=target["sha256"], bytes=len(target["script"].encode()))
        original = copy.deepcopy(state["contract"])
        state["cut"] = {"manifest_sha256": "1" * 64, "census_sha256": "2" * 64}
        events = [{"kind": "nats_contract_old_cut_verified",
            "plan_sha256": cohort.digest(state["nats_contract"]),
            "old_manifest_sha256": state["cut"]["manifest_sha256"],
            "old_census_sha256": state["cut"]["census_sha256"], "proof": {}}]
        if executed:
            import nats_contract_plan as contract_plan
            from docker_runtime import NATS_IMAGE
            attempt = {"container_id": "f" * 64, "server_image": NATS_IMAGE,
                "started_at": "2026-10-29T00:00:00+00:00"}
            events.append({"kind": "nats_contract_broker_opened", **attempt})
            actor = plan_fixture.Actor(state["nats_contract"])
            if recovered:
                for action in state["nats_contract"]["actions"]:
                    actor.current[action["object"]] = copy.deepcopy(action["after"].get("config", action["after"]))
            state["nats_migration"]["applied"] = contract_plan.execute(state["nats_contract"], actor,
                lambda event: events.append({**event, "migration_attempt": attempt["container_id"]}))
            events.append({"kind": "nats_contract_broker_closed", **attempt,
                "finished_at": "2026-10-29T00:00:01+00:00"})
        active = custody.advance(kube, state, stage, events.append)
        state["active_contract"] = active
        state["events"] = events + list(copy.deepcopy(extra))
        if mutate is not None:
            mutate(state["events"])
        installed = root / "installed"
        installed.mkdir(mode=0o700)
        cohort._write(installed / "active-contract.json", active)
        with patch.object(guard, "ROOT", root):
            cohort._validate_owned_nats_transition(session, state, stage, original)
        return state, rows

    def test_actual_configmap_custody_accepts_completed_changed_and_noop_transitions(self):
        for noop, executed, recovered in ((False, False, False), (True, False, False),
                (False, True, False), (True, True, True)):
            with self.subTest(noop=noop, executed=executed, recovered=recovered), tempfile.TemporaryDirectory(dir="/root") as directory:
                self.exercise(Path(directory), noop=noop, executed=executed, recovered=recovered)

    def test_pending_foreign_and_duplicated_transition_events_are_rejected(self):
        incomplete = (
            {"kind": "nats_contract_broker_opened", "container_id": "f" * 64,
             "server_image": "foreign", "started_at": "2026-10-29T00:00:00+00:00"},
            {"kind": "nats_contract_intent", "object": "foreign", "api": "foreign",
             "before_sha256": "1" * 64, "target_sha256": "2" * 64, "migration_attempt": "f" * 64},
            {"kind": "nats_contract_mutation_issued", "object": "foreign", "api": "foreign",
             "target_sha256": "2" * 64, "migration_attempt": "f" * 64},
            {"kind": "nats_contract_cm_intent", "part": "foreign", "uid": "foreign",
             "resourceVersion": "1", "target_sha256": "2" * 64},
            {"kind": "nats_contract_cm_applied", "part": "foreign", "uid": "foreign",
             "resourceVersion": "2", "target_sha256": "2" * 64},
        )
        for event in incomplete:
            with self.subTest(event=event["kind"]), tempfile.TemporaryDirectory(dir="/root") as directory:
                from controller import Blocked
                with self.assertRaisesRegex(Blocked, "cohort_nats_transition_incomplete"):
                    self.exercise(Path(directory), extra=(event,))

    def test_executed_transition_rejects_missing_reordered_or_forged_completions(self):
        from controller import Blocked
        def remove_issued(events):
            events.remove(next(event for event in events if event["kind"]=="nats_contract_mutation_issued"))
        def reorder_write(events):
            intent=next(event for event in events if event["kind"]=="nats_contract_intent")
            issued=next(event for event in events if event["kind"]=="nats_contract_mutation_issued")
            left,right=events.index(intent),events.index(issued);events[left],events[right]=events[right],events[left]
        def foreign_attempt(events):
            next(event for event in events if event["kind"]=="nats_contract_applied")["migration_attempt"]="9"*64
        def duplicate_completion(events):
            events.append(copy.deepcopy(next(event for event in events if event["kind"]=="nats_contract_cm_applied")))
        def malformed_completion(events):
            next(event for event in events if event["kind"]=="nats_contract_cm_applied")["status"]="PENDING"
        def premature_custody(events):
            event=next(event for event in events if event["kind"]=="nats_contract_cm_applied")
            events.remove(event);events.insert(0,event)
        for mutation in (remove_issued,reorder_write,foreign_attempt,duplicate_completion,
                malformed_completion,premature_custody):
            with self.subTest(mutation=mutation.__name__),tempfile.TemporaryDirectory(dir="/root") as directory:
                with self.assertRaisesRegex(Blocked,"cohort_nats_transition_incomplete"):
                    self.exercise(Path(directory),executed=True,mutate=mutation)

if __name__=="__main__":unittest.main()
