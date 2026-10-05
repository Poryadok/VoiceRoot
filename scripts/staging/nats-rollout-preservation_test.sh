#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/repo/scripts/staging" "$tmp/repo/scripts/storage" "$tmp/repo/deploy/nats" "$tmp/bin"
cp -R "$ROOT/scripts/staging/nats-rollout-preservation" "$tmp/repo/scripts/staging/"
mkdir -p "$tmp/repo/scripts/staging/nats-known-baseline"
cp "$ROOT/scripts/staging/nats-known-baseline/"{commands,controller,stage_runtime,docker_runtime}.py "$tmp/repo/scripts/staging/nats-known-baseline/"
cp "$ROOT/scripts/staging/render-and-apply.sh" "$ROOT/scripts/staging/nats-generation.sh" "$tmp/repo/scripts/staging/"
cp "$ROOT/deploy/nats/acl-intent.yaml" "$tmp/repo/deploy/nats/"
printf '#!/usr/bin/env bash\n' > "$tmp/repo/scripts/staging/load-staging-domains.sh"
for name in preflight-resend-key apply-gateway-ingress rollout-subset apply-livekit-ingress apply-migrate-jobs apply-app-manifests apply-infra rollout-app-tier; do
  printf '#!/usr/bin/env bash\nprintf "%%s\\n" "%s" >> "$MUTATIONS"\n' "$name" > "$tmp/repo/scripts/staging/$name.sh"
done
for name in apply-signing-env apply-minio-cors; do
  printf '#!/usr/bin/env bash\nprintf "%%s\\n" "%s" >> "$MUTATIONS"\n' "$name" > "$tmp/repo/scripts/storage/$name.sh"
done
cat > "$tmp/bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
if [[ "$*" == 'get configmap voice-nats-generation -n voice-staging -o json' ]]; then
  printf '%s\n' '{"metadata":{"uid":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","resourceVersion":"100"},"data":{"phase":"active","generation":"r20260930a4","previousGeneration":"r20260930a3","dataPVC":"voice-nats-jsdata-d202610040049430b"}}'
elif [[ "$1" == get ]]; then
  exit 1
else
  printf 'kubectl %s\n' "$*" >> "$MUTATIONS"
fi
MOCK
chmod +x "$tmp/bin/kubectl"
export PATH="$tmp/bin:$PATH" MUTATIONS="$tmp/mutations" VOICE_IMAGE_TAG=next-version
export VOICE_NATS_ACL_PROOF_SHA="$(sha256sum "$tmp/repo/deploy/nats/acl-intent.yaml" | cut -d' ' -f1)"
export VOICE_NATS_ACL_PROOF_GENERATION=r20260930a4
failures=0
for mode in full app-only images-only; do
 for receipt in '' "$tmp/nonexistent-receipt.json"; do
  : > "$MUTATIONS"
  if VOICE_NATS_PRESERVATION_RECEIPT="$receipt" DEPLOY_MODE="$mode" bash "$tmp/repo/scripts/staging/render-and-apply.sh" > "$tmp/output" 2>&1; then
    echo "FAIL $mode accepted an ordinary version rollout with missing or nonexistent preservation receipt"
    failures=$((failures+1))
  fi
  if [[ -s "$MUTATIONS" ]]; then
    echo "FAIL $mode mutated staging before preservation ownership was verified"
    failures=$((failures+1))
  fi
 done
done
[[ "$failures" == 0 ]] || exit 1
echo 'NATS_ROLLOUT_PRESERVATION=PASS missing-receipt-zero-mutations-all-modes'

# Root-owned paths below exist only inside this disposable Linux test container.
# Exercise real guard, claim, generation check and paused apply subprocesses.
export TEST_SOURCE="$tmp/repo" TEST_STATE="$tmp/state.json" TEST_APPLIED="$tmp/applied.json"
export VOICE_NATS_PRESERVATION_RECEIPT=/var/lib/voice-nats-preservation/rollout-1a2b3c4d5e6f/apply-authorization.json
cat > "$tmp/bin/kubectl" <<'MOCK'
#!/usr/bin/env python3
import json,os,sys
from pathlib import Path
p=Path(os.environ['TEST_STATE']);s=json.loads(p.read_bytes());a=sys.argv[1:]
if a[0]=='get':print(json.dumps(s[a[1]]));sys.exit(0)
if a[0]=='create' and '--dry-run=client' in a:
    raw=sys.stdin.read();name=next(line.split(':',1)[1].strip() for line in raw.splitlines() if line.strip().startswith('name:'))
    print(json.dumps({'kind':'ConfigMap','metadata':{'name':name},'data':{'bootstrap.sh':'SAFE_FIXTURE\n'}}));sys.exit(0)
body=json.load(sys.stdin)
if a[0]=='patch':
    tests={x['path']:x['value'] for x in body if x['op']=='test'}
    m=s['configmap'];assert tests['/metadata/uid']==m['metadata']['uid'];assert tests['/metadata/resourceVersion']==m['metadata']['resourceVersion']
    assert m['data']['phase']=='rollout-prepared';m['data']['phase']='rollout-applying';m['metadata']['resourceVersion']='101'
    p.write_text(json.dumps(s));label='MARKER_CAS';output=json.dumps(m)
elif a[0]=='apply':
    assert body['kind']=='List';Path(os.environ['TEST_APPLIED']).write_text(json.dumps(body));label='APPLY_LIST';output='deployment.apps/voice-gateway'
else:raise AssertionError('unexpected mutation')
with open(os.environ['MUTATIONS'],'a') as f:f.write(label+'\n')
print(output)
MOCK
chmod +x "$tmp/bin/kubectl"
for mode in full app-only images-only; do
 export DEPLOY_MODE="$mode"
 python3 - <<'FIXTURE'
