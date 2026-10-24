"""Fixed documentation-only Native successor; no new runtime path permission."""
import json
from pathlib import Path
import sys

sys.path.insert(0,str(Path(__file__).resolve().parent))
import f13_native_p3_source as legacy
import f13_native_p3_successor as previous

VERSION='NATIVE-P3-SOURCE-v3'
RECORD_PATH='docs/testing/native-p3-source-v3.json'
VERIFIER_PATH='scripts/ci/f13_native_p3_documentation.py'
PINNED_RECORD_COMMIT='f87ee933aa1f599284e1932e44bf32b4e2af48ba'
PINNED_RECORD_SHA256='4ed3acd7cecfef97d03724115e57bf418600bd8e3210d992fd7ed1a325de8a67'
PINNED_SOURCE='e409cd05c40e857b61109513eb2154f968970c02'
DOCUMENT='docs/microservices/space-service.md'
CONTEXT_VERSION='NATIVE-P3-SOURCE-v4'
CONTEXT_RECORD_PATH='docs/testing/native-p3-source-v4.json'
CONTEXT_RECORD_COMMIT='60c0a1fd66d090956113c395d8447edc47b95772'
CONTEXT_RECORD_SHA256='3bec6a20906f6a96ba2e77d5f4b243e415b5f0ed7f724d88e6c4643446099e53'
CONTEXT_SOURCE='ef3dd7f3888bbe2792a9dcec836b2773c4e77962'
CONTEXT_EVENT_BASE='2c8bdf9e2f420b724575096beb4fcf5cb4ea00be'
CONTEXT_MAKEFILE_SHA256='3a461f577654391b6957c6ec5470e5c706dcc4e721f29a7d80751ee117c8a7ec'
CARRIER_PATHS=(VERIFIER_PATH,'scripts/ci/f13_native_p3_documentation_test.py',
    'scripts/ci/f13_native_p3_successor_integration_test.py',
    'scripts/ci/f13_native_p3_authority.py','scripts/ci/f13_f54_history.py',
    'scripts/ci/f13_f54_history_test.py','scripts/ci/f13_production_source.py',
    'scripts/ci/f13_production_source_test.py','scripts/ci/voice-db-runtime-provisioning-contract_test.sh',
    '.agent/execplans/pr875-current-native-source.md')


def tree_entries(repo,revision):
    return {v.split('\t',1)[1]:v for v in legacy.git(repo,'ls-tree','-r','-z',revision).decode().split('\0') if v}


