import copy
import unittest

import space_fresh_install_phase_job as subject


class JobTest(unittest.TestCase):
    def setUp(self):
        self.digest = 'sha256:' + 'a' * 64
        self.authored = {'apiVersion':'batch/v1','kind':'Job',
            'metadata':{'name':'voice-space-native-test','namespace':'voice-dev','labels':{'app':'space'},
                        'annotations':{'voice.io/source-sha':'b'*40}},
            'spec':{'backoffLimit':0,'template':{'metadata':{'labels':{'app':'space'}},
                'spec':{'nodeName':'node1','restartPolicy':'Never','containers':[{'name':'migrate','image':'image'}]}}}}
        self.actual_job = copy.deepcopy(self.authored)
        self.actual_job['metadata'].update(uid='job1',resourceVersion='1')
        self.actual_job['spec']['selector']={'matchLabels':{'batch.kubernetes.io/controller-uid':'job1'}}
        self.actual_job['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid']='job1'
        self.actual = {'apiVersion':'v1','kind':'Pod',
            'metadata':{'name':'pod1','namespace':'voice-dev','uid':'poduid','resourceVersion':'2',
                'labels':copy.deepcopy(self.actual_job['spec']['template']['metadata']['labels']),
                'ownerReferences':[{'kind':'Job','name':'voice-space-native-test','uid':'job1','controller':True}]},
            'spec':copy.deepcopy(self.authored['spec']['template']['spec']),
            'status':{'phase':'Running','podIP':'10.0.0.2','containerStatuses':[{'name':'migrate','containerID':'containerd://abc',
                           'imageID':'containerd://' + self.digest,'restartCount':0}]}}

    def test_owned_pod(self):
        self.assertEqual(subject.pod(self.authored,self.actual_job,self.actual,self.digest)['uid'], 'poduid')

    def test_wrong_owner_image_or_injected_spec_hold(self):
        for field in ('owner','image','restart','spec'):
            actual = copy.deepcopy(self.actual)
            if field == 'owner': actual['metadata']['ownerReferences'][0]['uid'] = 'other'
            if field == 'image': actual['status']['containerStatuses'][0]['imageID'] = 'containerd://sha256:' + 'c'*64
            if field == 'restart': actual['status']['containerStatuses'][0]['restartCount'] = 1
            if field == 'spec': actual['spec']['hostNetwork'] = True
            with self.subTest(field=field), self.assertRaises(subject.JobError):
                subject.pod(self.authored,self.actual_job,actual,self.digest)

    def test_job_owner_and_finalizer_hold(self):
        for key, value in (('ownerReferences',[{'uid':'foreign'}]), ('finalizers',['foreign'])):
            actual = copy.deepcopy(self.actual_job); actual['metadata'][key] = value
            with self.subTest(key=key), self.assertRaises(subject.JobError):
                subject.job(self.authored, actual)


if __name__ == '__main__': unittest.main()
