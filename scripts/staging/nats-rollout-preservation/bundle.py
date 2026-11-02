"""Build the reviewable offline human-root launcher and exact code archive."""
import hashlib
import io
import json
from pathlib import Path
import sys
import tarfile

KNOWN=('root_main.py','controller.py','commands.py','docker_runtime.py','stage_runtime.py','scenario.py','deployed-contract.json')
ROLLOUT=('apply.py','bridge.py','bridge_client.py','bridge_root.py','compiler.py','encrypted_cut.py','errors.py','github_custody.py','gis_first_install.py','gis_prerequisite.py','guard.py','installer.py','migrations.py','native_store.py','normalize.py','nonnats_plan.py','nonnats_runtime.py','source_authority.py','source_plan.py','rollout_census.py','preserve.py','root_cli.py','runner.py','runtime_stage.py','transaction.py','workflow_entry.py')
ROLLOUT+=('bootstrap_auth.py','bootstrap_enrollment.py','bootstrap_renewal.py','bootstrap_root.py','bootstrap_selection.py',
    'nats_active_contract.py','nats_contract_actor.py','nats_contract_plan.py','nats_contract_custody.py','nats_migration.py','nats_root_plan.py','nats_deliver_all.py')
ROLLOUT+=('actor_auth.py','actor_root.py','actor_verification.py','actor_permissions.py','prebuild_disposition.py',
    'space_authority.py','space_backup.py','space_migration.py','space_restore.py','space_safeguards.py')
ROLLOUT+=('renderer_transition.py','renderer_execution.py','renderer_root.py','bot_migration.py')
ROLLOUT+=('prebuild_v5_disposition.py',)
ROLLOUT+=('story_witness.py','story_fixture.py')
ROLLOUT+=('paused_recovery.py','v8_upgrade.py')
ROLLOUT+=('preserved_upload.py',)
ROLLOUT+=('expired_recovery.py','expired_proof.py','expired_admission.py','expired_transport.py','expired_finish.py')
ROLLOUT+=('hub_bridge_identity.py','hub_bridge_unit.py','operator_retirement.py')
ROLLOUT+=('closed_archive.py','closed_durable.py','closed-seal-source-policies.json')
ROLLOUT+=('operator_reset.py',)
ROLLOUT+=('deployment_report.py',)
ROLLOUT+=('provisional.py',)
ROLLOUT+=('release_policy_wiring.py',)
ROLLOUT+=('resource_claim.py',)
ROLLOUT+=('gis_existing_update.py',)
ROLLOUT+=('realtime_bootstrap.py','realtime_bootstrap_existing.py')
# bridge_root prepares these source-bound children before capturing workloads.
ROLLOUT+=('staging_prerequisites.py','staging_release_policy_prepare.py','staging_p3_prerequisites.py',
    'staging_p3_prerequisites_crypto.py','staging_gis_material.py')
ROLLOUT+=('staging_cohort_preservation.py','staging_cohort_preservation_pg.py')
ROLLOUT+=('staging_cohort_preservation_auxiliary.py','staging_cohort_preservation_runtime.py','staging_cohort_preservation_auth.py')

def build(kernel,output,renewer,story_probe,*,story_target8=None):
    source=Path(__file__).resolve().parents[1];kernel=Path(kernel);output=Path(output)
    raw=kernel.read_bytes()
    if not raw.startswith(b'\x7fELF') or len(raw)>32<<20:raise ValueError('Linux kernel required')
    contents={}
    for directory,names in (('nats-known-baseline',KNOWN),('nats-rollout-preservation',ROLLOUT)):
        for name in names:
            path=source/directory/name
            if path.is_symlink():raise ValueError('source symlink')
            contents[directory+'/'+name]=path.read_bytes()
    contents['nats-known-baseline/kernel']=raw
    raw=Path(renewer).read_bytes()
    if not raw.startswith(b'\x7fELF') or len(raw)>32<<20:raise ValueError('Linux bootstrap renewer required')
    contents['nats-rollout-preservation/bootstrap-renewer']=raw
    raw=Path(story_probe).read_bytes()
    if not raw.startswith(b'\x7fELF') or not 0<len(raw)<=32<<20:raise ValueError('Linux Story compatibility probe required')
    contents['nats-rollout-preservation/story-current4-probe']=raw
    if story_target8 is not None:
        path=Path(story_target8)
        if path.is_symlink():raise ValueError('Qualified restored8 Story probe required')
        raw=path.read_bytes()
        if (not raw.startswith(b'\x7fELF') or not 0<len(raw)<=32<<20
            or hashlib.sha256(raw).hexdigest()!='458e4e3e5e940137cddddaf42dda1c22505f92de5792600364907a343b209be0'):
            raise ValueError('Qualified restored8 Story probe required')
        contents['nats-rollout-preservation/story-restored8-probe']=raw
    contents['configure-kubectl-ci.sh']=(source/'configure-kubectl-ci.sh').read_bytes()
    contents['mail-only-patch.py']=(source/'mail-only-patch.py').read_bytes()
    contents['capture-manifest.json']=json.dumps({n:hashlib.sha256(v).hexdigest() for n,v in contents.items()},sort_keys=True).encode()
    result=io.BytesIO()
    with tarfile.open(fileobj=result,mode='w',format=tarfile.USTAR_FORMAT) as archive:
        for name,data in sorted(contents.items()):
            member=tarfile.TarInfo(name);member.size=len(data);member.mode=0o400;archive.addfile(member,io.BytesIO(data))
    bundle=result.getvalue();sha=hashlib.sha256(bundle).hexdigest()
    output.mkdir(parents=True,exist_ok=True)
    (output/'rollout-bundle.tar').write_bytes(bundle)
    template=(source/'nats-rollout-preservation/root-launch.template.sh').read_text()
    launcher=template.replace('__BUNDLE_SHA256__',sha).replace('__BUNDLE_BYTES__',str(len(bundle))).replace('__BUNDLE_FILES__',repr(sorted(contents)))
    (output/'rollout-root.sh').write_text(launcher,newline='\n')
    print('NATS_ROLLOUT_BUNDLE_SHA256='+sha)
    return sha

if __name__=='__main__':
    if len(sys.argv) not in (5,6):raise SystemExit('usage: bundle.py LINUX_KERNEL OUTPUT_DIR LINUX_BOOTSTRAP_RENEWER LINUX_STORY_PROBE [LINUX_STORY_TARGET8_PROBE]')
    build(*sys.argv[1:5],**({'story_target8':sys.argv[5]} if len(sys.argv)==6 else {}))
