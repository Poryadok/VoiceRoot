"""Pinned source verification for the current release history successor."""
import f13_native_p3_source as legacy
import f13_native_p3_documentation as documentation
import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
import io
import zipfile
from pathlib import Path, PurePosixPath

Rejected = legacy.Rejected
require, git = legacy.require, legacy.git
ROOT = Path(__file__).resolve().parents[2]
FOUNDATION_BASE = '5dbc0bb4836a5d0cc9d0343d516ab43c551a9a29'  # Set to the final immutable H only after the extension review.
EVIDENCE_SOURCE = 'b2b566f1103e29058da0e8b786cbd8f0dd028d83'
COLLECTION_BASE = '2c8bdf9e2f420b724575096beb4fcf5cb4ea00be'
RECORD_COMMIT = 'b5d37fba693ccb2a321484fcb5fbe4f31dd9370c'  # Set only after the independently reviewed record is committed.
NATIVE_SHA256 = '61e3725265c7df86ccbc6afcdabfed046863cf1ca7ab8560dc0f90b9acb97568'
PRODUCTION_SHA256 = 'ab6ea434482fdb02d749e9897b89a58a811f91f224afeda5fc9956952041eea7'
ROLE_SHA256 = '1f5cb6d3c925a93881ecc26a52f84b3be5172de7e21515cee8a143e24fc2f376'
NATIVE_VERSION = 'NATIVE-P3-SOURCE-v5'
PRODUCTION_VERSION = 'F13-PRODUCTION-SOURCE-v11'
REUSE_POLICY = 'UNCHANGED-NATIVE-UNIT-AFTER-FOUNDATION-v1'
RECORD_PATH = 'docs/testing/native-p3-source-v5.json'
PRODUCTION_RECORD_PATH = 'docs/testing/f13-production-source-v11.json'
ROLE_RECORD_PATH = 'docs/testing/role-checkpermission-historical-source-v3.json'
ROLE_CLOSURE_PATH = 'docs/testing/role-checkpermission-source-v3-closure.json'
ROLE_VERSION = 'MASTER-ROLE-CHECKPERMISSION-HISTORICAL-CONSUMPTION-v3'
ROLE_EVIDENCE_SOURCE = '70a3ca533e23d7454782428304c180e76fad74e1'
CARRIER_PATHS = frozenset(('scripts/ci/f13_release_source.py',
    'scripts/ci/f13_release_source_test.py', 'scripts/ci/f13_native_p3_authority.py',
    'scripts/ci/f13_native_p3_successor_integration_test.py',
    'scripts/ci/f13_role_conditional_successor.py', 'scripts/ci/f13_role_conditional_successor_test.py', 'scripts/ci/f13_production_source_test.py',
    'scripts/ci/f13_production_source.py', 'scripts/ci/f13_f54_history.py',
    'scripts/ci/f13_f54_history_test.py', 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh',
    '.agent/execplans/current-release-history-source.md'))


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def artifact(repo, proof):
    require(isinstance(proof, dict) and set(proof) == {'path', 'sha256'}, 'release_artifact_shape')
    path, digest = proof['path'], proof['sha256']
    require(isinstance(path, str) and path.startswith('docs/testing/') and
            '..' not in path.split('/') and '\\' not in path, 'release_artifact_path')
    require(isinstance(digest, str) and re.fullmatch('[0-9a-f]{64}', digest), 'release_artifact_digest')
    meta = legacy.entry(repo, RECORD_COMMIT, path)
    require(meta is not None and meta['mode'] == '100644' and meta['type'] == 'blob', 'release_artifact_mode')
    raw = git(repo, 'cat-file', 'blob', meta['oid'])
    require(sha(raw) == digest, 'release_artifact_hash')
    return raw


def read_json(raw):
    return json.loads(raw, object_pairs_hook=legacy.unique_pairs)


def pinned(repo, path, digest):
    require(RECORD_COMMIT is not None and digest is not None, 'release_record_not_pinned')
    legacy.oid(RECORD_COMMIT)
    require(git(repo,'rev-list','--parents','-n','1',RECORD_COMMIT).decode().split() ==
            [RECORD_COMMIT, FOUNDATION_BASE], 'release_record_direct_parent')
    return read_json(artifact(repo, dict(path=path, sha256=digest)))


def _meta(tree, path):
    value = tree.get(path)
    if value is None:
        return None
    header = value.split('\t', 1)[0].split()
    return dict(mode=header[0], type=header[1], oid=header[2])


def _proofs(repo, proofs):
    require(isinstance(proofs, list) and proofs, 'release_evidence_missing')
    names = []
    for proof in proofs:
        artifact(repo, proof)
        names.append(proof['path'])
    require(names == sorted(set(names)), 'release_evidence_order')
    return names


def _record_delta(repo, native, production, role=None):
    before = documentation.tree_entries(repo, FOUNDATION_BASE)
    after = documentation.tree_entries(repo, RECORD_COMMIT)
    additions = {RECORD_PATH, PRODUCTION_RECORD_PATH, native['delta_review']['path'],
                 production['current_inputs']['path'], *[p['path'] for p in native['evidence']],
                 *[p['path'] for p in production['evidence']]}
    if role is not None:
        additions |= {ROLE_RECORD_PATH, role['closure']['path'], *[p['path'] for p in role['evidence']]}
    plan = '.agent/execplans/current-release-history-source.md'
    if plan in after and plan not in before:
        additions.add(plan)
    require(not set(before) & additions and set(after) - set(before) == additions and
            all(after.get(p) == v for p, v in before.items()), 'release_record_only_additions')


