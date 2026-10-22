// Package spaceaccountcapture validates the private, complete User association
// capture wire contract shared by the User producer and Space consumer.
package spaceaccountcapture

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	userv1 "voice.app/voice/user/v1"
)

var ErrContract = errors.New("invalid Space account capture contract")

func encoded(m proto.Message) []byte {
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	return b
}

// CohortDigest binds the complete ordered manifest including expectations.
// ValidateRequest must also succeed before the digest carries any authority.
func CohortDigest(c *userv1.SpaceAccountCaptureCohort) []byte {
	h := sha256.New()
	h.Write([]byte("voice.space.account-binding.v1\x00"))
	h.Write(encoded(c))
	return h.Sum(nil)
}

// RequestDigest is the binary form of the ordinary deterministic protobuf
// request hash used by the signed principal. The transport binds request_id too.
func RequestDigest(r *userv1.CaptureSpaceMemberAccountBindingsRequest) []byte {
	d := sha256.Sum256(encoded(r))
	return d[:]
}

func validID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}

func positive(n uint64) bool { return n > 0 && n <= math.MaxInt64 }

func unknown(m protoreflect.Message) bool {
	if !m.IsValid() || len(m.GetUnknown()) != 0 {
		return true
	}
	found := false
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if f.IsList() && f.Kind() == protoreflect.MessageKind {
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				if unknown(list.Get(i).Message()) {
					found = true
					return false
				}
			}
		} else if f.Kind() == protoreflect.MessageKind && unknown(v.Message()) {
			found = true
			return false
		}
		return true
	})
	return found
}

func validEntry(e *userv1.SpaceAccountCaptureEntry) bool {
	if e == nil || !validID(e.ProfileId) || (e.ExpectedAssociationId == "") != (e.ExpectedAssociationRevision == 0) {
		return false
	}
	if e.ExpectedAssociationId != "" && (!validID(e.ExpectedAssociationId) || !positive(e.ExpectedAssociationRevision)) {
		return false
	}
	switch s := e.Source.(type) {
	case *userv1.SpaceAccountCaptureEntry_Ordinary:
		return s.Ordinary != nil && s.Ordinary.Generation <= math.MaxInt64 && validID(s.Ordinary.PublicationId)
	case *userv1.SpaceAccountCaptureEntry_Community:
		return s.Community != nil && positive(s.Community.OwnerGeneration) && positive(s.Community.MemberGeneration) && validID(s.Community.EpisodeId)
	case *userv1.SpaceAccountCaptureEntry_ProspectiveCommunity:
		return s.ProspectiveCommunity != nil && positive(s.ProspectiveCommunity.OwnerGeneration) && positive(s.ProspectiveCommunity.SourceRevision) && validID(s.ProspectiveCommunity.OperationId)
	}
	return false
}

// ValidateRequest rejects unknown fields, noncanonical order and duplicate
// source episodes before any owner lookup or receipt write. Empty manifests
// are valid: they describe an actual empty cohort, never an omitted page.
func ValidateRequest(r *userv1.CaptureSpaceMemberAccountBindingsRequest) error {
	if r == nil || r.Protocol != 1 || !validID(r.OperationId) || !validID(r.SpaceId) || r.Cohort == nil || unknown(r.ProtoReflect()) {
		return ErrContract
	}
	if r.Purpose < 1 || r.Purpose > 3 || len(r.CohortDigest) != sha256.Size || !bytes.Equal(r.CohortDigest, CohortDigest(r.Cohort)) {
		return ErrContract
	}
	seen := make(map[string]bool, len(r.Cohort.Entries))
	var previous []byte
	for _, entry := range r.Cohort.Entries {
		if !validEntry(entry) {
			return ErrContract
		}
		body := encoded(entry)
		if previous != nil && bytes.Compare(previous, body) >= 0 {
			return ErrContract
		}
		previous = body
		source := proto.Clone(entry).(*userv1.SpaceAccountCaptureEntry)
		source.ExpectedAssociationId, source.ExpectedAssociationRevision = "", 0
		key := string(encoded(source))
		if seen[key] {
			return ErrContract
		}
		seen[key] = true
	}
	return nil
}

