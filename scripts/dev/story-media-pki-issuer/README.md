# DEV Story media offline issuer

This initial-stage issuer generates an isolated `voice-dev` certificate and
signing-key packet for the direct Story/File/Messaging JWKS route. It has no
Kubernetes client, export operation, replay credential, deploy operation or
rotation workflow. A staged packet is not publication or rollout authority.
Do not run it against DEV until a separately reviewed operator binds the issuer
image, namespace UID, current resource claim and exact input/template hashes,
qualifies private key custody and encrypted recovery, and implements
create-only Secret reconciliation without printing Secret payloads.

The three CA domains separate the server endpoints, Story's File client and
File's Messaging JWKS client. Their DNS names, purposes, key pairs and exact
extensions are checked on stage and reload. The publisher's source SHA is
independent of the issuer source hash and of the product commit. The matching
authenticated Redis replay identity is a separate required input and must not
be invented for a shared Redis instance.

Run only in an isolated Linux container with an immutable reviewed issuer image,
network disabled, a private UID-owned `0700` state volume, a private temporary
filesystem and no capabilities. A persistent exclusive lock prevents concurrent
staging. A `.building` directory means an interrupted operation: preserve it and
reconcile it; never silently generate a replacement. Repeated successful staging
requires the same operation and issuer source hash. Stage hashes detect accidental
changes inside the single-owner private custody boundary; they are not an
authenticated receipt against an actor who can replace the whole state. Rotation
requires a separate reviewed versioned-key rollout with overlapping JWKS keys.
