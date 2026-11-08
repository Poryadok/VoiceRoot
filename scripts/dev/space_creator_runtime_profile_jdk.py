"""Capture only public certificate fingerprints from the protected P1 Auth image."""
import copy
import json
import re
import secrets

import auth_live_migration_phase_eligibility as eligibility
import full_cohort_schema_metadata as metadata
import publish_candidate
import space_creator_runtime_profile_auth as auth
import space_creator_runtime_profile_trust as trust
import writer_cut

STEP='space-creator-target-jdk'
LEDGER='space_creator_target_jdk'
class JdkError(RuntimeError): pass


def _image_info(info,image,config_digest):
    if (not isinstance(info,list) or len(info)!=1 or info[0].get('Id')!=config_digest or
            info[0].get('Os')!='linux' or info[0].get('Architecture')!='amd64' or image not in (info[0].get('RepoDigests') or []) or
            info[0].get('Config',{}).get('Volumes') not in (None,{})): raise ValueError()


class TargetJdkPhase(auth.AuthTrustPhase):
    def _state(self,value):
        self._guard(); row,rev=self._ledger(); marker=row.get(LEDGER)
        if not isinstance(marker,dict) or marker.get('status')!='pending' or self.journal.state['steps'].get(STEP)!=marker: raise ValueError()
        next_marker=copy.deepcopy(marker); next_marker['tool']=copy.deepcopy(value)
        self.journal.state['steps'][STEP]=copy.deepcopy(next_marker); self.journal.save(); self._guard()
        current,version=self._ledger()
        if current!=row or version!=rev: raise ValueError()
        current[LEDGER]=copy.deepcopy(next_marker); self.fence.ledger.write(version,current)
        self.record=copy.deepcopy(next_marker); self._guard()
        if self._ledger()[0].get(LEDGER)!=next_marker: raise ValueError()

    def _docker(self,args,limit=1048576):
        self._guard(); raw=metadata._bounded_process(['docker',*args],limit); self._guard(); return raw

    def _capture(self,token,actor):
        package=self._package(token); image=package['images']['voice-auth/auth']; platform=package['platform_digests']['voice-auth/auth']
        if (not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/auth@sha256:[a-f0-9]{64}',image) or
                not re.fullmatch(r'sha256:[a-f0-9]{64}',platform) or not isinstance(actor,str) or not actor): raise ValueError()
        index=image.rsplit('@',1)[1]; self._guard()
        config=eligibility.read_image_config(publish_candidate.Registry(actor,token),index,platform); self._guard()
        if any(config['env'].get(k) not in (None,'') for k in ('JAVA_TOOL_OPTIONS','_JAVA_OPTIONS','JDK_JAVA_OPTIONS','LD_PRELOAD','LD_LIBRARY_PATH')): raise ValueError()
        self._state({'status':'pull-pending','image':image,'platform':platform,'config':config['config_digest']})
        self._docker(['pull','--platform','linux/amd64',image])
        image_info=json.loads(self._docker(['image','inspect',image]))
        _image_info(image_info,image,config['config_digest'])
        name='voice-creator-auth-jdk-'+secrets.token_hex(16)
        self._state({'status':'create-pending','name':name,'image_id':config['config_digest']})
        args=['-list','-v','-keystore','/opt/java/openjdk/lib/security/cacerts','-storepass','changeit',
              '-J-Duser.language=en','-J-Duser.country=US']
        cid=self._docker(['create','--name',name,'--network','none','--read-only','--cap-drop','ALL',
                          '--security-opt','no-new-privileges','--label','voice.test.owner='+name,
                          '--entrypoint','/opt/java/openjdk/bin/keytool',config['config_digest'],*args],256).decode('ascii').strip()
        if not re.fullmatch(r'[a-f0-9]{64}',cid): raise ValueError()
        detail=json.loads(self._docker(['inspect',cid]))
        if (not isinstance(detail,list) or len(detail)!=1 or detail[0].get('Id')!=cid or detail[0].get('Image')!=config['config_digest'] or
                detail[0].get('Config',{}).get('Labels',{}).get('voice.test.owner')!=name or
                detail[0].get('Config',{}).get('Entrypoint')!=['/opt/java/openjdk/bin/keytool'] or detail[0].get('Config',{}).get('Cmd')!=args or
                detail[0].get('HostConfig',{}).get('NetworkMode')!='none' or detail[0].get('HostConfig',{}).get('PortBindings') or detail[0].get('Mounts')): raise ValueError()
        self._state({'status':'created','name':name,'image_id':config['config_digest'],'id':cid})
        self._state({'status':'start-pending','name':name,'image_id':config['config_digest'],'id':cid})
        listing=self._docker(['start','-a',cid]); roots=trust.parse(listing)
        if not roots: raise ValueError()
        status=json.loads(self._docker(['inspect',cid]))
        if (not isinstance(status,list) or len(status)!=1 or status[0].get('Id')!=cid or
                status[0].get('State',{}).get('Status')!='exited' or status[0].get('State',{}).get('ExitCode')!=0): raise ValueError()
        self._state({'status':'public-verified','name':name,'image_id':config['config_digest'],'id':cid})
        self._state({'status':'remove-pending','name':name,'image_id':config['config_digest'],'id':cid})
        self._docker(['rm','-f','-v',cid],256)
        remaining=self._docker(['ps','-a','--no-trunc','--format','{{.ID}}']).decode('ascii').splitlines()
        if cid in remaining: raise ValueError()
        self._state({'status':'removed','name':name,'image_id':config['config_digest'],'id':cid})
        if self._package(token)!=package: raise ValueError()
        return {'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                'namespace_uid':self.namespace_uid,'image':image,'platform_digest':platform,'config_digest':config['config_digest'],
                'root_fingerprints':sorted(roots),'root_count':len(roots),
                'scope':'p1-auth-image-keytool-public-fingerprints-only-no-runtime-readiness'}

    def run(self,token,actor):
        pending=False
        try:
            self._guard(); self._inventory(token); self._package(token); row,rev=self._ledger()
            if STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save(); self._guard()
            current,version=self._ledger()
            if current!=row or version!=rev: raise ValueError()
            current[LEDGER]=copy.deepcopy(marker); current['step']=STEP+'-pending'; self.fence.ledger.write(version,current)
            self.record=copy.deepcopy(marker); receipt=self._capture(token,actor); self._guard(); current,version=self._ledger()
            if current.get(LEDGER)!=self.record or self.journal.state['steps'].get(STEP)!=self.record or self.record.get('tool',{}).get('status')!='removed': raise ValueError()
            done=self.record|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(done)|{'receipt':copy.deepcopy(receipt)}; self.journal.save()
            current[LEDGER]=copy.deepcopy(done); current['step']=STEP+'-done'; self.fence.ledger.write(version,current)
            self.record=copy.deepcopy(done); self._guard()
            if self._ledger()[0].get(LEDGER)!=done: raise ValueError()
            self._local()
            if self.record!=done or self.journal.state['steps'].get(STEP)!=done|{'receipt':receipt}: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException as error:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('target Auth JDK capture uncertain')
                except BaseException:pass
            if isinstance(error,metadata.GuardLost): raise
            raise JdkError('target Auth JDK capture unavailable') from None


def verify_completed(phase,token):
    try:
        if type(phase) is not TargetJdkPhase: raise ValueError()
        phase._guard(); row,rev=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        marker={k:v for k,v in local.items() if k!='receipt'}
        if marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or row.get(LEDGER)!=marker: raise ValueError()
        package=phase._package(token)
        if (receipt['image']!=package['images']['voice-auth/auth'] or receipt['platform_digest']!=package['platform_digests']['voice-auth/auth'] or
                not isinstance(receipt['root_fingerprints'],list) or len(receipt['root_fingerprints'])!=receipt['root_count'] or
                any(not re.fullmatch(r'[a-f0-9]{64}',x) for x in receipt['root_fingerprints']) or
                receipt['root_fingerprints']!=sorted(set(receipt['root_fingerprints']))): raise ValueError()
        phase._guard(); after,end=phase._ledger()
        if after!=row or end!=rev or phase.journal.state['steps'].get(STEP)!=local: raise ValueError()
        phase._local()
        return copy.deepcopy(receipt)
    except metadata.GuardLost: raise
    except BaseException: raise JdkError('target Auth JDK receipt unavailable') from None
