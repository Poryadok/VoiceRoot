"""Bounded read-only User/Notification effective-config metadata observation."""
import copy
import json
import re

import full_cohort_schema_metadata as metadata

COMMON = ('VOICE_RELEASE_PROFILE','VOICE_RELEASE_POLICY_FILE','VOICE_RELEASE_POLICY_SHA256',
          'VOICE_RELEASE_SNAPSHOT_FILE','DATABASE_URL')
CAPTURE = tuple('USER_SPACE_CAPTURE_PRINCIPAL_'+name for name in
    ('GRPC_LISTEN','TLS_CERT_FILE','TLS_KEY_FILE','CLIENT_CA_FILE','REPLAY_REDIS_ADDR',
     'REPLAY_REDIS_PASSWORD','JWKS_CA_FILE','JWKS_CLIENT_CERT_FILE','JWKS_CLIENT_KEY_FILE'))
NOTIFICATION = ('NOTIFICATION_RECORD_PUSHES','NOTIFICATION_DEBUG_HTTP_ENABLED','FCM_CREDENTIALS_JSON',
    'FCM_PROJECT_ID','FCM_SERVICE_ACCOUNT_JSON','APNS_KEY_ID','APNS_TEAM_ID','APNS_BUNDLE_ID',
    'APNS_AUTH_KEY','APNS_PRIVATE_KEY','APNS_AUTH_KEY_PATH','APNS_VOIP_TOPIC','APNS_PRODUCTION')
FIELDS = {'user':COMMON+('USER_REDIS_ADDR',)+CAPTURE, 'notification':COMMON+NOTIFICATION}
BASE = ['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default',
        '--namespace','voice-dev','--request-timeout=15s']
SCRIPT = ("LC_ALL=C; export LC_ALL; [ \"$(readlink /proc/1/exe)\" = /service ] || exit 1; "
    "printf 'x\\000' | (IFS= read -r -d '' probe && [ \"$probe\" = x ]) || exit 1; "
    "n=0; value=; while IFS= read -r -d '' line; do "
    "case \"$line\" in \"$1\"=*) n=$((n+1)); value=${line#*=};; esac; done </proc/1/environ; "
    "[ \"$n\" -le 1 ] || exit 1; if [ \"$n\" = 0 ]; then printf absent; exit; fi; "
    "if [ -z \"$value\" ]; then printf empty; exit; fi; "
    "case \"$1\" in NOTIFICATION_RECORD_PUSHES|NOTIFICATION_DEBUG_HTTP_ENABLED|APNS_PRODUCTION) "
    "if printf '%s' \"$value\" | grep -q '[^[:print:][:space:]]'; then printf unsupported; exit; fi; "
    "v=$value; while :; do case \"$v\" in [[:space:]]*) v=${v#?};; *) break;; esac; done; "
    "while :; do case \"$v\" in *[[:space:]]) v=${v%?};; *) break;; esac; done;; esac; "
    "case \"$1\" in VOICE_RELEASE_PROFILE) case \"$value\" in development) printf development;; "
    "free-invite-alpha) printf free-invite-alpha;; *) printf other;; esac;; "
    "NOTIFICATION_RECORD_PUSHES|NOTIFICATION_DEBUG_HTTP_ENABLED) "
    "v=$(printf '%s' \"$v\" | tr '[:upper:]' '[:lower:]'); "
    "case \"$v\" in true) printf true;; false) printf false;; *) printf other;; esac;; "
    "APNS_PRODUCTION) case \"$v\" in false) printf false;; *) printf other;; esac;; "
    "*) printf present;; esac")


class ProbeError(RuntimeError): pass


