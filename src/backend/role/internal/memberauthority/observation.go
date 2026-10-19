// Package memberauthority validates fresh protected Space observations. Its
// values are request-scoped eligibility evidence, never portable permissions.
package memberauthority

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/role/internal/memberbinding"
)

var ErrUnavailable = errors.New("Space membership authority unavailable")

type Observation struct {
	space, profile           string
	revision                 uint64
	deadline                 time.Time
	active                   bool
	owner                    string
	chat                     string
	treePresent              bool
	treeRevision             uint64
	lifecycleGeneration      uint64
	ownershipGeneration      uint64
	ordinaryPublicationKind  spacev1.MemberAdmissionPublicationKind
	admissionReady           bool
	nativeProofDigest        []byte
	ordinary, community      *memberadmission.MembershipBinding
	preparing                *memberadmission.MembershipBinding
	preparingDigest, receipt []byte
}

func canonical(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
func positive(value uint64) bool { return value > 0 && value <= math.MaxInt64 }
func counter(value uint64) bool  { return value <= math.MaxInt64 }

// Parse starts its validity clock BEFORE the RPC dispatch, conservatively
// subtracting the whole roundtrip and any subsequent local lock wait. Timestamp
// is diagnostic only; service clocks need not be synchronized.
func Parse(req *spacev1.GetMemberAdmissionAuthorityRequest, reply *spacev1.GetMemberAdmissionAuthorityResponse, started, now time.Time) (Observation, error) {
	var out Observation
	if req == nil || reply == nil || req.ProtocolVersion != 1 || reply.ProtocolVersion != 1 || !canonical(req.SpaceId) || !canonical(req.ProfileId) || reply.SpaceId != req.SpaceId || reply.ProfileId != req.ProfileId || !positive(reply.SpaceAuthorityRevision) || unknown(req) || unknown(reply) || started.IsZero() || now.Before(started) {
		return out, ErrUnavailable
	}
	if (req.ExpectedGeneration == nil) != (req.ExpectedAdmissionOperationId == nil) {
		return out, ErrUnavailable
	}
	if req.ExpectedGeneration != nil && (!positive(*req.ExpectedGeneration) || !canonical(*req.ExpectedAdmissionOperationId)) {
		return out, ErrUnavailable
	}
	if req.ChatId != nil {
		if !canonical(*req.ChatId) || reply.TreeRelation == nil || reply.TreeRelation.ChatId != *req.ChatId || !positive(reply.TreeRelation.Revision) {
			return out, ErrUnavailable
		}
		out.chat, out.treePresent, out.treeRevision = *req.ChatId, reply.TreeRelation.Present, reply.TreeRelation.Revision
	} else if reply.TreeRelation != nil {
		return out, ErrUnavailable
	}
	if reply.SampledAt == nil || reply.SampledAt.CheckValid() != nil || reply.ValidityRemaining == nil || reply.ValidityRemaining.CheckValid() != nil {
		return out, ErrUnavailable
	}
	validity := reply.ValidityRemaining.AsDuration()
	if validity <= 0 || validity > 5*time.Second {
		return out, ErrUnavailable
	}
	life, owner, ordinary, community := reply.Lifecycle, reply.Ownership, reply.Ordinary, reply.Community
	if life == nil || owner == nil || ordinary == nil || community == nil || !counter(life.Generation) || !positive(owner.Generation) || !canonical(owner.CurrentOwnerProfileId) || !counter(ordinary.HeadGeneration) || !counter(ordinary.RetiredThrough) || !counter(community.RetiredThrough) || !counter(community.RosterSourceRevision) {
		return out, ErrUnavailable
	}
	// Explicit authenticated initial LIVE0 preserves the first P3 FROZEN1.
	// Space proves existing Space plus absent lifecycle history under its fence;
	// missing state or owner/storage failure cannot manufacture initial LIVE.
	if life.Generation == 0 && life.State != spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE {
		return out, ErrUnavailable
	}
	if life.State < spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE || life.State > spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_RETIRED || owner.State < spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE || owner.State > spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_RETIRED {
		return out, ErrUnavailable
	}
	for _, state := range []spacev1.MemberAdmissionSourceState{ordinary.State, community.State} {
		if state < spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE || state > spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED {
			return out, ErrUnavailable
		}
	}
	out.space, out.profile, out.revision = req.SpaceId, req.ProfileId, reply.SpaceAuthorityRevision
	out.owner = owner.CurrentOwnerProfileId
	out.lifecycleGeneration, out.ownershipGeneration = life.Generation, owner.Generation
	out.active = life.State == spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE && owner.State == spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE
	if out.chat != "" && !out.treePresent {
		out.active = false
	}
	if ordinary.State == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE {
		binding, err := memberbinding.FromProto(ordinary.PublishedBinding)
		if err != nil || binding.Ordinary == nil || binding.Ordinary.Legacy != ordinary.ExplicitLegacy || binding.Ordinary.MembershipGeneration != ordinary.HeadGeneration {
			return Observation{}, ErrUnavailable
		}
		if !binding.Ordinary.Legacy && binding.Ordinary.MembershipGeneration <= ordinary.RetiredThrough {
			return Observation{}, ErrUnavailable
		}
		switch ordinary.PublicationKind {
		case spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_ADMISSION_ROLE_READY:
			if binding.Ordinary.Legacy || len(ordinary.RoleReceiptSha256) != 32 || ordinary.NativePublication != nil {
				return Observation{}, ErrUnavailable
			}
			out.admissionReady = true
		case spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_LEGACY_MIGRATED:
			if !binding.Ordinary.Legacy || len(ordinary.RoleReceiptSha256) != 0 || ordinary.RetiredThrough != 0 || ordinary.NativePublication != nil {
				return Observation{}, ErrUnavailable
			}
		case spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE,
			spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE,
			spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY,
			spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL:
			proof := ordinary.NativePublication
			if binding.Ordinary.Legacy || len(ordinary.RoleReceiptSha256) != 0 || proof == nil || proof.Kind != ordinary.PublicationKind || proof.OperationId != binding.Ordinary.PublicationIdentity || len(proof.BindingSha256) != 32 {
				return Observation{}, ErrUnavailable
			}
			out.nativeProofDigest = bytes.Clone(proof.BindingSha256)
		default:
			return Observation{}, ErrUnavailable
		}
		out.ordinaryPublicationKind = ordinary.PublicationKind
		out.ordinary = &binding
		out.receipt = bytes.Clone(ordinary.RoleReceiptSha256)
	}
	if ordinary.State == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_PREPARING {
		if (ordinary.PublicationKind != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_UNSPECIFIED && ordinary.PublicationKind != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_ADMISSION_ROLE_READY) || ordinary.NativePublication != nil || len(ordinary.RoleReceiptSha256) != 0 {
			return Observation{}, ErrUnavailable
		}
		binding, err := memberbinding.FromProto(ordinary.PreparingBinding)
		if err != nil || binding.Ordinary == nil || binding.Ordinary.Legacy || binding.Ordinary.MembershipGeneration != ordinary.HeadGeneration || binding.Ordinary.MembershipGeneration <= ordinary.RetiredThrough || len(ordinary.PreparingAdmissionBindingSha256) != 32 {
			return Observation{}, ErrUnavailable
		}
		out.preparing = &binding
		out.preparingDigest = bytes.Clone(ordinary.PreparingAdmissionBindingSha256)
	}
	if community.State == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_PREPARING {
		return Observation{}, ErrUnavailable
	}
	if community.State == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE {
		binding, err := memberbinding.FromProto(community.Binding)
		if err != nil || binding.Community == nil || binding.Community.MembershipGeneration <= community.RetiredThrough || !positive(community.RosterSourceRevision) || community.ValidityRemaining == nil || community.ValidityRemaining.CheckValid() != nil {
			return Observation{}, ErrUnavailable
		}
		lease := community.ValidityRemaining.AsDuration()
		if lease <= 0 {
			return Observation{}, ErrUnavailable
		}
		if lease < validity {
			validity = lease
		}
		out.community = &binding
	}
	out.deadline = started.Add(validity)
	if !now.Before(out.deadline) {
		return Observation{}, ErrUnavailable
	}
	return out, nil
}

func (o Observation) SpaceID() string              { return o.space }
func (o Observation) ProfileID() string            { return o.profile }
func (o Observation) Revision() uint64             { return o.revision }
func (o Observation) LifecycleGeneration() uint64  { return o.lifecycleGeneration }
func (o Observation) OwnershipGeneration() uint64  { return o.ownershipGeneration }
func (o Observation) ChatID() string               { return o.chat }
func (o Observation) TreeRelationRevision() uint64 { return o.treeRevision }
func (o Observation) BoundToChat(chat string, now time.Time) bool {
	return o.Available(now) && o.chat != "" && o.chat == chat && o.treePresent
}
func (o Observation) SameSnapshot(other Observation, now time.Time) bool {
	if !o.Available(now) || !other.Available(now) {
		return false
	}
	o.deadline, other.deadline = time.Time{}, time.Time{}
	return reflect.DeepEqual(o, other)
}
func (o Observation) Available(now time.Time) bool { return o.active && now.Before(o.deadline) }
func (o Observation) IsOwner(profile string, now time.Time) bool {
	return o.Available(now) && canonical(profile) && profile == o.owner
}

// Bindings enumerates the current exact source selectors in deterministic
// ordinary/community order. Copies prevent a consumer from changing a saved
// observation; PREPARING and retired sources were never admitted to this set.
func (o Observation) Bindings(now time.Time) ([]memberadmission.MembershipBinding, error) {
	if !o.Available(now) {
		return nil, ErrUnavailable
	}
	var out []memberadmission.MembershipBinding
	if o.ordinary != nil {
		ordinary := *o.ordinary.Ordinary
		out = append(out, memberadmission.MembershipBinding{Ordinary: &ordinary})
	}
	if o.community != nil {
		community := *o.community.Community
		out = append(out, memberadmission.MembershipBinding{Community: &community})
	}
	return out, nil
}

func (o Observation) Eligible(binding memberadmission.MembershipBinding, receipt []byte, now time.Time) bool {
	if !o.BindingEligible(binding, now) {
		return false
	}
	if binding.Ordinary != nil {
		return o.admissionReady && bytes.Equal(receipt, o.receipt)
	}
	return true
}

// BindingEligible qualifies interactive assignments/grants by exact incarnation.
// Admission-default contributions additionally use Eligible to bind their saved
// READY receipt digest; an interactive assignment is a separate origin.
func (o Observation) BindingEligible(binding memberadmission.MembershipBinding, now time.Time) bool {
	if !o.Available(now) || memberadmission.ValidateMembershipBinding(binding) != nil {
		return false
	}
	if binding.Ordinary != nil {
		return o.ordinary != nil && reflect.DeepEqual(binding, *o.ordinary)
	}
	return o.community != nil && reflect.DeepEqual(binding, *o.community)
}

func (o Observation) CanPrepare(generation uint64, operation string, digest []byte, now time.Time) bool {
	if !o.Available(now) || !positive(generation) || !canonical(operation) || len(digest) != 32 {
		return false
	}
	if o.preparing != nil {
		return o.preparing.Ordinary.MembershipGeneration == generation && o.preparing.Ordinary.PublicationIdentity == operation && bytes.Equal(digest, o.preparingDigest)
	}
	return false
}

type observationsKey struct{}

func WithObservations(ctx context.Context, values ...Observation) (context.Context, error) {
	entries := make(map[string]Observation, len(values))
	revisions := make(map[string]uint64)
	for _, value := range values {
		if value.space == "" || value.profile == "" || value.revision == 0 || value.deadline.IsZero() {
			return nil, ErrUnavailable
		}
		if revision, ok := revisions[value.space]; ok && revision != value.revision {
			return nil, ErrUnavailable
		}
		revisions[value.space] = value.revision
		key := value.space + "/" + value.profile + "/" + value.chat
		if _, duplicate := entries[key]; duplicate {
			return nil, ErrUnavailable
		}
		entries[key] = value
	}
	return context.WithValue(ctx, observationsKey{}, entries), nil
}

func FromContext(ctx context.Context, space, profile string) (Observation, bool) {
	values, _ := ctx.Value(observationsKey{}).(map[string]Observation)
	value, ok := values[space+"/"+profile+"/"]
	return value, ok
}

// FromChatContext never substitutes unscoped or another chat's evidence.
func FromChatContext(ctx context.Context, space, profile, chat string) (Observation, bool) {
	if !canonical(chat) {
		return Observation{}, false
	}
	values, _ := ctx.Value(observationsKey{}).(map[string]Observation)
	value, ok := values[space+"/"+profile+"/"+chat]
	return value, ok
}

func unknown(message proto.Message) bool {
	if message == nil {
		return true
	}
	m := message.ProtoReflect()
	if !m.IsValid() || len(m.GetUnknown()) != 0 {
		return true
	}
	bad := false
	m.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Kind() != protoreflect.MessageKind {
			return true
		}
		if field.IsList() {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if unknown(list.Get(i).Message().Interface()) {
					bad = true
					return false
				}
			}
		} else if !field.IsMap() && unknown(value.Message().Interface()) {
			bad = true
			return false
		}
		return true
	})
	return bad
}