def preserve_predecessor(repo, predecessor, source, archive_path):
    before = documentation.tree_entries(repo, predecessor)
    after = documentation.tree_entries(repo, source)
    protected = {p for p in before if p.startswith(('docs/testing/native-p3-source',
        'docs/testing/f13-production', 'docs/testing/role-checkpermission',
        'docs/testing/role-source-v1-evidence/', 'docs/testing/role-source-v2-evidence/'))} | {archive_path}
    require(archive_path in before and all(before.get(p) == after.get(p) for p in protected),
            'release_historical_artifact_drift')


def load_native(repo):
    old = documentation.load_context_record(repo)
    r = pinned(repo, RECORD_PATH, NATIVE_SHA256)
    require(set(r) == {'version', 'status', 'source', 'source_tree', 'predecessor', 'projection',
        'context', 'closure', 'compiled_evidence', 'additional_context', 'delta_review', 'evidence',
        'evidence_source', 'extension_evidence',
        'clauses', 'runtime_pass', 'activation_authority', 'current_or_future_path_authority'},
        'release_native_shape')
    require(r['version'] == NATIVE_VERSION and r['status'] == 'SOURCE_APPLICABILITY_CLEAR' and
        r['source'] == FOUNDATION_BASE and all(r[k] is False for k in
        ('runtime_pass', 'activation_authority', 'current_or_future_path_authority')), 'release_native_status')
    require(r['predecessor'] == dict(version=documentation.CONTEXT_VERSION,
        source=documentation.CONTEXT_SOURCE, record=documentation.CONTEXT_RECORD_COMMIT,
        path=documentation.CONTEXT_RECORD_PATH, sha256=documentation.CONTEXT_RECORD_SHA256),
        'release_native_predecessor')
    legacy.ancestor(repo, documentation.CONTEXT_RECORD_COMMIT, FOUNDATION_BASE)
    require(r['source_tree'] == git(repo, 'rev-parse', FOUNDATION_BASE+'^{tree}').decode().strip(), 'release_native_tree')
    names = _proofs(repo, r['evidence'])
    require(r['compiled_evidence']['path'] in names and r['delta_review']['path'] in names and
        r['extension_evidence']['path'] in names and r['evidence_source'] == EVIDENCE_SOURCE,
        'release_native_evidence_bindings')
    raw = read_json(artifact(repo, r['compiled_evidence']))
    require(raw['source'] == EVIDENCE_SOURCE and raw['tree'] ==
        git(repo,'rev-parse',EVIDENCE_SOURCE+'^{tree}').decode().strip() and
        raw['command'] == 'go list -mod=readonly -deps -test -json ./...' and
        raw['image'] == 'golang:1.26.5-bookworm', 'release_compiled_evidence')
    compiled = raw['paths']
    require(isinstance(compiled, list) and compiled == sorted(set(compiled)) and compiled,
        'release_compiled_paths')
    require(set(old['compiled_closure']) <= set(compiled), 'release_dropped_compiled_path')
    old_tree = documentation.tree_entries(repo, documentation.CONTEXT_SOURCE)
    current = documentation.tree_entries(repo, FOUNDATION_BASE)
    evidence_tree = documentation.tree_entries(repo, EVIDENCE_SOURCE)
    legacy.ancestor(repo, EVIDENCE_SOURCE, FOUNDATION_BASE)
    projection = [x['path'] for x in old['projection']]
    require(r['projection'] == [dict(path=x['path'], before=x['before'], after=_meta(current, x['path']))
        for x in old['projection']] and all(x['after'] is not None for x in r['projection']),
        'release_complete_projection')
    extras = r['additional_context']
    require(isinstance(extras, list) and extras == sorted(set(extras)) and all(
        isinstance(p, str) and p.startswith('protos/') and p.endswith('.proto') for p in extras),
        'release_additional_context')
    contexts = sorted(({x['path'] for x in old['context']} | set(compiled) | set(extras)) - set(projection))
    require(r['context'] == [dict(path=p, entry=_meta(current, p)) for p in contexts] and
        all(x['entry'] is not None for x in r['context']), 'release_complete_context')
    require(all(current.get(p) == evidence_tree.get(p) for p in contexts
        if p.startswith('protos/') and p.endswith('.proto')), 'release_context_proto_drift')
    directories = sorted({str(Path(p).parent).replace('\\', '/') for p in old['compiled_closure'] + compiled})
    old_dirs, new_dirs = documentation.immediate_directory_entries(old_tree), documentation.immediate_directory_entries(current)
    inventories = [dict(directory=d, entries=new_dirs.get(d, {})) for d in directories]
    evidence_dirs = documentation.immediate_directory_entries(evidence_tree)
    require(all(evidence_tree.get(p) == current.get(p) for p in compiled) and
        all(evidence_dirs.get(d,{}) == new_dirs.get(d,{}) for d in directories) and
        {p:v for p,v in evidence_tree.items() if p.startswith('src/backend/voice/')} ==
        {p:v for p,v in current.items() if p.startswith('src/backend/voice/')},
        'release_evidence_source_unit_drift')
    require(r['closure'] == dict(projection_paths=projection, context_paths=contexts,
        compiled_paths=compiled, directory_inventories=inventories), 'release_complete_closure')
    relevant = set(projection) | set(contexts) | set(old['compiled_closure']) | set(compiled)
    relevant |= {p for d in directories for p in set(old_dirs.get(d, {})) | set(new_dirs.get(d, {}))}
    relevant |= {p for p in set(old_tree) | set(current) if p.startswith('src/backend/voice/')}
    delta = [dict(path=p, before=_meta(old_tree,p), after=_meta(current,p), classification='REVIEWED')
             for p in sorted(relevant) if old_tree.get(p) != current.get(p)]
    require(read_json(artifact(repo, r['delta_review'])) == dict(source=FOUNDATION_BASE,
        predecessor=documentation.CONTEXT_SOURCE, entries=delta), 'release_complete_delta_review')
    require(isinstance(r['clauses'], dict) and set(r['clauses']) ==
        {f'{group}{i}' for group in ('A','B') for i in range(1,5)}, 'release_clause_set')
    extension = read_json(artifact(repo, r['extension_evidence']))
    require(isinstance(extension, dict) and set(extension) ==
        {'source','evidence_source','status','delta_sha256','review'} and
        extension['source'] == FOUNDATION_BASE and extension['evidence_source'] == EVIDENCE_SOURCE and
        extension['status'] == 'ACCEPTED' and extension['delta_sha256'] == sha(git(repo,'diff','--raw',
            '--no-abbrev','--no-renames',EVIDENCE_SOURCE,FOUNDATION_BASE)) and
        isinstance(extension['review'], list) and extension['review'] and
        all(p in names for p in extension['review']), 'release_extension_review')
    for clause in r['clauses'].values():
        require(isinstance(clause, dict) and set(clause) == {'status','source','evidence','review'} and
            clause['status'] == 'ACCEPTED' and clause['source'] == EVIDENCE_SOURCE and
            all(isinstance(clause[k], list) and clause[k] and all(p in names for p in clause[k])
                for k in ('evidence','review')), 'release_clause_not_accepted')
    effective = dict(old)
    effective.update(version=NATIVE_VERSION, source=FOUNDATION_BASE, projection=r['projection'], context=r['context'],
                     compiled_closure=compiled, release_record=r)
    return effective


