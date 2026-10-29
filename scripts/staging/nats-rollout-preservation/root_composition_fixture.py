"""Private synthetic cluster fixture with source-checked installed helpers."""
import hashlib
import os
from pathlib import Path
import shutil
import tarfile

import staging_prerequisites as prerequisites
from root_main import save


ARCHIVE_SHA256='a3b48489d83965fbd96512d501989551db063dcab6d46943dea413a2428d6a1a'
ARCHIVE_SIZE=38502400
SYNTHETIC_BINARIES={name:('synthetic installed binary: '+name+'\n').encode() for name in
    ('nats-known-baseline/kernel','nats-rollout-preservation/bootstrap-renewer',
     'nats-rollout-preservation/story-current4-probe')}
SYNTHETIC_PINS={name:hashlib.sha256(data).hexdigest() for name,data in SYNTHETIC_BINARIES.items()}


def helper_binding(source, code, *, with_contract=False):
    """Use current source for Python helpers and independently pinned binary bytes."""
    archive_name=os.environ.get('VOICE_TEST_PINNED_ARCHIVE')
    archive=Path(archive_name) if archive_name else None
    if archive is not None:
        raw=archive.read_bytes()
        if len(raw)!=ARCHIVE_SIZE or hashlib.sha256(raw).hexdigest()!=ARCHIVE_SHA256:
            raise ValueError('pinned_fixture_archive_changed')
    code.mkdir(parents=True,mode=0o700)
    members=set(prerequisites.REQUIRED_HELPERS)|{'nats-rollout-preservation/realtime_bootstrap_existing.py'}
    if with_contract:members.add('nats-known-baseline/deployed-contract.json')
    binding={}
    for name in sorted(members):
        path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
        shutil.copyfile(source/'scripts'/'staging'/name,path);path.chmod(0o600)
        binding[name]=hashlib.sha256(path.read_bytes()).hexdigest()
    binaries=SYNTHETIC_BINARIES
    if archive is not None:
        binaries={}
        with tarfile.open(archive,'r:') as bundle:
            for name in prerequisites.BINARY_PINS:
                matched=[entry for entry in bundle if entry.name==name]
                if len(matched)!=1 or not matched[0].isfile():raise ValueError('pinned_fixture_member_invalid')
                binaries[name]=bundle.extractfile(matched[0]).read()
    for name,wanted in prerequisites.BINARY_PINS.items():
        data=binaries[name]
        if hashlib.sha256(data).hexdigest()!=wanted:raise ValueError('pinned_fixture_member_changed')
        path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(data);path.chmod(0o600)
        binding[name]=wanted
    save(code/'capture-manifest.json',binding)
    return binding


def exercise_candidate_drift(test):
    """Join real bridge/source/candidate admission; cluster and OCI are fixtures."""
    import copy
    from unittest.mock import patch
    import bridge_root
    import commands
    import resource_claim
    import resource_claim_test
    import source_authority
    import staging_prerequisites_test
    import staging_release_policy_prepare as policy

    real_prepare=prerequisites.run_prepare
    real_candidate=prerequisites._candidate_images
    real_policy_source=policy.validate_source
    required=prerequisites.REQUIRED_HELPERS
    production_pins=prerequisites.BINARY_PINS
    source=Path(__file__).resolve().parents[3]

    class Rejected(Exception):pass

    def join(actions,request,approved,execution,workspace):
        root=workspace.parents[2];installed=workspace.parents[1];old_session=actions.tt_session
        saved=workspace.with_name(workspace.name+'-fixture');workspace.rename(saved)
        (installed/'source-authority'/(old_session.operation+'.json')).unlink()
        code=installed/'code';shutil.rmtree(code)
        pins=production_pins if os.environ.get('VOICE_TEST_PINNED_ARCHIVE') else SYNTHETIC_PINS
        with patch.object(prerequisites,'BINARY_PINS',pins),patch.object(prerequisites,'REQUIRED_HELPERS',required):
            binding=helper_binding(source,code,with_contract=True)
            images={name:source_authority.REGISTRY+'/'+name+'@sha256:'+'a'*64 for name in source_authority.IMAGE_NAMES}
            context=copy.deepcopy(old_session.context)
            context['images']={'voice-gameintegration/gameintegration':images['gameintegration'][:-1]+'b'}
            card=resource_claim_test.card(context);card['observed']['images']={'voice-user/user':'registry/user@sha256:'+'b'*64}
            session=resource_claim.Session(context,old_session.operation,resource_claim.digest(binding),
                lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal());session.admit(start=True)
            actions.code=code;actions.binding=binding;actions.tt_session=session
            execution={'dispatcher_run_id':17,'head_sha':context['commit']}
            approved={'schema':'voice-source-authority-v1','source_sha':context['commit'],'repository':source_authority.REPO,
                'workflow_id':source_authority.WORKFLOW,'run_id':17,'run_attempt':1,'tree_sha':'b'*40,'lock_artifact_id':19,
                'lock_sha256':'d'*64,'image_provenance':'synthetic test','verified':True,'source_files':{},'images':images}
            def capture(_token,_run,_sha,destination):
                for part in ('scripts','deploy','src/backend/migrations','src/backend/auth/src/main/resources/db/migration','docker'):
                    shutil.copytree(source/part,destination/part,dirs_exist_ok=True)
                for path in destination.rglob('*'):path.chmod(0o700 if path.is_dir() else 0o600)
                approved['source_files']={path.relative_to(destination).as_posix():hashlib.sha256(path.read_bytes()).hexdigest()
                    for path in destination.rglob('*') if path.is_file()}
                return copy.deepcopy(approved)
            request=dict(request,action='prepare',nonce=session.operation+'a'*52,token='fixture')
            with patch.object(prerequisites,'run_prepare',real_prepare),patch.object(prerequisites,'_candidate_images',real_candidate), \
                 patch.object(policy,'validate_source',real_policy_source),patch.object(bridge_root,'INSTALLED',installed), \
                 patch.object(actions,'_idle'),patch.object(actions,'_dispatcher',return_value=(17,execution)), \
                 patch.object(source_authority,'capture_source',side_effect=capture), \
                 patch.object(prerequisites,'submit_effect',wraps=prerequisites.submit_effect) as submit, \
                 patch.object(session,'bind_prerequisites',wraps=session.bind_prerequisites) as bind, \
                 resource_claim.command_scope(session.check):
                with test.assertRaisesRegex(bridge_root.Blocked,'realtime_bootstrap_candidate_changed'):
                    actions._prepare(request)
                submit.assert_not_called();bind.assert_not_called()
                for call in commands.capture.call_args_list:
                    argv=call.args[0]
                    test.assertFalse(any(verb in argv for verb in ('patch','apply','create','delete','replace'))
                                     and not any(arg.startswith('--dry-run=') for arg in argv))
                test.assertEqual(session.check()['phase'],'prepare')
                test.assertFalse((installed/'prerequisites'/session.operation/'checkpoint.json').exists())
        raise Rejected()

    fixture=staging_prerequisites_test.PrepareCompositionTests()
    with patch.object(prerequisites,'run_prepare',side_effect=join):
        with test.assertRaises(Rejected):fixture._cold_prepare()
