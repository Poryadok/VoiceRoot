"""Pinned production applicability; historical facts never grant runtime paths."""
import argparse
import hashlib
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