def inspect(workload, pod_name, run_bytes, guard):
    """Return fixed enums and non-secret references; this is observation, not admission."""
    reason = 'target'
    try:
        if workload not in FIELDS or not callable(run_bytes) or not callable(guard): raise ValueError()
        reason = 'workload-before'
        snap = metadata.kube_workload_snapshot(workload,pod_name,run_bytes,guard,namespace='voice-dev')
        metadata._fence(guard); before = snap()
        fields = FIELDS[workload]; objects = {}
        def reference(kind,name):
            if (kind not in ('secret','configmap') or not isinstance(name,str)
                    or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?',name)): raise ValueError()
            key = (kind,name)
            if key not in objects:
                metadata._fence(guard)
                template = '{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}{{range $k,$v := .data}}{{$k}}{{"\\n"}}{{end}}'
                raw = metadata._external(run_bytes, BASE+['get',kind,name,'-o','go-template='+template])
                metadata._fence(guard)
                if not isinstance(raw,bytes) or len(raw) > 65536: raise ValueError()
                rows = raw.decode('utf-8').splitlines()
                if len(rows) < 2 or not rows[0] or not rows[1] or len(rows[2:]) != len(set(rows[2:])): raise ValueError()
                objects[key] = {'kind':kind,'name':name,'uid':rows[0],'resource_version':rows[1],'keys':set(rows[2:])}
            return objects[key]
        reason = 'env-from'
        origins = {name:None for name in fields}; env_sources = []
        for item in before['envFrom']:
            if not isinstance(item,dict) or set(item)-{'prefix','secretRef','configMapRef'}: raise ValueError()
            prefix = item.get('prefix','')
            if not isinstance(prefix,str): raise ValueError()
            refs = [(k,item[k]) for k in ('secretRef','configMapRef') if k in item]
            if len(refs) != 1: raise ValueError()
            typ, ref = refs[0]
            if not isinstance(ref,dict) or set(ref)-{'name','optional'} or ref.get('optional',False) is not False: raise ValueError()
            obj = reference('secret' if typ == 'secretRef' else 'configmap',ref['name'])
            selected = sorted(name for name in fields if name.startswith(prefix) and name[len(prefix):] in obj['keys'])
            env_sources.append({'kind':obj['kind'],'name':obj['name'],'uid':obj['uid'],
                                'resource_version':obj['resource_version'],'prefix':prefix,'fields':selected})
            for name in fields:
                if name.startswith(prefix) and name[len(prefix):] in obj['keys']:
                    origins[name] = {'kind':obj['kind'],'name':obj['name'],'uid':obj['uid'],
                                     'resource_version':obj['resource_version'],'key':name[len(prefix):],'source':'envFrom'}
        reason = 'explicit-env'
        explicit = before['env']; names = [entry.get('name') for entry in explicit if isinstance(entry,dict)]
        reason = 'explicit-env-malformed'
        if len(names) != len(explicit) or any(not isinstance(name,str) for name in names): raise ValueError()
        reason = 'explicit-env-duplicate'
        if len(names) != len(set(names)): raise ValueError()
        for entry in explicit:
            name = entry['name']
            if name not in origins: continue
            reason = 'explicit-env-malformed-'+name
            if set(entry)-{'name','value','valueFrom'}: raise ValueError()
            if 'value' in entry and 'valueFrom' not in entry:
                if not isinstance(entry['value'],str): raise ValueError()
                origins[name] = {'source':'literal'}
            elif 'value' not in entry and 'valueFrom' not in entry:
                origins[name] = {'source':'literal'}
            elif 'valueFrom' in entry and 'value' not in entry:
                value = entry['valueFrom']
                if not isinstance(value,dict) or len(value) != 1: raise ValueError()
                typ, ref = next(iter(value.items()))
                reason = 'explicit-env-unsupported-ref-'+name
                if typ not in ('secretKeyRef','configMapKeyRef') or not isinstance(ref,dict) or set(ref)-{'name','key','optional'}: raise ValueError()
                reason = 'explicit-env-optional-'+name
                if ref.get('optional',False) is not False: raise ValueError()
                reason = 'explicit-env-reference-read-'+name
                obj = reference('secret' if typ == 'secretKeyRef' else 'configmap',ref['name'])
                reason = 'explicit-env-missing-key-'+name
                if not isinstance(ref.get('key'),str) or ref['key'] not in obj['keys']: raise ValueError()
                origins[name] = {'kind':obj['kind'],'name':obj['name'],'uid':obj['uid'],
                                 'resource_version':obj['resource_version'],'key':ref['key'],'source':'explicit'}
            else: raise ValueError()
        observed = {}
        for name in fields:
            reason = 'runtime-' + name
            metadata._fence(guard)
            args = BASE+['exec',pod_name,'-c',workload,'--','sh','-ec',SCRIPT,'--',name]
            raw = metadata._external(run_bytes,args); metadata._fence(guard)
            allowed = {'absent','empty','present'}
            if name == 'VOICE_RELEASE_PROFILE': allowed = {'absent','empty','development','free-invite-alpha','other'}
            if name in ('NOTIFICATION_RECORD_PUSHES','NOTIFICATION_DEBUG_HTTP_ENABLED','APNS_PRODUCTION'):
                allowed = {'absent','empty','true','false','other'}
            if not isinstance(raw,bytes) or len(raw)>32 or raw.decode('ascii') not in allowed: raise ValueError()
            status = raw.decode('ascii')
            reason = 'origin-' + name
            if (origins[name] is None) != (status == 'absent'): raise ValueError()
            observed[name] = {'state':status,'origin':origins[name]}
        reason = 'reference-recheck'
        expected_objects = copy.deepcopy(objects); objects.clear()
        for kind,name in expected_objects: reference(kind,name)
        reason = 'workload-after'
        metadata._fence(guard); after = snap(); metadata._fence(guard)
        reason = 'identity-drift'
        if before != after or expected_objects != objects: raise ValueError()
        identity = {k:before[k] for k in ('namespace','namespace_uid','deployment','deployment_uid','deployment_rv',
                    'pod_name','pod_uid','pod_rv','container_name','container_id','restart_count')}
        return {'status':'observed','identity':identity,'fields':observed,'env_sources':env_sources,
                'scope':'runtime-config-metadata-only-no-provider-validation'}
    except BaseException:
        raise ProbeError('runtime configuration observation unavailable: '+reason) from None
