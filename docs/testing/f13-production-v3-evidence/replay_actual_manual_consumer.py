"""Replay one preserved failing consumer; never resumes or seals its trace."""
import json,os,sys,zipfile
from pathlib import Path
repo=Path(sys.argv[1]);observed=Path(sys.argv[2]);supplement=Path(sys.argv[3]);out=Path(sys.argv[4]);out.mkdir(parents=True,exist_ok=True)
sys.path.insert(0,str(repo/'scripts/ci'))
import f13_f54_history as h
import f13_production_source as p
trace=observed/'f13-production-trace.jsonl'
last=json.loads(trace.read_text().splitlines()[-1])
print('last observed',json.dumps(last)[:1600],flush=True)
assert '3071ddad390443de4119c796cdad9899ff08f7fd' in json.dumps(last)
assert 'protos/voice/auth/v1/auth.proto' in json.dumps(last)
inputs=out/'inputs';p.materialize(repo,inputs)
cap=h.prepare_3071_production_capabilities(repo,inputs/'fixed3071-unit',observed/'f13-native-p3-binding.json')
cap_path=out/'reconstructed-3071-capability.json';cap_path.write_text(json.dumps(cap))
name='protos/voice/auth/v1/auth.proto'
args=(repo,cap_path,h.J3071,h.J3071_O,h.J3071_P1,h.J3071_P2,name,h.tree_entry(repo,h.J3071,name).rstrip('\0'))
def rejected():
 try:h.composition_3071_capability(*args)
 except ValueError as error:
  assert str(error)=='fixed3071_all_manual_evidence_bindings',str(error)
  return str(error)
 raise AssertionError('missing/tampered evidence unexpectedly accepted')
red=rejected();print('RED',red,flush=True)
folder=inputs/'fixed3071-unit/manual';facts=json.loads((folder/'facts.json').read_text())
with zipfile.ZipFile(supplement) as z:
 assert sorted(z.namelist())==sorted(facts['bindings']) and len(z.namelist())==11
 for filename in sorted(z.namelist()):
  assert Path(filename).name==filename
  data=z.read(filename);assert dict(bytes=len(data),sha256=h.sha(data))==facts['bindings'][filename]
  assert not (folder/filename).exists();(folder/filename).write_bytes(data)
green=h.composition_3071_capability(*args)
assert green['composition']=='REVIEWED_MANUAL_WHOLE' and green['merge_tree_exit']==1
print('GREEN unchanged actual composition consumer',flush=True)
negative=[]
for filename in sorted(facts['bindings']):
 file=folder/filename;data=file.read_bytes();file.write_bytes(data+b'altered')
 negative.append(dict(name=filename,rejection=rejected()));file.write_bytes(data)
print('TAMPERS',len(negative),'rejected',flush=True)
proof=dict(scope='ONE_ACTUAL_FAILED_CONSUMER_REPLAY_NOT_PRODUCER_COMPLETION',
 actual_target=h.git(repo,'rev-parse','HEAD').decode().strip(),trace_sha256=h.sha(trace.read_bytes()),
 last_observed_event=last,reconstructed_capability_sha256=h.sha(cap_path.read_bytes()),
 original_native_binding_sha256=h.sha((observed/'f13-native-p3-binding.json').read_bytes()),
 original_facts_sha256=h.sha((folder/'facts.json').read_bytes()),supplement_sha256=h.sha(supplement.read_bytes()),
 red=red,green=green,negative=negative,
 note='Original production code at exact observed merge HEAD. Fresh private extraction/capability reconstruction with same actual Native binding and final observed J/path arguments. Preserved trace is read-only, never appended, resumed, sealed or counted as terminal success.')
(out/'actual3071-consumer-replay.json').write_bytes((json.dumps(proof,indent=2,sort_keys=True)+'\n').encode())
