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
require=gitutil.require
git=gitutil.git


def sha(raw):return hashlib.sha256(raw).hexdigest()


def tree_entry(repo,rev,name):
    return git(repo,'ls-tree','-z',rev,'--',':(literal)'+name).decode()


def entries(repo,revision):
    return {item.split('\t',1)[1]:item+'\0' for item in
            git(repo,'ls-tree','-r','-z',revision).decode().split('\0') if item}


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


def qualify(repo):
    require(Path(repo).resolve()==ROOT.resolve(),'production_actual_root')
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
    if binding['type']=='F13-PRODUCTION-SOURCE-v3':
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
