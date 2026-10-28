package inventorygrpc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/referenceinventory"
)

func identity() *commonv1.ReferenceInventoryIdentity {
	return &commonv1.ReferenceInventoryIdentity{ProtocolVersion: 1, WriterProtocolVersion: 1, SourceService: "story", SourceComponent: "story.media.v1", InstallationId: uuid.NewString(), RestoreCohortId: uuid.NewString(), MigrationId: uuid.NewString(), FileIncarnationId: uuid.NewString(), OwnerIncarnationId: uuid.NewString(), SnapshotId: uuid.NewString()}
}

func TestInventoryPageAndChangesValidateExactStoreBinding(t *testing.T) {
	id := identity()
	page := &commonv1.ReferenceInventoryPage{Identity: id, BoundaryRevision: 5, PageIndex: 3, Entries: []*commonv1.ReferenceInventoryEntry{{Key: &commonv1.ReferenceInventoryKey{FileId: uuid.NewString(), OwnerId: uuid.NewString(), OwnerType: commonv1.ReferenceInventoryOwnerType_REFERENCE_INVENTORY_OWNER_TYPE_STORY}, OwnerRevision: 5, Reasons: []commonv1.ReferenceInventoryProtectionReason{commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_RETAINED}, MediaType: "photo"}}}
	changes := &commonv1.ReadReferenceInventoryChangesResponse{Identity: id, HighWatermark: 5, RetainedFloor: 0, Revisions: []*commonv1.ReferenceInventoryRevision{{Revision: 5, FinalCount: 1, Entries: page.Entries}}}
	st := &memoryStore{page: page, changes: changes}
	svc := New(st)
	pageReq := &commonv1.ReadReferenceInventoryPageRequest{Identity: id, PageIndex: 3}
	got, err := svc.ReadReferenceInventoryPage(fileContext(PageMethod, pageReq), pageReq)
	if err != nil || !proto.Equal(got.GetPage(), page) {
		t.Fatalf("page: %v %v", got, err)
	}
	changesReq := &commonv1.ReadReferenceInventoryChangesRequest{Identity: id, AfterRevision: 4, Limit: 1000}
	actual, err := svc.ReadReferenceInventoryChanges(fileContext(ChangesMethod, changesReq), changesReq)
	if err != nil || !proto.Equal(actual, changes) {
		t.Fatalf("changes: %v %v", actual, err)
	}
	st.page = proto.Clone(page).(*commonv1.ReferenceInventoryPage)
	st.page.PageIndex = 2
	if _, err := svc.ReadReferenceInventoryPage(fileContext(PageMethod, pageReq), pageReq); status.Code(err) != codes.Unavailable {
		t.Fatalf("substituted page: %v", err)
	}
	st.changes = proto.Clone(changes).(*commonv1.ReadReferenceInventoryChangesResponse)
	st.changes.Revisions[0].FinalCount = 2
	if _, err := svc.ReadReferenceInventoryChanges(fileContext(ChangesMethod, changesReq), changesReq); status.Code(err) != codes.Unavailable {
		t.Fatalf("incomplete revision: %v", err)
	}
	st.changes = proto.Clone(changes).(*commonv1.ReadReferenceInventoryChangesResponse)
	st.changes.Identity.SnapshotId = uuid.NewString()
	if _, err := svc.ReadReferenceInventoryChanges(fileContext(ChangesMethod, changesReq), changesReq); status.Code(err) != codes.Unavailable {
		t.Fatalf("mixed snapshot: %v", err)
	}
}

func TestInventoryInvalidInputAndPrincipalNeverReachStore(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*commonv1.ReadReferenceInventoryChangesRequest)
	}{
		{"missing identity", func(r *commonv1.ReadReferenceInventoryChangesRequest) { r.Identity = nil }},
		{"wrong source", func(r *commonv1.ReadReferenceInventoryChangesRequest) { r.Identity.SourceService = "user" }},
		{"zero limit", func(r *commonv1.ReadReferenceInventoryChangesRequest) { r.Limit = 0 }},
		{"large limit", func(r *commonv1.ReadReferenceInventoryChangesRequest) { r.Limit = 1001 }},
		{"unknown", func(r *commonv1.ReadReferenceInventoryChangesRequest) {
			r.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 1})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &memoryStore{}
			req := &commonv1.ReadReferenceInventoryChangesRequest{Identity: identity(), Limit: 1}
			tc.mutate(req)
			_, err := New(st).ReadReferenceInventoryChanges(fileContext(ChangesMethod, req), req)
			if status.Code(err) != codes.InvalidArgument || st.calls != 0 {
				t.Fatalf("%v calls %d", err, st.calls)
			}
		})
	}
	req := &commonv1.BeginReferenceInventoryRequest{Identity: identity(), OperationId: uuid.NewString()}
	for _, mutate := range []func(*principal.Principal){
		func(p *principal.Principal) { p.Issuer = "story" }, func(p *principal.Principal) { p.Subject = "service:gateway" }, func(p *principal.Principal) { p.Kind = "delegated_user" }, func(p *principal.Principal) { p.ProfileID = uuid.NewString() }, func(p *principal.Principal) { p.AccountID = uuid.NewString() }, func(p *principal.Principal) { p.ExpiresAt = time.Now().Add(-time.Second) },
	} {
		p, _ := principal.FromContext(fileContext(BeginMethod, req))
		mutate(&p)
		st := &memoryStore{}
		_, err := New(st).BeginReferenceInventory(principal.WithVerified(context.Background(), p), req)
		if status.Code(err) != codes.Unauthenticated || st.calls != 0 {
			t.Fatalf("principal accepted %v calls %d", err, st.calls)
		}
	}
}

