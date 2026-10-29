"""Bind a single qualified cohort upload to the fixed GitHub workflow run."""
import copy
import re
import media_operation_guard
import story_media_backup_cohort as cohort

REPOSITORY = 'Poryadok/VoiceRoot'
WORKFLOW_REF = REPOSITORY + '/.github/workflows/dev-cohort-backup-upload.yml@refs/heads/master'


class CohortRunError(RuntimeError):
    pass


def dispatch_marker(planned):
    return {'status':'pending', 'operation_id':planned['operation_id'],
            'challenge':planned['challenge'], 'invocation_nonce':planned['invocation_nonce'],
            'source_sha':planned['source_sha']}


def validate_environment(fence, environment, challenge):
    required = {'GITHUB_REPOSITORY','GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_WORKFLOW_REF',
                'GITHUB_SHA','GITHUB_WORKFLOW_SHA','GITHUB_RUN_ID','GITHUB_RUN_ATTEMPT'}
    if (not isinstance(environment,dict) or not required <= environment.keys() or
            environment['GITHUB_REPOSITORY'] != REPOSITORY or environment['GITHUB_EVENT_NAME'] != 'workflow_dispatch' or
            environment['GITHUB_REF'] != 'refs/heads/master' or environment['GITHUB_WORKFLOW_REF'] != WORKFLOW_REF or
            environment['GITHUB_SHA'] != fence.context['commit'] or environment['GITHUB_WORKFLOW_SHA'] != environment['GITHUB_SHA'] or
            not isinstance(environment['GITHUB_RUN_ID'],str) or not re.fullmatch(r'[1-9][0-9]{0,19}',environment['GITHUB_RUN_ID']) or
            environment['GITHUB_RUN_ATTEMPT'] != '1' or not isinstance(challenge,str) or not re.fullmatch(r'[0-9a-f]{32}',challenge)):
        raise CohortRunError('workflow identity invalid')


