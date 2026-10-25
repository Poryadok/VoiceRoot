"""One historical Role duplicate, with a separately reviewed current correction.

The old predicate runs unchanged against its fixed accepted carrier. That is
historical evidence only; actual target correction and dependencies are checked
independently before the original producer may preserve and replace its row.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
import f13_role_conditional as legacy

HISTORICAL_CARRIER = '8fc9e542acd8039a356c4149ccd0fd4a16f79564'
ORIGINAL_CORRECTION = '2ce88a9892989686cd4fd5734e8b414a3c914564'
RECORD_PATH = 'docs/testing/role-checkpermission-historical-source-v2.json'
VERIFIER_PATH = 'scripts/ci/f13_role_conditional_successor.py'
PINNED_RECORD_COMMIT = 'd739c1f514eba722d3fd13c08983c954b286b018'
PINNED_RECORD_SHA256 = '967753244e8a72983df5a1b58bc9bdde3bee90d39633a658cf81e6711eeb721a'
VERSION = 'MASTER-ROLE-CHECKPERMISSION-HISTORICAL-CONSUMPTION-v2'
FALSE_FLAGS = ('runtime_pass','generic_identical_blob_authority','current_or_future_path_authority','activation_authority')


def package_inventory(repo, revision, directory):
    raw=legacy.git(repo,'ls-tree','-r','--name-only','-z',revision,'--',':(literal)'+directory)
    return sorted(name.decode() for name in raw.split(b'\0') if name
                  and str(Path(name.decode()).parent).replace('\\','/')==directory)


def artifact(repo, name, expected_hash):
    legacy.require(isinstance(expected_hash,str) and re.fullmatch(r'[0-9a-f]{64}',expected_hash),
                   'role_successor_hash_shape')
    meta=legacy.entry(repo,PINNED_RECORD_COMMIT,name)
    legacy.require(meta['mode']=='100644' and meta['type']=='blob','role_successor_artifact_mode')
    raw=legacy.git(repo,'cat-file','blob',meta['oid'])
    legacy.require(hashlib.sha256(raw).hexdigest()==expected_hash,'role_successor_artifact_hash')
    return raw


def load_record(repo):
    require = legacy.require
    require(PINNED_RECORD_COMMIT is not None and PINNED_RECORD_SHA256 is not None,
            'role_successor_carrier_not_pinned')
    legacy.oid(PINNED_RECORD_COMMIT)
    require(isinstance(PINNED_RECORD_SHA256,str) and re.fullmatch(r'[0-9a-f]{64}',PINNED_RECORD_SHA256),
            'role_successor_record_pin')
    raw = legacy.git(repo,'show',PINNED_RECORD_COMMIT+':'+RECORD_PATH)
    require(hashlib.sha256(raw).hexdigest()==PINNED_RECORD_SHA256,'role_successor_record_hash')
    meta=legacy.entry(repo,PINNED_RECORD_COMMIT,RECORD_PATH)
    require(meta['mode']=='100644' and meta['type']=='blob','role_successor_record_mode')
    record=json.loads(raw,object_pairs_hook=legacy.pairs)
    require(set(record)=={'version','status','source','predecessor','original_correction',
                         'required_current_source','context','closure_manifest','historical_runtime_gap',
                         'evidence','review',*FALSE_FLAGS},'role_successor_shape')
    require(record['version']==VERSION and record['status']=='SOURCE_CONSUMPTION_CONTRACT_CLEAR',
            'role_successor_status')
    require(record['predecessor']==dict(record=legacy.PINNED_RECORD_COMMIT,
            sha256=legacy.PINNED_RECORD_SHA256,path=legacy.RECORD_PATH,carrier=HISTORICAL_CARRIER),
            'role_successor_lineage')
    require(record['original_correction']==ORIGINAL_CORRECTION,'role_successor_original_correction')
    require(all(record[key] is False for key in FALSE_FLAGS),'role_successor_no_broader_authority')
    old_raw=legacy.git(repo,'show',legacy.PINNED_RECORD_COMMIT+':'+legacy.RECORD_PATH)
    require(hashlib.sha256(old_raw).hexdigest()==legacy.PINNED_RECORD_SHA256,'role_successor_predecessor_hash')
    old=json.loads(old_raw,object_pairs_hook=legacy.pairs)
    require(record['historical_runtime_gap']==old['historical_runtime_gap'],'role_successor_historical_gap')
    source=legacy.oid(record['source'])
    for older,newer in ((ORIGINAL_CORRECTION,source),(HISTORICAL_CARRIER,source),(source,PINNED_RECORD_COMMIT)):
        legacy.git(repo,'merge-base','--is-ancestor',older,newer)
    require(source!=PINNED_RECORD_COMMIT,'role_successor_self_record')
    expected_names=[row['path'] for row in old['required_current_source']]
    names=[]
    for group in ('required_current_source','context'):
        rows=record[group]
        require(isinstance(rows,list) and rows,'role_successor_complete_bindings')
        group_names=[]
        for row in rows:
            require(isinstance(row,dict) and set(row)=={'path','entry','sha256'},'role_successor_binding_shape')
            name=row['path']
            require(isinstance(name,str) and re.fullmatch(r'[A-Za-z0-9_./-]+',name)
                    and not name.startswith('/') and '..' not in name.split('/'),'role_successor_path')
            group_names.append(name)
            legacy.actual_entry(repo,source,name,row['entry'])
            require(hashlib.sha256(legacy.git(repo,'cat-file','blob',row['entry']['oid'])).hexdigest()
                    ==row['sha256'],'role_successor_binding_hash')
        require(group_names==sorted(set(group_names)),'role_successor_binding_order')
        if group=='required_current_source': require(group_names==expected_names,'role_successor_all_eleven')
        names += group_names
    require(len(names)==len(set(names)),'role_successor_disjoint_context')
    closure=record['closure_manifest']
    require(isinstance(closure,dict) and set(closure)=={'path','sha256'}
            and closure['path']=='docs/testing/role-checkpermission-source-v2-closure.json','role_successor_closure_reference')
    closure_raw=artifact(repo,closure['path'],closure['sha256'])
    manifest=json.loads(closure_raw,object_pairs_hook=legacy.pairs)
    require(set(manifest)=={'source','paths','package_inventories','command','image','go_list_sha256'},
            'role_successor_closure_shape')
    require(manifest['source']==source and manifest['paths']==sorted(names),'role_successor_closure_complete')
    inventories=manifest['package_inventories']
    require(isinstance(inventories,list) and inventories,'role_successor_packages')
    directories=[]
    for item in inventories:
        require(isinstance(item,dict) and set(item)=={'directory','paths'},'role_successor_package_shape')
        directory=item['directory']
        require(isinstance(directory,str) and directory.startswith('src/backend/')
                and '..' not in directory.split('/'),'role_successor_package_directory')
        directories.append(directory)
        require(item['paths']==package_inventory(repo,source,directory)
                and item['paths'] and set(item['paths'])<=set(names),'role_successor_package_complete')
    require(directories==sorted(set(directories)),'role_successor_package_order')
    evidence=record['evidence'];require(isinstance(evidence,list) and evidence,'role_successor_evidence')
    proof_names=[]
    for proof in evidence:
        require(isinstance(proof,dict) and set(proof)=={'path','sha256'},'role_successor_evidence_shape')
        name=proof['path'];require(isinstance(name,str) and name.startswith('docs/testing/role-source-v2-evidence/')
                                 and '..' not in name.split('/'),'role_successor_evidence_path')
        proof_names.append(name)
        artifact(repo,name,proof['sha256'])
    require(proof_names==sorted(set(proof_names)),'role_successor_evidence_order')
    require(isinstance(record['review'],list) and record['review']
            and all(isinstance(ref,str) and ref in proof_names for ref in record['review']),'role_successor_review')
    return record


def qualify(repo,target,prior_raw,incoming_raw):
    legacy.oid(target)
    # This cannot grant permission for a changed historical tuple or prior row.
    historical=legacy.qualify(repo,HISTORICAL_CARRIER,prior_raw,incoming_raw)
    record=load_record(repo)
    legacy.git(repo,'merge-base','--is-ancestor',PINNED_RECORD_COMMIT,target)
    legacy.require(target!=PINNED_RECORD_COMMIT,'role_successor_later_carrier')
    legacy.require(legacy.entry(repo,target,legacy.RECORD_PATH)==
                   legacy.entry(repo,legacy.PINNED_RECORD_COMMIT,legacy.RECORD_PATH),
                   'role_successor_predecessor_record_drift')
    root=Path(__file__).resolve().parents[2]
    for name in (VERIFIER_PATH,legacy.VERIFIER_PATH):
        meta=legacy.entry(repo,target,name)
        legacy.require(meta['mode']=='100644' and meta['type']=='blob','role_successor_carrier_mode')
        legacy.require(legacy.git(repo,'show',target+':'+name)==(root/name).read_bytes(),
                       'role_successor_executed_carrier')
    for group in ('required_current_source','context'):
        for row in record[group]: legacy.actual_entry(repo,target,row['path'],row['entry'])
    manifest=json.loads(artifact(repo,record['closure_manifest']['path'],record['closure_manifest']['sha256']),
                        object_pairs_hook=legacy.pairs)
    for item in manifest['package_inventories']:
        legacy.require(package_inventory(repo,target,item['directory'])==item['paths'],
                       'role_successor_target_package_drift')
    for name in (RECORD_PATH,record['closure_manifest']['path'],*[p['path'] for p in record['evidence']]):
        legacy.require(legacy.entry(repo,target,name)==legacy.entry(repo,PINNED_RECORD_COMMIT,name),
                       'role_successor_artifact_drift')
    return dict(version=VERSION,record=PINNED_RECORD_COMMIT,record_sha256=PINNED_RECORD_SHA256,
                source_boundary=record['source'],actual_target=target,historical_validation_target=HISTORICAL_CARRIER,
                historical=historical,prior_row=legacy.row(prior_raw),incoming_row=legacy.row(incoming_raw),
                historical_runtime_gap=record['historical_runtime_gap'],retain_runtime_and_content_taint=True,
                **{key:False for key in FALSE_FLAGS})


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    for key in ('repo','target','prior-row','incoming-row'):parser.add_argument('--'+key,required=True)
    args=parser.parse_args()
    try:
        try: result=legacy.qualify(args.repo,args.target,args.prior_row,args.incoming_row)
        except legacy.Rejected: result=qualify(args.repo,args.target,args.prior_row,args.incoming_row)
        print(json.dumps(result,sort_keys=True))
    except (legacy.Rejected,ValueError,KeyError,TypeError,AttributeError,OSError) as exc:
        print('Conditional Role successor rejected: '+str(exc),file=sys.stderr);return 1
    return 0


if __name__=='__main__':sys.exit(main())
