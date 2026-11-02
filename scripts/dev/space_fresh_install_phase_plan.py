"""Render the fixed source-owned DEV Space installer plan."""
import hashlib
import json
from pathlib import Path
import re

import publish_candidate

TEMPLATE = 'deploy/templates/migrate-space-db-job.yaml'
SQL_DIR = 'src/backend/migrations/space_db'
TEMPLATE_SHA256 = '7498969df753504e584988568dfe8582257d85284e896bbac9e5de714c02f5c0'

class PlanError(RuntimeError): pass

def render(root, source, operation, image, build_source_sha256, node):
    try:
        root = Path(root)
        if (not isinstance(source,str) or not re.fullmatch(r'[0-9a-f]{40}',source) or
            not isinstance(operation,str) or not re.fullmatch(r'media-c1-[0-9a-f]{32}',operation) or
            not isinstance(image,str) or not re.fullmatch(r'[A-Za-z0-9._/:-]+@sha256:[0-9a-f]{64}',image) or
            not isinstance(build_source_sha256,str) or not re.fullmatch(r'[0-9a-f]{64}',build_source_sha256) or
            not isinstance(node,str) or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]{0,251}[a-z0-9])?',node)):
            raise ValueError()
        snapshot = publish_candidate.source_snapshot(root,source)
        if snapshot.get('source_sha') != source or not isinstance(snapshot.get('files'),dict): raise ValueError()
        template = root / TEMPLATE
        raw = template.read_bytes()
        if (template.is_symlink() or hashlib.sha256(raw).hexdigest()!=TEMPLATE_SHA256 or
                snapshot['files'].get(TEMPLATE)!=TEMPLATE_SHA256): raise ValueError()
        prefix = SQL_DIR + '/'
        expected = {key:value for key,value in snapshot['files'].items() if key.startswith(prefix)}
        paths = list((root / SQL_DIR).iterdir())
        if not paths or any(not p.is_file() or p.is_symlink() or not re.fullmatch(r'[0-9]{6}_[a-z0-9_]+\.(?:up|down)\.sql',p.name) for p in paths): raise ValueError()
        if {prefix+p.name for p in paths} != set(expected): raise ValueError()
        data = {}
        for path in sorted(paths):
            contents = path.read_bytes()
            if hashlib.sha256(contents).hexdigest()!=expected[prefix+path.name] or b'\0' in contents: raise ValueError()
            data[path.name] = contents.decode('utf-8')
        if sum(len(v.encode('utf-8')) for v in data.values()) > 900000: raise ValueError()
        input_hash = hashlib.sha256(json.dumps({'image':image,'source':build_source_sha256,
            'sql':{name:hashlib.sha256(value.encode()).hexdigest() for name,value in sorted(data.items())}},sort_keys=True).encode()).hexdigest()
        suffix=operation[-32:]; config_name='voice-space-native-sql-'+suffix; job_name='voice-space-native-'+suffix
        cm={'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':config_name,'namespace':'voice-dev',
            'annotations':{'voice.dev/cohort-operation':operation,'voice.io/source-sha':source}},'immutable':True,'data':data}
        container={'name':'migrate','image':image,'command':['/bin/sh','-ec'],
            'args':['if [ "$SPACE_NATIVE_INSTALLER_MODE" = verify ]; then\n  exec native-migrate --mode=verify --prior-evidence=/evidence/prior.jsonl\nfi\nexec native-migrate --mode=provision\n'],
            'env':[{'name':'PGHOST','value':'voice-postgres'},{'name':'PGSSLMODE','value':'disable'},
                {'name':'PGUSER','valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}},
                {'name':'PGPASSWORD','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}}},
                {'name':'SPACE_NATIVE_INSTALLER_MODE','value':'provision'},
                {'name':'SPACE_NATIVE_REQUIRED_SOURCE_SHA256','value':build_source_sha256}],
            'resources':{'requests':{'cpu':'20m','memory':'64Mi'},'limits':{'cpu':'2','memory':'3Gi'}},
            'volumeMounts':[{'name':'migrations','mountPath':'/migrations','readOnly':True},
                            {'name':'evidence','mountPath':'/evidence','readOnly':True}]}
        job={'apiVersion':'batch/v1','kind':'Job','metadata':{'name':job_name,'namespace':'voice-dev',
            'annotations':{'voice.dev/cohort-operation':operation,'voice.io/native-input-hash':input_hash,
                           'voice.io/source-sha':source},
            'labels':{'app.kubernetes.io/name':'voice-migrate-space-db','app.kubernetes.io/component':'migration'}},
            'spec':{'ttlSecondsAfterFinished':86400,'backoffLimit':0,'activeDeadlineSeconds':600,
                'template':{'metadata':{'labels':{'app.kubernetes.io/name':'voice-migrate-space-db'}},
                  'spec':{'nodeName':node,'restartPolicy':'Never','automountServiceAccountToken':False,
                          'containers':[container],'volumes':[{'name':'migrations','configMap':{'name':config_name,'optional':False}},
                          {'name':'evidence','configMap':{'name':'voice-space-native-installer-evidence','optional':True}}]}}}}
        return cm,job
    except BaseException:
        raise PlanError('source-bound Space installer plan unavailable') from None