def load_record(repo):
    require=legacy.require
    require(PINNED_RECORD_COMMIT is not None and PINNED_RECORD_SHA256 is not None and PINNED_SOURCE is not None,
            'documentation_successor_not_pinned')
    old=previous.load_record(repo)
    raw=legacy.git(repo,'show',PINNED_RECORD_COMMIT+':'+RECORD_PATH)
    require(legacy.digest(raw)==PINNED_RECORD_SHA256,'documentation_record_hash')
    require(legacy.entry(repo,PINNED_RECORD_COMMIT,RECORD_PATH)['mode']=='100644','documentation_record_mode')
    record=json.loads(raw,object_pairs_hook=legacy.unique_pairs)
    require(set(record)=={'version','status','source','source_tree','predecessor','document','closure','evidence',
        'runtime_pass','activation_authority','current_or_future_path_authority'},'documentation_record_shape')
    require(record['version']==VERSION and record['status']=='SOURCE_APPLICABILITY_CLEAR','documentation_status')
    require(record['source']==PINNED_SOURCE and all(record[k] is False for k in
        ('runtime_pass','activation_authority','current_or_future_path_authority')),'documentation_no_runtime_grant')
    require(record['predecessor']==dict(version=previous.VERSION,source=old['source'],record=previous.PINNED_RECORD_COMMIT,
        sha256=previous.PINNED_RECORD_SHA256,path=previous.RECORD_PATH),'documentation_predecessor')
    require(legacy.git(repo,'rev-list','--parents','-n','1',PINNED_RECORD_COMMIT).decode().split()==
        [PINNED_RECORD_COMMIT,PINNED_SOURCE],'documentation_direct_record_parent')
    require(legacy.git(repo,'rev-parse',PINNED_SOURCE+'^{tree}').decode().strip()==record['source_tree'],
        'documentation_source_tree')
    legacy.ancestor(repo,previous.PINNED_RECORD_COMMIT,PINNED_SOURCE)
    predecessor_artifacts=[previous.RECORD_PATH,legacy.RECORD_PATH,old['compiled_closure_manifest']['path'],
        *[p['name'] for p in old['evidence']]]
    for revision in (PINNED_SOURCE,PINNED_RECORD_COMMIT):
        for name in predecessor_artifacts:
            require(legacy.entry(repo,revision,name)==legacy.entry(repo,previous.PINNED_RECORD_COMMIT,name),
                'documentation_predecessor_artifact_drift')
    doc=record['document']
    require(doc==dict(path=DOCUMENT,before=legacy.entry(repo,old['source'],DOCUMENT),
        after=legacy.entry(repo,PINNED_SOURCE,DOCUMENT)) and doc['before']!=doc['after'],'documentation_exact_changed_entry')
    require(doc['after']['mode']=='100644' and doc['after']['type']=='blob','documentation_document_mode')
    old_tree,new_tree=tree_entries(repo,old['source']),tree_entries(repo,PINNED_SOURCE)
    for row in old['projection']:
        if row['path']!=DOCUMENT:
            require(old_tree.get(row['path'])==new_tree.get(row['path']),'documentation_other_projection_drift')
    for row in old['context']:
        require(old_tree.get(row['path'])==new_tree.get(row['path']),'documentation_context_drift')
    require({p:v for p,v in old_tree.items() if p.startswith('src/backend/voice/')}==
        {p:v for p,v in new_tree.items() if p.startswith('src/backend/voice/')},'documentation_complete_voice_tree')
    directories=sorted({str(Path(p).parent).replace('\\','/') for p in old['compiled_closure']})
    inventories=[]
    for directory in directories:
        prefix=directory+'/'
        before={p:v for p,v in old_tree.items() if p.startswith(prefix) and '/' not in p[len(prefix):]}
        after={p:v for p,v in new_tree.items() if p.startswith(prefix) and '/' not in p[len(prefix):]}
        require(before==after,'documentation_dependency_directory_drift')
        inventories.append(dict(directory=directory,entries=after))
    require(record['closure']==dict(projection_paths=[r['path'] for r in old['projection']],
        context_paths=[r['path'] for r in old['context']],compiled_paths=old['compiled_closure'],
        directory_inventories=inventories),'documentation_complete_closure')
    require(isinstance(record['evidence'],list) and record['evidence'],'documentation_review_evidence')
    names=[]
    for proof in record['evidence']:
        require(isinstance(proof,dict) and set(proof)=={'path','sha256'},'documentation_evidence_shape')
        name=proof['path'];require(isinstance(name,str) and name.startswith('docs/testing/') and '..' not in name.split('/'),
            'documentation_evidence_path')
        require(isinstance(proof['sha256'],str) and legacy.re.fullmatch('[0-9a-f]{64}',proof['sha256']),
            'documentation_evidence_digest')
        require(legacy.entry(repo,PINNED_RECORD_COMMIT,name)['mode']=='100644' and
            legacy.digest(legacy.git(repo,'show',PINNED_RECORD_COMMIT+':'+name))==proof['sha256'],
            'documentation_evidence_binding');names.append(name)
    require(names==sorted(set(names)),'documentation_evidence_order')
    # The effective unit retains all previous predicates and proof metadata;
    # only the reviewed document entry and explicit lineage/source change.
    effective=json.loads(json.dumps(old))
    effective.update(version=VERSION,source=PINNED_SOURCE,predecessor=record['predecessor'])
    for row in effective['projection']:
        if row['path']==DOCUMENT:row['after']=doc['after']
    effective['documentation_record']=record
    return effective


