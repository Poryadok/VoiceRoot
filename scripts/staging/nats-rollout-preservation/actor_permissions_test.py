import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import actor_permissions as module


def effective(pub=None, sub=None, pub_deny=None, sub_deny=None):
    return {'pub': {'allow': pub, 'deny': pub_deny},
            'sub': {'allow': sub, 'deny': sub_deny}, 'response': None}


class PermissionTests(unittest.TestCase):
    def check(self, supplied, pub=(), sub=()):
        return module.evaluate(supplied, {'pub': list(pub), 'sub': list(sub)})

    def reject(self, supplied, pub=(), sub=()):
        with self.assertRaisesRegex(module.PermissionError, '^actor_permissions_unsupported_or_missing$'):
            self.check(supplied, pub, sub)

    def test_exact_actor_rights_and_neighbor_denial(self):
        info='$JS.API.CONSUMER.INFO.social_events.rt_realtime1_friend_removed'
        ack='$JS.ACK.social_events.rt_realtime1_friend_removed.>'
        inbox='_INBOX.voice.realtime1.friend_removed'
        row=effective([info,ack],[inbox],['$JS.ACK.social_events.neighbor.>'])
        result=self.check(row,[info,ack],[inbox])
        self.assertTrue(result['effective_permission_language_compatible'])
        self.assertNotIn('server_authentication_verified',result)
        self.reject(row,['$JS.ACK.social_events.neighbor.1'])
        self.reject(row,sub=['_INBOX.voice.realtime1.neighbor'])

    def test_allow_union_covers_infinite_required_family(self):
        self.check(effective(['x.*','x.*.>']),['x.>'])
        self.reject(effective(['x.*','x.*.*']),['x.>'])
        self.reject(effective(['x.a.>','x.b.>']),['x.*.>'])
        self.check(effective(['x.*','x.*.>'],['x.*','x.*.>']),['x.>'],['x.>'])

    def test_full_wildcard_requires_at_least_one_token(self):
        self.reject(effective(['x.>']),['x'])
        self.check(effective(['x.>']),['x.a','x.a.b'])
        self.reject(effective(['x.*']),['x.a.b'])
        self.reject(effective(['*.*']),['x.>'])
        self.check(effective(['>']),['*','x.>'])

    def test_any_deny_intersection_vetoes_family(self):
        self.reject(effective(['x.>'],pub_deny=['x.*.forbidden']),['x.>'])
        self.reject(effective(None,sub_deny=['x.a']),sub=['x.*'])
        self.check(effective(['x.>'],pub_deny=['x']),['x.>'])
        self.check(effective(['x.*'],pub_deny=['x.*.>']),['x.*'])

    def test_pinned_role_specific_nil_empty_semantics_and_hashes(self):
        nil=effective(); empty=effective([],[])
        self.check(nil,['anything'],['anything'])
        self.reject(empty,['anything'])
        a=self.check(nil,sub=['anything']); b=self.check(empty,sub=['anything'])
        self.assertNotEqual(a['effective_sha256'],b['effective_sha256'])
        self.assertEqual(a['effective_sha256'],hashlib.sha256(json.dumps(nil,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()).hexdigest())
        self.reject(effective(None,[],['x'],['x']),['x'])
        self.reject(effective(None,[],['x'],['x']),sub=['x'])

    def test_malformed_queue_and_unsupported_inputs_fail_closed(self):
        for subject in ('','a..b','.a','a.','a.>.b','a.*tail','a.b>','a queue','a\tqueue','a\0b'):
            with self.subTest(subject=subject): self.reject(effective([subject]),['ok'])
        for change in ({'response':{}},{'unknown':True},{'pub':{'allow':None}}, {'sub':None}):
            row=effective();row.update(change);self.reject(row)
        for required in ({'pub':None,'sub':[]},{'pub':[],'sub':[],'extra':[]},{'pub':[1],'sub':[]}):
            with self.assertRaises(module.PermissionError):module.evaluate(effective(),required)

    def test_input_limits_fail_before_pathological_exploration(self):
        self.reject(effective(['x']*4097),['x'])
        self.reject(effective(['x']*4096,pub_deny=['neighbor']),['x'])
        self.reject(effective(),['x']*257)
        self.reject(effective(['x'*4097]),['x'])
        self.reject(effective(['\u00e9'*2049]),['x'])
        self.reject(effective(['a'*4096]*256),['x'])
        # Valid grammar/count/byte size, but excessive total pattern nodes.
        self.reject(effective(['.'.join(['*']*1024)]*64))

    def test_exploration_budget_is_fail_closed(self):
        for limit in ('MAX_STATES','MAX_STATE_ITEMS','MAX_WORK','MAX_NODES'):
            with self.subTest(limit=limit), patch.object(module,limit,1):
                self.reject(effective(['x.*','x.*.>']),['x.>'])

    def test_deep_hole_is_not_finite_sampling(self):
        # Cover every shorter length, but leave the length-130 family uncovered.
        allow=['x.'+'.'.join(['*']*n) for n in range(1,130)]
        self.reject(effective(allow),['x.>'])
        allow.append('x.'+'.'.join(['*']*130)+'.>')
        self.reject(effective(allow),['x.>'])
        allow.append('x.'+'.'.join(['*']*130))
        self.check(effective(allow),['x.>'])

    def test_pinned_official_helper_literal_pattern_parity(self):
        # Independent exported server helper, no Server/CONNECT/broker created.
        pairs=[('x','x.>'),('x.a','x.>'),('x.a.b','x.*'),
            ('$JS.ACK.s.c.1.2','$JS.ACK.s.c.>'),('x.a','x.*'),
            ('a.b','>'),('other','x.>'),('x.a.c','x.*.c')]
        root=Path(__file__).resolve().parents[3]
        go_module=root/'scripts/staging/nats-known-baseline'
        self.assertIn('github.com/nats-io/nats-server/v2 v2.12.12',(go_module/'go.mod').read_text())
        code=('package main\nimport ("encoding/json";"os";"github.com/nats-io/nats-server/v2/server")\n'
            'func main(){var pairs [][2]string;json.Unmarshal([]byte(`'+json.dumps(pairs)+'`),&pairs);'
            'result:=[]bool{};for _,p:=range pairs{result=append(result,server.SubjectMatchesFilter(p[0],p[1]))};json.NewEncoder(os.Stdout).Encode(result)}')
        with tempfile.TemporaryDirectory(prefix='actor-permission-parity-') as directory:
            source=Path(directory)/'parity.go';source.write_text(code)
            result=subprocess.run(['go','run',str(source)],cwd=go_module,capture_output=True,timeout=120)
        self.assertEqual(result.returncode,0,'pinned_official_helper_failed')
        for (literal,pattern),allowed in zip(pairs,json.loads(result.stdout),strict=True):
            with self.subTest(literal=literal,pattern=pattern):
                if allowed:self.check(effective([pattern]),[literal])
                else:self.reject(effective([pattern]),[literal])


if __name__=='__main__': unittest.main()
