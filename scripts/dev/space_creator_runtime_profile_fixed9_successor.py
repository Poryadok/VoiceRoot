"""Fixed Creator -> Space -> Role custody successor; no runtime-readiness claim."""
import base64
import copy
import hashlib

import dev_peer_issuer_metadata as metadata
import full_cohort_package_phase as package
import full_cohort_schema_metadata as schema
import space_creator_runtime_profile_delegated as delegated
import space_role_peer_create as role
import writer_cut


def _current_crypto(phase, expected, ca):
    """Read current profile under the already reconstructed fixed-nine state."""
    phase._guard()
    if phase.preflight._ca()!=ca or phase.preflight._creator()!=expected: raise ValueError()
    names=sorted(metadata.PROFILES[phase.PROFILE]); objects=[]; jwks={}; public_keys=[]
    for name in names:
        current=delegated._get(name,phase.PROFILE); identity=expected[name]
        if (current is None or identity.get('state')!='present' or
                any(current['metadata'].get(k)!=identity[k] for k in ('uid','resourceVersion'))): raise ValueError()
        obj=delegated._normalized(current,name); objects.append(obj)
        values={k:base64.b64decode(v,validate=True) for k,v in obj['data'].items()}
        key_names=('current.pem','next.pem') if name.endswith('-principal-signing') else ('tls.key',)
        if name.endswith('-principal-signing'):
            if values['active-kid'] not in (b'current',b'next'): raise ValueError()
            jwks[name]=delegated.packet.crypto._jwks(values)
            obj['data']['active-kid']=base64.b64encode(b'current').decode('ascii')
        for key in key_names:
            public_keys.append(hashlib.sha256(delegated.packet.crypto._openssl(['pkey','-in','{0}','-pubout'],[values[key]])).digest())
    if len(public_keys)!=len(set(public_keys)): raise ValueError()
    receipt={'schema':'voice-dev-peer-issuer-receipt-v1','status':'staged','profile':phase.PROFILE,'object_names':names,
             'media_ca_der_sha256':ca['der_sha256'],'public_jwks':jwks,'runtime_qualified':False,'cluster_applied':False}
    envelope={'schema':'voice-dev-peer-issuer-packet-v1','profile':phase.PROFILE,'operation_id':phase.fence.context['operation_id'],
              'source_sha':phase.fence.context['commit'],'namespace_uid':phase.namespace_uid,'preflight_sha256':writer_cut.digest(phase.initial),
              'issuer_receipt':receipt,'objects':{'apiVersion':'v1','kind':'List','items':objects}}
    delegated.packet.validate(envelope,phase.PROFILE,phase.fence.context['operation_id'],phase.fence.context['commit'],
                              phase.namespace_uid,writer_cut.digest(phase.initial),names,phase.media)
    if phase.preflight._ca()!=ca or phase.preflight._creator()!=expected: raise ValueError()
    phase._guard()
    return jwks