def role_evidence_paths(raw, tree):
    """Derive local inputs from the original concatenated Go JSON stream."""
    fields = ('GoFiles','CgoFiles','CFiles','CXXFiles','MFiles','HFiles','FFiles','SFiles',
              'SwigFiles','SwigCXXFiles','SysoFiles','TestGoFiles','XTestGoFiles',
              'EmbedFiles','TestEmbedFiles','XTestEmbedFiles')
    text = raw.decode('utf-8')
    decoder = json.JSONDecoder(object_pairs_hook=legacy.unique_pairs)
    pos = 0
    paths, directories, modules = set(), set(), set()
    def local(value):
        if not isinstance(value, str) or not value.startswith('/work/'):
            return None
        part = value[len('/work/'):]
        require(part and '..' not in part.split('/') and '\\' not in part, 'release_role_go_path')
        return part
    while pos < len(text):
        while pos < len(text) and text[pos].isspace(): pos += 1
        if pos == len(text): break
        obj, pos = decoder.raw_decode(text, pos)
        require(isinstance(obj, dict), 'release_role_go_object')
        directory = local(obj.get('Dir'))
        if directory is not None:
            directories.add(directory)
            for field in fields:
                values = obj.get(field, [])
                require(isinstance(values, list) and all(isinstance(v, str) for v in values), 'release_role_go_files')
                for name in values:
                    if name.startswith('/'):
                        require(name.startswith('/root/.cache/go-build/') and
                            '..' not in name.split('/') and '\\' not in name, 'release_role_absolute_go_path')
                        continue  # Generated build-cache input is outside the source tree.
                    require('..' not in name.split('/') and '\\' not in name,
                            'release_role_go_path')
                    path = str(PurePosixPath(directory) / name)
                    require(path in tree, 'release_role_missing_go_input')
                    paths.add(path)
        module = obj.get('Module') or {}
        require(isinstance(module, dict), 'release_role_go_module')
        replace = module.get('Replace') or {}
        require(isinstance(replace, dict), 'release_role_go_module')
        for mod in (module, replace):
            directory = local(mod.get('Dir'))
            if directory is not None: modules.add(directory)
    manifests = {p for d in modules for p in (d+'/go.mod', d+'/go.sum') if p in tree}
    return sorted(paths), sorted(directories), sorted(manifests)


def role_review_block(raw, kind):
    blocks = re.findall(r'```json\s*\n(.*?)\n```',raw.decode('utf-8'),re.S)
    values = [read_json(block) for block in blocks]
    matches = [value for value in values if isinstance(value,dict) and value.get('type') == kind]
    require(len(matches) == 1, 'release_role_review_block')
    value = matches[0]
    require(isinstance(value.get('reviewer'),str) and value['reviewer'].strip() and all(
        value.get(flag) is False for flag in ('runtime_pass','activation_authority',
        'current_or_future_path_authority','generic_duplicate_authority')), 'release_role_review_authority')
    return value