def claim(fence, client, lock, namespace_uid, environment, challenge, expected_members):
    attempted = False
    try:
        validate_environment(fence,environment,challenge)
        media_operation_guard.verify(fence,client,lock,namespace_uid)
        row, revision = fence.ledger.read()
        planned = row.get('cohort_upload') if isinstance(row,dict) else None
        required = {'status','operation_id','snapshot_id','source_sha','workflow_sha','invocation_nonce',
                    'challenge','bundle_bytes','bundle_sha256','members','receipt_sha256','manifest_sha256'}
        invocation = row.get('backup_invocation') if isinstance(row,dict) else None
        if (not fence.owned(row) or row.get('status')!='active' or not isinstance(planned,dict) or
                set(planned)!=required or planned['status']!='dispatch-pending' or
                planned['operation_id'] != fence.context['operation_id'] or
                not isinstance(planned['operation_id'],str) or not re.fullmatch(r'media-c[12]-[0-9a-f]{32}',planned['operation_id']) or
                planned['source_sha'] != fence.context['commit'] or planned['workflow_sha'] != environment['GITHUB_SHA'] or
                planned['challenge'] != challenge or not isinstance(planned['invocation_nonce'],str) or
                not re.fullmatch(r'[0-9a-f]{32}',planned['invocation_nonce']) or
                invocation != {'nonce':planned['invocation_nonce'],'operation_id':planned['operation_id']} or
                not isinstance(planned['snapshot_id'],str) or not re.fullmatch(r'capture-[0-9a-f]{32}',planned['snapshot_id']) or
                not isinstance(planned['bundle_sha256'],str) or not re.fullmatch(r'[0-9a-f]{64}',planned['bundle_sha256']) or
                type(planned['bundle_bytes']) is not int or not 0 < planned['bundle_bytes'] <= 64*1024**3):
            raise CohortRunError('cohort reservation invalid')
        dispatch = dispatch_marker(planned)
        if row.get('cohort_dispatch') != dispatch:
            raise CohortRunError('cohort dispatch intent missing')
        import cohort_upload_runner as runner
        mode=runner.route_mode(fence,lock,namespace_uid)
        route=row.get('full_cohort_backup_route')
        if (mode=='full-cohort-v1') != (route is not None):
            raise CohortRunError('cohort route mode differs')
        if route is not None:
            if expected_members!=runner.full_members(fence,lock,namespace_uid,row):
                raise CohortRunError('full cohort member set differs')
        elif 'full_cohort_backup_route' in row:
            raise CohortRunError('full cohort route invalid')
        if (not isinstance(expected_members,list) or expected_members != sorted(set(expected_members)) or
                'roles' not in expected_members or len(expected_members)<2 or
                (route is None and set(expected_members)-({'roles'} | cohort.ALLOWED_DATABASES)) or
                planned['members'] != expected_members or not isinstance(planned['receipt_sha256'],dict) or
                set(planned['receipt_sha256']) != set(expected_members) or
                any(not isinstance(v,str) or not re.fullmatch(r'[0-9a-f]{64}',v) for v in planned['receipt_sha256'].values()) or
                not isinstance(planned['manifest_sha256'],str) or not re.fullmatch(r'[0-9a-f]{64}',planned['manifest_sha256'])):
            raise CohortRunError('cohort selected members invalid')
        marker = row.get('cohort_bundle')
        members = row.get('cohort_members')
        if (not isinstance(marker,dict) or set(marker)!={'status','operation_id','snapshot_id','invocation_nonce',
                'bundle_bytes','bundle_sha256','manifest_sha256','receipt_sha256'} or marker.get('status')!='done' or
                not isinstance(members,dict) or set(members) != set(expected_members) or
                'backup_members' in row or not isinstance(marker.get('receipt_sha256'),dict) or
                marker['receipt_sha256'] != planned['receipt_sha256'] or marker.get('manifest_sha256') != planned['manifest_sha256'] or
                any(not isinstance(v,dict) or v.get('status')!='done' or
                    set(v)!={'status','nonce','snapshot_id','operation_id','receipt_sha256'} or
                    not isinstance(v.get('nonce'),str) or not re.fullmatch(r'[0-9a-f]{32}',v['nonce']) or
                    v.get('snapshot_id') != planned['snapshot_id'] or v.get('operation_id') != planned['operation_id'] or
                    v.get('receipt_sha256') != marker['receipt_sha256'][k] for k,v in members.items()) or
                marker.get('operation_id') != planned['operation_id'] or marker.get('snapshot_id') != planned['snapshot_id'] or
                marker.get('invocation_nonce') != planned['invocation_nonce'] or
                marker.get('bundle_bytes') != planned['bundle_bytes'] or marker.get('bundle_sha256') != planned['bundle_sha256']):
            raise CohortRunError('cohort bundle marker invalid')
        actual = copy.deepcopy(planned)
        actual.update(status='upload-running',run_id=environment['GITHUB_RUN_ID'],run_attempt=1)
        row['cohort_upload'] = actual; row['step'] = 'cohort-upload-running'
        fence.authority(); attempted = True
        fence.ledger.write(revision,row)
        media_operation_guard.verify(fence,client,lock,namespace_uid)
        checked, checked_revision = fence.ledger.read()
        if (not fence.owned(checked) or checked.get('status')!='active' or checked.get('backup_invocation')!=invocation or
                checked.get('cohort_upload')!=actual or checked.get('cohort_dispatch')!=dispatch or checked.get('cohort_bundle')!=marker or
                checked.get('full_cohort_backup_route')!=route or
                checked.get('cohort_members') != members or 'backup_members' in checked or
                not isinstance(checked_revision,tuple) or len(checked_revision)!=2 or
                any(not isinstance(v,str) or not v for v in checked_revision)):
            raise CohortRunError('cohort upload claim changed')
        if runner.route_mode(fence,lock,namespace_uid)!=mode:
            raise CohortRunError('cohort root route changed')
        if route is not None and runner.full_members(fence,lock,namespace_uid,checked)!=expected_members:
            raise CohortRunError('full cohort source route changed')
        return actual
    except BaseException:
        if attempted:
            try: fence.uncertain('cohort upload run claim outcome uncertain')
            except BaseException: pass
        raise CohortRunError('cohort upload run refused; reconcile if attempted') from None