def verify(trust, token, creator, space, role_phase):
    """Bind exactly three completed typed receivers to their successive nine-name snapshots."""
    import space_creator_runtime_profile_target_trust as target
    try:
        if (type(trust) is not target.TargetTrustPhase or type(creator) is not delegated.CreatorReceiver or
                type(space) is not delegated.SpaceReceiver or type(role_phase) is not role.Session or
                creator.preflight is not trust.preflight or any(p.package is not trust.package or
                p.fence.context!=trust.fence.context or p.lock!=trust.lock or p.namespace_uid!=trust.namespace_uid or
                p.token!=token for p in (creator,space,role_phase))): raise ValueError()
        phases=(creator,space,role_phase); expected=None; anchors=[]
        trust._guard(); row,rev=trust._ledger()
        for phase,profile in zip(phases,('creator','space','role')):
            if phase.preflight.profile!=profile or not phase.completed or phase.record.get('status')!='done': raise ValueError()
            initial=copy.deepcopy(phase.initial)
            if (not isinstance(initial,dict) or initial.get('profile')!=profile or set(initial.get('creator_secrets',{}))!=set(metadata.MEMBERS)
                    or (expected is not None and initial['creator_secrets']!=expected)
                    or (anchors and initial['ca']!=anchors[0][0]['ca'])): raise ValueError()
            actual_row,actual_rev=phase._guard()
            if (actual_row,actual_rev)!=(row,rev): raise ValueError()
            record=copy.deepcopy(phase.record); effects=copy.deepcopy(phase.effects); proof=copy.deepcopy(phase.proof)
            ledger=phase.ledger if profile!='role' else role.LEDGER
            effects_key=phase.effects_key if profile!='role' else role.EFFECTS
            step=phase.step if profile!='role' else role.STEP
            if (row.get(ledger)!=record or row.get(effects_key,{})!=effects or phase.journal.state['steps'].get(step)!=record or
                    record.get('receipt')!=proof or set(proof)!={'schema','profile','operation_id','source_sha','namespace_uid','preflight_sha256','ca','objects','public_jwks','scope'} or
                    proof.get('schema')!='voice-dev-peer-receiver-v1' or proof.get('scope')!='fixed-peer-secrets-no-runtime-readiness' or proof.get('profile')!=profile or
                    proof.get('preflight_sha256')!=writer_cut.digest(initial) or proof.get('ca')!=initial['ca'] or
                    proof.get('operation_id')!=trust.fence.context['operation_id'] or proof.get('source_sha')!=trust.fence.context['commit'] or
                    proof.get('namespace_uid')!=trust.namespace_uid or set(proof.get('objects',{}))!=set(metadata.PROFILES[profile]) or
                    set(proof.get('public_jwks',{}))!={name for name in metadata.PROFILES[profile] if name.endswith('-principal-signing')} or
                    set(effects)-set(metadata.PROFILES[profile])): raise ValueError()
            expected=copy.deepcopy(initial['creator_secrets'])
            for name,effect in effects.items():
                if effect.get('status')!='done' or initial['creator_secrets'][name]!={'state':'absent'}: raise ValueError()
                expected[name]={'state':'present','uid':effect['uid'],'resourceVersion':effect['resourceVersion'],'keys':sorted(metadata.MEMBERS[name])}
            for name,identity in proof['objects'].items():
                if expected[name].get('state')!='present' or identity!={k:expected[name][k] for k in ('uid','resourceVersion')}: raise ValueError()
            anchors.append((initial,record,effects,proof))
        if writer_cut.digest(anchors[0][0])!=trust.journal.state['steps'][target.STEP]['receipt']['preflight_receipt_sha256']: raise ValueError()
        if role.verify_completed(role_phase,token)!=anchors[2][3]: raise ValueError()
        package_local=copy.deepcopy(trust.package.journal.state['steps'].get(package.STEP))
        if not isinstance(package_local,dict) or package_local.get('status')!='done' or not isinstance(package_local.get('receipt'),dict): raise ValueError()
        for phase,anchor in zip((creator,space),anchors):
            if _current_crypto(phase,expected,anchors[0][0]['ca'])!=anchor[3]['public_jwks']: raise ValueError()
        if creator.preflight._ca()!=anchors[0][0]['ca'] or creator.preflight._creator()!=expected: raise ValueError()
        trust._guard(); final,end=trust._ledger()
        if (final,end)!=(row,rev): raise ValueError()
        for phase,profile,anchor in zip(phases,('creator','space','role'),anchors):
            if phase.fence.ledger.read()!=(row,rev): raise ValueError()
            phase._local()
            ledger=phase.ledger if profile!='role' else role.LEDGER
            effects_key=phase.effects_key if profile!='role' else role.EFFECTS
            step=phase.step if profile!='role' else role.STEP
            if (phase.initial,phase.record,phase.effects,phase.proof)!=anchor or row.get(ledger)!=anchor[1] or row.get(effects_key,{})!=anchor[2] or phase.journal.state['steps'].get(step)!=anchor[1]: raise ValueError()
        trust._local()
        for phase,profile,anchor in zip(phases,('creator','space','role'),anchors):
            phase._local()
            step=phase.step if profile!='role' else role.STEP
            if (phase.initial,phase.record,phase.effects,phase.proof)!=anchor or phase.journal.state['steps'].get(step)!=anchor[1]: raise ValueError()
        if trust.package.journal.state['steps'].get(package.STEP)!=package_local: raise ValueError()
        return copy.deepcopy(anchors[0][0])
    except schema.GuardLost: raise
    except BaseException: raise schema.GuardLost('fixed peer successor unavailable') from None
