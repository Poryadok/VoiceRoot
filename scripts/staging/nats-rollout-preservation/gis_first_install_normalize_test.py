import copy
from types import SimpleNamespace
import unittest
from controller import Blocked
import gis_first_install as gis
import gis_first_install_test as fixtures
import normalize

class Kube(fixtures.RunKube):
    def run(self,args,body=None):
        if args[:2]==['create','--dry-run=server']:
            row=copy.deepcopy(body)
            if row['kind']=='Service':
                row['spec'].update(type='ClusterIP',clusterIP='10.43.0.10',clusterIPs=['10.43.0.10'],ipFamilies=['IPv4'],sessionAffinity='None')
            else:
                row['spec'].setdefault('strategy',{'type':'RollingUpdate'})
            return row
        if args[0]=='create':
            row=copy.deepcopy(body)
            if row['kind']=='Service':row['spec'].update(clusterIP='10.43.0.11',clusterIPs=['10.43.0.11'])
            return super().run(args,row)
        return super().run(args,body)

def inputs():
    plan=fixtures.plan();deployment=next(o for o in plan['objects'] if o['kind']=='Deployment')
    template=deployment['spec']['template'];image=template['spec']['containers'][0]['image']
    target={'mode':'app-only','tag':'a'*40,'gis_first_install_sha256':plan['source_plan_sha256'],
        'manifest_sha256':normalize.digest([]),'template_hashes':{'voice-gameintegration':normalize.digest(template)},
        'images':{'voice-gameintegration/gameintegration':image}}
    return plan,target,SimpleNamespace(operation='d'*12,expected={'source_claim':'claim'})

class NormalizeFirstInstallTests(unittest.TestCase):
    def test_baseline_includes_exact_22_apps_publisher_and_hub(self):
        names=gis.baseline_workload_names(include_hub=True)
        self.assertEqual(len(names),24)
        self.assertEqual(len(gis.baseline_workload_names(include_hub=False)),23)
        snapshots={name:{'kind':'Deployment','metadata':{'name':name,'namespace':'voice-staging',
                    'uid':f'00000000-0000-0000-0000-{index:012x}'}} for index,name in enumerate(sorted(names),1)}
        self.assertTrue(gis.validate_baseline_snapshots(snapshots))
        for fault in ('missing','extra','uid','namespace'):
            changed=copy.deepcopy(snapshots)
            if fault=='missing':del changed['voice-p3-principal-jwks']
            if fault=='extra':changed['voice-other']=copy.deepcopy(changed['voice-web'])
            if fault=='uid':changed['voice-web']['metadata']['uid']=''
            if fault=='namespace':changed['voice-web']['metadata']['namespace']='voice-dev'
            with self.subTest(fault=fault),self.assertRaises(gis.Blocked):gis.validate_baseline_snapshots(changed)

    def test_create_only_normalized_pair_pins_actual_service_allocation(self):
        plan,target,stage=inputs();kube=Kube()
        result,owned=normalize.normalize_first_install(kube,stage,[],target,plan)
        self.assertEqual(result['input_template_hashes'],target['template_hashes'])
        self.assertEqual(result['template_hashes']['voice-gameintegration'],target['template_hashes']['voice-gameintegration'])
        self.assertEqual(owned['service_allocation'],'server')
        service=next(o for o in owned['objects'] if o['kind']=='Service')
        self.assertNotIn('clusterIP',service['spec'])
        checkpoints=[];receipt=gis.execute(kube,owned,lambda row:checkpoints.append(copy.deepcopy(row)),start=False)
        live=kube.objects[('Service','voice-gameintegration')]
        self.assertEqual(live['spec']['clusterIP'],'10.43.0.11')
        self.assertEqual(receipt['objects']['Service/voice-gameintegration']['spec_sha256'],gis.digest(gis.semantic(live)))
        for field,value in (('clusterIP','10.43.0.12'),('selector',{'app':'other'}),('type','NodePort')):
            changed=copy.deepcopy(live);changed['spec'][field]=value
            if field=='clusterIP':changed['spec']['clusterIPs']=['10.43.0.11']
            self.assertFalse(gis._matches_owned(changed,service,plan['operation'],True))

    def test_source_image_or_server_mutation_rejected_before_create(self):
        plan,target,stage=inputs();kube=Kube()
        altered=copy.deepcopy(target);altered['images']['voice-gameintegration/gameintegration']='repo@sha256:'+'f'*64
        with self.assertRaises(Blocked):normalize.normalize_first_install(kube,stage,[],altered,plan)
        self.assertEqual(kube.events,[])
        original=kube.run
        def changed(args,body=None):
            row=original(args,body)
            if args[:2]==['create','--dry-run=server'] and row['kind']=='Service':row['spec']['selector']={'app':'other'}
            return row
        kube.run=changed
        with self.assertRaisesRegex(Blocked,'source_fields_changed'):normalize.normalize_first_install(kube,stage,[],target,plan)

if __name__=='__main__':unittest.main()
