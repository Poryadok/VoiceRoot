// Package authprofileauthority validates Auth's finite User authority observations.
package authprofileauthority

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"math"
	"time"
	userv1 "voice.app/voice/user/v1"
)

var ErrContract = errors.New("invalid Auth profile authority contract")

func canonical(s string) bool {
	id, e := uuid.Parse(s)
	return e == nil && id != uuid.Nil && id.String() == s
}
func positive(n uint64) bool { return n > 0 && n <= math.MaxInt64 }
func clean(m proto.Message) bool {
	if m == nil || !m.ProtoReflect().IsValid() {
		return false
	}
	r := m.ProtoReflect()
	if len(r.GetUnknown()) != 0 {
		return false
	}
	ok := true
	r.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if f.Kind() == protoreflect.MessageKind {
			if f.IsList() {
				l := v.List()
				for i := 0; i < l.Len(); i++ {
					if !clean(l.Get(i).Message().Interface()) {
						ok = false
						break
					}
				}
			} else if !clean(v.Message().Interface()) {
				ok = false
			}
		}
		return ok
	})
	return ok
}
func pin(p *userv1.AuthProfileSourcePin) bool {
	return p != nil && clean(p) && canonical(p.UserSourceGeneration) && canonical(p.HistoryGeneration) && positive(p.KeysetRevision)
}
func Digest(m proto.Message) []byte {
	b, e := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if e != nil {
		return nil
	}
	prefix := append([]byte(m.ProtoReflect().Descriptor().FullName()), 0)
	d := sha256.Sum256(append(prefix, b...))
	return d[:]
}

func ValidateMatchRequest(r *userv1.MatchAuthProfileReferencesRequest) error {
	if !clean(r) || r.ProtocolVersion != 1 || !canonical(r.RequestId) || !canonical(r.InventoryId) || !positive(r.InventoryRevision) || !canonical(r.AccountId) || !canonical(r.OwnerHandle) || !canonical(r.DeletionCycleId) || len(r.ProfileIds) == 0 || len(r.ProfileIds) > 256 || (r.ExpectedSource != nil && !pin(r.ExpectedSource)) {
		return ErrContract
	}
	for i, id := range r.ProfileIds {
		if !canonical(id) || (i > 0 && r.ProfileIds[i-1] >= id) {
			return ErrContract
		}
	}
	return nil
}
func ValidateTargetRequest(r *userv1.ResolveAuthOwnershipTargetRequest) error {
	if !clean(r) || r.ProtocolVersion != 1 || !canonical(r.RequestId) || !canonical(r.OperationId) || !canonical(r.ProfileId) || (r.ExpectedSource != nil && !pin(r.ExpectedSource)) {
		return ErrContract
	}
	return nil
}
func validateCut(c *userv1.AuthProfileSourceCut, expected *userv1.AuthProfileSourcePin, now time.Time) bool {
	if c == nil || !clean(c) || !pin(c.Source) || !positive(c.UserAuthorityRevision) || c.ThroughRevision > math.MaxInt64 || len(c.HeadDigest) != 32 || c.RetainedFloor > c.ThroughRevision || c.VerifiedErasureCovered != c.UnresolvedErasureIssued || c.UnresolvedErasureIssued > math.MaxInt64 || c.ObservedAt == nil || c.ObservedAt.CheckValid() != nil {
		return false
	}
	observed := c.ObservedAt.AsTime()
	return (expected == nil || proto.Equal(expected, c.Source)) && !observed.After(now.Add(time.Second)) && now.Before(observed.Add(2*time.Second))
}
func ValidateMatchResponse(r *userv1.MatchAuthProfileReferencesRequest, o *userv1.MatchAuthProfileReferencesResponse, now time.Time) error {
	if ValidateMatchRequest(r) != nil || !clean(o) || o.ProtocolVersion != 1 || o.RequestId != r.RequestId || o.InventoryId != r.InventoryId || o.InventoryRevision != r.InventoryRevision || o.AccountId != r.AccountId || o.OwnerHandle != r.OwnerHandle || o.DeletionCycleId != r.DeletionCycleId || !bytes.Equal(o.RequestDigest, Digest(r)) || len(o.Results) != len(r.ProfileIds) || !validateCut(o.SourceCut, r.ExpectedSource, now) {
		return ErrContract
	}
	resolved := true
	for i, v := range o.Results {
		if v == nil || v.ProfileId != r.ProfileIds[i] {
			return ErrContract
		}
		switch v.Outcome {
		case userv1.AuthProfileMatchOutcome_AUTH_PROFILE_MATCH_OUTCOME_INCOMPLETE:
			resolved = false
			if v.AssociationId != "" || v.AssociationRevision != 0 || v.CaptureOrigin != "" {
				return ErrContract
			}
		case userv1.AuthProfileMatchOutcome_AUTH_PROFILE_MATCH_OUTCOME_MATCH, userv1.AuthProfileMatchOutcome_AUTH_PROFILE_MATCH_OUTCOME_DIFFERENT_ACCOUNT:
			if !canonical(v.AssociationId) || !positive(v.AssociationRevision) || (v.CaptureOrigin != "BIRTH" && v.CaptureOrigin != "RETAINED_NOW" && v.CaptureOrigin != "VERIFIED_HISTORY") {
				return ErrContract
			}
		default:
			return ErrContract
		}
	}
	if resolved != o.AllRequestedResolved {
		return ErrContract
	}
	return nil
}
func ValidateTargetResponse(r *userv1.ResolveAuthOwnershipTargetRequest, o *userv1.ResolveAuthOwnershipTargetResponse, now time.Time) error {
	if ValidateTargetRequest(r) != nil || !clean(o) || o.ProtocolVersion != 1 || o.RequestId != r.RequestId || o.OperationId != r.OperationId || o.ProfileId != r.ProfileId || !bytes.Equal(o.RequestDigest, Digest(r)) || !validateCut(o.SourceCut, r.ExpectedSource, now) {
		return ErrContract
	}
	noTerminal := o.TerminalOperationId == "" && o.TerminalRevision == 0 && o.TerminalReceiptId == "" && len(o.TerminalReceiptDigest) == 0
	noCurrent := o.AccountId == "" && o.ProfileRevision == 0 && !o.Deleted && !o.Frozen && !o.AccountInactive
	switch o.Outcome {
	case userv1.AuthOwnershipTargetOutcome_AUTH_OWNERSHIP_TARGET_OUTCOME_CURRENT_PROFILE:
		if !canonical(o.AccountId) || !positive(o.ProfileRevision) || !canonical(o.AssociationId) || !positive(o.AssociationRevision) || !noTerminal {
			return ErrContract
		}
	case userv1.AuthOwnershipTargetOutcome_AUTH_OWNERSHIP_TARGET_OUTCOME_TERMINAL_PROFILE:
		if !noCurrent || !canonical(o.AssociationId) || !positive(o.AssociationRevision) || !canonical(o.TerminalOperationId) || !positive(o.TerminalRevision) || !canonical(o.TerminalReceiptId) || len(o.TerminalReceiptDigest) != 32 {
			return ErrContract
		}
	case userv1.AuthOwnershipTargetOutcome_AUTH_OWNERSHIP_TARGET_OUTCOME_INCOMPLETE:
		if !noCurrent || !noTerminal || o.AssociationId != "" || o.AssociationRevision != 0 {
			return ErrContract
		}
	default:
		return ErrContract
	}
	return nil
}