def load_role(repo):
    import f13_role_conditional_successor as role
    try:
        old = role.load_record(repo)
    except role.legacy.Rejected as error:
        raise Rejected('release_role_predecessor: '+str(error)) from error
    r = pinned(repo, ROLE_RECORD_PATH, ROLE_SHA256)
    require(set(r) == {'version','status','source','source_tree','predecessor','original_correction',
        'required_current_source','context','closure','compiled_evidence','evidence_source',
        'extension_evidence','evidence','review','historical_runtime_gap',*role.FALSE_FLAGS}, 'release_role_shape')
    require(r['version'] == ROLE_VERSION and r['status'] == 'SOURCE_CONSUMPTION_CONTRACT_CLEAR' and
        r['source'] == FOUNDATION_BASE and r['original_correction'] == role.ORIGINAL_CORRECTION and
        r['historical_runtime_gap'] == old['historical_runtime_gap'] and
        all(r[k] is False for k in role.FALSE_FLAGS), 'release_role_status')
    require(r['predecessor'] == dict(version=role.VERSION, source=old['source'],
        record=role.PINNED_RECORD_COMMIT, path=role.RECORD_PATH, sha256=role.PINNED_RECORD_SHA256),
        'release_role_predecessor')
    for older in (role.PINNED_RECORD_COMMIT, role.ORIGINAL_CORRECTION, role.HISTORICAL_CARRIER,
                  ROLE_EVIDENCE_SOURCE): legacy.ancestor(repo, older, FOUNDATION_BASE)
    require(r['source_tree'] == git(repo,'rev-parse',FOUNDATION_BASE+'^{tree}').decode().strip(),
        'release_role_tree')
    before = documentation.tree_entries(repo, role.PINNED_RECORD_COMMIT)
    current = documentation.tree_entries(repo, FOUNDATION_BASE)
    evidence_tree = documentation.tree_entries(repo, ROLE_EVIDENCE_SOURCE)
    protected = {p for p in before if p.startswith(('docs/testing/role-checkpermission',
        'docs/testing/role-source-v1-evidence/', 'docs/testing/role-source-v2-evidence/'))}
    require(all(before[p] == current.get(p) for p in protected), 'release_role_historical_artifact_drift')
    names = _proofs(repo, r['evidence'])
    prefix = 'docs/testing/role-source-v3-evidence/'
    require(set(names) == {prefix+p for p in ('source-review.md','source-inventory.json',
        'runtime-evidence.zip','extension-review.md','extension-inventory.json')} and
        r['compiled_evidence']['path'] == prefix+'source-inventory.json' and
        r['extension_evidence']['path'] == prefix+'extension-inventory.json' and
        r['review'] == [prefix+'source-review.md',prefix+'extension-review.md'] and
        r['evidence_source'] == ROLE_EVIDENCE_SOURCE, 'release_role_evidence')
    proofs = {p['path']:p for p in r['evidence']}
    require(r['compiled_evidence'] == proofs[r['compiled_evidence']['path']] and
        r['extension_evidence'] == proofs[r['extension_evidence']['path']], 'release_role_evidence_binding')
    inv = read_json(artifact(repo, r['compiled_evidence']))
    require(set(inv) == {'source','tree','command','image','raw_go_list_member','raw_go_list_sha256',
        'compiled_paths','directories','module_manifests','entries','directory_inventories'} and inv['source'] == ROLE_EVIDENCE_SOURCE and
        inv['tree'] == git(repo,'rev-parse',ROLE_EVIDENCE_SOURCE+'^{tree}').decode().strip() and
        inv['command'] == 'go list -mod=readonly -deps -test -json ./internal/principalruntime ./internal/principalgrpc ./internal/grpcsvc' and
        inv['image'] == 'golang:1.26.5-bookworm' and inv['raw_go_list_member'] == 'closure.jsonl',
        'release_role_inventory')
    source_review = role_review_block(artifact(repo,proofs[prefix+'source-review.md']),
        'ROLE-V3-INDEPENDENT-SOURCE-REVIEW-v1')
    native = pinned(repo,RECORD_PATH,NATIVE_SHA256)
    wire = [p for p in native['evidence'] if p['path'] == 'docs/testing/native-p3-source-v5-evidence.zip']
    require(len(wire) == 1, 'release_role_wire_archive')
    try:
        with zipfile.ZipFile(io.BytesIO(artifact(repo,wire[0]))) as archive:
            require(all(archive.namelist().count(name)==1 for name in
                ('review/wire-audit.json','review/current-descriptors.json')), 'release_role_wire_members')
            audit_hash = sha(archive.read('review/wire-audit.json'))
            descriptor_hash = sha(archive.read('review/current-descriptors.json'))
    except zipfile.BadZipFile as error:
        raise Rejected('release_role_wire_archive') from error
    require(source_review.get('status') == 'SOURCE_CONSUMPTION_CONTRACT_CLEAR' and
        source_review.get('source') == ROLE_EVIDENCE_SOURCE and source_review.get('source_tree') == inv['tree'] and
        source_review.get('predecessor_source') == old['source'] and
        source_review.get('predecessor_record') == role.PINNED_RECORD_COMMIT and
        source_review.get('inventory_sha256') == r['compiled_evidence']['sha256'] and
        source_review.get('runtime_evidence_sha256') == proofs[prefix+'runtime-evidence.zip']['sha256'] and
        source_review.get('raw_go_list_sha256') == inv['raw_go_list_sha256'] and
        source_review.get('native_wire_archive_sha256') == wire[0]['sha256'] and
        source_review.get('native_wire_audit_sha256') == audit_hash and
        source_review.get('native_wire_descriptors_sha256') == descriptor_hash, 'release_role_source_review')
    try:
        with zipfile.ZipFile(io.BytesIO(artifact(repo, proofs[prefix+'runtime-evidence.zip']))) as archive:
            require(archive.namelist().count('closure.jsonl') == 1, 'release_role_raw_member')
            raw = archive.read('closure.jsonl')
    except zipfile.BadZipFile as error:
        raise Rejected('release_role_raw_archive') from error
    require(sha(raw) == inv['raw_go_list_sha256'], 'release_role_raw_hash')
    compiled, directories, manifests = role_evidence_paths(raw, evidence_tree)
    old_closure = read_json(role.artifact(repo,old['closure_manifest']['path'],old['closure_manifest']['sha256']))
    directories = sorted(set(directories) | {x['directory'] for x in old_closure['package_inventories']})
    require((compiled,directories,manifests) == (inv['compiled_paths'],inv['directories'],inv['module_manifests'])
        and compiled and directories, 'release_role_derived_inputs')
    evidence_dirs = documentation.immediate_directory_entries(evidence_tree)
    current_dirs = documentation.immediate_directory_entries(current)
    inventory = [dict(directory=d, entries=current_dirs.get(d,{})) for d in directories]
    require(all(current_dirs.get(d,{}) == evidence_dirs.get(d,{}) for d in directories), 'release_role_package_drift')
    required = old['required_current_source']
    require(len(required) == 11 and r['required_current_source'] == required, 'release_role_all_eleven')
    required_names = {x['path'] for x in required}
    contexts = sorted(({x['path'] for x in old['context']} | set(compiled) | set(manifests) |
        {p for d in directories for p in current_dirs.get(d,{})}) - required_names)
    def row(path):
        meta = _meta(current,path)
        require(meta is not None and meta['mode']=='100644' and meta['type']=='blob', 'release_role_entry')
        return dict(path=path,entry=meta,sha256=sha(git(repo,'cat-file','blob',meta['oid'])))
    require(r['context'] == [row(p) for p in contexts] and required == [row(x['path']) for x in required] and
        all(current.get(p) == evidence_tree.get(p) for p in required_names | set(contexts)), 'release_role_complete_unit')
    require(inv['entries'] == [row(p) for p in sorted(required_names | set(contexts))] and
        inv['directory_inventories'] == inventory, 'release_role_reviewed_inventory')
    require(r['closure']['path'] == ROLE_CLOSURE_PATH, 'release_role_closure_path')
    closure = read_json(artifact(repo,r['closure']))
    require(closure == dict(source=FOUNDATION_BASE,compiled_paths=compiled,directory_inventories=inventory),
        'release_role_closure')
    extension = read_json(artifact(repo,r['extension_evidence']))
    require(extension == dict(source=FOUNDATION_BASE,evidence_source=ROLE_EVIDENCE_SOURCE,status='ACCEPTED',
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',ROLE_EVIDENCE_SOURCE,FOUNDATION_BASE)),
        review=[prefix+'extension-review.md']), 'release_role_extension')
    ext_review = role_review_block(artifact(repo,proofs[prefix+'extension-review.md']),
        'ROLE-V3-INDEPENDENT-EXTENSION-REVIEW-v1')
    require(ext_review.get('status') == 'ACCEPT' and ext_review.get('source') == FOUNDATION_BASE and
        ext_review.get('source_tree') == r['source_tree'] and ext_review.get('evidence_source') == ROLE_EVIDENCE_SOURCE and
        ext_review.get('evidence_tree') == inv['tree'] and ext_review.get('delta_sha256') == extension['delta_sha256'] and
        ext_review.get('inventory_sha256') == r['compiled_evidence']['sha256'] and
        ext_review.get('extension_evidence_sha256') == r['extension_evidence']['sha256'] and
        ext_review.get('source_review_sha256') == proofs[prefix+'source-review.md']['sha256'],
        'release_role_extension_review')
    return dict(r, closure=closure, release_record=r)


