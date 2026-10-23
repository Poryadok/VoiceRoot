"""Reproduce only the eleven immutable historical manual packet bindings."""
import hashlib,importlib.util,json,subprocess,sys,zipfile
from pathlib import Path
repo=Path(sys.argv[1]);out=Path(sys.argv[2]);out.mkdir(parents=True,exist_ok=True)
sys.path.insert(0,str(repo/'scripts/ci'))
import f13_f54_history as h
h.manual_3071_composition(repo)
with zipfile.ZipFile(repo/'scripts/ci/testdata/f13-exact-j-547/historical-inputs.zip') as z:
 manifest=json.loads(z.read('transport-manifest.json'))
 item=next(x for x in manifest['logical_files'] if x['logical_path']=='/fixed3071-unit/manual/facts.json')
 raw=z.read('objects/'+item['sha256']);assert h.sha(raw)==h.J3071_MANUAL_PACKET_SHA
 packet=json.loads(raw)
refs=dict(O=h.J3071_O,bestA=h.J3071_BASES[0],bestB=h.J3071_BASES[1],P1=h.J3071_P1,P2=h.J3071_P2,J=h.J3071,AUTO=h.J3071_AUTO_TREE)
data={label+'.docs-DEPLOYMENT.md':h.git(repo,'show',ref+':'+h.J3071_DOC) for label,ref in refs.items()}
args=['git','--no-replace-objects','-C',str(repo),'merge-tree','--write-tree','-z',h.J3071_P1,h.J3071_P2]
result=subprocess.run(args,capture_output=True);assert result.returncode==1
data.update({'merge-tree.messages.stdout':result.stdout,'merge-tree.messages.stderr':result.stderr,
 'whole-auto-to-committed.patch':h.git(repo,'diff','--binary',h.J3071_AUTO_TREE,h.J3071),
 'whole-auto-to-committed.raw':h.git(repo,'diff','--raw','--no-abbrev','--no-renames','-z',h.J3071_AUTO_TREE,h.J3071)})
assert set(data)==set(packet['bindings'])
bindings={name:dict(bytes=len(raw),sha256=h.sha(raw)) for name,raw in sorted(data.items())}
assert bindings==packet['bindings']
archive=out/'fixed3071-manual-v1.zip'
with zipfile.ZipFile(archive,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=9) as z:
 for name,raw in sorted(data.items()):
  info=zipfile.ZipInfo(name,(1980,1,1,0,0,0));info.compress_type=zipfile.ZIP_DEFLATED;info.external_attr=0o100644<<16;z.writestr(info,raw)
proof=dict(status='EXACT_HISTORICAL_BYTES_REPRODUCED_NOT_CURRENT_PRODUCER_PASS',facts_sha256=h.sha(raw) if False else h.J3071_MANUAL_PACKET_SHA,
 original_archive_unchanged=True,original_facts_unchanged=True,refs=refs,document=h.J3071_DOC,
 merge_tree_arguments=args[4:],merge_tree_exit=result.returncode,git_version=h.git(repo,'--version').decode().strip(),
 bindings=bindings,supplement=dict(path=archive.name,bytes=archive.stat().st_size,sha256=h.sha(archive.read_bytes())),
 note='Original transport manifest omitted these eleven transitive facts.json dependencies. Every reproduced length and SHA equals the unchanged frozen packet.')
(out/'fixed3071-manual-derivation.json').write_bytes((json.dumps(proof,indent=2,sort_keys=True)+'\n').encode())
print(json.dumps(proof,indent=2))
