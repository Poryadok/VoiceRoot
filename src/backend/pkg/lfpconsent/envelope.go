package lfpconsent

import (
	"bytes"
	"voice/backend/pkg/storylfpadmission"
)

const (
	QualifiedConsentDomain       = "voice.story.lfp-actor-bound-consent.v1\x00"
	ActorBoundRegistrationDomain = "voice.mm.lfp-actor-bound-registration.v1\x00"
	ActorQualifiedSourceDomain   = "voice.mm.lfp-actor-qualified-source.v1\x00"
)

// Envelope is immutable historical data. Construction validates data only,
// never authenticated delivery, current cohort admission or source eligibility.
type Envelope struct {
	consent                                     Consent
	actor                                       Actor
	origin                                      storylfpadmission.Origin
	consentBytes, actorBytes                    []byte
	consentSHA256, actorSHA256, qualifiedSHA256 [32]byte
}

func NewEnvelope(consent, actor []byte, o storylfpadmission.Origin) (Envelope, error) {
	c, ch, e := Decode(consent)
	if e != nil || !o.Valid() {
		return Envelope{}, ErrInvalid
	}
	a, ah, e := DecodeActor(actor)
	if e != nil || a.ActionKind != Respond || a.AccountID != c.ResponderAccountID || a.ProfileID != c.ResponderProfileID || a.SessionEpoch != c.ResponderSessionEpoch || a.OperationID != c.ResponseOperationID || a.PrincipalRequestSHA256 != c.PrincipalRequestSHA256 {
		return Envelope{}, ErrInvalid
	}
	b := append([]byte{}, ch[:]...)
	b = append(b, ah[:]...)
	environment, installation, incarnation, binding := o.Environment(), o.Installation(), o.Incarnation(), o.DatabaseBinding()
	b = append(b, environment[:]...)
	b = append(b, installation[:]...)
	b = append(b, incarnation[:]...)
	b = append(b, binding[:]...)
	return Envelope{consent: c, actor: a, origin: o, consentBytes: bytes.Clone(consent), actorBytes: bytes.Clone(actor), consentSHA256: ch, actorSHA256: ah, qualifiedSHA256: materialHash(QualifiedConsentDomain, b)}, nil
}
func (e Envelope) Consent() Consent                 { return e.consent }
func (e Envelope) Actor() Actor                     { return e.actor }
func (e Envelope) Origin() storylfpadmission.Origin { return e.origin }
func (e Envelope) ConsentBytes() []byte             { return bytes.Clone(e.consentBytes) }
func (e Envelope) ActorBytes() []byte               { return bytes.Clone(e.actorBytes) }
func (e Envelope) ConsentSHA256() [32]byte          { return e.consentSHA256 }
func (e Envelope) ActorSHA256() [32]byte            { return e.actorSHA256 }
func (e Envelope) QualifiedSHA256() [32]byte        { return e.qualifiedSHA256 }
func (e Envelope) Valid() bool                      { return e.qualifiedSHA256 != [32]byte{} && e.origin.Valid() }

func HashActorBoundRegistration(registration, qualified [32]byte) ([32]byte, error) {
	if registration == [32]byte{} || qualified == [32]byte{} {
		return [32]byte{}, ErrInvalid
	}
	b := append([]byte{}, registration[:]...)
	b = append(b, qualified[:]...)
	return materialHash(ActorBoundRegistrationDomain, b), nil
}

// HashActorQualifiedSource only encodes the specified hash domain. The owning
// MM validator must also cross-check the full source/actor/consent chain.
func HashActorQualifiedSource(source, qualified, registration, authorActor [32]byte) ([32]byte, error) {
	if source == [32]byte{} || qualified == [32]byte{} || registration == [32]byte{} || authorActor == [32]byte{} {
		return [32]byte{}, ErrInvalid
	}
	b := append([]byte{}, source[:]...)
	b = append(b, qualified[:]...)
	b = append(b, registration[:]...)
	b = append(b, authorActor[:]...)
	return materialHash(ActorQualifiedSourceDomain, b), nil
}

func (Envelope) String() string   { return "LFP actor-bound consent" }
func (Envelope) GoString() string { return "LFP actor-bound consent" }
