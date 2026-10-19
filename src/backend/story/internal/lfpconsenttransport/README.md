# Private LFP consent transport components

`OfferServer` implements the existing private MM-to-Story validation method.
It requires verified TLS peer chains, an exact signed Matchmaking service
credential, mandatory replay admission and controller-admitted peer origin.
The origin accompanies the exact request into journal validation. Current
responder visibility/eligibility is a mandatory external dependency; it executes
outside SQL locks, followed by the admitted transaction's exact state check.

`RegistryProducer` first loads the exact same-cohort durable consent locally.
It then reads MM's original registration with a fresh signed Story request.
Only a definite NotFound from the admitted actual TLS endpoint permits Register
of that same receipt/operation. A lost ACK or unknown outcome never recaptures
newer search intent. Successful readback checks canonical registration bytes,
hash and exact consent binding before saving the immutable acknowledgement in
Story. No network calls run under journal SQL locks. Public receipt/account/
epoch data is not returned by these components.

There is no production listener registration, credential loader, replay store,
controller factory, peer trust adapter or responder eligibility implementation
here. Tests use actual RSA verification and synthetic TLS/cohort/dependency
fixtures; they do not prove a TLS handshake or installed authority. In particular,
guest/account-type provenance and its later fresh visibility context require the
accepted producer c12 continuation before the eligibility adapter can qualify
any real flow. Empty type or an account/profile/epoch tuple cannot imply regular.

The additive private action protobuf defines stable publication/response
operations and an exact profile-scoped offer view. Its presence does not implement
the Gateway/Story delegated action handler. Reviewed protocol1 consent and
registration hash material remain unchanged.