def qualify(repo,base,source,target,event,ref,event_sha,before,base_ref):
    require=legacy.require
    repo=str(Path(repo).resolve());base,source,target=map(legacy.oid,(base,source,target))
    require(event_sha==target and event in ('push','pull_request'),'documentation_event')
    if event=='push' and ref=='refs/heads/develop':
        require(source==target and before==base,'develop_event_before');consumer='DEVELOP-SOURCE-v1'
    else:
        require((event=='push' and ref=='refs/heads/master' and before==base) or
            (event=='pull_request' and base_ref=='master' and legacy.re.fullmatch(r'refs/pull/[0-9]+/merge',ref)),
            'master_event')
        require(legacy.git(repo,'rev-list','--parents','-n','1',target).decode().split()==[target,base,source],
            'master_shape_or_before')
        require(legacy.git(repo,'diff','--raw','--no-abbrev','--no-renames',base,source)==
            legacy.git(repo,'diff','--raw','--no-abbrev','--no-renames',base,target),'master_result')
        consumer='MASTER-SOURCE-v1'
    require(base!='0'*40,'event_before_zero');legacy.ancestor(repo,base,source)
    record=load_record(repo);legacy.ancestor(repo,PINNED_RECORD_COMMIT,source)
    require(source!=PINNED_RECORD_COMMIT,'missing_carrier')
    root=Path(__file__).resolve().parents[2]
    for name in CARRIER_PATHS:
        meta=legacy.entry(repo,source,name)
        require(meta is not None and meta['mode']=='100644' and meta['type']=='blob','documentation_carrier_mode')
        require(legacy.git(repo,'show',source+':'+name)==(root/name).read_bytes(),'documentation_executed_carrier')
        require(legacy.entry(repo,target,name)==meta,'documentation_carrier_result')
    require(legacy.entry(repo,PINNED_RECORD_COMMIT,VERIFIER_PATH)!=legacy.entry(repo,source,VERIFIER_PATH),'later_carrier')
    expected=tree_entries(repo,PINNED_SOURCE)
    old=previous.load_record(repo)
    protected=[RECORD_PATH,previous.RECORD_PATH,legacy.RECORD_PATH,old['compiled_closure_manifest']['path'],
        *[p['name'] for p in old['evidence']],*[p['path'] for p in record['documentation_record']['evidence']]]
    directories=[x['directory'] for x in record['documentation_record']['closure']['directory_inventories']]
    for revision in (source,target):
        current=tree_entries(repo,revision)
        require({p:v for p,v in current.items() if p.startswith('src/backend/voice/')}==
            {p:v for p,v in expected.items() if p.startswith('src/backend/voice/')},'documentation_complete_voice_tree')
        for row in record['projection']:
            require(legacy.entry(repo,revision,row['path'])==row['after'],'projection_drift')
        for row in record['context']:
            require(legacy.entry(repo,revision,row['path'])==row['entry'],'context_drift')
        for directory in directories:
            prefix=directory+'/'
            select=lambda tree:{p:v for p,v in tree.items() if p.startswith(prefix) and '/' not in p[len(prefix):]}
            require(select(current)==select(expected),'documentation_dependency_directory_drift')
        for name in protected:
            require(legacy.entry(repo,revision,name)==legacy.entry(repo,PINNED_RECORD_COMMIT,name),
                'documentation_record_or_evidence_drift')
    return dict(version=VERSION,consumer=consumer,base=base,source=source,target=target,content_base=record['base'],
        snapshot=record['source'],record=PINNED_RECORD_COMMIT,record_sha256=PINNED_RECORD_SHA256,
        paths=[r['path'] for r in record['projection']],retain_guarded_and_runtime_history=True)