func terminalEmpty(r *userv1.SpaceAccountCaptureResult) bool {
	return r.TerminalOperationId == "" && r.TerminalRevision == 0 && r.TerminalReceiptId == "" && len(r.TerminalReceiptDigest) == 0
}

func associationEmpty(r *userv1.SpaceAccountCaptureResult) bool {
	return r.AccountId == "" && r.AssociationId == "" && r.AssociationRevision == 0 && r.ProfileRevision == 0 && !r.Deleted && !r.Frozen && !r.AccountInactive && r.Origin == 0 && r.AssociationOrigin == 0
}

func associated(r *userv1.SpaceAccountCaptureResult) bool {
	return validID(r.AccountId) && validID(r.AssociationId) && positive(r.AssociationRevision) && positive(r.ProfileRevision) && r.Origin >= 1 && r.Origin <= 2 && r.AssociationOrigin >= 1 && r.AssociationOrigin <= 3
}

// ValidateResponse proves exact request/cohort coverage and structural owner
// evidence. It cannot manufacture or independently authenticate an owner fact;
// the caller must obtain it over the protected verified transport.
func ValidateResponse(req *userv1.CaptureSpaceMemberAccountBindingsRequest, out *userv1.CaptureSpaceMemberAccountBindingsResponse) error {
	if ValidateRequest(req) != nil || out == nil || unknown(out.ProtoReflect()) || out.Protocol != req.Protocol || out.OperationId != req.OperationId || out.SpaceId != req.SpaceId || out.Purpose != req.Purpose || !bytes.Equal(out.CohortDigest, req.CohortDigest) || !bytes.Equal(out.RequestDigest, RequestDigest(req)) || !positive(out.OwnerSourceRevision) || len(out.Results) != len(req.Cohort.Entries) {
		return ErrContract
	}
	cut := out.IdentityCut
	if !validID(out.UserSourceGeneration) || cut == nil || cut.Owner != "user" || cut.Stream != "identity-association" || !validID(cut.HistoryGeneration) || !positive(cut.ThroughRevision) || !positive(cut.KeysetRevision) || len(cut.Digest) != sha256.Size || cut.RetainedFloor > cut.ThroughRevision {
		return ErrContract
	}
	associations := make(map[string]*userv1.SpaceAccountCaptureResult)
	for i, r := range out.Results {
		if r == nil || !proto.Equal(r.Entry, req.Cohort.Entries[i]) {
			return ErrContract
		}
		if r.Outcome != userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE && (!validID(r.OwnerReceiptId) || len(r.OwnerReceiptDigest) != sha256.Size || r.CaptureCut == nil || r.CaptureCut.CheckValid() != nil) {
			return ErrContract
		}
		if r.AssociationRevision > cut.ThroughRevision {
			return ErrContract
		}
		switch r.Outcome {
		case userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED:
			if !associated(r) || !terminalEmpty(r) {
				return ErrContract
			}
		case userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_TERMINAL_PROFILE:
			if !validID(r.TerminalOperationId) || !positive(r.TerminalRevision) || !validID(r.TerminalReceiptId) || len(r.TerminalReceiptDigest) != sha256.Size {
				return ErrContract
			}
			if !associationEmpty(r) && !associated(r) {
				return ErrContract
			}
		case userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE:
			if !associationEmpty(r) || !terminalEmpty(r) || r.OwnerReceiptId != "" || len(r.OwnerReceiptDigest) != 0 || r.CaptureCut != nil {
				return ErrContract
			}
		default:
			return ErrContract
		}
		if r.Entry.ExpectedAssociationId != "" && r.Outcome != userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE && (r.AssociationId != r.Entry.ExpectedAssociationId || r.AssociationRevision != r.Entry.ExpectedAssociationRevision) {
			return ErrContract
		}
		if r.AssociationId != "" {
			if previous, ok := associations[r.Entry.ProfileId]; ok && (previous.AccountId != r.AccountId || previous.AssociationId != r.AssociationId || previous.AssociationRevision != r.AssociationRevision) {
				return ErrContract
			}
			associations[r.Entry.ProfileId] = r
		}
	}
	return nil
}
