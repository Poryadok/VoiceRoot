package catalogassets

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type reconcileStoreFixture struct {
	rows            []Binding
	retired, purged []Binding
}

func (f *reconcileStoreFixture) ScanBindings(context.Context, uuid.UUID, int) ([]Binding, error) {
	return f.rows, nil
}
func (f *reconcileStoreFixture) RetireExactBinding(_ context.Context, b Binding) error {
	f.retired = append(f.retired, b)
	return nil
}
func (f *reconcileStoreFixture) PurgeCapturedProfile(_ context.Context, b Binding) error {
	f.purged = append(f.purged, b)
	return nil
}

type reconcileProfilesFixture map[Actor]ProfileState

func (f reconcileProfilesFixture) ProfileStates(context.Context, []Actor) (map[Actor]ProfileState, error) {
	return f, nil
}

type reconcileFilesFixture struct {
	FileClient
	state FileState
	err   error
	calls int
}

func (f *reconcileFilesFixture) Status(context.Context, Binding) (FileState, error) {
	f.calls++
	return f.state, f.err
}

func TestCatalogReconciliationRequiresExactLifecycleEvidence(t *testing.T) {
	b := Binding{Actor: Actor{AccountID: uuid.New(), ProfileID: uuid.New()}, ActorCapture: ActorCapture{AccountGeneration: uuid.New(), ProfileGeneration: uuid.New()}, Selection: Selection{Provider: "fixture", ProviderID: "one"}, SourceID: uuid.New(), FileID: uuid.New()}
	for _, tc := range []struct {
		name            string
		profile         ProfileState
		fileStatus      string
		fence           bool
		err             error
		forged          bool
		retired, purged int
	}{
		{name: "unknown owner is not purge", profile: ProfileUnknown},
		{name: "explicit deleted owner", profile: ProfileSoftDeleted, purged: 1},
		{name: "ready projection remains", profile: ProfileLive, fileStatus: "ready"},
		{name: "exact terminal active projection", profile: ProfileLive, fileStatus: "failed", retired: 1},
		{name: "qualified compact fence", profile: ProfileLive, fileStatus: "deleted", fence: true, retired: 1},
		{name: "not found is not proof", profile: ProfileLive, err: status.Error(codes.NotFound, "absent")},
		{name: "unavailable is not proof", profile: ProfileLive, err: status.Error(codes.Unavailable, "offline")},
		{name: "forged exact tuple", profile: ProfileLive, fileStatus: "deleted", fence: true, forged: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &reconcileStoreFixture{rows: []Binding{b}}
			files := &reconcileFilesFixture{state: FileState{Actor: b.Actor, ActorCapture: b.ActorCapture, Selection: b.Selection, SourceID: b.SourceID, FileID: b.FileID, Status: tc.fileStatus}, err: tc.err}
			if tc.fence {
				files.state.ActorCapture = ActorCapture{}
				files.state.TerminalFenceID = uuid.New()
			}
			if tc.forged {
				files.state.SourceID = uuid.New()
			}
			r := &Reconciler{Store: store, Profiles: reconcileProfilesFixture{b.Actor: tc.profile}, Files: files}
			next, _ := r.Step(context.Background(), uuid.Nil, 100)
			require.Equal(t, b.FileID, next)
			require.Len(t, store.retired, tc.retired)
			require.Len(t, store.purged, tc.purged)
			if tc.profile != ProfileLive {
				require.Zero(t, files.calls)
			}
		})
	}
}
