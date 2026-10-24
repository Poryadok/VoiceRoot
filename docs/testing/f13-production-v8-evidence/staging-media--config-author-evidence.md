# Staging native media configuration evidence

Author: bot_installation. Source checkout: WT43, base 3a2b8e19b0f8fe7a6104b753340f62864e039326. Four-file uncommitted bounded delta; root owns final integration and commit.

Final SHA256:

- deploy/staging/services.yaml: ad16980877bfc889e36d48edccd3b2b245f13dfd1384901a255964b579223efa
- scripts/staging/nats-rollout-preservation/nonnats_plan.py: 8401079ced4444e2f236cff4217ea5a218979c6f73fedcd1a746f33ebd11e9a1
- scripts/staging/nats-rollout-preservation/media_projection_test.py: 5275cc1234002b15396b1cdafaf2619446073ef9f5ef463288f8a2b102c138b4
- docs/DEPLOYMENT.md: 8887f26e9f708185f3d43c9a3f4ce0ea73fbbdbe7209a1821e007e1a6d1d5dd7

Meaningful RED: immutable original services plus test snapshot; missing Chat Role address, Chat/Voice/Role env coverage, and Chat 8443 service port. Two selected methods: one error and four failures. Log media-red-snapshot.log SHA256 b5f2f168179063544f8a5955c2e567b0ad8f3a7bd2d52c50d1865616fb5c0048. The host os.O_NOFOLLOW and initial missing PyYAML attempts were setup failures, not RED evidence.

GREEN: complete media_projection_test class, six tests, 5.351 seconds, Linux Python 3.12.15/PyYAML 6.0.3 with read-only immutable overlays, 2 CPU/3 GiB. Log media-green.log SHA256 05733e1bc533248678f5e872f83e33291d069c20994eece50be617ac7cbc6627. Config/test file hashes match final source. The only subsequent change clarified baseline versus Chat opt-in issuer wording in documentation. git diff --check passed.

Scope: parsed listener/client/JWKS/Secret symmetry for Chat -> Role, Role -> Auth and Space, Voice -> Space; exact optional issuer sets; canonical Space JWKS URL; read-only explicit private key projections and eight missing-Secret preflight negatives. Bot disabled, delegated Role disabled, dormant Voice lifecycle absent. Existing independent helper receives explicit TLS key projection and immutable public document prerequisite; compiler enrollment is a separate author's change.

These checks do not validate real certificate contents, cluster provisioning, startup reachability, deployment, or live media behavior. Full staging suite remains pending combined compiler/fixture integration. Independent review is supplied separately by role_admission; author does not self-accept.