def production_record(repo):
    import f13_production_source as production
    production.load_bootstrap_fix(repo)  # Recursively revalidate v1-v10.
    preserve_predecessor(repo, production.BOOTSTRAP_RECORD_COMMIT, FOUNDATION_BASE, production.ARCHIVE_PATH)
    native = load_native(repo)
    r = pinned(repo, PRODUCTION_RECORD_PATH, PRODUCTION_SHA256)
    require(set(r) == {'version','status','source','source_tree','predecessor','foundation_base','reuse_policy',
        'collection_base','current_inputs','native_record','role_record','evidence','review','runtime_pass',
        'activation_authority','current_or_future_path_authority'}, 'release_production_shape')
    require(r['version'] == PRODUCTION_VERSION and r['status'] == 'SOURCE_CONSUMPTION_CONTRACT_CLEAR'
        and r['source'] == FOUNDATION_BASE and r['foundation_base'] == FOUNDATION_BASE and r['reuse_policy'] == REUSE_POLICY and
        r['collection_base'] == COLLECTION_BASE and all(r[k] is False for k in
        ('runtime_pass','activation_authority','current_or_future_path_authority')), 'release_production_status')
    require(r['predecessor'] == dict(version=production.BOOTSTRAP_VERSION,
        record=production.BOOTSTRAP_RECORD_COMMIT, path=production.BOOTSTRAP_RECORD_PATH,
        sha256=production.BOOTSTRAP_RECORD_SHA256), 'release_production_predecessor')
    legacy.ancestor(repo, production.BOOTSTRAP_RECORD_COMMIT, FOUNDATION_BASE)
    require(r['source_tree'] == git(repo, 'rev-parse', FOUNDATION_BASE+'^{tree}').decode().strip(),
        'release_production_tree')
    require(r['native_record'] == dict(version=NATIVE_VERSION, source=FOUNDATION_BASE,
        path=RECORD_PATH, sha256=NATIVE_SHA256), 'release_production_native')
    require(r['role_record'] == dict(version=ROLE_VERSION, source=FOUNDATION_BASE,
        path=ROLE_RECORD_PATH, sha256=ROLE_SHA256), 'release_production_role')
    reference = {p:v.rstrip('\0') for p,v in production.entries(repo, production.REFERENCE).items()}
    current = {p:v.rstrip('\0') for p,v in production.entries(repo, FOUNDATION_BASE).items()}
    inventory = read_json(artifact(repo, r['current_inputs']))
    require(inventory == dict(source=FOUNDATION_BASE, reference=production.REFERENCE,
        delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',production.REFERENCE,FOUNDATION_BASE)),
        entries=[dict(path=p, reference_entry=reference.get(p,''), current_entry=current.get(p,''),
            disposition='ORIGINAL_CONTRIBUTION_GATES_REQUIRED') for p in
            git(repo,'diff','--name-only',production.REFERENCE,FOUNDATION_BASE).decode().splitlines()]),
        'release_complete_current_inventory')
    names = _proofs(repo, r['evidence'])
    require(isinstance(r['review'], list) and r['review'] and
        all(p in names for p in r['review']), 'release_production_review')
    role = load_role(repo)
    _record_delta(repo, native['release_record'], r, role['release_record'])
    return r