func TestInventoryStoreErrorsAreFixedAndNonSensitive(t *testing.T) {
	req := &commonv1.BeginReferenceInventoryRequest{Identity: identity(), OperationId: uuid.NewString()}
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{{referenceinventory.ErrInvalid, codes.InvalidArgument}, {fmt.Errorf("binding: %w", referenceinventory.ErrConflict), codes.FailedPrecondition}, {referenceinventory.ErrUnavailable, codes.Unavailable}, {errors.New("secret SQL details"), codes.Unavailable}, {context.Canceled, codes.Canceled}, {context.DeadlineExceeded, codes.DeadlineExceeded}} {
		_, err := New(&memoryStore{err: tc.err}).BeginReferenceInventory(fileContext(BeginMethod, req), req)
		if status.Code(err) != tc.code || status.Convert(err).Message() == "secret SQL details" {
			t.Fatalf("%v mapped %v", tc.err, err)
		}
	}
}

type memoryStore struct {
	calls      int
	descriptor *commonv1.ReferenceInventoryDescriptor
	page       *commonv1.ReferenceInventoryPage
	changes    *commonv1.ReadReferenceInventoryChangesResponse
	err        error
}

func (s *memoryStore) BeginReferenceInventory(context.Context, *commonv1.BeginReferenceInventoryRequest) (*commonv1.ReferenceInventoryDescriptor, error) {
	s.calls++
	return s.descriptor, s.err
}
func (s *memoryStore) ReadReferenceInventoryPage(context.Context, *commonv1.ReferenceInventoryIdentity, uint32) (*commonv1.ReferenceInventoryPage, error) {
	s.calls++
	return s.page, s.err
}
func (s *memoryStore) ReadReferenceInventoryChanges(context.Context, *commonv1.ReferenceInventoryIdentity, uint64, uint32) (*commonv1.ReadReferenceInventoryChangesResponse, error) {
	s.calls++
	return s.changes, s.err
}

func fileContext(method string, req proto.Message) context.Context {
	hash, _ := principal.RequestHash(req)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "file", Subject: "service:file", Audience: "story", RPC: method, RequestID: uuid.NewString(), RequestHash: hash, ExpiresAt: time.Now().Add(time.Minute)})
}

func TestInventoryBeginRequiresFileAndReturnsExactDescriptor(t *testing.T) {
	id := identity()
	descriptor, err := referenceinventory.Describe(nil, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	st := &memoryStore{descriptor: descriptor}
	svc := New(st)
	req := &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id}
	if _, err := svc.BeginReferenceInventory(context.Background(), req); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unverified: %v", err)
	}
	if st.calls != 0 {
		t.Fatal("unverified request reached Store")
	}
	got, err := svc.BeginReferenceInventory(fileContext(BeginMethod, req), req)
	if err != nil || !proto.Equal(got.GetDescriptor_(), descriptor) {
		t.Fatalf("descriptor: %v %v", got, err)
	}
	if st.calls != 1 {
		t.Fatalf("calls %d", st.calls)
	}
}

func TestInventoryBeginRejectsAnotherRegisteredOwner(t *testing.T) {
	id := identity()
	id.SourceService, id.SourceComponent = "messaging", "messaging.message.v1"
	descriptor, err := referenceinventory.Describe(nil, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	st := &memoryStore{descriptor: descriptor}
	req := &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id}
	_, err = New(st).BeginReferenceInventory(fileContext(BeginMethod, req), req)
	if status.Code(err) != codes.InvalidArgument || st.calls != 0 {
		t.Fatalf("Messaging identity reached Story store: %v calls=%d", err, st.calls)
	}
}

func TestInventoryRejectsNestedUnknownAndCannotSeal(t *testing.T) {
	id := identity()
	st := &memoryStore{}
	svc := New(st)
	id.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 1})
	req := &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id}
	_, err := svc.BeginReferenceInventory(fileContext(BeginMethod, req), req)
	if status.Code(err) != codes.InvalidArgument || st.calls != 0 {
		t.Fatalf("unknown reached Store: %v calls=%d", err, st.calls)
	}
	id.ProtoReflect().SetUnknown(nil)
	seal := &commonv1.SealReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id}
	_, err = svc.SealReferenceInventory(fileContext(SealMethod, seal), seal)
	if status.Code(err) != codes.FailedPrecondition || st.calls != 0 {
		t.Fatalf("unproven seal: %v calls=%d", err, st.calls)
	}
}