def load_context_record(repo):
    """Reconstruct v3 and rebind only the exact reviewed Makefile context."""
    require=legacy.require
    require(CONTEXT_RECORD_COMMIT is not None and CONTEXT_RECORD_SHA256 is not None
        and CONTEXT_SOURCE is not None,'context_successor_not_pinned')
    old=load_record(repo)
    raw=legacy.git(repo,'show',CONTEXT_RECORD_COMMIT+':'+CONTEXT_RECORD_PATH)
    require(legacy.digest(raw)==CONTEXT_RECORD_SHA256,'context_record_hash')
    require(legacy.entry(repo,CONTEXT_RECORD_COMMIT,CONTEXT_RECORD_PATH)['mode']=='100644','context_record_mode')
    record=json.loads(raw,object_pairs_hook=legacy.unique_pairs)
    require(set(record)=={'version','status','source','source_tree','predecessor','context_change','closure','evidence',
        'runtime_pass','activation_authority','current_or_future_path_authority'},'context_record_shape')
    require(record['version']==CONTEXT_VERSION and record['status']=='SOURCE_APPLICABILITY_CLEAR'
        and record['source']==CONTEXT_SOURCE,'context_record_source')
    require(all(record[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),
        'context_no_runtime_grant')
    require(record['predecessor']==dict(version=VERSION,source=PINNED_SOURCE,record=PINNED_RECORD_COMMIT,
        path=RECORD_PATH,sha256=PINNED_RECORD_SHA256),'context_exact_predecessor')
    require(legacy.git(repo,'rev-list','--parents','-n','1',CONTEXT_RECORD_COMMIT).decode().split()==
        [CONTEXT_RECORD_COMMIT,CONTEXT_SOURCE],'context_direct_record_parent')
    require(legacy.git(repo,'rev-parse',CONTEXT_SOURCE+'^{tree}').decode().strip()==record['source_tree'],
        'context_source_tree')
    legacy.ancestor(repo,PINNED_RECORD_COMMIT,CONTEXT_SOURCE)
    def bound(ref):
        data=legacy.git(repo,'show',ref+':Makefile')
        return dict(entry=legacy.entry(repo,ref,'Makefile'),bytes=len(data),sha256=legacy.digest(data))
    before,after=bound(PINNED_SOURCE),bound(CONTEXT_SOURCE)
    require(record['context_change']==dict(path='Makefile',before=before,after=after)
        and after['sha256']==CONTEXT_MAKEFILE_SHA256 and after['entry']['mode']=='100644'
        and after['entry']['type']=='blob' and before!=after,'context_exact_makefile')
    old_tree,new_tree=tree_entries(repo,PINNED_SOURCE),tree_entries(repo,CONTEXT_SOURCE)
    for row in old['projection']:
        require(old_tree.get(row['path'])==new_tree.get(row['path']),'context_projection_drift')
    for row in old['context']:
        if row['path']!='Makefile':require(old_tree.get(row['path'])==new_tree.get(row['path']),'context_other_context_drift')
    require({p:v for p,v in old_tree.items() if p.startswith('src/backend/voice/')}==
        {p:v for p,v in new_tree.items() if p.startswith('src/backend/voice/')},'context_complete_voice_tree')
    inventories=[]
    for directory in sorted({str(Path(p).parent).replace('\\','/') for p in old['compiled_closure']}):
        prefix=directory+'/'
        select=lambda tree:{p:v for p,v in tree.items() if p.startswith(prefix) and '/' not in p[len(prefix):]}
        require(select(old_tree)==select(new_tree),'context_dependency_directory_drift')
        inventories.append(dict(directory=directory,entries=select(new_tree)))
    require(record['closure']==dict(projection_paths=[r['path'] for r in old['projection']],
        context_paths=[r['path'] for r in old['context']],compiled_paths=old['compiled_closure'],
        directory_inventories=inventories),'context_complete_closure')
    protected=[RECORD_PATH,previous.RECORD_PATH,legacy.RECORD_PATH,old['compiled_closure_manifest']['path'],
        *[p['name'] for p in old['evidence']],*[p['path'] for p in old['documentation_record']['evidence']]]
    for revision in (CONTEXT_SOURCE,CONTEXT_RECORD_COMMIT):
        for name in protected:
            require(legacy.entry(repo,revision,name)==legacy.entry(repo,PINNED_RECORD_COMMIT,name),'context_predecessor_artifact_drift')
    names=[]
    for proof in record['evidence']:
        require(set(proof)=={'path','sha256'},'context_evidence_shape')
        name=proof['path']
        require(isinstance(name,str) and name.startswith('docs/testing/') and '..' not in name.split('/'),
            'context_evidence_path')
        require(legacy.entry(repo,CONTEXT_RECORD_COMMIT,name)['mode']=='100644'
            and legacy.digest(legacy.git(repo,'show',CONTEXT_RECORD_COMMIT+':'+name))==proof['sha256'],'context_evidence_binding')
        names.append(name)
    require(names and names==sorted(set(names)),'context_evidence_order')
    effective=json.loads(json.dumps(old))
    effective.update(version=CONTEXT_VERSION,source=CONTEXT_SOURCE,predecessor=record['predecessor'],context_record=record)
    for row in effective['context']:
        if row['path']=='Makefile':row['entry']=after['entry']
    return effective


def qualify_context(repo,base,source,target,event,ref,event_sha,before,base_ref):
    import f13_production_source as production
    if production.fixture_applicable(repo) or production.fixture_applicable(repo,target):
        return qualify_fixture_context(repo,base,source,target,event,ref,event_sha,before,base_ref)
    require=legacy.require
    repo=str(Path(repo).resolve());base,source,target=map(legacy.oid,(base,source,target))
    require(base==CONTEXT_EVENT_BASE and event_sha==target and event in ('push','pull_request'),'context_exact_event_base')
    require((event=='push' and ref=='refs/heads/master' and before==base) or
        (event=='pull_request' and base_ref=='master' and legacy.re.fullmatch(r'refs/pull/[0-9]+/merge',ref)),
        'context_master_event')
    require(legacy.git(repo,'rev-list','--parents','-n','1',target).decode().split()==[target,base,source],
        'context_ordered_master_parents')
    require(legacy.git(repo,'rev-parse',source+'^{tree}')==legacy.git(repo,'rev-parse',target+'^{tree}'),
        'context_source_result_tree')
    record=load_context_record(repo);legacy.ancestor(repo,base,source);legacy.ancestor(repo,CONTEXT_RECORD_COMMIT,source)
    require(source!=CONTEXT_RECORD_COMMIT,'context_missing_carrier')
    expected=tree_entries(repo,CONTEXT_RECORD_COMMIT);root=Path(__file__).resolve().parents[2]
    for revision in (source,target):
        current=tree_entries(repo,revision)
        require({p:v for p,v in current.items() if p not in CARRIER_PATHS}==
            {p:v for p,v in expected.items() if p not in CARRIER_PATHS},'context_post_record_source_drift')
        for name in CARRIER_PATHS:
            meta=legacy.entry(repo,revision,name)
            require(meta is not None and meta['mode']=='100644' and meta['type']=='blob','context_carrier_mode')
            require(legacy.git(repo,'show',revision+':'+name)==(root/name).read_bytes(),'context_executed_carrier')
    require(legacy.entry(repo,CONTEXT_RECORD_COMMIT,VERIFIER_PATH)!=legacy.entry(repo,source,VERIFIER_PATH),'context_later_carrier')
    return dict(version=CONTEXT_VERSION,consumer='MASTER-SOURCE-v1',base=base,source=source,target=target,
        content_base=record['base'],snapshot=CONTEXT_SOURCE,record=CONTEXT_RECORD_COMMIT,
        record_sha256=CONTEXT_RECORD_SHA256,paths=[r['path'] for r in record['projection']],
        retain_guarded_and_runtime_history=True)


def fixture_bridge_binding(source,target):
    import f13_production_source as production
    return dict(type='NATIVE-P3-v4-FIXTURE-SOURCE-APPLICABILITY-v1',
        source_snapshot=production.FIXTURE_SOURCE,record=production.FIXTURE_RECORD_COMMIT,
        record_sha256=production.FIXTURE_RECORD_SHA256,base=CONTEXT_EVENT_BASE,source=source,target=target)


def qualify_fixture_context(repo,base,source,target,event,ref,event_sha,before,base_ref):
    # Source loading is deliberately acyclic: the production source constructor
    # reconstructs immutable v8/v4 first and never calls current qualification.
    import f13_production_source as production
    require=legacy.require
    require(Path(repo).resolve()==production.ROOT.resolve(),'production_actual_root')
    require(target==legacy.git(repo,'rev-parse','HEAD').decode().strip(),'context_fixture_actual_head')
    record=production.load_fixture_fix(repo)[0]
    unit=load_context_record(repo)
    base,source,target=map(legacy.oid,(base,source,target))
    require(base==CONTEXT_EVENT_BASE and event_sha==target and event in ('push','pull_request'),'context_exact_event_base')
    require((event=='push' and ref=='refs/heads/master' and before==base) or
        (event=='pull_request' and base_ref=='master' and legacy.re.fullmatch(r'refs/pull/[0-9]+/merge',ref)),
        'context_master_event')
    require(legacy.git(repo,'rev-list','--parents','-n','1',target).decode().split()==[target,base,source],
        'context_ordered_master_parents')
    require(legacy.git(repo,'rev-parse',source+'^{tree}')==legacy.git(repo,'rev-parse',target+'^{tree}'),
        'context_source_result_tree')
    legacy.ancestor(repo,base,source);legacy.ancestor(repo,production.FIXTURE_RECORD_COMMIT,source)
    require(source!=production.FIXTURE_RECORD_COMMIT,'context_missing_carrier')
    expected=tree_entries(repo,production.FIXTURE_RECORD_COMMIT);root=Path(__file__).resolve().parents[2]
    for revision in (source,target):
        current=tree_entries(repo,revision)
        require({p:v for p,v in current.items() if p not in CARRIER_PATHS}==
            {p:v for p,v in expected.items() if p not in CARRIER_PATHS},'context_fixture_post_record_source_drift')
        for name in CARRIER_PATHS:
            meta=legacy.entry(repo,revision,name)
            require(meta is not None and meta['mode']=='100644' and meta['type']=='blob','context_carrier_mode')
            require(legacy.git(repo,'show',revision+':'+name)==(root/name).read_bytes(),'context_executed_carrier')
    require(legacy.entry(repo,production.FIXTURE_RECORD_COMMIT,VERIFIER_PATH)!=legacy.entry(repo,source,VERIFIER_PATH),
        'context_later_carrier')
    require(record['native_record']==dict(version=CONTEXT_VERSION,source=CONTEXT_SOURCE,record=CONTEXT_RECORD_COMMIT,
        path=CONTEXT_RECORD_PATH,sha256=CONTEXT_RECORD_SHA256),'context_fixture_exact_retained_tuple')
    return dict(version=CONTEXT_VERSION,consumer='MASTER-SOURCE-v1',base=base,source=source,target=target,
        content_base=unit['base'],snapshot=CONTEXT_SOURCE,record=CONTEXT_RECORD_COMMIT,
        record_sha256=CONTEXT_RECORD_SHA256,paths=[r['path'] for r in unit['projection']],
        retain_guarded_and_runtime_history=True,source_applicability=fixture_bridge_binding(source,target))


def verify_context_binding(repo,binding):
    import f13_production_source as production
    if production.fixture_applicable(repo) or 'source_applicability' in binding:
        event=production.os.environ
        expected=qualify_fixture_context(repo,binding['base'],binding['source'],binding['target'],
            event.get('GITHUB_EVENT_NAME',''),event.get('GITHUB_REF',''),event.get('GITHUB_SHA',''),
            event.get('VOICE_CI_PUSH_BEFORE_SHA',''),event.get('GITHUB_BASE_REF',''))
        legacy.require(binding==expected,'context_fixture_current_binding')
    else:
        legacy.require('source_applicability' not in binding,'context_unexpected_fixture_bridge')


def audit_map(repo, binding, map_path, history_path):
    """Validate real collector rows; never synthesize or edit the map."""
    require, git, oid = legacy.require, legacy.git, legacy.oid
    ancestor, entry, digest = legacy.ancestor, legacy.entry, legacy.digest
    subprocess, re, runtime_changed = legacy.subprocess, legacy.re, legacy.runtime_changed
    if binding['version']==CONTEXT_VERSION:verify_context_binding(repo,binding)
    r = load_context_record(repo) if binding['version']==CONTEXT_VERSION else load_record(repo)
    expected = {row["path"]: row["after"] for row in r["projection"]}
    historical = {}
    history = Path(history_path).read_text().splitlines()
    for line in history:
        fields = line.split("\t")
        if fields[0] == "supersession":
            require(len(fields) == 6 and fields[-1] in ("0", "2"), "history_supersession")
            continue
        require(len(fields) == 8 and fields[0] in expected and fields[1] in ("true", "false"), "history_row")
        historical.setdefault(fields[0], []).append(line)
    selected = {}
    for line in Path(map_path).read_text().splitlines():
        fields = line.split("\t")
        require(len(fields) == 8, "map_shape")
        name, trigger, owner, parent, source_base, source_head, content_base, best = fields
        if name not in expected:
            continue
        require(name not in selected and trigger in ("true", "false"), "map_duplicate_or_trigger")
        require(name in historical and historical[name][-1] == line, "map_history_binding")
        for value in (owner, parent, source_base, source_head, content_base):
            oid(value)
        owner_parents = git(repo, "rev-list", "--parents", "-n", "1", owner).decode().split()
        require(len(owner_parents) >= 2 and owner_parents[1] == parent, "map_owner_parent")
        ancestor(repo, owner, binding["source"])
        ancestor(repo, source_head, r['source'])
        ancestor(repo, source_head, owner)
        ancestor(repo, source_base, source_head)
        ancestor(repo, content_base, parent)
        require(all(re.fullmatch(r"[0-9a-f]{40}", v) for v in best.split(",")), "map_best_bases")
        recorded_bases = best.split(",")
        if len(owner_parents) == 2:
            require(source_head == owner and source_base == parent and recorded_bases == [parent], "map_direct_owner")
        else:
            require(any(sorted(git(repo, "merge-base", "--all", parent, actual_source).decode().splitlines())
                        == sorted(recorded_bases) and (source_head == owner or
                        subprocess.run(["git", "--no-replace-objects", "-C", str(repo), "merge-base", "--is-ancestor",
                                        source_head, actual_source], capture_output=True).returncode == 0)
                        for actual_source in owner_parents[2:]), "map_imported_source")
        require(entry(repo, source_head, name) == expected[name] == entry(repo, binding["target"], name), "map_entry")
        delta = git(repo, "diff", "--name-only", "--no-renames", source_base, source_head).decode().splitlines()
        require(name in delta, "map_source_delta")
        require(trigger == "true" or not runtime_changed(delta), "map_runtime_taint")
        require(trigger == "true" or all(row.split("\t")[1] == "false" for row in historical[name]), "map_prior_runtime_taint")
        selected[name] = line
    changed = set(git(repo, "diff", "--name-only", "--no-renames", binding["base"], binding["source"]).decode().splitlines())
    require(set(selected) == set(expected) & changed, "map_incomplete")
    return {"binding": binding, "map_sha256": digest(Path(map_path).read_bytes()),
            "history_sha256": digest(Path(history_path).read_bytes()), "rows": selected}