def applicable(repo, revision='HEAD'):
    return FOUNDATION_BASE is not None and git(repo, 'merge-base', FOUNDATION_BASE, revision).decode().strip() == FOUNDATION_BASE


def _carrier(repo, source):
    expected = documentation.tree_entries(repo, RECORD_COMMIT)
    actual = documentation.tree_entries(repo, source)
    require({p:v for p,v in actual.items() if p not in CARRIER_PATHS} ==
        {p:v for p,v in expected.items() if p not in CARRIER_PATHS}, 'release_carrier_delta')
    for path in CARRIER_PATHS:
        meta = legacy.entry(repo, source, path)
        require(meta is not None and meta['mode'] == '100644' and meta['type'] == 'blob' and
            git(repo, 'cat-file', 'blob', meta['oid']) == (ROOT/path).read_bytes(), 'release_executed_carrier')
    require(legacy.entry(repo, RECORD_COMMIT, 'scripts/ci/f13_release_source.py') !=
            legacy.entry(repo, source, 'scripts/ci/f13_release_source.py'), 'release_later_carrier_bytes')


def check_continuation(repo, foundation, source, record, role=None):
    """Compare the whole reviewed unit at every newly introduced DAG commit."""
    import f13_production_source as production
    h, carrier, first = (foundation[k] for k in ('base','source','target'))
    projection={row['path'] for row in record['projection']}
    contexts={row['path'] for row in record['context']}
    compiled=set(record['compiled_closure'])
    require(not CARRIER_PATHS & (projection | compiled) and
        not any(p.startswith('src/backend/voice/') for p in CARRIER_PATHS), 'release_carrier_runtime_overlap')
    baseline=documentation.tree_entries(repo,h)
    recorded=documentation.tree_entries(repo,RECORD_COMMIT)
    carrier_tree=documentation.tree_entries(repo,carrier)
    protected=(set(recorded)-set(baseline))-CARRIER_PATHS
    protected |= {p for p in baseline if p.startswith(('docs/testing/native-p3-source',
        'docs/testing/f13-production', 'docs/testing/role-checkpermission',
        'docs/testing/role-source-v1-evidence/', 'docs/testing/role-source-v2-evidence/'))}
    protected.add(production.ARCHIVE_PATH)
    require(production.ARCHIVE_PATH in baseline, 'release_missing_historical_archive')
    role = role or {'required_current_source': [], 'context': [], 'closure': {'directory_inventories': []}}
    role_paths = {row['path'] for group in ('required_current_source', 'context') for row in role[group]}
    require(not CARRIER_PATHS & role_paths, 'release_carrier_role_overlap')
    names=projection | contexts | compiled | role_paths | protected | set(CARRIER_PATHS)
    expected={p:(carrier_tree.get(p) if p in CARRIER_PATHS else
                 recorded.get(p) if p in protected and p not in baseline else baseline.get(p)) for p in names}
    directories=sorted({x['directory'] for x in record['release_record']['closure']['directory_inventories'] +
                        role['closure']['directory_inventories']})
    base_dirs=documentation.immediate_directory_entries(baseline)
    carrier_dirs=documentation.immediate_directory_entries(carrier_tree)
    expected_dirs={d:dict(base_dirs.get(d,{})) for d in directories}
    for d in directories:
        for p in CARRIER_PATHS:
            if p.rpartition('/')[0]==d:
                expected_dirs[d].pop(p,None)
                if p in carrier_dirs.get(d,{}):expected_dirs[d][p]=carrier_dirs[d][p]
    voice={p:v for p,v in baseline.items() if p.startswith('src/backend/voice/')}
    def verify(revision):
        actual=documentation.tree_entries(repo,revision)
        require(all(actual.get(p)==v for p,v in expected.items()), 'release_continuation_entry_drift')
        actual_dirs=documentation.immediate_directory_entries(actual)
        require(all(actual_dirs.get(d,{})==expected_dirs[d] for d in directories), 'release_continuation_directory_drift')
        require({p:v for p,v in actual.items() if p.startswith('src/backend/voice/')}==voice,
                'release_continuation_voice_drift')
    verify(carrier)
    verify(source)
    verify(first)
    for revision in git(repo,'rev-list','--topo-order','--reverse',source,'^'+first).decode().splitlines():
        legacy.ancestor(repo,first,revision)
        verify(revision)


