import copy,hashlib,json,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import operator_retirement as r
import nonnats_plan
from controller import Blocked

# Literal names/order from the retained 90ffab5b failed progress and independent
# read-only inventory. Never generate this fixture from MEDIA_FAILED_APPLIED:
# that previously copied its missing voice- prefix into both sides of the test.
RECORDED_APPLIED=(
    'Service/voice-social','Service/voice-user','Service/voice-space','Service/voice-chat',
    'Service/voice-file','Service/voice-messaging','Service/voice-matchmaking','Service/voice-search',
    'Service/voice-notification','Service/voice-realtime','Service/voice-bot','Service/voice-subscription',
    'Service/voice-moderation','Service/voice-story','Service/voice-analytics','Service/voice-gateway',
    'NetworkPolicy/voice-social-privacy-consumers','NetworkPolicy/voice-social-principal-jwks',
    'NetworkPolicy/voice-file-user-principal-listener','NetworkPolicy/voice-file-principal-jwks',
    'NetworkPolicy/voice-search-user-projection','NetworkPolicy/voice-search-projection-jwks')


def fixture(base):
    def row(kind,name,value):
        result={'kind':kind,'metadata':{'name':name,'uid':kind+'/'+name,'resourceVersion':'1'}}
        result['spec']={'replicas':0,'template':{'spec':{'containers':[{'name':'app','image':value}]}}} if kind=='Deployment' else {'value':value}
        return result
    keys=[*RECORDED_APPLIED,'Deployment/voice-auth','Deployment/voice-user','Deployment/voice-space']
    before={key:row(*key.split('/'), 'old') for key in keys}
    desired=[row(*key.split('/'), 'new') for key in keys]
    observed=copy.deepcopy(before)
    for target in desired[:23]:observed[target['metadata']['uid']]=copy.deepcopy(target)
    observed['Deployment/voice-auth']['metadata']['annotations']={'deployment.kubernetes.io/revision':'42'}
    state={'operation':r.MEDIA_ABORT_OPERATION,'phase':'OPERATOR_RETIRING','status':'BLOCKED','fence_status':'UNKNOWN',
           'context':{},'events':[],'migrations':[],'migration_jobs':[],'active_contract':{}}
    progress={'phase':'BLOCKED','context':{},'events':[{'kind':'target_resource_applied','resource':key} for key in RECORDED_APPLIED]}
    values={r.MEDIA_RECONCILED_PREFIX+'-blocked.json':state,r.MEDIA_TARGET_PREFIX+'-progress.json':progress,
            r.MEDIA_TARGET_PREFIX+'-normalized.json':desired,
            r.MEDIA_TARGET_PREFIX+'-prepared.json':{'before':before,'normalized_sha256':r.canonical(desired)}}
    pins={}
    for name,value in values.items():
        raw=json.dumps(value,sort_keys=True,separators=(',',':')).encode();path=base/name;path.write_bytes(raw);path.chmod(0o600)
        pins[name]={'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
    return state,pins,observed,values


class ReconciliationTests(unittest.TestCase):
    def test_recorded_resource_names_pass_exact_admission_without_aliases(self):
        with tempfile.TemporaryDirectory() as td:
            base=Path(td);state,pins,observed,values=fixture(base)
            with patch.object(r,'MEDIA_FAILED_APPLY',pins),patch.object(r,'media_target_failed',return_value=state):
                self.assertEqual(r.media_failed_apply(base),state)
                path=base/(r.MEDIA_TARGET_PREFIX+'-progress.json')
                progress=values[path.name]
                for change in ('missing-prefix','reordered'):
                    changed=copy.deepcopy(progress)
                    if change=='missing-prefix':changed['events'][16]['resource']='NetworkPolicy/social-privacy-consumers'
                    else:changed['events'][16:18]=reversed(changed['events'][16:18])
                    raw=json.dumps(changed).encode();path.write_bytes(raw)
                    pins[path.name]={'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
                    with self.subTest(change=change),self.assertRaises(Blocked):r.media_failed_apply(base)

    def test_admin_semantics_derive_from_nonempty_retained_old_descriptors(self):
        descriptors=[];oldpins={};expected={}
        for kind,name in (('Deployment','voice-admin'),('Service','voice-admin'),('Ingress','voice-admin-http')):
            desired={'kind':kind,'metadata':{'annotations':{'deployment.kubernetes.io/revision':'17','application':'keep'}},'spec':{'preserved':True}}
            key=kind+'/'+name;oldpins[key]={'uid':key,'semantic_sha256':r.canonical(desired)}
            expected[key]={'uid':key,'semantic_sha256':r.canonical(nonnats_plan.semantic(desired))}
            descriptors.append({'kind':kind,'name':name,'uid':key,'disposition':'preserve','desired':desired})
        prepared={'outside_owned':oldpins,'preflight':{'objects':descriptors}}
        with patch.object(r,'media_failed_apply'),patch.object(r,'document',return_value=prepared),patch.object(r,'MEDIA_FAILED_ADMIN',oldpins):
            self.assertEqual(r.media_failed_admin(Path('/retained')),expected)
            self.assertNotEqual(expected['Deployment/voice-admin'],oldpins['Deployment/voice-admin'])
            self.assertEqual(expected['Service/voice-admin'],oldpins['Service/voice-admin'])
            for mutate in (lambda rows:rows.pop(),lambda rows:rows.append(copy.deepcopy(rows[0])),
                           lambda rows:rows[0].update(uid='new-owner'),
                           lambda rows:rows[0]['desired']['spec'].update(preserved=False),
                           lambda rows:rows[0].update(disposition='action')):
                prior=copy.deepcopy(descriptors);mutate(descriptors)
                with self.assertRaises(Blocked):r.media_failed_admin(Path('/retained'))
                descriptors[:]=prior

    def test_exact_partial_effects_are_reconciled_and_all_other_mixtures_refused(self):
        with tempfile.TemporaryDirectory() as td:
            base=Path(td);state,pins,observed,values=fixture(base)
            with patch.object(r,'MEDIA_FAILED_APPLY',pins),patch.object(r,'media_target_failed',return_value=state):
                result=r.media_failed_reconciliation(base,observed)
                self.assertEqual(set(result),set(observed))
                for name in observed:
                    changed=copy.deepcopy(observed);changed[name]['metadata']['uid']='replacement'
                    with self.subTest(identity=name),self.assertRaises(Blocked):r.media_failed_reconciliation(base,changed)
                for name in ('Deployment/voice-auth','Deployment/voice-user'):
                    changed=copy.deepcopy(observed)
                    changed[name]['spec']['template']['spec']['containers'][0]['image']='old' if name.endswith('auth') else 'new'
                    with self.subTest(mixture=name),self.assertRaises(Blocked):r.media_failed_reconciliation(base,changed)
                for mutate in (lambda x:x.pop('Deployment/voice-user'),lambda x:x.update(extra={}),
                               lambda x:x['Deployment/voice-auth']['metadata']['annotations'].update({'application':'drift'})):
                    changed=copy.deepcopy(observed);mutate(changed)
                    with self.assertRaises(Blocked):r.media_failed_reconciliation(base,changed)
                for name in pins:
                    path=base/name;prior=path.read_bytes();path.write_bytes(b'{}')
                    with self.subTest(retained=name),self.assertRaises(Blocked):r.media_failed_reconciliation(base,observed)
                    path.write_bytes(prior)

    def test_duplicate_normalized_row_cannot_be_rehashed_into_authority(self):
        with tempfile.TemporaryDirectory() as td:
            base=Path(td);state,pins,observed,values=fixture(base)
            key=r.MEDIA_TARGET_PREFIX+'-normalized.json';values[key].append(copy.deepcopy(values[key][-1]))
            values[r.MEDIA_TARGET_PREFIX+'-prepared.json']['normalized_sha256']=r.canonical(values[key])
            for name in (key,r.MEDIA_TARGET_PREFIX+'-prepared.json'):
                raw=json.dumps(values[name]).encode();(base/name).write_bytes(raw)
                pins[name]={'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
            with patch.object(r,'MEDIA_FAILED_APPLY',pins),patch.object(r,'media_target_failed',return_value=state),self.assertRaises(Blocked):
                r.media_failed_reconciliation(base,observed)

if __name__=='__main__':unittest.main()
