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

Those original bare-v1 components remain unqualified for the actor-bound path.
`ActorOfferServer` implements ONLY the new ActorBound Validate method, loading
the original immutable actor + current bound audience in one admitted SQL
transaction. It checks the complete v1/Q/actor/origin tuple, evaluates fresh
privacy outside SQL locks, and repeats the admitted state check. Its response
requires exact immutable PostgreSQL criteria bytes matching v1 criteriaSHA;
MM must check bytes/hash/criteria shape before capture. A public listing or a
generic Go JSON re-encoding cannot substitute for those bytes.

`ActorPrivacy` supplies current ShowStoriesAudience + privacy.Matcher checks
using the explicit SAVED human guest/regular actor for both floor and immutable
Story audience. Unknown type, malformed audience and missing needed authority
fail closed; service guestguard defaults/current-ID type lookups are unused.
`HumanVerifier` checks actual incoming TLS + signed exact delegated Gateway
request, replay and live epoch. It rechecks credential expiry after dependencies
and derives original actor operation/body/type only from the verified request.
Its credential expiry is an in-flight bound, never delayed persisted authority.

Actual Linux PostgreSQL and mTLS gRPC component tests cover guest AND regular
validation, fresh privacy deny/outage, final audience changes, expiry, exact
signed-body tampering, replay and absence of bare-v1 fallback. Admission/peer
authority and privacy remote responses remain synthetic component fixtures.
There is no production listener registration, credential loader, replay store,
controller factory or installed peer trust adapter here. Real controller,
retirement, retention and activation remain separate prerequisites.

`ActorRegistryProducer` uses ONLY new ActorBound methods. It reloads the exact
immutable local envelope and admitted cohort before EACH outbound call. Only a
definite NotFound from the admitted actual TLS endpoint allows Register of that
original envelope/operation; outage and ambiguous results never recapture newer
intent. Base registration + Q + origin + boundACK hashes all must match before
atomic durable local ACK. Local write failure recovers the same remote witness.

`ActionServer` implements protected Publish/Observe/Respond. It captures the
exact HumanVerifier actor, never raw metadata or a caller-supplied account/type.
Original operation replay loads its immutable sidecar before fresh candidate
privacy; neither conversion/current epoch nor a retracted/expired Story rewrites
the original consent/MM capture. Fresh operations need current privacy and exact
SQL offer. Publish/Observe expose profile-scoped offer views; Respond has empty
public success only after durable qualified ACK. Actual Linux PostgreSQL+mTLS
tests cover guest AND regular JOIN/INVITE and lost ACK recovery after type/epoch
change. Synthetic MM fixture does not implement MM acceptance/source policy.
Gateway routes and installed protected root composition remain prerequisites.
Reviewed protocol1 consent and registration hash material remain unchanged.