def native_qualify(repo, base, source, target, event, ref, event_sha, before, base_ref):
    require(Path(repo).resolve() == ROOT.resolve(), 'release_actual_root')
    require(git(repo,'rev-parse','HEAD').decode().strip() == target, 'release_actual_head')
    r = production_record(repo)
    foundation=check_event(repo, base, source, target, event, ref, event_sha, before, base_ref, FOUNDATION_BASE, RECORD_COMMIT)
    _carrier(repo, foundation['source'])
    record = load_native(repo)
    check_continuation(repo, foundation, source, record, load_role(repo))
    return dict(version=NATIVE_VERSION, consumer='MASTER-SOURCE-v1', base=base, source=source,
        target=target, content_base=record['base'], snapshot=FOUNDATION_BASE, record=RECORD_COMMIT,
        record_sha256=NATIVE_SHA256, paths=[x['path'] for x in record['projection']],
        retain_guarded_and_runtime_history=True, collection_base=COLLECTION_BASE, foundation=foundation,
        qualification='FOUNDATION' if base==FOUNDATION_BASE else 'UNCHANGED_UNIT_CONTINUATION',reuse_policy=REUSE_POLICY,
        event_delta_sha256=sha(git(repo,'diff','--raw','--no-abbrev','--no-renames',base,source)))


def verified_record(repo, binding):
    event = os.environ
    expected = native_qualify(repo, binding['base'], binding['source'], binding['target'],
        event.get('GITHUB_EVENT_NAME',''), event.get('GITHUB_REF',''), event.get('GITHUB_SHA',''),
        event.get('VOICE_CI_PUSH_BEFORE_SHA',''), event.get('GITHUB_BASE_REF',''))
    require(binding == expected, 'release_native_binding')
    return load_native(repo)


def predecessor_record(repo, binding):
    verified_record(repo, binding)
    return documentation, documentation.load_context_record(repo)


def collection_base(repo, binding):
    verified_record(repo, binding)
    return COLLECTION_BASE


def audit_map(repo, binding, map_path, history_path):
    """Retain the original row predicates and use the distinct collection base."""
    record = verified_record(repo, binding)
    expected = {row['path']: row['after'] for row in record['projection']}
    historical = {}
    for line in Path(history_path).read_text().splitlines():
        fields = line.split('\t')
        if fields[0] == 'supersession':
            require(len(fields) == 6 and fields[-1] in ('0','2'), 'history_supersession')
            continue
        require(len(fields) == 8 and fields[0] in expected and fields[1] in ('true','false'), 'history_row')
        historical.setdefault(fields[0], []).append(line)
    selected = {}
    for line in Path(map_path).read_text().splitlines():
        fields = line.split('\t')
        require(len(fields) == 8, 'map_shape')
        name, trigger, owner, parent, source_base, source_head, content_base, best = fields
        if name not in expected:
            continue
        require(name not in selected and trigger in ('true','false'), 'map_duplicate_or_trigger')
        require(name in historical and historical[name][-1] == line, 'map_history_binding')
        for value in (owner,parent,source_base,source_head,content_base):
            legacy.oid(value)
        owner_parents = git(repo,'rev-list','--parents','-n','1',owner).decode().split()
        require(len(owner_parents) >= 2 and owner_parents[1] == parent, 'map_owner_parent')
        legacy.ancestor(repo, owner, binding['source'])
        legacy.ancestor(repo, source_head, record['source'])
        legacy.ancestor(repo, source_head, owner)
        legacy.ancestor(repo, source_base, source_head)
        legacy.ancestor(repo, content_base, parent)
        require(all(re.fullmatch('[0-9a-f]{40}', v) for v in best.split(',')), 'map_best_bases')
        bases = best.split(',')
        if len(owner_parents) == 2:
            require(source_head == owner and source_base == parent and bases == [parent], 'map_direct_owner')
        else:
            require(any(sorted(git(repo,'merge-base','--all',parent,actual).decode().splitlines()) == sorted(bases)
                and (source_head == owner or subprocess.run(['git','--no-replace-objects','-C',str(repo),
                    'merge-base','--is-ancestor',source_head,actual],capture_output=True).returncode == 0)
                for actual in owner_parents[2:]), 'map_imported_source')
        require(legacy.entry(repo,source_head,name) == expected[name] == legacy.entry(repo,binding['target'],name),
                'map_entry')
        delta = git(repo,'diff','--name-only','--no-renames',source_base,source_head).decode().splitlines()
        require(name in delta, 'map_source_delta')
        require(trigger == 'true' or not legacy.runtime_changed(delta), 'map_runtime_taint')
        require(trigger == 'true' or all(row.split('\t')[1] == 'false' for row in historical[name]),
                'map_prior_runtime_taint')
        selected[name] = line
    changed = set(git(repo,'diff','--name-only','--no-renames',COLLECTION_BASE,binding['source']).decode().splitlines())
    require(set(selected) == set(expected) & changed, 'map_incomplete')
    return dict(binding=binding, map_sha256=legacy.digest(Path(map_path).read_bytes()),
        history_sha256=legacy.digest(Path(history_path).read_bytes()), rows=selected)


