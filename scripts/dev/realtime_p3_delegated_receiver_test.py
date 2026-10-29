import copy
import hashlib
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import realtime_p3_create_only as creation
import realtime_p3_create_only_packet as packet
import realtime_p3_delegated_receiver as receiver


class RequestTest(unittest.TestCase):
    def test_closed_request_binds_all_seven_and_three_public_roots(self):
        roots={identity:{'uid':'uid-'+identity[0],'resourceVersion':'1','certificate':identity[0].encode()} for identity in creation.PUBLIC}
        session=SimpleNamespace(roots=roots,fence=SimpleNamespace(context={'operation_id':'op'}),namespace_uid='ns',
                                lock={'uid':'lock'},plan={'issuer_source_sha':'b'*40})
        request={'context':{'operation_id':'op'},'namespace_uid':'ns','lock':{'uid':'lock'},'issuer_source_sha':'b'*40,
                 'capability_sha256':'a'*64,'objects':{kind+'/'+name:{'state':'absent'} for kind,name in packet.MEMBERS}}
        for name,identity in zip(('principal','media','space'),creation.PUBLIC):
            value=roots[identity]
            request[name]={'uid':value['uid'],'resourceVersion':'1','der_sha256':hashlib.sha256(value['certificate']).hexdigest()}
        with patch.object(receiver.crypto,'der',side_effect=lambda value:value):
            receiver._request_matches(session,request)
            for change in ('object','root','source','extra'):
                altered=copy.deepcopy(request)
                if change=='object':altered['objects'][next(iter(altered['objects']))]={'state':'present'}
                elif change=='root':altered['space']['resourceVersion']='2'
                elif change=='source':altered['issuer_source_sha']='c'*40
                else:altered['unexpected']=True
                with self.subTest(change=change),self.assertRaises(ValueError):receiver._request_matches(session,altered)


if __name__=='__main__':unittest.main()
