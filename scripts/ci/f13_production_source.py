"""Pinned production applicability; historical facts never grant runtime paths."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import re
import sys
import zipfile

sys.path.insert(0,str(Path(__file__).resolve().parent))
import f13_role_conditional as gitutil

ROOT=Path(__file__).resolve().parents[2]
RECORD_PATH='docs/testing/f13-production-source-v1.json'
VERIFIER_PATH='scripts/ci/f13_production_source.py'
PINNED_RECORD_COMMIT='0aaed85d16f94d75bb70481ab660ea5b63f55e62'
PINNED_RECORD_SHA256='bde38ddd6495b5bf309f89638adb5471c906fb0f35a60d01c653cf27334fe16e'
VERSION='F13-PRODUCTION-SOURCE-v1'
REFERENCE='d6ac6d2f4b85eb691a00f729eb2192e63ae0091f'
EVENT_BASE='288e6a4fb963b3d5775f885e14ecfda1b603a4ff'
HISTORICAL_SCOPE=dict(base='698d4f064f032ea74abf3ede2017100b26c8ee69',
    target='53f75f74523735ac594e21582bea659f567a7288',
    allow_direct_followup='true',partition_source_history='false')
NATIVE_CONTENT_BASE='8191a36ca3bea7d053df0dd25963ca5133284b41'
ARCHIVE_PATH='scripts/ci/testdata/f13-exact-j-547/historical-inputs.zip'
ARCHIVE_SHA='ba6a775f6c38664d8603ac8080abca5c5e464d865b2e94b297127b6217549bd1'
MANIFEST_SHA='01907e66d9442d383dd27a8a6acfbe8e7bd3896ce6e765b6c52191f78edc44a9'
CARRIER_PATHS=(VERIFIER_PATH,'scripts/ci/f13_production_source_test.py',
    'scripts/ci/f13_f54_history.py','scripts/ci/f13_f54_history_test.py',
    'scripts/ci/voice-db-runtime-provisioning-contract_test.sh',
    '.agent/execplans/pr875-current-native-source.md')
SUCCESSOR_SOURCE='e409cd05c40e857b61109513eb2154f968970c02'
SUCCESSOR_PREDECESSOR='3b53b0a9c761f340d20d23ed36f64a0ad18af33f'
SUCCESSOR_RECORD_PATH='docs/testing/f13-production-source-v3.json'
SUCCESSOR_RECORD_COMMIT='f87ee933aa1f599284e1932e44bf32b4e2af48ba'
SUCCESSOR_RECORD_SHA256='adf1d0cf60f2e1f81c22d112bf26243961bbaac7f4b25f142487264d86e14c8f'
SUCCESSOR_PATHS=('src/frontend/lib/backend/spaces_client.dart',
    'src/frontend/test/spaces_client_invites_test.dart','src/frontend/test/support/bot_live_harness.dart')
SUCCESSOR_CARRIERS=(*CARRIER_PATHS,'scripts/ci/f13_native_p3_authority.py',
    'scripts/ci/f13_native_p3_successor_integration_test.py',
    'scripts/ci/f13_native_p3_documentation.py','scripts/ci/f13_native_p3_documentation_test.py')
TRANSPORT_VERSION='F13-PRODUCTION-SOURCE-v4'
TRANSPORT_PREDECESSOR='d4d4c62d6f3c5e16113943e6f7b8e327c985c66f'
TRANSPORT_SOURCE='54e6271806e6f95e229f2d2244f9464ead61f067'
TRANSPORT_RECORD_PATH='docs/testing/f13-production-source-v4.json'
TRANSPORT_RECORD_COMMIT='af6dd22b23b9bfad060ef43cf7eb8703cabd4e07'
TRANSPORT_RECORD_SHA256='058e0d873a7dcb5315f03e35c59edc7b4c34353a9e72f541aba21d8e409cf751'
TRANSPORT_ADDITION=dict(path='docs/testing/native-p3-source-v2-evidence/f13-9df-role-space-inventory.json',
    mode='100644',bytes=184585,sha256='fee1fdda19f8245b1e97c39d771025764a912bff47a6186664a57aea9d4ffbc1')
TEST_VERSION='F13-PRODUCTION-SOURCE-v5'
TEST_PREDECESSOR='38330e20dfb23bb8be2298ba3e70182e01414481'
TEST_SOURCE='1038561dcff8413539da28355ee6a532d54fbcba'
TEST_RECORD_PATH='docs/testing/f13-production-source-v5.json'
TEST_RECORD_COMMIT='d0554787d0363548106fb869dcfa8e9cc972b6af'
TEST_RECORD_SHA256='26d4f8235d2d192fcf1e2691bd92337e390dbe443c14b6e7afa35171227decdc'
TEST_FIXES={
    'src/backend/messaging/internal/messageevents/jetstream_test.go':'827a06e8a46a1fe4e517b3b356c3293596ef28f874c55c96c44724c23e780fdc',
    'src/backend/moderation/internal/grpcsvc/automod_scan_outcomes_test.go':'5fbc9e9e92d193045605018056d799f3653543d7b9fa8d017efd5fc5042cb4bc'}
require=gitutil.require
git=gitutil.git


RENDER_VERSION='F13-PRODUCTION-SOURCE-v6'
RENDER_PREDECESSOR='122bcedd7757139d441bc7b40220d98e21b117c4'
RENDER_SOURCE='5a817a37e145c10d3b2b87b03454d777e36b10c4'
RENDER_RECORD_PATH='docs/testing/f13-production-source-v6.json'
RENDER_RECORD_COMMIT='28e929e65fa1a038662ea8620ac2bd8537c96291'
RENDER_RECORD_SHA256='c7241b4a59d8b571f44004ae9e3c8537698f2f4eb7342275396608d1f35b4a05'
RENDER_FIXES={
    'scripts/ci/f13_historical_composition.py':'e1ed9f7f4c779f83fa1872a34a00dd7181c93c01cab3a9c043457a50b829b2c1',
    'scripts/ci/f13_historical_composition_test.py':'2f4cdd12d2fc9a913a8f37fccdc5bf5d675544c0fd1fdadb178a2e4b9f79989b'}


SOCIAL_VERSION='F13-PRODUCTION-SOURCE-v7'
SOCIAL_PREDECESSOR='88856ee15a403e436b5f1d7f6d67e58d223e027b'
SOCIAL_SOURCE='bc8552fbec6432a36151ab9afe9f85fecb1ccd20'
SOCIAL_RECORD_PATH='docs/testing/f13-production-source-v7.json'
SOCIAL_RECORD_COMMIT='8e98ab843eaa72245126b02fadf52bb349c3a4c1'
SOCIAL_RECORD_SHA256='753a22d082aa11f7b6696ea7a08c7fea08897e553956b960b3a8c41cf5dc13ba'
SOCIAL_FIXES={
    'src/backend/social/internal/socialevents/jetstream_test.go':'3a6877f3fd60c93a0737424f9760df5ca836f76b8d44d5d40a1c34424d3c6e89'}


INTEGRATION_VERSION='F13-PRODUCTION-SOURCE-v8'
INTEGRATION_SOURCE='ef3dd7f3888bbe2792a9dcec836b2773c4e77962'
INTEGRATION_PREDECESSOR='ecde2e7b29d93dd01b233cd3128b373137aa6715'
INTEGRATION_MERGE='fb0df638da3a31258deea25c0653e05ae4d690cf'
INTEGRATION_EVENT_BASE='2c8bdf9e2f420b724575096beb4fcf5cb4ea00be'
INTEGRATION_RECORD_PATH='docs/testing/f13-production-source-v8.json'
INTEGRATION_RECORD_COMMIT='60c0a1fd66d090956113c395d8447edc47b95772'
INTEGRATION_RECORD_SHA256='90e6967cf14de9dbb349be06c51e10e06e8e280d8bd49c381a1ca6b2cfec1f5e'
INTEGRATION_CHAIN=(INTEGRATION_MERGE,'3a2b8e19b0f8fe7a6104b753340f62864e039326',
    'a8d5c2c04c4e8707d4cec6c3b06cce7ed68acb2b',INTEGRATION_SOURCE)


def sha(raw):return hashlib.sha256(raw).hexdigest()


def tree_entry(repo,rev,name):
    return git(repo,'ls-tree','-z',rev,'--',':(literal)'+name).decode()


def entries(repo,revision):
    return {item.split('\t',1)[1]:item+'\0' for item in
            git(repo,'ls-tree','-r','-z',revision).decode().split('\0') if item}


def directory_entries(entries):
    # Group this freshly read tree once; every inventory comparison remains below.
    grouped={}
    for path,entry in entries.items():
        directory=str(Path(path).parent).replace('\\','/')
        grouped.setdefault(directory,{})[path]=entry
    return grouped


def fixed_artifact(repo,revision,proof):
    require(isinstance(proof,dict) and set(proof)=={'path','sha256'},'production_artifact_shape')
    name=proof['path'];digest=proof['sha256']
    require(isinstance(name,str) and name.startswith('docs/testing/') and '..' not in name.split('/'),
            'production_artifact_path')
    require(isinstance(digest,str) and re.fullmatch('[0-9a-f]{64}',digest),'production_artifact_digest')
    entry=gitutil.entry(repo,revision,name)
    require(entry['mode']=='100644' and entry['type']=='blob','production_artifact_mode')
    raw=git(repo,'cat-file','blob',entry['oid'])
    require(sha(raw)==digest,'production_artifact_hash')
    return raw


def load_record(repo):
    require(PINNED_RECORD_COMMIT is not None and PINNED_RECORD_SHA256 is not None,'production_record_not_pinned')
    gitutil.oid(PINNED_RECORD_COMMIT)
    raw=fixed_artifact(repo,PINNED_RECORD_COMMIT,dict(path=RECORD_PATH,sha256=PINNED_RECORD_SHA256))
    record=json.loads(raw,object_pairs_hook=gitutil.pairs)
    require(set(record)=={'version','status','source','source_tree','reference','event_base','historical_scope',
        'native_content_base','archive','current_inputs','corrected_F','native_record','evidence','review',
        'runtime_pass','activation_authority','current_or_future_path_authority'},'production_record_shape')
    require(record['version']==VERSION and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR','production_record_status')
    require(all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
            'production_no_path_or_runtime_grant')
    require(record['reference']==REFERENCE and record['event_base']==EVENT_BASE
            and record['historical_scope']==HISTORICAL_SCOPE and record['native_content_base']==NATIVE_CONTENT_BASE,
            'production_separate_fixed_boundaries')
    source=gitutil.oid(record['source']);gitutil.oid(record['source_tree'])
    require(git(repo,'rev-parse',source+'^{tree}').decode().strip()==record['source_tree'],'production_source_tree')
    git(repo,'merge-base','--is-ancestor',REFERENCE,source)
    require(git(repo,'rev-list','--parents','-n','1',PINNED_RECORD_COMMIT).decode().split()==
            [PINNED_RECORD_COMMIT,source],'production_record_direct_source_parent')
    manifest=json.loads(fixed_artifact(repo,PINNED_RECORD_COMMIT,record['current_inputs']),object_pairs_hook=gitutil.pairs)
    require(set(manifest)=={'source','reference','delta_sha256','entries'} and manifest['source']==source
            and manifest['reference']==REFERENCE,'production_current_inputs_shape')
    names=git(repo,'diff','--name-only',REFERENCE,source).decode().splitlines()
    before,after=entries(repo,REFERENCE),entries(repo,source)
    require(manifest['delta_sha256']==sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',REFERENCE,source)),
            'production_complete_current_delta')
    require(manifest['entries']==[dict(path=name,reference_entry=before.get(name,''),
            current_entry=after.get(name,''),disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
            for name in names],'production_complete_current_input_inventory')
    archive=record['archive']
    require(set(archive)=={'path','sha256','manifest_sha256','entry','selected'} and archive['path']==ARCHIVE_PATH
            and archive['sha256']==ARCHIVE_SHA and archive['manifest_sha256']==MANIFEST_SHA,'production_fixed_archive')
    gitutil.actual_entry(repo,source,ARCHIVE_PATH,archive['entry'])
    require(isinstance(archive['selected'],list) and len(archive['selected'])==192,'production_complete_historical_data')
    with zipfile.ZipFile(ROOT/ARCHIVE_PATH) as transport:
        manifest_raw=transport.read('transport-manifest.json')
        require(sha(manifest_raw)==MANIFEST_SHA,'production_fixed_transport_manifest')
        historical=json.loads(manifest_raw,object_pairs_hook=gitutil.pairs)
        selected=[item for item in historical['logical_files'] if item['logical_path'].startswith(('/unit/','/fixed3071-unit/','/original/'))]
        require(archive['selected']==selected,'production_exact_selected_historical_closure')
        fixed_raw=transport.read('objects/dc4a28dc896a6bc1da54fcc01aceba94eaa63fd4b1b1eb175eeb9dc8cb9c31a3')
        require(sha(fixed_raw)=='dc4a28dc896a6bc1da54fcc01aceba94eaa63fd4b1b1eb175eeb9dc8cb9c31a3','production_fixed_F_unit')
        fixed=json.loads(fixed_raw,object_pairs_hook=gitutil.pairs)
    correction=record['corrected_F']
    require(correction==dict(unit_sha256=sha(fixed_raw),historical_comparison='1c47dce37a88c2539c8dabb074d9eda46fb84999',
            current_bindings={name:after.get(name,'') for name in fixed['current_bindings']},
            historical_gap=fixed['historical_gap']),'production_complete_F_current_source')
    require(record['native_record']==dict(commit='3ef197bc0fd6bde17a538581df4e4fe99a1765d4',
        path='docs/testing/native-p3-source-v2.json',sha256='edda541ab45be2ff17f74d626c657a7611274d0149c2bca808092b5a9bfe4f8b'),
        'production_fixed_current_native_record')
    evidence=record['evidence'];require(isinstance(evidence,list) and evidence,'production_evidence')
    proof_names=[]
    for proof in evidence:
        fixed_artifact(repo,PINNED_RECORD_COMMIT,proof);proof_names.append(proof['path'])
    require(proof_names==sorted(set(proof_names)),'production_evidence_order')
    require(isinstance(record['review'],list) and record['review'] and all(
        isinstance(name,str) and name in proof_names for name in record['review']),'production_independent_review')
    permitted={RECORD_PATH,record['current_inputs']['path'],*proof_names,
               '.agent/execplans/pr875-current-native-source.md'}
    require(set(git(repo,'diff','--name-only',source,PINNED_RECORD_COMMIT).decode().splitlines())<=permitted,
            'production_record_only_evidence_delta')
    return record


FIXTURE_VERSION='F13-PRODUCTION-SOURCE-v9'
FIXTURE_SOURCE='3a530edd97cbfe4beb9cea764eb0dd71acec2d2c'
FIXTURE_PREDECESSOR='c1c340c809aa619a4ee18230b99356959fdb4a40'
FIXTURE_PATH='scripts/ci/phase0-fixture_test.py'
FIXTURE_RECORD_PATH='docs/testing/f13-production-source-v9.json'
FIXTURE_RECORD_COMMIT='280d172b1ff9c9cd6b6ec02c9f924cac0148b354'
FIXTURE_RECORD_SHA256='062fbe47a0bb10c0b626173961db814148cb22ac72a348ba9618958937f1c305'
FIXTURE_AFTER_SHA256='4a358406e826b70832f774e69658d444690e708e0fef0bf82f669545b7cc91ab'
FIXTURE_BEFORE_SHA256='c681dd23967350f10cdecf278dfd7b4f0b3cd55491e85dfc000845ab80a0dce3'

BOOTSTRAP_VERSION='F13-PRODUCTION-SOURCE-v10'
BOOTSTRAP_SOURCE='af6729e443d25007664238190263142dc8a182b7'
BOOTSTRAP_PREDECESSOR='9ef1a2b147b00e26a3d98b09ac01e75df1409702'
BOOTSTRAP_PATH='docker/postgres/compose-realtime-migrations_test.py'
BOOTSTRAP_RECORD_PATH='docs/testing/f13-production-source-v10.json'
BOOTSTRAP_RECORD_COMMIT='d783ab03c35ad8a638ff052b90d330da3ffdda6c'
BOOTSTRAP_RECORD_SHA256='e2e4cc84465a8bc7ab1dc4a7f37fca24fe1803dbe497b5547180fa4460f61756'
BOOTSTRAP_AFTER_SHA256='ae275579b5f0d5c1ef91542f17d581e1380f9a85b324f1f720d15031ab38c997'
BOOTSTRAP_BEFORE_SHA256='f2bf782c84d2f1729cdb02cd205052ba960b5c92ece434cc7c9fe537fa7595b4'

def bootstrap_applicable(repo,target='HEAD'):
    return git(repo,'merge-base',BOOTSTRAP_SOURCE,target).decode().strip()==BOOTSTRAP_SOURCE


def fixture_applicable(repo,target='HEAD'):
    return git(repo,'merge-base',FIXTURE_SOURCE,target).decode().strip()==FIXTURE_SOURCE


def qualify(repo):
    require(Path(repo).resolve()==ROOT.resolve(),'production_actual_root')
    try:
        import f13_release_source as release
    except ModuleNotFoundError as error:
        if error.name != 'f13_release_source':raise
        release=None
    if release is not None and release.applicable(repo):return release.production_qualify(repo)
    if bootstrap_applicable(repo):return qualify_bootstrap_fix(repo)
    if fixture_applicable(repo):return qualify_fixture_fix(repo)
    if INTEGRATION_SOURCE is not None and git(repo,'merge-base',INTEGRATION_SOURCE,'HEAD').decode().strip()==INTEGRATION_SOURCE:
        return qualify_integration(repo)
    require(Path(repo).resolve()==ROOT.resolve(),'production_actual_root')
    if SOCIAL_SOURCE is not None and git(repo,'merge-base',SOCIAL_SOURCE,'HEAD').decode().strip()==SOCIAL_SOURCE:
        return qualify_social_fix(repo)
    if RENDER_SOURCE is not None and git(repo,'merge-base',RENDER_SOURCE,'HEAD').decode().strip()==RENDER_SOURCE:
        return qualify_rendering_fix(repo)
    if TEST_SOURCE is not None and git(repo,'merge-base',TEST_SOURCE,'HEAD').decode().strip()==TEST_SOURCE:
        return qualify_test_fixes(repo)
    if TRANSPORT_SOURCE is not None and git(repo,'merge-base',TRANSPORT_SOURCE,'HEAD').decode().strip()==TRANSPORT_SOURCE:
        return qualify_transport(repo)
    if git(repo,'merge-base',SUCCESSOR_SOURCE,'HEAD').decode().strip()==SUCCESSOR_SOURCE:
        return qualify_successor(repo)
    record=load_record(repo)
    target=git(repo,'rev-parse','HEAD').decode().strip()
    event={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    pr=event['GITHUB_EVENT_NAME']=='pull_request' and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',event['GITHUB_REF']) and event['GITHUB_BASE_REF']=='master'
    push=event['GITHUB_EVENT_NAME']=='push' and event['GITHUB_REF']=='refs/heads/master' and event['VOICE_CI_PUSH_BEFORE_SHA']==record['event_base']
    require(event['GITHUB_SHA']==target and (pr or push),'production_actual_master_event')
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and parents[1]==record['event_base'],'production_actual_ordered_parents')
    source=parents[2]
    git(repo,'merge-base','--is-ancestor',PINNED_RECORD_COMMIT,source)
    require(source!=PINNED_RECORD_COMMIT,'production_later_carrier')
    require(git(repo,'rev-parse',source+'^{tree}')==git(repo,'rev-parse',target+'^{tree}'),'production_source_result_tree')
    changed=git(repo,'diff','--name-only',PINNED_RECORD_COMMIT,source).decode().splitlines()
    require(set(changed)<=set(CARRIER_PATHS),'production_only_reviewed_carrier_delta')
    for name in CARRIER_PATHS:
        meta=gitutil.entry(repo,source,name)
        require(meta['mode']=='100644' and meta['type']=='blob','production_carrier_mode')
        require(git(repo,'cat-file','blob',meta['oid'])==(ROOT/name).read_bytes(),'production_executed_carrier')
    for name in (RECORD_PATH,record['current_inputs']['path'],*[p['path'] for p in record['evidence']]):
        require(tree_entry(repo,target,name)==tree_entry(repo,PINNED_RECORD_COMMIT,name),'production_artifact_drift')
    gitutil.actual_entry(repo,target,ARCHIVE_PATH,record['archive']['entry'])
    return dict(type=VERSION,record=PINNED_RECORD_COMMIT,record_sha256=PINNED_RECORD_SHA256,
        reviewed_source=record['source'],reviewed_tree=record['source_tree'],base=record['event_base'],source=source,target=target,
        historical_collector=record['historical_scope'],native_content_baseline=record['native_content_base'],
        current_inputs_sha256=record['current_inputs']['sha256'],event=event,
        admission=False,runtime_pass=False,current_or_future_path_authority=False)


def load_successor(repo):
    old=load_record(repo)
    require(SUCCESSOR_RECORD_COMMIT is not None and SUCCESSOR_RECORD_SHA256 is not None,
            'production_successor_not_pinned')
    raw=fixed_artifact(repo,SUCCESSOR_RECORD_COMMIT,dict(path=SUCCESSOR_RECORD_PATH,sha256=SUCCESSOR_RECORD_SHA256))
    record=json.loads(raw,object_pairs_hook=gitutil.pairs)
    require(set(record)=={'version','status','predecessor','source','source_tree','source_edges','reference','event_base',
        'historical_scope','native_content_base','change_summary','current_inputs','successor_closure',
        'native_record','manual_supplement','manual_derivation','evidence','review','runtime_pass','activation_authority','current_or_future_path_authority'},'production_successor_shape')
    require(record['version']=='F13-PRODUCTION-SOURCE-v3' and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR',
            'production_successor_status')
    require(all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
            'production_successor_no_path_grant')
    require(record['source']==SUCCESSOR_SOURCE and record['predecessor']==dict(record=PINNED_RECORD_COMMIT,
        path=RECORD_PATH,sha256=PINNED_RECORD_SHA256,carrier=SUCCESSOR_PREDECESSOR,
        carrier_tree=git(repo,'rev-parse',SUCCESSOR_PREDECESSOR+'^{tree}').decode().strip()),'production_successor_lineage')
    for key in ('reference','event_base','historical_scope','native_content_base'):
        require(record[key]==old[key],'production_successor_fixed_boundaries')
    require(isinstance(record['change_summary'],str) and record['change_summary'],'production_successor_summary')
    edges=[]
    for commit in git(repo,'rev-list','--reverse','--ancestry-path',SUCCESSOR_PREDECESSOR+'..'+SUCCESSOR_SOURCE).decode().splitlines():
        parents=git(repo,'rev-list','--parents','-n','1',commit).decode().split()[1:]
        require(len(parents)==1,'production_successor_source_parent')
        edges.append(dict(commit=commit,parent=parents[0],raw_diff_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',parents[0],commit))))
    require(record['source_edges']==edges and len(edges)==2,'production_successor_complete_source_edges')
    require(git(repo,'rev-list','--parents','-n','1',SUCCESSOR_RECORD_COMMIT).decode().split()==
            [SUCCESSOR_RECORD_COMMIT,SUCCESSOR_SOURCE],'production_successor_record_parent')
    require(record['source_tree']==git(repo,'rev-parse',SUCCESSOR_SOURCE+'^{tree}').decode().strip(),
            'production_successor_source_tree')
    git(repo,'merge-base','--is-ancestor',PINNED_RECORD_COMMIT,SUCCESSOR_PREDECESSOR)
    require(set(git(repo,'diff','--name-only',PINNED_RECORD_COMMIT,SUCCESSOR_PREDECESSOR).decode().splitlines())<=set(CARRIER_PATHS),
            'production_successor_predecessor_carrier')
    before,after,reference=entries(repo,SUCCESSOR_PREDECESSOR),entries(repo,SUCCESSOR_SOURCE),entries(repo,REFERENCE)
    changed=git(repo,'diff','--name-only',SUCCESSOR_PREDECESSOR,SUCCESSOR_SOURCE).decode().splitlines()
    require(len(changed)==24 and all(p in changed for p in SUCCESSOR_PATHS),'production_successor_exact_delta')
    require(all(before.get(p)==after.get(p) for p in set(before)|set(after) if p not in changed),
            'production_successor_complete_outside_equality')
    closure=json.loads(fixed_artifact(repo,SUCCESSOR_RECORD_COMMIT,record['successor_closure']),object_pairs_hook=gitutil.pairs)
    native=json.loads(git(repo,'show',SUCCESSOR_PREDECESSOR+':docs/testing/native-p3-source-v2.json'),object_pairs_hook=gitutil.pairs)
    role=json.loads(git(repo,'show',SUCCESSOR_PREDECESSOR+':docs/testing/role-checkpermission-historical-source-v2.json'),object_pairs_hook=gitutil.pairs)
    groups={'corrected_F':sorted(old['corrected_F']['current_bindings']),
        'native_participating':sorted(x['path'] for x in native['projection']+native['context']),
        'native_compiled':sorted(native['compiled_closure']),
        'role_participating':sorted(x['path'] for x in role['required_current_source']+role['context'])}
    require(closure==dict(predecessor=SUCCESSOR_PREDECESSOR,source=SUCCESSOR_SOURCE,
        complete_delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',SUCCESSOR_PREDECESSOR,SUCCESSOR_SOURCE)),
        entries=[dict(path=p,before=before.get(p,''),after=after.get(p,''),classification='REVIEWED_NATIVE_DOCUMENT_ONLY_NO_BACKEND_PATH_AUTHORITY' if p=='docs/microservices/space-service.md' else 'ORIGINAL_CONTRIBUTION_GATES_REQUIRED') for p in changed],
        participating={key:[dict(path=p,entry=after.get(p,'')) for p in paths] for key,paths in groups.items()},
        closure_manifests={p:after[p] for p in ['docs/testing/native-p3-source-v2-closure.json','docs/testing/role-checkpermission-source-v2-closure.json']},
        outside_delta_tree_entries_equal=True),'production_successor_complete_closure')
    import f13_native_p3_documentation as documentation
    require(documentation.PINNED_RECORD_COMMIT==SUCCESSOR_RECORD_COMMIT and documentation.PINNED_SOURCE==SUCCESSOR_SOURCE,
            'production_successor_native_same_record')
    native_ref=record['native_record']
    require(native_ref==dict(path=documentation.RECORD_PATH,sha256=documentation.PINNED_RECORD_SHA256,
        scope='SAME_RECORD_COMMIT'),'production_successor_native_reference')
    current_native=documentation.load_record(repo)
    require([r['path'] for r in current_native['projection']]==[r['path'] for r in native['projection']],
            'production_successor_native_no_new_paths')
    for key,paths in groups.items():
        require(all(before.get(p)==after.get(p) for p in paths if p!=documentation.DOCUMENT),
                'production_successor_participating_drift')
    validate_manual_supplement(repo,record)
    inventory=json.loads(fixed_artifact(repo,SUCCESSOR_RECORD_COMMIT,record['current_inputs']),object_pairs_hook=gitutil.pairs)
    require(inventory==dict(source=SUCCESSOR_SOURCE,reference=REFERENCE,
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',REFERENCE,SUCCESSOR_SOURCE)),
        entries=[dict(path=p,reference_entry=reference.get(p,''),current_entry=after.get(p,''),disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
            for p in git(repo,'diff','--name-only',REFERENCE,SUCCESSOR_SOURCE).decode().splitlines()]),'production_successor_complete_inventory')
    require(isinstance(record['evidence'],list) and record['evidence'],'production_successor_evidence')
    names=[]
    for proof in record['evidence']:
        fixed_artifact(repo,SUCCESSOR_RECORD_COMMIT,proof);names.append(proof['path'])
    require(names==sorted(set(names)) and isinstance(record['review'],list) and record['review'] and
        all(isinstance(p,str) and p in names for p in record['review']),'production_successor_review')
    permitted={SUCCESSOR_RECORD_PATH,record['current_inputs']['path'],record['successor_closure']['path'],documentation.RECORD_PATH,*names,
        '.agent/execplans/pr875-current-native-source.md'}
    require(set(git(repo,'diff','--name-only',SUCCESSOR_SOURCE,SUCCESSOR_RECORD_COMMIT).decode().splitlines())<=permitted,
            'production_successor_evidence_only_record')
    return record,old


def qualify_successor(repo):
    record,old=load_successor(repo)
    target=git(repo,'rev-parse','HEAD').decode().strip()
    event={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    pr=event['GITHUB_EVENT_NAME']=='pull_request' and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',event['GITHUB_REF']) and event['GITHUB_BASE_REF']=='master'
    push=event['GITHUB_EVENT_NAME']=='push' and event['GITHUB_REF']=='refs/heads/master' and event['VOICE_CI_PUSH_BEFORE_SHA']==EVENT_BASE
    require(event['GITHUB_SHA']==target and (pr or push),'production_actual_master_event')
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and parents[1]==EVENT_BASE,'production_actual_ordered_parents')
    source=parents[2];git(repo,'merge-base','--is-ancestor',SUCCESSOR_RECORD_COMMIT,source)
    require(source!=SUCCESSOR_RECORD_COMMIT,'production_later_carrier')
    require(git(repo,'rev-parse',source+'^{tree}')==git(repo,'rev-parse',target+'^{tree}'),'production_source_result_tree')
    require(set(git(repo,'diff','--name-only',SUCCESSOR_RECORD_COMMIT,source).decode().splitlines())<=set(SUCCESSOR_CARRIERS),
            'production_only_reviewed_carrier_delta')
    for name in SUCCESSOR_CARRIERS:
        meta=gitutil.entry(repo,source,name)
        require(meta['mode']=='100644' and meta['type']=='blob','production_carrier_mode')
        require(git(repo,'cat-file','blob',meta['oid'])==(ROOT/name).read_bytes(),'production_executed_carrier')
    for revision,names in ((PINNED_RECORD_COMMIT,[RECORD_PATH,old['current_inputs']['path'],*[p['path'] for p in old['evidence']]]),
        (SUCCESSOR_RECORD_COMMIT,[SUCCESSOR_RECORD_PATH,record['current_inputs']['path'],record['successor_closure']['path'],record['native_record']['path'],*[p['path'] for p in record['evidence']]])):
        for name in names:
            require(tree_entry(repo,target,name)==tree_entry(repo,revision,name),'production_artifact_drift')
    gitutil.actual_entry(repo,target,ARCHIVE_PATH,old['archive']['entry'])
    return dict(type=record['version'],record=SUCCESSOR_RECORD_COMMIT,record_sha256=SUCCESSOR_RECORD_SHA256,
        predecessor_record=PINNED_RECORD_COMMIT,historical_validation_source=old['source'],
        reviewed_source=record['source'],reviewed_tree=record['source_tree'],base=EVENT_BASE,source=source,target=target,
        historical_collector=HISTORICAL_SCOPE,native_content_baseline=NATIVE_CONTENT_BASE,
        current_inputs_sha256=record['current_inputs']['sha256'],event=event,
        admission=False,runtime_pass=False,current_or_future_path_authority=False)


def load_transport(repo):
    """One exact missing evidence companion; no documentation path permission."""
    prior,old=load_successor(repo)
    require(TRANSPORT_SOURCE is not None and TRANSPORT_RECORD_COMMIT is not None and
        TRANSPORT_RECORD_SHA256 is not None,'production_transport_not_pinned')
    raw=fixed_artifact(repo,TRANSPORT_RECORD_COMMIT,dict(path=TRANSPORT_RECORD_PATH,sha256=TRANSPORT_RECORD_SHA256))
    record=json.loads(raw,object_pairs_hook=gitutil.pairs)
    require(set(record)=={'version','status','source','source_tree','predecessor','addition','native_record',
        'current_inputs','evidence','review','runtime_pass','activation_authority','current_or_future_path_authority'},
        'production_transport_shape')
    require(record['version']==TRANSPORT_VERSION and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR' and
        all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
        'production_transport_no_authority')
    require(record['source']==TRANSPORT_SOURCE and record['predecessor']==dict(record=SUCCESSOR_RECORD_COMMIT,
        path=SUCCESSOR_RECORD_PATH,sha256=SUCCESSOR_RECORD_SHA256,carrier=TRANSPORT_PREDECESSOR),
        'production_transport_lineage')
    require(git(repo,'rev-list','--parents','-n','1',TRANSPORT_SOURCE).decode().split()==
        [TRANSPORT_SOURCE,TRANSPORT_PREDECESSOR],'production_transport_source_parent')
    require(git(repo,'rev-list','--parents','-n','1',TRANSPORT_RECORD_COMMIT).decode().split()==
        [TRANSPORT_RECORD_COMMIT,TRANSPORT_SOURCE],'production_transport_record_parent')
    require(record['source_tree']==git(repo,'rev-parse',TRANSPORT_SOURCE+'^{tree}').decode().strip(),
        'production_transport_source_tree')
    git(repo,'merge-base','--is-ancestor',SUCCESSOR_RECORD_COMMIT,TRANSPORT_PREDECESSOR)
    require(set(git(repo,'diff','--name-only',SUCCESSOR_RECORD_COMMIT,TRANSPORT_PREDECESSOR).decode().splitlines())
        <=set(SUCCESSOR_CARRIERS),'production_transport_predecessor_carrier')
    before,after=entries(repo,TRANSPORT_PREDECESSOR),entries(repo,TRANSPORT_SOURCE)
    addition=record['addition'];require(addition==TRANSPORT_ADDITION,'production_transport_exact_addition')
    name=addition['path']
    require(name not in before and set(after)-set(before)=={name} and all(after.get(p)==v for p,v in before.items()),
        'production_transport_only_missing_companion')
    meta=gitutil.entry(repo,TRANSPORT_SOURCE,name)
    data=git(repo,'cat-file','blob',meta['oid'])
    require(meta['mode']==addition['mode'] and meta['type']=='blob' and len(data)==addition['bytes'] and
        sha(data)==addition['sha256'],'production_transport_companion_bytes')
    import f13_native_p3_documentation as documentation
    documentation.load_record(repo)
    require(record['native_record']==dict(version=documentation.VERSION,commit=documentation.PINNED_RECORD_COMMIT,
        source=documentation.PINNED_SOURCE,path=documentation.RECORD_PATH,sha256=documentation.PINNED_RECORD_SHA256) and
        record['native_record']['commit']==SUCCESSOR_RECORD_COMMIT and record['native_record']['source']==SUCCESSOR_SOURCE,
        'production_transport_retained_native_tuple')
    inventory=json.loads(fixed_artifact(repo,TRANSPORT_RECORD_COMMIT,record['current_inputs']),object_pairs_hook=gitutil.pairs)
    reference=entries(repo,REFERENCE)
    require(inventory==dict(source=TRANSPORT_SOURCE,reference=REFERENCE,
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',REFERENCE,TRANSPORT_SOURCE)),
        entries=[dict(path=p,reference_entry=reference.get(p,''),current_entry=after.get(p,''),
            disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
            for p in git(repo,'diff','--name-only',REFERENCE,TRANSPORT_SOURCE).decode().splitlines()]),
        'production_transport_complete_inventory')
    require(isinstance(record['evidence'],list) and record['evidence'],'production_transport_evidence')
    names=[]
    for proof in record['evidence']:
        fixed_artifact(repo,TRANSPORT_RECORD_COMMIT,proof);names.append(proof['path'])
    require(names==sorted(set(names)) and isinstance(record['review'],list) and record['review'] and
        all(isinstance(p,str) and p in names for p in record['review']),'production_transport_review')
    additions={TRANSPORT_RECORD_PATH,record['current_inputs']['path'],*names}
    require(len(additions)==2+len(names) and all(p not in after for p in additions),'production_transport_new_evidence_only')
    recorded=entries(repo,TRANSPORT_RECORD_COMMIT)
    require(set(recorded)-set(after)==additions and all(recorded.get(p)==v for p,v in after.items()
        if p!='.agent/execplans/pr875-current-native-source.md'),'production_transport_immutable_predecessor')
    return record,prior,old


def qualify_transport(repo):
    record,prior,old=load_transport(repo)
    target=git(repo,'rev-parse','HEAD').decode().strip()
    event={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    pr=event['GITHUB_EVENT_NAME']=='pull_request' and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',event['GITHUB_REF']) and event['GITHUB_BASE_REF']=='master'
    push=event['GITHUB_EVENT_NAME']=='push' and event['GITHUB_REF']=='refs/heads/master' and event['VOICE_CI_PUSH_BEFORE_SHA']==EVENT_BASE
    require(event['GITHUB_SHA']==target and (pr or push),'production_actual_master_event')
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and parents[1]==EVENT_BASE,'production_actual_ordered_parents')
    source=parents[2];git(repo,'merge-base','--is-ancestor',TRANSPORT_RECORD_COMMIT,source)
    require(source!=TRANSPORT_RECORD_COMMIT,'production_later_carrier')
    require(git(repo,'rev-parse',source+'^{tree}')==git(repo,'rev-parse',target+'^{tree}'),'production_source_result_tree')
    require(set(git(repo,'diff','--name-only',TRANSPORT_RECORD_COMMIT,source).decode().splitlines())<=set(SUCCESSOR_CARRIERS),
        'production_only_reviewed_carrier_delta')
    for name in SUCCESSOR_CARRIERS:
        meta=gitutil.entry(repo,source,name)
        require(meta['mode']=='100644' and meta['type']=='blob','production_carrier_mode')
        require(git(repo,'cat-file','blob',meta['oid'])==(ROOT/name).read_bytes(),'production_executed_carrier')
    # R preserves every old entry; only named executable carriers may change after R.
    # Requalify the original Native tuple against this actual event, never a new record alias.
    import f13_native_p3_documentation as documentation
    current=documentation.qualify(repo,EVENT_BASE,source,target,event['GITHUB_EVENT_NAME'],event['GITHUB_REF'],
        event['GITHUB_SHA'],event['VOICE_CI_PUSH_BEFORE_SHA'],event['GITHUB_BASE_REF'])
    retained=record['native_record']
    require((current['version'],current['record'],current['snapshot'],current['record_sha256'])==
        (retained['version'],retained['commit'],retained['source'],retained['sha256']),
        'production_transport_actual_native_binding')
    return dict(type=TRANSPORT_VERSION,record=TRANSPORT_RECORD_COMMIT,record_sha256=TRANSPORT_RECORD_SHA256,
        predecessor_record=SUCCESSOR_RECORD_COMMIT,historical_validation_source=old['source'],
        reviewed_source=record['source'],reviewed_tree=record['source_tree'],base=EVENT_BASE,source=source,target=target,
        historical_collector=HISTORICAL_SCOPE,native_content_baseline=NATIVE_CONTENT_BASE,
        current_inputs_sha256=record['current_inputs']['sha256'],event=event,
        admission=False,runtime_pass=False,current_or_future_path_authority=False)


def load_test_fixes(repo):
    """Exactly two reviewed test repairs; never a test-path permission."""
    previous,prior,old=load_transport(repo)
    require(TEST_SOURCE is not None and TEST_RECORD_COMMIT is not None and
        TEST_RECORD_SHA256 is not None,'production_test_fixes_not_pinned')
    raw=fixed_artifact(repo,TEST_RECORD_COMMIT,dict(path=TEST_RECORD_PATH,sha256=TEST_RECORD_SHA256))
    record=json.loads(raw,object_pairs_hook=gitutil.pairs)
    require(set(record)=={'version','status','source','source_tree','predecessor','changes','delta_sha256',
        'native_record','current_inputs','evidence','review','runtime_pass','activation_authority',
        'current_or_future_path_authority'},'production_test_fixes_shape')
    require(record['version']==TEST_VERSION and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR' and
        all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
        'production_test_fixes_no_authority')
    require(record['source']==TEST_SOURCE and record['predecessor']==dict(record=TRANSPORT_RECORD_COMMIT,
        path=TRANSPORT_RECORD_PATH,sha256=TRANSPORT_RECORD_SHA256,carrier=TEST_PREDECESSOR),
        'production_test_fixes_lineage')
    require(git(repo,'rev-list','--parents','-n','1',TEST_SOURCE).decode().split()==
        [TEST_SOURCE,TEST_PREDECESSOR],'production_test_fixes_source_parent')
    require(git(repo,'rev-list','--parents','-n','1',TEST_RECORD_COMMIT).decode().split()==
        [TEST_RECORD_COMMIT,TEST_SOURCE],'production_test_fixes_record_parent')
    require(record['source_tree']==git(repo,'rev-parse',TEST_SOURCE+'^{tree}').decode().strip(),
        'production_test_fixes_source_tree')
    git(repo,'merge-base','--is-ancestor',TRANSPORT_RECORD_COMMIT,TEST_PREDECESSOR)
    require(set(git(repo,'diff','--name-only',TRANSPORT_RECORD_COMMIT,TEST_PREDECESSOR).decode().splitlines())
        <=set(SUCCESSOR_CARRIERS),'production_test_fixes_predecessor_carrier')
    before,after=entries(repo,TEST_PREDECESSOR),entries(repo,TEST_SOURCE)
    require(set(before)==set(after) and {p for p in before if before[p]!=after[p]}==set(TEST_FIXES),
        'production_test_fixes_exact_two_paths')
    changes=[]
    for name,expected_sha in sorted(TEST_FIXES.items()):
        item=dict(path=name,disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
        for label,revision in (('before',TEST_PREDECESSOR),('after',TEST_SOURCE)):
            meta=gitutil.entry(repo,revision,name)
            require(meta['mode']=='100644' and meta['type']=='blob','production_test_fixes_file_mode')
            data=git(repo,'cat-file','blob',meta['oid'])
            item[label]=dict(entry=tree_entry(repo,revision,name),bytes=len(data),sha256=sha(data))
        require(item['after']['sha256']==expected_sha,'production_test_fixes_exact_bytes')
        changes.append(item)
    require(record['changes']==changes and record['delta_sha256']==
        sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',TEST_PREDECESSOR,TEST_SOURCE)),
        'production_test_fixes_exact_delta')
    require(record['native_record']==previous['native_record'],'production_test_fixes_retained_native_tuple')
    inventory=json.loads(fixed_artifact(repo,TEST_RECORD_COMMIT,record['current_inputs']),object_pairs_hook=gitutil.pairs)
    reference=entries(repo,REFERENCE)
    require(inventory==dict(source=TEST_SOURCE,reference=REFERENCE,
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',REFERENCE,TEST_SOURCE)),
        entries=[dict(path=p,reference_entry=reference.get(p,''),current_entry=after.get(p,''),
            disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
            for p in git(repo,'diff','--name-only',REFERENCE,TEST_SOURCE).decode().splitlines()]),
        'production_test_fixes_complete_inventory')
    require(isinstance(record['evidence'],list) and record['evidence'],'production_test_fixes_evidence')
    names=[]
    for proof in record['evidence']:
        require(isinstance(proof,dict) and isinstance(proof.get('path'),str),'production_artifact_shape')
        require(gitutil.entry(repo,TEST_RECORD_COMMIT,proof['path']) is not None,'production_artifact_missing')
        fixed_artifact(repo,TEST_RECORD_COMMIT,proof);names.append(proof['path'])
    require(names==sorted(set(names)) and isinstance(record['review'],list) and record['review'] and
        all(isinstance(p,str) and p in names for p in record['review']),'production_test_fixes_review')
    additions={TEST_RECORD_PATH,record['current_inputs']['path'],*names}
    require(len(additions)==2+len(names) and all(p not in after for p in additions),'production_test_fixes_new_evidence_only')
    recorded=entries(repo,TEST_RECORD_COMMIT)
    require(set(recorded)-set(after)==additions and all(recorded.get(p)==v for p,v in after.items()
        if p!='.agent/execplans/pr875-current-native-source.md'),'production_test_fixes_immutable_predecessor')
    return record,previous,prior,old


def qualify_test_fixes(repo):
    record,previous,prior,old=load_test_fixes(repo)
    target=git(repo,'rev-parse','HEAD').decode().strip()
    event={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    pr=event['GITHUB_EVENT_NAME']=='pull_request' and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',event['GITHUB_REF']) and event['GITHUB_BASE_REF']=='master'
    push=event['GITHUB_EVENT_NAME']=='push' and event['GITHUB_REF']=='refs/heads/master' and event['VOICE_CI_PUSH_BEFORE_SHA']==EVENT_BASE
    require(event['GITHUB_SHA']==target and (pr or push),'production_actual_master_event')
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and parents[1]==EVENT_BASE,'production_actual_ordered_parents')
    source=parents[2];git(repo,'merge-base','--is-ancestor',TEST_RECORD_COMMIT,source)
    require(source!=TEST_RECORD_COMMIT,'production_later_carrier')
    require(git(repo,'rev-parse',source+'^{tree}')==git(repo,'rev-parse',target+'^{tree}'),'production_source_result_tree')
    require(set(git(repo,'diff','--name-only',TEST_RECORD_COMMIT,source).decode().splitlines())<=set(SUCCESSOR_CARRIERS),
        'production_only_reviewed_carrier_delta')
    for name in SUCCESSOR_CARRIERS:
        meta=gitutil.entry(repo,source,name)
        require(meta['mode']=='100644' and meta['type']=='blob','production_carrier_mode')
        require(git(repo,'cat-file','blob',meta['oid'])==(ROOT/name).read_bytes(),'production_executed_carrier')
    # R preserves every old entry; only named executable carriers may change after R.
    # Requalify the original Native tuple against this actual event, never a new record alias.
    import f13_native_p3_documentation as documentation
    current=documentation.qualify(repo,EVENT_BASE,source,target,event['GITHUB_EVENT_NAME'],event['GITHUB_REF'],
        event['GITHUB_SHA'],event['VOICE_CI_PUSH_BEFORE_SHA'],event['GITHUB_BASE_REF'])
    retained=record['native_record']
    require((current['version'],current['record'],current['snapshot'],current['record_sha256'])==
        (retained['version'],retained['commit'],retained['source'],retained['sha256']),
        'production_transport_actual_native_binding')
    return dict(type=TEST_VERSION,record=TEST_RECORD_COMMIT,record_sha256=TEST_RECORD_SHA256,
        predecessor_record=TRANSPORT_RECORD_COMMIT,historical_validation_source=old['source'],
        reviewed_source=record['source'],reviewed_tree=record['source_tree'],base=EVENT_BASE,source=source,target=target,
        historical_collector=HISTORICAL_SCOPE,native_content_baseline=NATIVE_CONTENT_BASE,
        current_inputs_sha256=record['current_inputs']['sha256'],event=event,
        admission=False,runtime_pass=False,current_or_future_path_authority=False)


def load_rendering_fix(repo):
    """Exact historical rendering source repair; no helper-path permission."""
    previous,transport,prior,old=load_test_fixes(repo)
    require(RENDER_SOURCE is not None and RENDER_RECORD_COMMIT is not None and
        RENDER_RECORD_SHA256 is not None,'production_rendering_fix_not_pinned')
    raw=fixed_artifact(repo,RENDER_RECORD_COMMIT,dict(path=RENDER_RECORD_PATH,sha256=RENDER_RECORD_SHA256))
    record=json.loads(raw,object_pairs_hook=gitutil.pairs)
    require(set(record)=={'version','status','source','source_tree','predecessor','changes','delta_sha256',
        'native_record','current_inputs','evidence','review','runtime_pass','activation_authority',
        'current_or_future_path_authority'},'production_rendering_fix_shape')
    require(record['version']==RENDER_VERSION and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR' and
        all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
        'production_rendering_fix_no_authority')
    require(record['source']==RENDER_SOURCE and record['predecessor']==dict(record=TEST_RECORD_COMMIT,
        path=TEST_RECORD_PATH,sha256=TEST_RECORD_SHA256,carrier=RENDER_PREDECESSOR),
        'production_rendering_fix_lineage')
    require(git(repo,'rev-list','--parents','-n','1',RENDER_SOURCE).decode().split()==
        [RENDER_SOURCE,RENDER_PREDECESSOR],'production_rendering_fix_source_parent')
    require(git(repo,'rev-list','--parents','-n','1',RENDER_RECORD_COMMIT).decode().split()==
        [RENDER_RECORD_COMMIT,RENDER_SOURCE],'production_rendering_fix_record_parent')
    require(record['source_tree']==git(repo,'rev-parse',RENDER_SOURCE+'^{tree}').decode().strip(),
        'production_rendering_fix_source_tree')
    git(repo,'merge-base','--is-ancestor',TEST_RECORD_COMMIT,RENDER_PREDECESSOR)
    require(set(git(repo,'diff','--name-only',TEST_RECORD_COMMIT,RENDER_PREDECESSOR).decode().splitlines())
        <=set(SUCCESSOR_CARRIERS),'production_rendering_fix_predecessor_carrier')
    before,after=entries(repo,RENDER_PREDECESSOR),entries(repo,RENDER_SOURCE)
    require(set(before)==set(after) and {p for p in before if before[p]!=after[p]}==set(RENDER_FIXES),
        'production_rendering_fix_exact_two_paths')
    changes=[]
    for name,expected_sha in sorted(RENDER_FIXES.items()):
        item=dict(path=name,disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
        for label,revision in (('before',RENDER_PREDECESSOR),('after',RENDER_SOURCE)):
            meta=gitutil.entry(repo,revision,name)
            require(meta['mode']=='100644' and meta['type']=='blob','production_rendering_fix_file_mode')
            data=git(repo,'cat-file','blob',meta['oid'])
            item[label]=dict(entry=tree_entry(repo,revision,name),bytes=len(data),sha256=sha(data))
        require(item['after']['sha256']==expected_sha,'production_rendering_fix_exact_bytes')
        changes.append(item)
    require(record['changes']==changes and record['delta_sha256']==
        sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',RENDER_PREDECESSOR,RENDER_SOURCE)),
        'production_rendering_fix_exact_delta')
    require(record['native_record']==previous['native_record'],'production_rendering_fix_retained_native_tuple')
    inventory=json.loads(fixed_artifact(repo,RENDER_RECORD_COMMIT,record['current_inputs']),object_pairs_hook=gitutil.pairs)
    reference=entries(repo,REFERENCE)
    require(inventory==dict(source=RENDER_SOURCE,reference=REFERENCE,
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',REFERENCE,RENDER_SOURCE)),
        entries=[dict(path=p,reference_entry=reference.get(p,''),current_entry=after.get(p,''),
            disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
            for p in git(repo,'diff','--name-only',REFERENCE,RENDER_SOURCE).decode().splitlines()]),
        'production_rendering_fix_complete_inventory')
    require(isinstance(record['evidence'],list) and record['evidence'],'production_rendering_fix_evidence')
    names=[]
    for proof in record['evidence']:
        require(isinstance(proof,dict) and isinstance(proof.get('path'),str),'production_artifact_shape')
        require(gitutil.entry(repo,RENDER_RECORD_COMMIT,proof['path']) is not None,'production_artifact_missing')
        fixed_artifact(repo,RENDER_RECORD_COMMIT,proof);names.append(proof['path'])
    require(names==sorted(set(names)) and isinstance(record['review'],list) and record['review'] and
        all(isinstance(p,str) and p in names for p in record['review']),'production_rendering_fix_review')
    additions={RENDER_RECORD_PATH,record['current_inputs']['path'],*names}
    require(len(additions)==2+len(names) and all(p not in after for p in additions),'production_rendering_fix_new_evidence_only')
    recorded=entries(repo,RENDER_RECORD_COMMIT)
    require(set(recorded)-set(after)==additions and all(recorded.get(p)==v for p,v in after.items()
        if p!='.agent/execplans/pr875-current-native-source.md'),'production_rendering_fix_immutable_predecessor')
    return record,previous,transport,prior,old


def qualify_rendering_fix(repo):
    record,previous,transport,prior,old=load_rendering_fix(repo)
    target=git(repo,'rev-parse','HEAD').decode().strip()
    event={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    pr=event['GITHUB_EVENT_NAME']=='pull_request' and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',event['GITHUB_REF']) and event['GITHUB_BASE_REF']=='master'
    push=event['GITHUB_EVENT_NAME']=='push' and event['GITHUB_REF']=='refs/heads/master' and event['VOICE_CI_PUSH_BEFORE_SHA']==EVENT_BASE
    require(event['GITHUB_SHA']==target and (pr or push),'production_actual_master_event')
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and parents[1]==EVENT_BASE,'production_actual_ordered_parents')
    source=parents[2];git(repo,'merge-base','--is-ancestor',RENDER_RECORD_COMMIT,source)
    require(source!=RENDER_RECORD_COMMIT,'production_later_carrier')
    require(git(repo,'rev-parse',source+'^{tree}')==git(repo,'rev-parse',target+'^{tree}'),'production_source_result_tree')
    require(set(git(repo,'diff','--name-only',RENDER_RECORD_COMMIT,source).decode().splitlines())<=set(SUCCESSOR_CARRIERS),
        'production_only_reviewed_carrier_delta')
    for name in SUCCESSOR_CARRIERS:
        meta=gitutil.entry(repo,source,name)
        require(meta['mode']=='100644' and meta['type']=='blob','production_carrier_mode')
        require(git(repo,'cat-file','blob',meta['oid'])==(ROOT/name).read_bytes(),'production_executed_carrier')
    # R preserves every old entry; only named executable carriers may change after R.
    # Requalify the original Native tuple against this actual event, never a new record alias.
    import f13_native_p3_documentation as documentation
    current=documentation.qualify(repo,EVENT_BASE,source,target,event['GITHUB_EVENT_NAME'],event['GITHUB_REF'],
        event['GITHUB_SHA'],event['VOICE_CI_PUSH_BEFORE_SHA'],event['GITHUB_BASE_REF'])
    retained=record['native_record']
    require((current['version'],current['record'],current['snapshot'],current['record_sha256'])==
        (retained['version'],retained['commit'],retained['source'],retained['sha256']),
        'production_transport_actual_native_binding')
    return dict(type=RENDER_VERSION,record=RENDER_RECORD_COMMIT,record_sha256=RENDER_RECORD_SHA256,
        predecessor_record=TEST_RECORD_COMMIT,historical_validation_source=old['source'],
        reviewed_source=record['source'],reviewed_tree=record['source_tree'],base=EVENT_BASE,source=source,target=target,
        historical_collector=HISTORICAL_SCOPE,native_content_baseline=NATIVE_CONTENT_BASE,
        current_inputs_sha256=record['current_inputs']['sha256'],event=event,
        admission=False,runtime_pass=False,current_or_future_path_authority=False)


def load_social_fix(repo):
    """Exact Social subscription readiness repair; no test-path permission."""
    previous,rendering,transport,prior,old=load_rendering_fix(repo)
    require(SOCIAL_SOURCE is not None and SOCIAL_RECORD_COMMIT is not None and
        SOCIAL_RECORD_SHA256 is not None,'production_social_fix_not_pinned')
    raw=fixed_artifact(repo,SOCIAL_RECORD_COMMIT,dict(path=SOCIAL_RECORD_PATH,sha256=SOCIAL_RECORD_SHA256))
    record=json.loads(raw,object_pairs_hook=gitutil.pairs)
    require(set(record)=={'version','status','source','source_tree','predecessor','changes','delta_sha256',
        'native_record','current_inputs','evidence','review','runtime_pass','activation_authority',
        'current_or_future_path_authority'},'production_social_fix_shape')
    require(record['version']==SOCIAL_VERSION and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR' and
        all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
        'production_social_fix_no_authority')
    require(record['source']==SOCIAL_SOURCE and record['predecessor']==dict(record=RENDER_RECORD_COMMIT,
        path=RENDER_RECORD_PATH,sha256=RENDER_RECORD_SHA256,carrier=SOCIAL_PREDECESSOR),
        'production_social_fix_lineage')
    require(git(repo,'rev-list','--parents','-n','1',SOCIAL_SOURCE).decode().split()==
        [SOCIAL_SOURCE,SOCIAL_PREDECESSOR],'production_social_fix_source_parent')
    require(git(repo,'rev-list','--parents','-n','1',SOCIAL_RECORD_COMMIT).decode().split()==
        [SOCIAL_RECORD_COMMIT,SOCIAL_SOURCE],'production_social_fix_record_parent')
    require(record['source_tree']==git(repo,'rev-parse',SOCIAL_SOURCE+'^{tree}').decode().strip(),
        'production_social_fix_source_tree')
    git(repo,'merge-base','--is-ancestor',RENDER_RECORD_COMMIT,SOCIAL_PREDECESSOR)
    require(set(git(repo,'diff','--name-only',RENDER_RECORD_COMMIT,SOCIAL_PREDECESSOR).decode().splitlines())
        <=set(SUCCESSOR_CARRIERS),'production_social_fix_predecessor_carrier')
    before,after=entries(repo,SOCIAL_PREDECESSOR),entries(repo,SOCIAL_SOURCE)
    require(set(before)==set(after) and {p for p in before if before[p]!=after[p]}==set(SOCIAL_FIXES),
        'production_social_fix_exact_one_path')
    changes=[]
    for name,expected_sha in sorted(SOCIAL_FIXES.items()):
        item=dict(path=name,disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
        for label,revision in (('before',SOCIAL_PREDECESSOR),('after',SOCIAL_SOURCE)):
            meta=gitutil.entry(repo,revision,name)
            require(meta['mode']=='100644' and meta['type']=='blob','production_social_fix_file_mode')
            data=git(repo,'cat-file','blob',meta['oid'])
            item[label]=dict(entry=tree_entry(repo,revision,name),bytes=len(data),sha256=sha(data))
        require(item['after']['sha256']==expected_sha,'production_social_fix_exact_bytes')
        changes.append(item)
    require(record['changes']==changes and record['delta_sha256']==
        sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',SOCIAL_PREDECESSOR,SOCIAL_SOURCE)),
        'production_social_fix_exact_delta')
    require(record['native_record']==previous['native_record'],'production_social_fix_retained_native_tuple')
    inventory=json.loads(fixed_artifact(repo,SOCIAL_RECORD_COMMIT,record['current_inputs']),object_pairs_hook=gitutil.pairs)
    reference=entries(repo,REFERENCE)
    require(inventory==dict(source=SOCIAL_SOURCE,reference=REFERENCE,
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',REFERENCE,SOCIAL_SOURCE)),
        entries=[dict(path=p,reference_entry=reference.get(p,''),current_entry=after.get(p,''),
            disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
            for p in git(repo,'diff','--name-only',REFERENCE,SOCIAL_SOURCE).decode().splitlines()]),
        'production_social_fix_complete_inventory')
    require(isinstance(record['evidence'],list) and record['evidence'],'production_social_fix_evidence')
    names=[]
    for proof in record['evidence']:
        require(isinstance(proof,dict) and isinstance(proof.get('path'),str),'production_artifact_shape')
        require(gitutil.entry(repo,SOCIAL_RECORD_COMMIT,proof['path']) is not None,'production_artifact_missing')
        fixed_artifact(repo,SOCIAL_RECORD_COMMIT,proof);names.append(proof['path'])
    require(names==sorted(set(names)) and isinstance(record['review'],list) and record['review'] and
        all(isinstance(p,str) and p in names for p in record['review']),'production_social_fix_review')
    additions={SOCIAL_RECORD_PATH,record['current_inputs']['path'],*names}
    require(len(additions)==2+len(names) and all(p not in after for p in additions),'production_social_fix_new_evidence_only')
    recorded=entries(repo,SOCIAL_RECORD_COMMIT)
    require(set(recorded)-set(after)==additions and all(recorded.get(p)==v for p,v in after.items()
        if p!='.agent/execplans/pr875-current-native-source.md'),'production_social_fix_immutable_predecessor')
    return record,previous,rendering,transport,prior,old


def qualify_social_fix(repo):
    record,previous,rendering,transport,prior,old=load_social_fix(repo)
    target=git(repo,'rev-parse','HEAD').decode().strip()
    event={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    pr=event['GITHUB_EVENT_NAME']=='pull_request' and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',event['GITHUB_REF']) and event['GITHUB_BASE_REF']=='master'
    push=event['GITHUB_EVENT_NAME']=='push' and event['GITHUB_REF']=='refs/heads/master' and event['VOICE_CI_PUSH_BEFORE_SHA']==EVENT_BASE
    require(event['GITHUB_SHA']==target and (pr or push),'production_actual_master_event')
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and parents[1]==EVENT_BASE,'production_actual_ordered_parents')
    source=parents[2];git(repo,'merge-base','--is-ancestor',SOCIAL_RECORD_COMMIT,source)
    require(source!=SOCIAL_RECORD_COMMIT,'production_later_carrier')
    require(git(repo,'rev-parse',source+'^{tree}')==git(repo,'rev-parse',target+'^{tree}'),'production_source_result_tree')
    require(set(git(repo,'diff','--name-only',SOCIAL_RECORD_COMMIT,source).decode().splitlines())<=set(SUCCESSOR_CARRIERS),
        'production_only_reviewed_carrier_delta')
    for name in SUCCESSOR_CARRIERS:
        meta=gitutil.entry(repo,source,name)
        require(meta['mode']=='100644' and meta['type']=='blob','production_carrier_mode')
        require(git(repo,'cat-file','blob',meta['oid'])==(ROOT/name).read_bytes(),'production_executed_carrier')
    # R preserves every old entry; only named executable carriers may change after R.
    # Requalify the original Native tuple against this actual event, never a new record alias.
    import f13_native_p3_documentation as documentation
    current=documentation.qualify(repo,EVENT_BASE,source,target,event['GITHUB_EVENT_NAME'],event['GITHUB_REF'],
        event['GITHUB_SHA'],event['VOICE_CI_PUSH_BEFORE_SHA'],event['GITHUB_BASE_REF'])
    retained=record['native_record']
    require((current['version'],current['record'],current['snapshot'],current['record_sha256'])==
        (retained['version'],retained['commit'],retained['source'],retained['sha256']),
        'production_transport_actual_native_binding')
    return dict(type=SOCIAL_VERSION,record=SOCIAL_RECORD_COMMIT,record_sha256=SOCIAL_RECORD_SHA256,
        predecessor_record=RENDER_RECORD_COMMIT,historical_validation_source=old['source'],
        reviewed_source=record['source'],reviewed_tree=record['source_tree'],base=EVENT_BASE,source=source,target=target,
        historical_collector=HISTORICAL_SCOPE,native_content_baseline=NATIVE_CONTENT_BASE,
        current_inputs_sha256=record['current_inputs']['sha256'],event=event,
        admission=False,runtime_pass=False,current_or_future_path_authority=False)


def load_integration(repo):
    prior=load_social_fix(repo)
    require(INTEGRATION_SOURCE is not None and INTEGRATION_RECORD_COMMIT is not None
        and INTEGRATION_RECORD_SHA256 is not None,'production_integration_not_pinned')
    raw=fixed_artifact(repo,INTEGRATION_RECORD_COMMIT,dict(path=INTEGRATION_RECORD_PATH,sha256=INTEGRATION_RECORD_SHA256))
    record=json.loads(raw,object_pairs_hook=gitutil.pairs)
    require(set(record)=={'version','status','source','source_tree','predecessor','integration','event_base',
        'historical_event_base','changes','delta_sha256','native_record','current_inputs','evidence','review',
        'runtime_pass','activation_authority','current_or_future_path_authority'},'production_integration_shape')
    require(record['version']==INTEGRATION_VERSION and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR'
        and all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
        'production_integration_no_authority')
    require(record['source']==INTEGRATION_SOURCE and record['predecessor']==dict(record=SOCIAL_RECORD_COMMIT,
        path=SOCIAL_RECORD_PATH,sha256=SOCIAL_RECORD_SHA256,carrier=INTEGRATION_PREDECESSOR),
        'production_integration_lineage')
    require(record['event_base']==INTEGRATION_EVENT_BASE and record['historical_event_base']==EVENT_BASE,
        'production_integration_distinct_event_base')
    require(git(repo,'rev-list','--parents','-n','1',INTEGRATION_MERGE).decode().split()==
        [INTEGRATION_MERGE,INTEGRATION_PREDECESSOR,INTEGRATION_EVENT_BASE],'production_integration_ordered_master_merge')
    chain=git(repo,'rev-list','--first-parent','--reverse',INTEGRATION_PREDECESSOR+'..'+INTEGRATION_SOURCE).decode().splitlines()
    require(chain==list(INTEGRATION_CHAIN),'production_integration_complete_source_chain')
    require(record['integration']==dict(master=INTEGRATION_EVENT_BASE,merge=INTEGRATION_MERGE,
        ordered_parents=[INTEGRATION_PREDECESSOR,INTEGRATION_EVENT_BASE],source_chain=[dict(commit=c,
        parents=git(repo,'rev-list','--parents','-n','1',c).decode().split()[1:],
        tree=git(repo,'rev-parse',c+'^{tree}').decode().strip()) for c in chain]),'production_integration_source_chain_binding')
    require(git(repo,'rev-list','--parents','-n','1',INTEGRATION_RECORD_COMMIT).decode().split()==
        [INTEGRATION_RECORD_COMMIT,INTEGRATION_SOURCE],'production_integration_record_parent')
    require(record['source_tree']==git(repo,'rev-parse',INTEGRATION_SOURCE+'^{tree}').decode().strip(),
        'production_integration_source_tree')
    git(repo,'merge-base','--is-ancestor',SOCIAL_RECORD_COMMIT,INTEGRATION_PREDECESSOR)
    require(set(git(repo,'diff','--name-only',SOCIAL_RECORD_COMMIT,INTEGRATION_PREDECESSOR).decode().splitlines())
        <=set(SUCCESSOR_CARRIERS),'production_integration_predecessor_carriers')
    before={p:v.rstrip('\0') for p,v in entries(repo,INTEGRATION_PREDECESSOR).items()}
    after={p:v.rstrip('\0') for p,v in entries(repo,INTEGRATION_SOURCE).items()}
    changes=[]
    for name in git(repo,'diff','--name-only',INTEGRATION_PREDECESSOR,INTEGRATION_SOURCE).decode().splitlines():
        item=dict(path=name,disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
        for label,tree in (('before',before),('after',after)):
            entry=tree.get(name,'');data=git(repo,'cat-file','blob',entry.split('\t')[0].split()[2]) if entry else b''
            item[label]=dict(entry=entry,bytes=len(data),sha256=sha(data))
        changes.append(item)
    require(record['changes']==changes and record['delta_sha256']==
        sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',INTEGRATION_PREDECESSOR,INTEGRATION_SOURCE)),
        'production_integration_complete_delta')
    reference={p:v.rstrip('\0') for p,v in entries(repo,REFERENCE).items()}
    inventory=json.loads(fixed_artifact(repo,INTEGRATION_RECORD_COMMIT,record['current_inputs']),object_pairs_hook=gitutil.pairs)
    require(inventory==dict(source=INTEGRATION_SOURCE,reference=REFERENCE,
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',REFERENCE,INTEGRATION_SOURCE)),
        entries=[dict(path=p,reference_entry=reference.get(p,''),current_entry=after.get(p,''),
        disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED') for p in git(repo,'diff','--name-only',REFERENCE,INTEGRATION_SOURCE).decode().splitlines()]),
        'production_integration_complete_inventory')
    import f13_native_p3_documentation as documentation
    native=documentation.load_context_record(repo)
    require(documentation.CONTEXT_RECORD_COMMIT==INTEGRATION_RECORD_COMMIT and native['source']==INTEGRATION_SOURCE,
        'production_integration_same_record_native')
    require(record['native_record']==dict(version=documentation.CONTEXT_VERSION,source=INTEGRATION_SOURCE,
        path=documentation.CONTEXT_RECORD_PATH,sha256=documentation.CONTEXT_RECORD_SHA256,
        record_relation='SAME_SOURCE_RECORD_COMMIT'),'production_integration_native_tuple')
    names=[]
    for proof in record['evidence']:
        fixed_artifact(repo,INTEGRATION_RECORD_COMMIT,proof);names.append(proof['path'])
    require(names and names==sorted(set(names)) and record['review'] and
        all(p in names for p in record['review']),'production_integration_review')
    additions={INTEGRATION_RECORD_PATH,documentation.CONTEXT_RECORD_PATH,record['current_inputs']['path'],*names}
    recorded={p:v.rstrip('\0') for p,v in entries(repo,INTEGRATION_RECORD_COMMIT).items()}
    require(len(additions)==3+len(names) and not set(after)&additions and set(recorded)-set(after)==additions
        and all(recorded.get(p)==v for p,v in after.items() if p!='.agent/execplans/pr875-current-native-source.md'),
        'production_integration_immutable_source_and_named_metadata')
    return (record,*prior)


def qualify_integration(repo):
    records=load_integration(repo);record,old=records[0],records[-1]
    target=git(repo,'rev-parse','HEAD').decode().strip()
    event={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    pr=event['GITHUB_EVENT_NAME']=='pull_request' and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',event['GITHUB_REF']) and event['GITHUB_BASE_REF']=='master'
    push=event['GITHUB_EVENT_NAME']=='push' and event['GITHUB_REF']=='refs/heads/master' and event['VOICE_CI_PUSH_BEFORE_SHA']==INTEGRATION_EVENT_BASE
    require(event['GITHUB_SHA']==target and (pr or push),'production_actual_master_event')
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and parents[1]==INTEGRATION_EVENT_BASE,'production_actual_ordered_parents')
    source=parents[2];git(repo,'merge-base','--is-ancestor',INTEGRATION_RECORD_COMMIT,source)
    require(source!=INTEGRATION_RECORD_COMMIT,'production_later_carrier')
    require(git(repo,'rev-parse',source+'^{tree}')==git(repo,'rev-parse',target+'^{tree}'),'production_source_result_tree')
    require(set(git(repo,'diff','--name-only',INTEGRATION_RECORD_COMMIT,source).decode().splitlines())<=set(SUCCESSOR_CARRIERS),
        'production_only_reviewed_carrier_delta')
    for name in SUCCESSOR_CARRIERS:
        meta=gitutil.entry(repo,source,name)
        require(meta['mode']=='100644' and meta['type']=='blob','production_carrier_mode')
        require(git(repo,'cat-file','blob',meta['oid'])==(ROOT/name).read_bytes(),'production_executed_carrier')
    import f13_native_p3_documentation as documentation
    current=documentation.qualify_context(repo,INTEGRATION_EVENT_BASE,source,target,event['GITHUB_EVENT_NAME'],event['GITHUB_REF'],
        event['GITHUB_SHA'],event['VOICE_CI_PUSH_BEFORE_SHA'],event['GITHUB_BASE_REF'])
    require((current['version'],current['record'],current['snapshot'],current['record_sha256'])==
        (record['native_record']['version'],INTEGRATION_RECORD_COMMIT,INTEGRATION_SOURCE,record['native_record']['sha256']),
        'production_integration_actual_native_binding')
    return dict(type=INTEGRATION_VERSION,record=INTEGRATION_RECORD_COMMIT,record_sha256=INTEGRATION_RECORD_SHA256,
        predecessor_record=SOCIAL_RECORD_COMMIT,historical_validation_source=old['source'],
        reviewed_source=record['source'],reviewed_tree=record['source_tree'],base=INTEGRATION_EVENT_BASE,source=source,target=target,
        historical_collector=HISTORICAL_SCOPE,native_content_baseline=NATIVE_CONTENT_BASE,
        current_inputs_sha256=record['current_inputs']['sha256'],event=event,
        admission=False,runtime_pass=False,current_or_future_path_authority=False)


def load_fixture_fix(repo):
    prior=load_integration(repo)
    require(FIXTURE_RECORD_COMMIT is not None and FIXTURE_RECORD_SHA256 is not None,'production_fixture_not_pinned')
    record=json.loads(fixed_artifact(repo,FIXTURE_RECORD_COMMIT,dict(path=FIXTURE_RECORD_PATH,
        sha256=FIXTURE_RECORD_SHA256)),object_pairs_hook=gitutil.pairs)
    require(set(record)=={'version','status','source','source_tree','predecessor','event_base','changes',
        'delta_sha256','native_record','native_closure','current_inputs','evidence','review','runtime_pass',
        'activation_authority','current_or_future_path_authority'},'production_fixture_shape')
    require(record['version']==FIXTURE_VERSION and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR'
        and all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
        'production_fixture_no_authority')
    require(record['source']==FIXTURE_SOURCE and record['predecessor']==dict(record=INTEGRATION_RECORD_COMMIT,
        path=INTEGRATION_RECORD_PATH,sha256=INTEGRATION_RECORD_SHA256,carrier=FIXTURE_PREDECESSOR),
        'production_fixture_lineage')
    require(record['event_base']==INTEGRATION_EVENT_BASE,'production_fixture_event_base')
    require(git(repo,'rev-list','--parents','-n','1',FIXTURE_SOURCE).decode().split()==
        [FIXTURE_SOURCE,FIXTURE_PREDECESSOR],'production_fixture_source_parent')
    require(git(repo,'rev-list','--parents','-n','1',FIXTURE_RECORD_COMMIT).decode().split()==
        [FIXTURE_RECORD_COMMIT,FIXTURE_SOURCE],'production_fixture_record_parent')
    require(record['source_tree']==git(repo,'rev-parse',FIXTURE_SOURCE+'^{tree}').decode().strip(),
        'production_fixture_source_tree')
    git(repo,'merge-base','--is-ancestor',INTEGRATION_RECORD_COMMIT,FIXTURE_PREDECESSOR)
    require(set(git(repo,'diff','--name-only',INTEGRATION_RECORD_COMMIT,FIXTURE_PREDECESSOR).decode().splitlines())
        <=set(SUCCESSOR_CARRIERS),'production_fixture_predecessor_carriers')
    before={p:v.rstrip('\0') for p,v in entries(repo,FIXTURE_PREDECESSOR).items()}
    after={p:v.rstrip('\0') for p,v in entries(repo,FIXTURE_SOURCE).items()}
    require({p:v for p,v in before.items() if p!=FIXTURE_PATH}=={p:v for p,v in after.items() if p!=FIXTURE_PATH},
        'production_fixture_only_exact_test')
    change=dict(path=FIXTURE_PATH,disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
    for label,tree,expected in (('before',before,FIXTURE_BEFORE_SHA256),('after',after,FIXTURE_AFTER_SHA256)):
        entry=tree[FIXTURE_PATH];require(entry.startswith('100644 blob '),'production_fixture_file_mode')
        raw=git(repo,'cat-file','blob',entry.split()[2]);require(sha(raw)==expected,'production_fixture_exact_bytes')
        change[label]=dict(entry=entry,bytes=len(raw),sha256=sha(raw))
    require(record['changes']==[change] and record['delta_sha256']==sha(git(repo,'diff','--raw','--no-abbrev',
        '--no-renames',FIXTURE_PREDECESSOR,FIXTURE_SOURCE)),'production_fixture_complete_delta')
    reference={p:v.rstrip('\0') for p,v in entries(repo,REFERENCE).items()}
    inventory=json.loads(fixed_artifact(repo,FIXTURE_RECORD_COMMIT,record['current_inputs']),object_pairs_hook=gitutil.pairs)
    require(inventory==dict(source=FIXTURE_SOURCE,reference=REFERENCE,
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',REFERENCE,FIXTURE_SOURCE)),
        entries=[dict(path=p,reference_entry=reference.get(p,''),current_entry=after.get(p,''),
        disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED') for p in git(repo,'diff','--name-only',REFERENCE,FIXTURE_SOURCE).decode().splitlines()]),
        'production_fixture_complete_inventory')
    import f13_native_p3_documentation as documentation
    native=documentation.load_context_record(repo)
    expected_native=dict(version=documentation.CONTEXT_VERSION,source=documentation.CONTEXT_SOURCE,
        record=documentation.CONTEXT_RECORD_COMMIT,path=documentation.CONTEXT_RECORD_PATH,sha256=documentation.CONTEXT_RECORD_SHA256)
    require(record['native_record']==expected_native,'production_fixture_retained_native_tuple')
    raw_native=json.loads(fixed_artifact(repo,documentation.CONTEXT_RECORD_COMMIT,
        dict(path=documentation.CONTEXT_RECORD_PATH,sha256=documentation.CONTEXT_RECORD_SHA256)),object_pairs_hook=gitutil.pairs)
    closure=raw_native['closure'];require(record['native_closure']==closure,'production_fixture_native_closure')
    native_tree=documentation.tree_entries(repo,documentation.CONTEXT_SOURCE)
    for name in set(closure['projection_paths']+closure['context_paths']+closure['compiled_paths']):
        require(name!=FIXTURE_PATH and native_tree.get(name)==after.get(name),'production_fixture_native_entry')
    directories=directory_entries(after)
    for row in closure['directory_inventories']:
        require(directories.get(row['directory'],{})==row['entries'],
            'production_fixture_native_directory')
    require({p:v for p,v in native_tree.items() if p.startswith('src/backend/voice/')}==
        {p:v for p,v in after.items() if p.startswith('src/backend/voice/')},'production_fixture_voice_tree')
    names=[]
    for proof in record['evidence']:
        fixed_artifact(repo,FIXTURE_RECORD_COMMIT,proof);names.append(proof['path'])
    require(names and names==sorted(set(names)) and record['review'] and all(p in names for p in record['review']),
        'production_fixture_review')
    additions={FIXTURE_RECORD_PATH,record['current_inputs']['path'],*names}
    recorded={p:v.rstrip('\0') for p,v in entries(repo,FIXTURE_RECORD_COMMIT).items()}
    require(len(additions)==2+len(names) and not set(after)&additions and set(recorded)-set(after)==additions
        and all(recorded.get(p)==v for p,v in after.items() if p!='.agent/execplans/pr875-current-native-source.md'),
        'production_fixture_immutable_source_and_named_metadata')
    return (record,*prior)


def qualify_fixture_fix(repo):
    records=load_fixture_fix(repo);record,old=records[0],records[-1]
    target=git(repo,'rev-parse','HEAD').decode().strip()
    event={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    pr=event['GITHUB_EVENT_NAME']=='pull_request' and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',event['GITHUB_REF']) and event['GITHUB_BASE_REF']=='master'
    push=event['GITHUB_EVENT_NAME']=='push' and event['GITHUB_REF']=='refs/heads/master' and event['VOICE_CI_PUSH_BEFORE_SHA']==INTEGRATION_EVENT_BASE
    require(event['GITHUB_SHA']==target and (pr or push),'production_actual_master_event')
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and parents[1]==INTEGRATION_EVENT_BASE,'production_actual_ordered_parents')
    source=parents[2];git(repo,'merge-base','--is-ancestor',FIXTURE_RECORD_COMMIT,source)
    require(source!=FIXTURE_RECORD_COMMIT,'production_later_carrier')
    require(git(repo,'rev-parse',source+'^{tree}')==git(repo,'rev-parse',target+'^{tree}'),'production_source_result_tree')
    require(set(git(repo,'diff','--name-only',FIXTURE_RECORD_COMMIT,source).decode().splitlines())<=set(SUCCESSOR_CARRIERS),
        'production_only_reviewed_carrier_delta')
    for name in SUCCESSOR_CARRIERS:
        meta=gitutil.entry(repo,source,name)
        require(meta['mode']=='100644' and meta['type']=='blob','production_carrier_mode')
        require(git(repo,'cat-file','blob',meta['oid'])==(ROOT/name).read_bytes(),'production_executed_carrier')
    import f13_native_p3_documentation as documentation
    current=documentation.qualify_context(repo,INTEGRATION_EVENT_BASE,source,target,event['GITHUB_EVENT_NAME'],event['GITHUB_REF'],
        event['GITHUB_SHA'],event['VOICE_CI_PUSH_BEFORE_SHA'],event['GITHUB_BASE_REF'])
    require(record['native_record']==dict(version=current['version'],source=current['snapshot'],
        record=current['record'],path=documentation.CONTEXT_RECORD_PATH,sha256=current['record_sha256']),
        'production_fixture_actual_retained_native')
    require(current['source_applicability']==documentation.fixture_bridge_binding(source,target),
        'production_fixture_current_native_bridge')
    return dict(type=FIXTURE_VERSION,record=FIXTURE_RECORD_COMMIT,record_sha256=FIXTURE_RECORD_SHA256,
        predecessor_record=INTEGRATION_RECORD_COMMIT,historical_validation_source=old['source'],
        reviewed_source=record['source'],reviewed_tree=record['source_tree'],base=INTEGRATION_EVENT_BASE,source=source,target=target,
        historical_collector=HISTORICAL_SCOPE,native_content_baseline=NATIVE_CONTENT_BASE,
        current_inputs_sha256=record['current_inputs']['sha256'],event=event,
        admission=False,runtime_pass=False,current_or_future_path_authority=False)

def load_bootstrap_fix(repo):
    prior=load_fixture_fix(repo)
    require(BOOTSTRAP_RECORD_COMMIT is not None and BOOTSTRAP_RECORD_SHA256 is not None,'production_bootstrap_not_pinned')
    record=json.loads(fixed_artifact(repo,BOOTSTRAP_RECORD_COMMIT,dict(path=BOOTSTRAP_RECORD_PATH,
        sha256=BOOTSTRAP_RECORD_SHA256)),object_pairs_hook=gitutil.pairs)
    require(set(record)=={'version','status','source','source_tree','predecessor','event_base','changes',
        'delta_sha256','native_record','native_closure','current_inputs','evidence','review','runtime_pass',
        'activation_authority','current_or_future_path_authority'},'production_bootstrap_shape')
    require(record['version']==BOOTSTRAP_VERSION and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR'
        and all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
        'production_bootstrap_no_authority')
    require(record['source']==BOOTSTRAP_SOURCE and record['predecessor']==dict(record=FIXTURE_RECORD_COMMIT,
        path=FIXTURE_RECORD_PATH,sha256=FIXTURE_RECORD_SHA256,carrier=BOOTSTRAP_PREDECESSOR),
        'production_bootstrap_lineage')
    require(record['event_base']==INTEGRATION_EVENT_BASE,'production_bootstrap_event_base')
    require(git(repo,'rev-list','--parents','-n','1',BOOTSTRAP_SOURCE).decode().split()==
        [BOOTSTRAP_SOURCE,BOOTSTRAP_PREDECESSOR],'production_bootstrap_source_parent')
    require(git(repo,'rev-list','--parents','-n','1',BOOTSTRAP_RECORD_COMMIT).decode().split()==
        [BOOTSTRAP_RECORD_COMMIT,BOOTSTRAP_SOURCE],'production_bootstrap_record_parent')
    require(record['source_tree']==git(repo,'rev-parse',BOOTSTRAP_SOURCE+'^{tree}').decode().strip(),
        'production_bootstrap_source_tree')
    git(repo,'merge-base','--is-ancestor',FIXTURE_RECORD_COMMIT,BOOTSTRAP_PREDECESSOR)
    require(set(git(repo,'diff','--name-only',FIXTURE_RECORD_COMMIT,BOOTSTRAP_PREDECESSOR).decode().splitlines())
        <=set(SUCCESSOR_CARRIERS),'production_bootstrap_predecessor_carriers')
    before={p:v.rstrip('\0') for p,v in entries(repo,BOOTSTRAP_PREDECESSOR).items()}
    after={p:v.rstrip('\0') for p,v in entries(repo,BOOTSTRAP_SOURCE).items()}
    require({p:v for p,v in before.items() if p!=BOOTSTRAP_PATH}=={p:v for p,v in after.items() if p!=BOOTSTRAP_PATH},
        'production_bootstrap_only_exact_test')
    change=dict(path=BOOTSTRAP_PATH,disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED')
    for label,tree,expected in (('before',before,BOOTSTRAP_BEFORE_SHA256),('after',after,BOOTSTRAP_AFTER_SHA256)):
        entry=tree[BOOTSTRAP_PATH];require(entry.startswith('100644 blob '),'production_bootstrap_file_mode')
        raw=git(repo,'cat-file','blob',entry.split()[2]);require(sha(raw)==expected,'production_bootstrap_exact_bytes')
        change[label]=dict(entry=entry,bytes=len(raw),sha256=sha(raw))
    require(record['changes']==[change] and record['delta_sha256']==sha(git(repo,'diff','--raw','--no-abbrev',
        '--no-renames',BOOTSTRAP_PREDECESSOR,BOOTSTRAP_SOURCE)),'production_bootstrap_complete_delta')
    reference={p:v.rstrip('\0') for p,v in entries(repo,REFERENCE).items()}
    inventory=json.loads(fixed_artifact(repo,BOOTSTRAP_RECORD_COMMIT,record['current_inputs']),object_pairs_hook=gitutil.pairs)
    require(inventory==dict(source=BOOTSTRAP_SOURCE,reference=REFERENCE,
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',REFERENCE,BOOTSTRAP_SOURCE)),
        entries=[dict(path=p,reference_entry=reference.get(p,''),current_entry=after.get(p,''),
        disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED') for p in git(repo,'diff','--name-only',REFERENCE,BOOTSTRAP_SOURCE).decode().splitlines()]),
        'production_bootstrap_complete_inventory')
    import f13_native_p3_documentation as documentation
    native=documentation.load_context_record(repo)
    expected_native=dict(version=documentation.CONTEXT_VERSION,source=documentation.CONTEXT_SOURCE,
        record=documentation.CONTEXT_RECORD_COMMIT,path=documentation.CONTEXT_RECORD_PATH,sha256=documentation.CONTEXT_RECORD_SHA256)
    require(record['native_record']==expected_native,'production_bootstrap_retained_native_tuple')
    raw_native=json.loads(fixed_artifact(repo,documentation.CONTEXT_RECORD_COMMIT,
        dict(path=documentation.CONTEXT_RECORD_PATH,sha256=documentation.CONTEXT_RECORD_SHA256)),object_pairs_hook=gitutil.pairs)
    closure=raw_native['closure'];require(record['native_closure']==closure,'production_bootstrap_native_closure')
    native_tree=documentation.tree_entries(repo,documentation.CONTEXT_SOURCE)
    for name in set(closure['projection_paths']+closure['context_paths']+closure['compiled_paths']):
        require(name!=BOOTSTRAP_PATH and native_tree.get(name)==after.get(name),'production_bootstrap_native_entry')
    directories=directory_entries(after)
    for row in closure['directory_inventories']:
        require(directories.get(row['directory'],{})==row['entries'],
            'production_bootstrap_native_directory')
    require({p:v for p,v in native_tree.items() if p.startswith('src/backend/voice/')}==
        {p:v for p,v in after.items() if p.startswith('src/backend/voice/')},'production_bootstrap_voice_tree')
    names=[]
    for proof in record['evidence']:
        fixed_artifact(repo,BOOTSTRAP_RECORD_COMMIT,proof);names.append(proof['path'])
    require(names and names==sorted(set(names)) and record['review'] and all(p in names for p in record['review']),
        'production_bootstrap_review')
    additions={BOOTSTRAP_RECORD_PATH,record['current_inputs']['path'],*names}
    recorded={p:v.rstrip('\0') for p,v in entries(repo,BOOTSTRAP_RECORD_COMMIT).items()}
    require(len(additions)==2+len(names) and not set(after)&additions and set(recorded)-set(after)==additions
        and all(recorded.get(p)==v for p,v in after.items() if p!='.agent/execplans/pr875-current-native-source.md'),
        'production_bootstrap_immutable_source_and_named_metadata')
    return (record,*prior)


def qualify_bootstrap_fix(repo):
    records=load_bootstrap_fix(repo);record,old=records[0],records[-1]
    target=git(repo,'rev-parse','HEAD').decode().strip()
    event={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    pr=event['GITHUB_EVENT_NAME']=='pull_request' and re.fullmatch(r'refs/pull/[1-9][0-9]*/merge',event['GITHUB_REF']) and event['GITHUB_BASE_REF']=='master'
    push=event['GITHUB_EVENT_NAME']=='push' and event['GITHUB_REF']=='refs/heads/master' and event['VOICE_CI_PUSH_BEFORE_SHA']==INTEGRATION_EVENT_BASE
    require(event['GITHUB_SHA']==target and (pr or push),'production_actual_master_event')
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and parents[1]==INTEGRATION_EVENT_BASE,'production_actual_ordered_parents')
    source=parents[2];git(repo,'merge-base','--is-ancestor',BOOTSTRAP_RECORD_COMMIT,source)
    require(source!=BOOTSTRAP_RECORD_COMMIT,'production_later_carrier')
    require(git(repo,'rev-parse',source+'^{tree}')==git(repo,'rev-parse',target+'^{tree}'),'production_source_result_tree')
    require(set(git(repo,'diff','--name-only',BOOTSTRAP_RECORD_COMMIT,source).decode().splitlines())<=set(SUCCESSOR_CARRIERS),
        'production_only_reviewed_carrier_delta')
    for name in SUCCESSOR_CARRIERS:
        meta=gitutil.entry(repo,source,name)
        require(meta['mode']=='100644' and meta['type']=='blob','production_carrier_mode')
        require(git(repo,'cat-file','blob',meta['oid'])==(ROOT/name).read_bytes(),'production_executed_carrier')
    import f13_native_p3_documentation as documentation
    current=documentation.qualify_context(repo,INTEGRATION_EVENT_BASE,source,target,event['GITHUB_EVENT_NAME'],event['GITHUB_REF'],
        event['GITHUB_SHA'],event['VOICE_CI_PUSH_BEFORE_SHA'],event['GITHUB_BASE_REF'])
    require(record['native_record']==dict(version=current['version'],source=current['snapshot'],
        record=current['record'],path=documentation.CONTEXT_RECORD_PATH,sha256=current['record_sha256']),
        'production_bootstrap_actual_retained_native')
    require(current['source_applicability']==documentation.bootstrap_bridge_binding(source,target),
        'production_bootstrap_current_native_bridge')
    return dict(type=BOOTSTRAP_VERSION,record=BOOTSTRAP_RECORD_COMMIT,record_sha256=BOOTSTRAP_RECORD_SHA256,
        predecessor_record=FIXTURE_RECORD_COMMIT,historical_validation_source=old['source'],
        reviewed_source=record['source'],reviewed_tree=record['source_tree'],base=INTEGRATION_EVENT_BASE,source=source,target=target,
        historical_collector=HISTORICAL_SCOPE,native_content_baseline=NATIVE_CONTENT_BASE,
        current_inputs_sha256=record['current_inputs']['sha256'],event=event,
        admission=False,runtime_pass=False,current_or_future_path_authority=False)


def validate_manual_supplement(repo,record):
    """Preserve the original facts predicate and verify its complete closure."""
    raw=fixed_artifact(repo,SUCCESSOR_RECORD_COMMIT,record['manual_supplement'])
    derivation=json.loads(fixed_artifact(repo,SUCCESSOR_RECORD_COMMIT,record['manual_derivation']),object_pairs_hook=gitutil.pairs)
    with zipfile.ZipFile(ROOT/ARCHIVE_PATH) as archive:
        facts_raw=archive.read('objects/0b1d6efdf6441ca3bece84d968d72ab464f4dba378aecbfcd370813ee3e5148f')
    require(sha(facts_raw)=='0b1d6efdf6441ca3bece84d968d72ab464f4dba378aecbfcd370813ee3e5148f',
            'production_original_manual_facts')
    facts=json.loads(facts_raw,object_pairs_hook=gitutil.pairs)
    require(derivation['facts_sha256']==sha(facts_raw) and derivation['bindings']==facts['bindings'] and
        derivation['original_archive_unchanged'] is True and derivation['original_facts_unchanged'] is True,
        'production_manual_derivation_bindings')
    require(len(facts['bindings'])==11,'production_complete_manual_binding_count')
    result={}
    with zipfile.ZipFile(io.BytesIO(raw)) as supplement:
        require(sorted(supplement.namelist())==sorted(facts['bindings']),'production_exact_manual_supplement_members')
        for name,expected in facts['bindings'].items():
            require(Path(name).name==name and '\\' not in name,'production_manual_supplement_path')
            data=supplement.read(name)
            require(dict(bytes=len(data),sha256=sha(data))==expected,'production_manual_supplement_bytes')
            result[name]=data
    return result


def materialize(repo,destination):
    binding=qualify(repo);record=load_record(repo)
    folder=Path(destination)
    require(not folder.exists(),'production_fresh_input_directory')
    raw=(ROOT/ARCHIVE_PATH).read_bytes();require(sha(raw)==ARCHIVE_SHA,'production_actual_archive_bytes')
    with zipfile.ZipFile(ROOT/ARCHIVE_PATH) as archive:
        manifest_raw=archive.read('transport-manifest.json')
        require(sha(manifest_raw)==MANIFEST_SHA,'production_transport_manifest')
        manifest=json.loads(manifest_raw,object_pairs_hook=gitutil.pairs)
        selected=[item for item in manifest['logical_files'] if item['logical_path'].startswith(('/unit/','/fixed3071-unit/','/original/'))]
        require(selected==record['archive']['selected'],'production_selected_exact_historical_data')
        folder.mkdir(parents=True)
        for item in selected:
            name=item['logical_path']
            require('..' not in name.split('/') and '\\' not in name,'production_logical_path')
            data=archive.read('objects/'+item['sha256'])
            require(len(data)==item['bytes'] and sha(data)==item['sha256'],'production_materialized_historical_hash')
            path=folder/name.lstrip('/');path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
    if binding['type'] in ('F13-PRODUCTION-SOURCE-v3',TRANSPORT_VERSION,TEST_VERSION,RENDER_VERSION,SOCIAL_VERSION,INTEGRATION_VERSION,FIXTURE_VERSION,BOOTSTRAP_VERSION,'F13-PRODUCTION-SOURCE-v11'):
        current,_=load_successor(repo)
        supplemental=validate_manual_supplement(repo,current)
        destination=folder/'fixed3071-unit/manual'
        for name,data in supplemental.items():
            require(not (destination/name).exists(),'production_manual_supplement_no_overwrite')
            (destination/name).write_bytes(data)
    return binding


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('command',choices=('qualify','materialize'))
    parser.add_argument('repo');parser.add_argument('destination',nargs='?');args=parser.parse_args()
    try:
        result=materialize(args.repo,args.destination) if args.command=='materialize' else qualify(args.repo)
        print(json.dumps(result,sort_keys=True))
    except (gitutil.Rejected,ValueError,KeyError,TypeError,OSError,zipfile.BadZipFile) as error:
        print('Production F13 source rejected: '+str(error),file=sys.stderr);return 1
    return 0


if __name__=='__main__':sys.exit(main())
