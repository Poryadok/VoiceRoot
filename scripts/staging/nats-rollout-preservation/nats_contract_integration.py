"""Owned Linux fixture: pinned server defaults and additive native preservation.

Synthetic realm only. Same-host copy is a mechanism fixture, not off-node live
custody or accepted staging rollout. No live credentials or PVC are referenced.
"""
import base64,copy,hashlib,json,os
from pathlib import Path
import shutil,sys,tempfile,time,uuid
import rollout_census_integration as fixture

def inner(base):
    sys.path[:0]=['/bundle','/source/scripts/staging/nats-known-baseline']
    from docker_runtime import DockerRuntime,NATS_IMAGE
    from scenario import ready
    from native_store import archive_closed_store,verify_archive
    from nats_migration import apply_contract
    import nats_migration
    from preserve import verify_post_apply,canonical,capture_cut
    from nats_contract_actor import Actor
    from nats_contract_plan import compile_plan,execute,verify_census,DECLARATIONS
    from rollout_census import census
    from bootstrap_renewal import invoke
    import bootstrap_auth,guard
    base=Path(base);fixture.call(['/bundle/helper','prepare',str(base)])
    for name in ('kernel','helper'):
        shutil.copyfile('/bundle/'+name,base/name);(base/name).chmod(0o550);os.chown(base/name,0,65532)
    for role in ('realtime','notification','analytics-chat','search'):
        path=base/('bootstrap-'+role+'.sh')
        shutil.copyfile('/source/scripts/staging/nats-known-baseline/testdata/deployed-bootstrap-20261004/'+role+'-bootstrap.sh',path)
        path.chmod(0o440);os.chown(path,0,65532)
    for p in [base/'inputs',base/'server.conf',*list((base/'inputs').iterdir())]:os.chown(p,0,65532)
    base.chmod(0o755)
    # These are disposable fixture identities, created inside this volume.
    # No installed signer, live credentials, or production authority is read.
    old=(base/'inputs/bootstrap.creds').read_text()
    token=old.split('-----BEGIN NATS USER JWT-----\n',1)[1].split('\n',1)[0]
    claims=json.loads(base64.urlsafe_b64decode(token.split('.')[1]+'=='))
    actor_sha=hashlib.sha256(claims['sub'].encode()).hexdigest()
    public=(base/'inputs/account.public').read_text().strip();account_sha=hashlib.sha256(public.encode()).hexdigest()
    request={'schema':'voice-nats-existing-bootstrap-renewal-v1','old_creds':old,
        'account_jwt':(base/'inputs/account.jwt').read_text().strip(),'operator_jwt':(base/'inputs/operator.jwt').read_text().strip(),
        'signer_seed':(base/'inputs/synthetic-account.seed').read_text(),'actor_sha256':actor_sha,'account_sha256':account_sha}
    binary=base/'bootstrap-renewer';shutil.copyfile('/bundle/bootstrap-renewer',binary);binary.chmod(0o550)
    renewed=invoke(binary,request)
    operator={'data':{key:base64.b64encode((base/'inputs'/key).read_bytes()).decode() for key in ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}}
    guard.ROOT=base/'root-custody';guard.ROOT.mkdir(mode=0o700)
    bootstrap_auth.ACCOUNT_SHA=account_sha
    auth=bootstrap_auth.prove(renewed,operator,lambda:None)
    assert auth['verified'] and auth['reply_prefix']=='_INBOX.voice.bootstrap.reply'
    (base/'inputs/bootstrap.creds').write_bytes(renewed)
    operation='contract'+uuid.uuid4().hex[:12]
    runtime=DockerRuntime(base,operation);removed=[];fresh_runtime=None
    try:
        store=base/'store';store.mkdir(mode=0o700);os.chown(store,65532,65532)
        broker=runtime.start_broker('original',store);ready(runtime,broker);runtime.bootstrap(broker)
        seeded=runtime.kernel(broker,'seed')/'fixture.json'
        snapshot=base/'inputs/fixture.json';snapshot.write_bytes(seeded.read_bytes());snapshot.chmod(0o440);os.chown(snapshot,0,65532)
        extra=runtime.create('extra-seed',NATS_IMAGE,[(base/'helper','/kernel',False),(base/'inputs','/inputs',False)],['/kernel','extra'],broker)
        runtime.run(['start',extra]);assert runtime.run(['wait',extra])=='0'
        account=(base/'inputs/account.public').read_text().strip()
        before=census(runtime,broker,account);assert sum(s['state']['messages'] for s in before['streams'])==5
        raw=runtime.monitor_jsz(broker);details=next(a for a in raw['account_details'] if a['id']==account)['stream_detail']
        infos={s['name']:{'config':s['config'],'state':s['state']} for s in details}
        runtime.stop(broker)
        assert not runtime.inspect(broker)['State']['Running']
        native=archive_closed_store(store,base/'before.tar')
        shutil.copyfile(base/'before.tar',base/'copy.tar');assert verify_archive(base/'copy.tar',native)
        def fenced():assert not runtime.inspect(broker)['State']['Running']
        # Defaults originate from this exact pinned server, account and source
        # declaration on an empty owned scratch store, never a supplied hash.
        scratch=base/'auth-store';scratch.mkdir(mode=0o700);os.chown(scratch,65532,65532)
        normalizer=runtime.start_broker('normalizer',scratch);ready(runtime,normalizer)
        actor=Actor(runtime,normalizer,runtime.owned[normalizer]['id'],scratch,fenced)
        for stream in ('chat_events','social_events','role_events'):
            actor._request('$JS.API.STREAM.CREATE.'+stream,copy.deepcopy(infos[stream]['config']))
        normalized_consumers={}
        for stream,declaration in DECLARATIONS:
            durable=declaration['durable_name']
            actor.mutate('$JS.API.CONSUMER.CREATE.'+stream+'.'+durable,{'stream_name':stream,'action':'create','config':copy.deepcopy(declaration)})
            normalized=actor.info_config(stream+'/'+durable)
            assert normalized['deliver_policy']=='new' and normalized['ack_policy']=='explicit'
            assert len(normalized)>len(declaration)
            normalized_consumers[(stream,durable)]=normalized
        runtime.stop(normalizer)
        plan=compile_plan('/source',infos,{},normalized_consumers)
        assert len(plan['actions'])==5
        from nats_contract_plan import digest
        enrollment={'fixture_authenticated_existing_actor':auth}
        state={'operation':operation,'events':[],'custody':{'verified':True,'scope':'same-host-fixture-only'},
            'bootstrap_enrollment':enrollment,'nats_contract':plan,
            'nats_contract_binding':{'plan_sha256':digest(plan),'server_image':NATS_IMAGE,'enrollment_sha256':digest(enrollment),'sources':plan['sources']},
            'cut':{'manifest':native,'manifest_sha256':digest(native),'census':before,'census_sha256':canonical(before)}}
        class Stage:
            final_path=store
            final_claim={};final_pv={} # private fixture store already inside runtime.base
            def verify_final_storage(self):fenced()
        original_verify=nats_migration.verify_census
        def diagnostic(plan,before,after):
            new=[c for c in after['consumers'] if (c['stream'],c.get('durable') or c['name']) not in {(c['stream'],c['name']) for c in before['consumers']}]
            if new:
                floors=[]
                for row in new:
                    last=next(s for s in before['streams'] if s['name']==row['stream'])['state']['last_seq']
                    floors.append({'stream':row['stream'],'durable':row.get('durable') or row['name'],'delivered':row['delivered'],'ack_floor':row['ack_floor'],'old_last_seq':last})
                print('FIXTURE_NEW_DURABLE_INITIAL_FLOORS='+json.dumps(floors,sort_keys=True),file=sys.stderr,flush=True)
            try:return original_verify(plan,before,after)
            except Exception:
                from rollout_census import semantic
                closed=archive_closed_store(store,base/'drift-diagnostic.tar')
                original={row['path']:row for row in native['files']}
                current_files={row['path']:row for row in closed['files']}
                consumer_paths=[path for path in original if '/obs/' in path]
                changed=[path for path in consumer_paths if original[path]!=current_files.get(path)]
                # Names and basenames only; no bytes, credentials or payloads.
                changed_kinds=sorted(set(Path(path).name for path in changed))
                state_paths=[path for path in consumer_paths if Path(path).name=='o.dat']
                print(json.dumps({'diagnostic_old_consumer_state_files':len(state_paths),
                    'diagnostic_old_consumer_state_bytes_equal':bool(state_paths) and all(original[path]==current_files.get(path) for path in state_paths),
                    'diagnostic_changed_consumer_file_kinds':changed_kinds}),flush=True)
                old=semantic(before);new=semantic(after);byname={(c['stream'],c['name']):c for c in new['consumers']}
                for row in old['consumers']:
                    current=byname.get((row['stream'],row['name']),{})
                    fields=sorted(k for k in set(row)|set(current) if row.get(k)!=current.get(k))
                    if fields:raise ValueError('fixture_old_consumer_drift name='+row['name']+' fields='+','.join(fields)+' old_state_files='+str(len(state_paths))+' state_bytes_equal='+str(bool(state_paths) and all(original[path]==current_files.get(path) for path in state_paths))+' changed_kinds='+','.join(changed_kinds)) from None
                stream_rows={s['name']:s for s in new['streams']}
                for original_row in old['streams']:
                    expected=copy.deepcopy(original_row)
                    for action in plan['actions']:
                        if action['object']==expected['name'] and '.STREAM.UPDATE.' in action['api']:expected['config_sha256']=digest(action['after'])
                        if action['api'].startswith('$JS.API.CONSUMER.CREATE.') and action['object'].startswith(expected['name']+'/'):expected['state']['consumer_count']+=1
                    observed=stream_rows[expected['name']]
                    fields=sorted(k for k in set(expected)|set(observed) if expected.get(k)!=observed.get(k))
                    states=sorted(k for k in set(expected['state'])|set(observed['state']) if expected['state'].get(k)!=observed['state'].get(k))
                    if fields:raise ValueError('fixture_old_stream_drift name='+expected['name']+' fields='+','.join(fields)+' state_fields='+','.join(states)) from None
                raise
        nats_migration.verify_census=diagnostic
        real_mutate=Actor.mutate
        def interrupted_update(actor,api,body):
            result=real_mutate(actor,api,body)
            if api=='$JS.API.STREAM.UPDATE.chat_events':raise RuntimeError('fixture_interrupted_after_update_before_info')
            return result
        Actor.mutate=interrupted_update
        try:
            apply_contract(base,state,Stage(),state['events'].append)
        except RuntimeError as error:
            assert str(error)=='fixture_interrupted_after_update_before_info'
        else:raise AssertionError('fixture_interruption_not_exercised')
        finally:Actor.mutate=real_mutate
        assert not any(e['kind']=='nats_contract_applied' for e in state['events'])
        assert any(e['kind']=='nats_contract_mutation_issued' for e in state['events'])
        migration=apply_contract(base,state,Stage(),state['events'].append)
        assert migration['verified'] and migration['old_record_files_verified']
        postproof=verify_post_apply(runtime,store,migration['cut'],fenced)
        assert postproof['native_files_verified'] and postproof['census_verified']
        proof=migration['proof'];events=state['events']
        runtime.restart(broker);ready(runtime,broker)
        # The old reset kernel compares an unchanged stream configuration;
        # our exact additive config delta is checked above instead. GetMsg
        # below verifies the original records without deliveries or ACKs.
        verifier=runtime.create('payload-verify',NATS_IMAGE,[(base/'helper','/kernel',False),(base/'inputs','/inputs',False)],['/kernel','verify'],broker)
        runtime.run(['start',verifier]);assert runtime.run(['wait',verifier])=='0'
        publish=runtime.create('post-release-record',NATS_IMAGE,[(base/'helper','/kernel',False),(base/'inputs','/inputs',False)],['/kernel','post'],broker)
        runtime.run(['start',publish]);assert runtime.run(['wait',publish])=='0'
        current_actor=Actor(runtime,broker,runtime.owned[broker]['id'],store,lambda:None)
        current_tree=runtime.monitor_jsz(broker)
        current_details=next(a for a in current_tree['account_details'] if a['id']==account)['stream_detail']
        current_infos={row['name']:{'config':current_actor.info_config(row['name']),'state':row['state'],'consumer_detail':row.get('consumer_detail',[])} for row in current_details}
        current_consumers={(stream['name'],consumer.get('durable') or consumer['name']):consumer for stream in current_details for consumer in stream.get('consumer_detail',[])}
        rollback_plan=compile_plan('/source',current_infos,current_consumers,normalized_consumers)
        assert rollback_plan['actions']==[]
        runtime.stop(broker)
        fresh_base=base/'fresh-operation';fresh_base.mkdir(mode=0o750);os.chown(fresh_base,0,65532)
        shutil.copytree(base/'inputs',fresh_base/'inputs')
        for path in [fresh_base/'inputs',*(fresh_base/'inputs').rglob('*')]:os.chown(path,0,65532)
        for name in ('kernel','server.conf'):
            shutil.copyfile(base/name,fresh_base/name);os.chown(fresh_base/name,0,65532);(fresh_base/name).chmod(0o550 if name=='kernel' else 0o440)
        fresh_runtime=DockerRuntime(fresh_base,'freshrollback'+uuid.uuid4().hex[:12])
        fresh_cut=capture_cut(fresh_runtime,store,fenced)
        assert sum(row['state']['messages'] for row in fresh_cut['census']['streams'])==6
        fresh_cut['manifest_sha256']=digest(fresh_cut['manifest'])
        fresh={'operation':fresh_runtime.operation,'events':[],'custody':{'verified':True,'scope':'same-host-fixture-only'},
            'bootstrap_enrollment':enrollment,'nats_contract':rollback_plan,
            'nats_contract_binding':{'plan_sha256':digest(rollback_plan),'server_image':NATS_IMAGE,'enrollment_sha256':digest(enrollment),'sources':rollback_plan['sources']},'cut':fresh_cut}
        # Rollback captures the current postrelease record and retains the
        # additive contract; it never restores the original five-record cut.
        fresh_migration=apply_contract(fresh_base,fresh,Stage(),fresh['events'].append)
        fresh_proof=verify_post_apply(fresh_runtime,store,fresh_migration['cut'],fenced)
        assert fresh_proof['messages']==6
        for name in list(fresh_runtime.owned):
            row=fresh_runtime.inspect(name);fresh_runtime.run(['rm','-f',row['Id']]);removed.append(row['Id'])
            assert not fresh_runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}'])
        fresh_runtime.owned.clear()
        runtime.restart(broker);ready(runtime,broker)
        post_verifier=runtime.create('post-record-verify',NATS_IMAGE,[(base/'helper','/kernel',False),(base/'inputs','/inputs',False)],['/kernel','verify-post'],broker)
        runtime.run(['start',post_verifier]);assert runtime.run(['wait',post_verifier])=='0'
        runtime.stop(broker);post=runtime.cold_archive(broker,store,base/'after.tar')
        def records(manifest):return {row['path']:row for row in manifest['files'] if '/msgs/' in row['path']}
        assert records(native) and records(fresh_cut['manifest'])==records(post)
        receipt={'schema':'voice-nats-contract-fixture-v1','messages':proof['messages'],
            'old_consumers_preserved':proof['old_consumers_preserved'],'created_consumers':proof['created_consumers'],
            'normalized_consumer_sha256':plan['normalized_consumer_sha256'],'native_record_files_equal':True,
            'known_payloads_verified':True,'same_original_store':True,'renewed_existing_actor_auth_verified':True,'normal_migration_phase_and_postapply_proof_verified':True,
            'fresh_rollback_current_cut_records':6,'post_release_record_payload_verified':True,'rollback_retained_additive_contract':True,
            'actual_update_before_info_interruption_recovered':True,
            'new_durable_initial_delivery_ack_floors_verified':True,
            'events':[e['kind'] for e in events]}
    finally:
        for owned_runtime in ([fresh_runtime] if fresh_runtime is not None else [])+[runtime]:
            for name in reversed(list(owned_runtime.owned)):
                row=owned_runtime.inspect(name);owned_runtime.run(['rm','-f',row['Id']]);removed.append(row['Id'])
                assert not owned_runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}'])
    receipt['removed_container_ids']=removed;print(json.dumps(receipt,sort_keys=True))