def production_qualify(repo):
    import f13_production_source as production
    require(Path(repo).resolve() == ROOT.resolve(), 'production_actual_root')
    record = production_record(repo)
    target = git(repo, 'rev-parse', 'HEAD').decode().strip()
    event = {k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA',
        'GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    parents = git(repo, 'rev-list', '--parents', '-n', '1', target).decode().split()
    require(len(parents) == 3, 'release_ordered_parents')
    native=native_qualify(repo, parents[1], parents[2], target, event['GITHUB_EVENT_NAME'], event['GITHUB_REF'],
        event['GITHUB_SHA'], event['VOICE_CI_PUSH_BEFORE_SHA'], event['GITHUB_BASE_REF'])
    old = production.load_record(repo)
    return dict(type=PRODUCTION_VERSION, record=RECORD_COMMIT, record_sha256=PRODUCTION_SHA256,
        predecessor_record=production.BOOTSTRAP_RECORD_COMMIT, historical_validation_source=old['source'],
        reviewed_source=FOUNDATION_BASE, reviewed_tree=record['source_tree'], base=parents[1],
        collection_base=COLLECTION_BASE, source=parents[2], target=target,
        foundation=native['foundation'],qualification=native['qualification'],reuse_policy=REUSE_POLICY,
        event_delta_sha256=native['event_delta_sha256'],
        historical_collector=production.HISTORICAL_SCOPE, native_content_baseline=production.NATIVE_CONTENT_BASE,
        current_inputs_sha256=record['current_inputs']['sha256'], event=event,
        admission=False, runtime_pass=False, current_or_future_path_authority=False)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=('collection-base',))
    parser.add_argument('repo')
    parser.add_argument('binding')
    args = parser.parse_args()
    try:
        binding = read_json(Path(args.binding).read_bytes())
        print(collection_base(args.repo, binding))
    except (Rejected, ValueError, KeyError, TypeError, OSError) as error:
        print('Release source rejected: '+str(error), file=sys.stderr)
        return 1
    return 0


def check_event(repo, base, source, target, event, ref, event_sha, before, base_ref,
                foundation_base, record_commit):
    require, git = legacy.require, legacy.git
    base, source, target, foundation_base, record_commit = map(
        legacy.oid, (base, source, target, foundation_base, record_commit))
    require(git(repo,'rev-parse','--is-shallow-repository').decode().strip()=='false','release_shallow_history')
    require(event_sha == target, 'release_exact_event_sha')
    require((event == 'push' and ref == 'refs/heads/master' and before == base) or
            (event == 'pull_request' and base_ref == 'master' and
             re.fullmatch(r'refs/pull/[1-9][0-9]*/merge', ref)), 'release_master_event')
    require(git(repo, 'rev-list', '--parents', '-n', '1', record_commit).decode().split() ==
            [record_commit, foundation_base], 'release_record_direct_parent')
    require(git(repo, 'rev-list', '--parents', '-n', '1', target).decode().split() ==
            [target, base, source], 'release_ordered_parents')
    legacy.ancestor(repo, base, source)
    for revision in (base, source):
        require(COLLECTION_BASE in git(repo,'rev-list','--first-parent',revision).decode().splitlines(),
                'release_collection_first_parent')
    require(git(repo, 'rev-parse', source + '^{tree}') ==
            git(repo, 'rev-parse', target + '^{tree}'), 'release_result_tree')
    if base == foundation_base:
        first,carrier=target,source
    else:
        chain=git(repo,'rev-list','--first-parent','--reverse',foundation_base+'..'+base).decode().splitlines()
        require(foundation_base in git(repo,'rev-list','--first-parent',base).decode().splitlines()
            and chain, 'release_foundation_first_parent')
        first=chain[0]
        parents=git(repo,'rev-list','--parents','-n','1',first).decode().split()
        require(len(parents)==3 and parents[1]==foundation_base, 'release_foundation_ordered_parents')
        carrier=parents[2]
        legacy.ancestor(repo,first,base)
        legacy.ancestor(repo,carrier,source)
    legacy.ancestor(repo,record_commit,carrier)
    require(carrier!=record_commit, 'release_later_carrier')
    require(git(repo,'rev-parse',carrier+'^{tree}')==git(repo,'rev-parse',first+'^{tree}'),
        'release_foundation_tree')
    return dict(base=foundation_base,source=carrier,target=first)


if __name__ == '__main__':
    sys.exit(main())