import datetime as dt,hashlib,json,os
from pathlib import Path
uid=lambda c:c*8+'-'+c*4+'-'+c*4+'-'+c*4+'-'+c*12
source=Path(os.environ['TEST_SOURCE']);path=Path(os.environ['VOICE_NATS_PRESERVATION_RECEIPT'])
path.parent.mkdir(parents=True,exist_ok=True);path.parent.parent.chmod(0o750);path.parent.chmod(0o750)
claim='voice-nats-jsdata-d202610040049430b';pvpath='/var/lib/rancher/k3s/storage/pvc-'+uid('b')+'_voice-staging_'+claim
image='example.invalid/gateway:next-version@sha256:'+'1'*64
template={'spec':{'containers':[{'name':'gateway','image':image}]}}
rows=[{'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-gateway','namespace':'voice-staging'},'spec':{'replicas':1,'template':template}}]
raw=json.dumps(rows).encode();manifest=path.parent/'apply-manifests.json';manifest.write_bytes(raw);manifest.chmod(0o640)
data={'phase':'rollout-prepared','knownRolloutOperation':'1a2b3c4d5e6f','generation':'r20260930a4','previousGeneration':'r20260930a3','dataPVC':claim}
state={'namespace':{'metadata':{'uid':uid('d')}},'configmap':{'metadata':{'uid':uid('a'),'resourceVersion':'100'},'data':data},
 'pvc':{'metadata':{'uid':uid('b')},'spec':{'volumeName':'pvc-'+uid('b')},'status':{'phase':'Bound'}},
 'pv':{'metadata':{'uid':uid('c')},'spec':{'claimRef':{'uid':uid('b')},'local':{'path':pvpath},'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}},
 'deployment':{'spec':{'replicas':0,'template':{'spec':{'volumes':[{'name':'jsdata','persistentVolumeClaim':{'claimName':claim}}]}}}}}
Path(os.environ['TEST_STATE']).write_text(json.dumps(state))
sha=lambda b:hashlib.sha256(b).hexdigest()
scripts=[]
for part in ('realtime','notification','search','analytics-chat'):
    file=source/'deploy/templates'/('nats-'+part+'-bootstrap.yaml');file.parent.mkdir(parents=True,exist_ok=True)
    file.write_text('apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: voice-nats-'+part+'-bootstrap\n')
    scripts.append({'part':part,'sha256':sha(b'SAFE_FIXTURE\n')})
(source/'scripts/staging/nats-known-baseline/deployed-contract.json').write_text(json.dumps({'scripts':scripts}))
receipt={'schema':'nats-rollout-preservation-v1','operation':'1a2b3c4d5e6f','namespace_uid':uid('d'),
 'marker':{'uid':uid('a'),'resourceVersion':'100',**{k:data[k] for k in ('generation','previousGeneration','dataPVC')}},
 'pvc':{'name':claim,'uid':uid('b'),'pv_name':'pvc-'+uid('b')},'pv':{'uid':uid('c'),'kind':'local','path':pvpath,'node':'pmdebook'},
 'target':{'registry':'ghcr.io/voiceroot/voiceroot','tag':'next-version','mode':os.environ['DEPLOY_MODE'],'changed_services':[],
 'source_hashes':{'deploy/nats/acl-intent.yaml':sha((source/'deploy/nats/acl-intent.yaml').read_bytes())},
 'template_hashes':{'voice-gateway':sha(json.dumps(template,sort_keys=True,separators=(',',':')).encode())},'input_template_hashes':{'voice-gateway':sha(json.dumps(template,sort_keys=True,separators=(',',':')).encode())},'images':{'voice-gateway/gateway':image},'manifest_sha256':sha(raw),'nonnats_sha256':'4'*64,'migration_sha256':sha(b'[]')},
 'backup':{'archive_sha256':'1'*64,'manifest_sha256':'2'*64,'census_sha256':'3'*64,'off_node_verified':True},
 'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat()}
path.write_text(json.dumps(receipt));path.chmod(0o640)
FIXTURE
 : > "$MUTATIONS"
 if VOICE_NATS_ACL_PROOF_SHA=invalid bash "$tmp/repo/scripts/staging/render-and-apply.sh" > "$tmp/output" 2>&1; then
  echo 'FAIL mismatched ACL proof accepted'; exit 1
 fi
 [[ ! -s "$MUTATIONS" ]] || { echo 'FAIL mismatched ACL proof consumed marker before read-only preflight'; exit 1; }
 bash "$tmp/repo/scripts/staging/render-and-apply.sh" > "$tmp/output" 2>&1 || { cat "$tmp/output"; exit 1; }
 [[ "$(cat "$MUTATIONS")" == $'MARKER_CAS\nAPPLY_LIST' ]] || { echo 'FAIL legacy mutation path executed'; exit 1; }
 python3 - <<'CHECK'
import json,os
from pathlib import Path
rows=json.loads(Path(os.environ['TEST_APPLIED']).read_bytes())['items']
assert all(r['kind']!='Job' for r in rows)
assert all(r['spec']['replicas']==0 for r in rows if r['kind']=='Deployment')
CHECK
 : > "$MUTATIONS"
 if bash "$tmp/repo/scripts/staging/render-and-apply.sh" > "$tmp/output" 2>&1; then echo 'FAIL single-use receipt reused'; exit 1; fi
 [[ ! -s "$MUTATIONS" ]] || { echo 'FAIL reused receipt mutated workloads'; exit 1; }
done
echo 'NATS_ROLLOUT_PRESERVATION=PASS valid-receipt-paused-apply-and-second-use-veto-all-modes'
