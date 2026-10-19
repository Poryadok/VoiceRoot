# LFP actor-bound protocol1

Accepted producer c14, `BE011-LFP-ACTOR-PROVENANCE-v1`, adds actor provenance
alongside the unchanged v1 consent/registration material. The exact additive
descriptors are in `protos/voice/story/v1/lfp_actor.proto`. Their protected
services are `LfpActorBoundConsentRegistryService` and
`LfpActorBoundConsentOfferService`; old v1 endpoints and ACKs cannot qualify the
actor-bound path and are never a fallback.

`Actor` encodes exactly the12 required ascending protobuf fields: protocol,
action kind, human actor kind, explicit account type, account/profile, original
epoch, stable operation, issuer, audience, full RPC and final request digest.
Only `regular` and `guest` HUMAN delegated-user actions are supported. Missing,
SDK, bot and service actor data cannot default to regular. Constructors/codecs
validate data only; a trusted network verifier must capture the original actor.

`NewEnvelope` cross-checks the RESPOND actor against the original v1 account,
profile, epoch, operation and request digest, then commits its full original
306/314 Origin. Returned byte buffers are copies. Domain hashes are:

- Actor: `voice.lfp.actor-provenance.v1` + NUL + canonical actor.
- Consent envelope: `voice.story.lfp-actor-bound-consent.v1` + NUL + consent
  hash32 + actor hash32 + environment16 + installation16 + incarnation16 +
  database binding32.
- Qualified ACK: `voice.mm.lfp-actor-bound-registration.v1` + NUL + original
  registration hash32 + qualified consent hash32.
- Qualified source: `voice.mm.lfp-actor-qualified-source.v1` + NUL + original
  source hash32 + qualified consent hash32 + qualified ACK hash32 + author actor
  hash32. The MM owner must cross-check the complete source/member/actor chain;
  computing a hash alone never authenticates that chain.

Independent literal Python vectors used by the Go golden tests are preserved in
the producer proof. Guest RESPOND actor hash is
`0ee8c407a9c8e01f9fbb593f8b4dc36fae509ad8b3f9c57e4602205eb223b85d`,
and its fixture envelope hash is
`963207a612af029c79f2ac76a6a260560076ec05fd8de4c4846477608bf0d7ce`.
Generated protobuf parity covers all four actions and both explicit types;
existing v1 goldens remain unchanged.

Story must atomically persist the original sidecar with consent/publication and
cohort, preserve it on exact replay even after guest conversion/epoch change,
and use the saved explicit viewer for fresh visibility checks. MM must persist
that exact envelope and captured intent atomically with qualified ACK, then bind
the separately verified author actor at acceptance/source creation. Missing old
sidecars remain unqualified. These persistence/transport owners are291/145;
this codec/descriptor pin does not implement their runtime composition, account
erasure, installation or acceptance. All linked data and hashes remain personal.