def main():
    source=Path(__file__).resolve().parents[3];module=source/'scripts/staging/nats-known-baseline'
    with tempfile.TemporaryDirectory(prefix='voice-contract-build-') as td:
        bundle=Path(td)
        helper=fixture.HELPER.replace('"fmt"; "os";', '"crypto/sha256"; "encoding/hex"; "encoding/json"; "fmt"; "os";')
        contract=json.loads((module/'deployed-contract.json').read_text())
        grants=['$JS.API.INFO']+[prefix+name for name in contract['streams'] for prefix in ('$JS.API.STREAM.CREATE.','$JS.API.STREAM.INFO.')]
        grants += [prefix+stream+'.'+consumer for stream,consumer in contract['consumer_pairs'] for prefix in ('$JS.API.CONSUMER.CREATE.','$JS.API.CONSUMER.INFO.')]
        grant_text=json.dumps(grants,separators=(',',':'))
        old_actor='''
 bootstrap,e:=nkeys.CreateUser();must(e);defer bootstrap.Wipe();bootstrapPublic,e:=bootstrap.PublicKey();must(e)
 restricted:=jwt.NewUserClaims(bootstrapPublic);must(json.Unmarshal([]byte(`GRANTS`),&restricted.Pub.Allow));restricted.Sub.Allow=[]string{"_INBOX.voice.bootstrap.reply.>"}
 restrictedToken,e:=restricted.Encode(app);must(e);bootstrapSeed,e:=bootstrap.Seed();must(e)
 restrictedCreds,e:=jwt.FormatUserConfig(restrictedToken,bootstrapSeed);must(e);write(base,"inputs/bootstrap.creds",string(restrictedCreds))
 accountSeed,e:=app.Seed();must(e);write(base,"inputs/synthetic-account.seed",string(accountSeed))
 write(base,"inputs/operator.jwt",operator);write(base,"inputs/account.jwt",account);write(base,"inputs/system-account.jwt",system);write(base,"inputs/system-account.public",sysPub)
'''.replace('GRANTS',grant_text)
        helper=helper.replace('\n}\nfunc extra()',old_actor+'\n}\nfunc extra()')
        # Payload/extra-fixture validation uses its separate disposable broad
        # role; the migration actor retains the exact limited old grant list.
        helper=helper.replace('"/inputs/bootstrap.creds"','"/inputs/social.creds"')
        payload_check=r'''
 raw,e:=os.ReadFile("/inputs/fixture.json");must(e)
 var snapshot struct {Records []struct {Subject string `json:"subject"`;Sequence uint64 `json:"sequence"`;Payload string `json:"payload_sha256"`;Headers string `json:"headers_sha256"`;Event string `json:"event_id"`} `json:"records"`}
 must(json.Unmarshal(raw,&snapshot));if len(snapshot.Records)!=3 {panic("known_record_count")}
 for _,record:=range snapshot.Records {
  message,e:=js.GetMsg("social_events",record.Sequence);must(e)
  payload:=sha256.Sum256(message.Data);headers,e:=json.Marshal(message.Header);must(e);header:=sha256.Sum256(headers)
  if message.Subject!=record.Subject || message.Header.Get(nats.MsgIdHdr)!=record.Event || hex.EncodeToString(payload[:])!=record.Payload || hex.EncodeToString(header[:])!=record.Headers {panic("known_record_changed")}
 }
'''
        helper=helper.replace('for index,payload:=range []string',payload_check+'\n for index,payload:=range []string')
        post=r'''
func post(check bool) {
 nc,e:=nats.Connect("nats://127.0.0.1:4222",nats.UserCredentials("/inputs/social.creds"),nats.Timeout(2*time.Second),nats.NoReconnect());must(e);defer nc.Close()
 js,e:=nc.JetStream();must(e)
 if check {verify();message,e:=js.GetMsg("rollout_extra",3);must(e);if string(message.Data)!="known-post-release-record" {panic("post_release_record_changed")};return}
 _,e=js.Publish("rollout.extra",[]byte("known-post-release-record"));must(e)
}
'''
        helper=helper.replace('func main() {',post+'\nfunc main() { if len(os.Args)==2 && os.Args[1]=="post" {post(false);return};if len(os.Args)==2 && os.Args[1]=="verify-post" {post(true);return};')
        (bundle/'helper.go').write_text(helper)
        env=dict(os.environ,GOOS='linux',GOARCH='amd64',CGO_ENABLED='0')
        fixture.call(['go','build','-o',str(bundle/'helper'),str(bundle/'helper.go')],cwd=module,env=env)
        fixture.call(['go','build','-o',str(bundle/'kernel'),'.'],cwd=module,env=env)
        fixture.call(['go','build','-o',str(bundle/'bootstrap-renewer'),'./cmd/nats-jwt-issuer'],cwd=source/'src/backend/pkg',env=env)
        records={n:base64.b64encode((bundle/n).read_bytes()).decode() for n in ('helper','kernel','bootstrap-renewer')}
        names=('rollout_census.py','rollout_census_integration.py','nats_contract_actor.py','nats_contract_plan.py','nats_migration.py','preserve.py','native_store.py','bootstrap_auth.py','bootstrap_root.py','bootstrap_enrollment.py','bootstrap_renewal.py','guard.py')
        for n in names:records[n]=base64.b64encode((Path(__file__).parent/n).read_bytes()).decode()
        records['contract_fixture.py']=base64.b64encode(Path(__file__).read_bytes()).decode()
        # Existing bounded outer fixture invokes this entry; it owns cleanup.
        transport=(Path(__file__).parent/'rollout_census_integration.py').read_bytes().replace(b'inner(sys.argv[2])',b"__import__('contract_fixture').inner(sys.argv[2])")
        transport=transport.replace(b"raise RuntimeError('owned_census_fixture_command_failed')",b"raise RuntimeError('owned_census_fixture_command_failed: '+ '\\n'.join(x for x in result.stderr.decode(errors='replace').splitlines()[-20:] if x.lstrip().startswith(('File ', 'AssertionError', 'TypeError', 'ValueError', 'KeyError', 'ContractError', 'Blocked', 'FIXTURE_NEW_DURABLE_INITIAL_FLOORS='))))")
        records['rollout_census_integration.py']=base64.b64encode(transport).decode()
        for n in ('docker_runtime.py','scenario.py','controller.py','commands.py','stage_runtime.py'):
            records['source/scripts/staging/nats-known-baseline/'+n]=base64.b64encode((module/n).read_bytes()).decode()
        for role in ('realtime','notification','analytics-chat','search'):
            n='testdata/deployed-bootstrap-20261004/'+role+'-bootstrap.sh';records['source/scripts/staging/nats-known-baseline/'+n]=base64.b64encode((module/n).read_bytes()).decode()
        for role in ('realtime','analytics-chat'):
            n='deploy/templates/nats-'+role+'-bootstrap.yaml';records['source/'+n]=base64.b64encode((source/n).read_bytes()).decode()
        script='import base64,pathlib,tempfile,sys,os,socket\nassert sys.platform=="linux" and os.getuid()==1000 and socket.gethostname()=="pmdebook"\n'
        script+='with tempfile.TemporaryDirectory(prefix="voice-contract-fixture-") as td:\n bundle=pathlib.Path(td)\n'
        script+=' for n,data in '+repr(records)+'.items():\n  p=bundle/n;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(base64.b64decode(data));p.chmod(0o755 if n in ("helper","kernel") else 0o644)\n'
        script+=' bundle.chmod(0o755);sys.path.insert(0,str(bundle));import rollout_census_integration as f;f.remote_run(bundle)\n'
        import subprocess
        result=subprocess.run(['ssh','voice-staging','python3 -'],input=script.encode(),stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=180)
        if result.returncode:
            print(result.stdout.decode(),flush=True)
            print('\n'.join(x for x in result.stderr.decode(errors='replace').splitlines()[-30:] if x.lstrip().startswith(('File ','AssertionError','TypeError','ValueError','KeyError','ContractError','Blocked','RuntimeError','FIXTURE_NEW_DURABLE_INITIAL_FLOORS='))),file=sys.stderr)
            raise RuntimeError('owned_contract_fixture_failed')
        print(result.stdout.decode())
if __name__=='__main__':main()
