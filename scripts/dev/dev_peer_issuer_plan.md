# ExecPlan: delegated DEV peer issuer

## Purpose and context

Let the ordinary `pmd` operator request only fixed DEV peer identities without a per-operation root login. The existing root-held media CA stays in its original custody; no CA, database, or retained identity is replaced. The one-time installer is a reviewed artifact, not an action performed by this change. STAGING uses its separate bridge and CA.

The fixed families are Creator five (Space/Gateway clients, Auth/Role servers, Gateway signer), Space two (Space signer/server), and Role two (Role client/signer). `reset_credentials.py`, `space_p3_issuer_preflight*`, `space_creator_runtime_profile_issuer*`, and `docs/DEPLOYMENT.md` define the existing material, TT/fence and peer rules. The Role receiver is separately owned.

## Scope and milestones

- [ ] Source-owned preflight and independent root metadata observer for the closed nine names, including exact current CA, namespace, TT claim and lock.
- [ ] One fixed socket-activated root service and one `pmd` client, fixed profile enum and immutable source/operation bindings; no arbitrary command, path, signing input, CA export or caller boolean.
- [ ] Guarded create-only profile packets and typed recipients. Repeat fresh preflight for each profile after any predecessor create; unknown/incomplete material holds.
- [ ] Source-pinned one-time installer artifact and focused documentation. Do not run the installer or issue live certificates during development.
- [ ] Immutable Linux end-to-end fixtures, negative lost-result/replay/authority tests, and independent security review.

## Validation and follow-up

Use isolated root/pmd fixture processes and real sealed FD/crypto; fake only TT/Kubernetes in fixture. Validate actual full run, wrong peer/source/profile, stale CA/lock, partial Secret and timeout/no replay. Installation requires an authorized operator only after the exact source and installer are reviewed. The current root CA key custody and downstream runtime qualification remain external prerequisites.

## Decisions and risks

Keep the existing CA in root custody: Docker's pmd access is not used to mount the host root or export the key. A separately installed service delegates only the fixed issuance operation. The current five-name preflight cannot authorize nine, and a historical preflight cannot silently cover intervening creates. Systemd socket peer identity alone does not establish source or operation authority; the root service must repeat the fixed TT, namespace, lock, CA and metadata checks.
