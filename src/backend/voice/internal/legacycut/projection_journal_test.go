package legacycut

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	rp "voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

// These installed test doubles exercise PG/Redis recovery mechanics only. They
// are not historical deployment inventory, native acceptance or custody proof.
type projectionAuthorityFixture struct {
	command  NativeProjectionCommand
	commands map[uuid.UUID]NativeProjectionCommand
	omit     bool
	scopeErr error
}

func (f *projectionAuthorityFixture) ResolveOwnership(_ context.Context, w rp.OwnerWork, obligations []Obligation) (OwnershipPlan, error) {
	p := OwnershipPlan{AdditionalObligations: []Obligation{}, Assignments: []OwnershipAssignment{}}
	for i, o := range obligations {
		if f.omit && i == 0 {
			continue
		}
		disposition := "DRAIN"
		if o.Retain {
			disposition = "RETAIN"
		}
		p.Assignments = append(p.Assignments, OwnershipAssignment{ObligationID: o.ID, ValueHash: o.ValueHash, Owner: w.Ref, OwnerWorkHash: workHash(w), TargetHash: f.command.Linkage.NativeTargetHash, Disposition: disposition})
	}
	return p, nil
}
func (*projectionAuthorityFixture) ReadAuthoritativeOwnerWork(context.Context, rp.OwnerWorkRef) (rp.OwnerWork, error) {
	return rp.OwnerWork{}, rp.ErrUnavailable
}
func (*projectionAuthorityFixture) ReadOwnerPartitionAcceptance(context.Context, rp.OwnerWorkRef, PartitionRef) (OwnerPartitionAcceptance, error) {
	return OwnerPartitionAcceptance{}, rp.ErrUnavailable
}
func (f *projectionAuthorityFixture) ResolveNativeCommand(_ context.Context, link ProjectionLinkage, _ ProjectionState) (NativeProjectionCommand, error) {
	if command, ok := f.commands[link.NativeCommandRef]; ok {
		return command, nil
	}
	return f.command, nil
}
func (f *projectionAuthorityFixture) ValidateResourceScope(context.Context, rp.CutBinding, rp.ProofRef, Endpoint) error {
	return f.scopeErr
}

func newProjectionJournalFixture(t *testing.T) (*ProjectionJournal, *projectionAuthorityFixture, *redis.Client, ProjectionLinkage) {
	t.Helper()
	addr := os.Getenv("VOICE_LEGACY_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("owned isolated Redis required")
	}
	o, inventory, _, cut := newOwnerFixture(t)
	proof := newProofFixture(t, &cut)
	inventory.inventory.Cut = cut
	o.Verifier = rp.TrustedVerifier{Journal: proof, Owner: o, Installation: proof, Resources: proof}
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	info, err := client.Info(ctx, "server").Result()
	require.NoError(t, err)
	identity, err := RedisInstanceIdentity(info)
	require.NoError(t, err)
	runID := ""
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, "run_id:") {
			runID = strings.TrimSpace(strings.TrimPrefix(line, "run_id:"))
		}
	}
	prefix := "legacy-journal-" + uuid.NewString() + ":"
	evidence := "legacy-evidence-" + uuid.NewString() + ":"
	inventory.inventory.Endpoints = []Endpoint{{ID: "historical", ResourceIdentity: identity, StorageLineageHash: rp.Hash("unit-lineage", []byte("not installed custody")), EvidencePrefix: evidence, Prefix: prefix}}
	o.Endpoints = RedisEndpoints{Clients: map[string]*redis.Client{"historical": client}}
	raw, err := json.Marshal(store.Call{RoomID: "room", ChatID: "unit"})
	require.NoError(t, err)
	require.NoError(t, client.Set(ctx, prefix+"call:room", raw, 0).Err())
	work, err := o.PrepareCutoverDrain(ctx, cut, uuid.New())
	require.NoError(t, err)
	link := ProjectionLinkage{NativeOwnerWorkRef: work, LegacyWorkRef: work, ResourceProofRef: proof.record.Ref, NativeTargetHash: rp.Hash("unit-target", []byte("legacy residual")), NativeCommandRef: uuid.New()}
	source := &projectionAuthorityFixture{command: NativeProjectionCommand{Linkage: link, ResourceID: "historical", MethodContractHash: ContractHash(), TargetHashes: []rp.Digest{link.NativeTargetHash}, Step: store.LegacyProjectionStep{ScopePrefix: prefix, EvidencePrefix: evidence, ExpectedRunID: runID, Transitions: []store.LegacyKeyTransition{{Key: prefix + "call:room", Before: store.LegacyKeyImage{Kind: "string", Value: string(raw)}, After: store.LegacyKeyImage{Kind: "none"}}}}}}
	return &ProjectionJournal{Owner: o, Authority: source, Clients: map[string]*redis.Client{"historical": client}}, source, client, link
}

func TestLegacyProjectionJournalRequiresTotalPartitionAndAcceptance(t *testing.T) {
	ctx := context.Background()
	j, source, client, link := newProjectionJournalFixture(t)
	source.omit = true
	_, err := j.RegisterPartition(ctx, link.LegacyWorkRef, link.ResourceProofRef, uuid.New())
	require.ErrorIs(t, err, rp.ErrConflict)
	source.omit = false
	partition, err := j.RegisterPartition(ctx, link.LegacyWorkRef, link.ResourceProofRef, uuid.New())
	require.NoError(t, err)
	link.PartitionRef = partition
	source.command.Linkage = link
	read, err := j.ReadPartition(ctx, partition)
	require.NoError(t, err, "native intake reads validated P before any owner ACK")
	require.Equal(t, link.LegacyWorkRef, read.LegacyWork)
	savedVerifier := j.Owner.Verifier
	j.Owner.Verifier = nil
	_, err = j.ReadPartition(ctx, partition)
	require.ErrorIs(t, err, rp.ErrUnavailable, "deserializing P is not proof-qualified intake")
	j.Owner.Verifier = savedVerifier
	source.omit = true
	_, err = j.ReadPartition(ctx, partition)
	require.ErrorIs(t, err, rp.ErrConflict, "every read rederives exhaustive ownership")
	source.omit = false
	_, err = j.PrepareProjection(ctx, link)
	require.ErrorIs(t, err, rp.ErrPending)
	require.EqualValues(t, 1, client.Exists(ctx, source.command.Step.Transitions[0].Key).Val())
	require.NoError(t, j.AcceptLegacyPartition(ctx, partition))
	ref, err := j.PrepareProjection(ctx, link)
	require.NoError(t, err)
	originalContract := source.command.MethodContractHash
	source.command.MethodContractHash = rp.Hash("wrong", []byte("unaccepted implementation"))
	_, err = j.ResolveProjectionStep(ctx, ref)
	require.ErrorIs(t, err, rp.ErrConflict)
	source.command.MethodContractHash = originalContract
	var receipt []byte
	require.NoError(t, j.Owner.Pool.QueryRow(ctx, `SELECT receipt_bytes FROM voice_legacy_projection_steps WHERE step_id=$1`, ref.ID).Scan(&receipt))
	require.Nil(t, receipt)
	require.EqualValues(t, 1, client.Exists(ctx, source.command.Step.Transitions[0].Key).Val(), "intent is durable before mutation")
	source.scopeErr = rp.ErrUnavailable
	_, err = j.ResumeProjection(ctx, link, ref)
	require.ErrorIs(t, err, rp.ErrUnavailable)
	source.scopeErr = nil
	// Redis succeeded but the caller lost its ACK before PG archival.
	physical, err := store.NewLegacyProjectionExecutor(client, j).Apply(ctx, ref)
	require.NoError(t, err)
	require.EqualValues(t, 0, client.Exists(ctx, source.command.Step.Transitions[0].Key).Val())
	_, err = j.ReadProjection(ctx, link, ref)
	require.ErrorIs(t, err, rp.ErrPending)
	progress, err := j.ResumeProjection(ctx, link, ref)
	require.NoError(t, err)
	require.Equal(t, physical, progress.CanonicalReceipt)
	retry, err := j.ResumeProjection(ctx, link, ref)
	require.NoError(t, err)
	require.Equal(t, progress, retry)
	replay, err := j.PrepareProjection(ctx, link)
	require.NoError(t, err)
	require.Equal(t, ref, replay)
	altered := link
	altered.NativeTargetHash = rp.Hash("bad", []byte("substitution"))
	_, err = j.ResumeProjection(ctx, altered, ref)
	require.ErrorIs(t, err, rp.ErrConflict)
	_, err = j.Owner.Pool.Exec(ctx, `UPDATE voice_legacy_projection_steps SET step_bytes='forged' WHERE step_id=$1`, ref.ID)
	require.Error(t, err)
	_, err = j.Owner.Pool.Exec(ctx, `UPDATE voice_legacy_projection_steps SET receipt_bytes='forged' WHERE step_id=$1`, ref.ID)
	require.Error(t, err)
	_, err = j.Owner.ReadCutoverDrain(ctx, link.LegacyWorkRef)
	require.ErrorIs(t, err, rp.ErrPending, "physical receipt is not final native or legacy drain")
}

func TestLegacyProjectionJournalRejectsUnassignedMutation(t *testing.T) {
	ctx := context.Background()
	j, source, client, link := newProjectionJournalFixture(t)
	partition, err := j.RegisterPartition(ctx, link.LegacyWorkRef, link.ResourceProofRef, uuid.New())
	require.NoError(t, err)
	link.PartitionRef = partition
	source.command.Linkage = link
	require.NoError(t, j.AcceptLegacyPartition(ctx, partition))
	extra := source.command.Step.ScopePrefix + "session:foreign"
	require.NoError(t, client.Set(ctx, extra, "foreign", 0).Err())
	source.command.Step.Transitions = append(source.command.Step.Transitions, store.LegacyKeyTransition{Key: extra, Before: store.LegacyKeyImage{Kind: "string", Value: "foreign"}, After: store.LegacyKeyImage{Kind: "none"}})
	_, err = j.PrepareProjection(ctx, link)
	require.ErrorIs(t, err, rp.ErrConflict)
	require.Equal(t, "foreign", client.Get(ctx, extra).Val())
}

func TestLegacyProjectionJournalSerializesAgainstArchivedPredecessor(t *testing.T) {
	ctx := context.Background()
	j, source, client, first := newProjectionJournalFixture(t)
	partition, err := j.RegisterPartition(ctx, first.LegacyWorkRef, first.ResourceProofRef, uuid.New())
	require.NoError(t, err)
	first.PartitionRef = partition
	source.command.Linkage = first
	middle, err := json.Marshal(store.Call{RoomID: "room"})
	require.NoError(t, err)
	command1 := source.command
	command1.Step.Transitions = append([]store.LegacyKeyTransition{}, command1.Step.Transitions...)
	command1.Step.Transitions[0].After = store.LegacyKeyImage{Kind: "string", Value: string(middle)}
	second := first
	second.NativeCommandRef = uuid.New()
	command2 := source.command
	command2.Linkage = second
	command2.Step.Transitions = append([]store.LegacyKeyTransition{}, command2.Step.Transitions...)
	command2.Step.Transitions[0].Before = command1.Step.Transitions[0].After
	source.commands = map[uuid.UUID]NativeProjectionCommand{first.NativeCommandRef: command1, second.NativeCommandRef: command2}
	require.NoError(t, j.AcceptLegacyPartition(ctx, partition))
	ref1, err := j.PrepareProjection(ctx, first)
	require.NoError(t, err)
	_, err = j.PrepareProjection(ctx, second)
	require.ErrorIs(t, err, rp.ErrPending)
	progress1, err := j.ResumeProjection(ctx, first, ref1)
	require.NoError(t, err)
	require.Equal(t, string(middle), client.Get(ctx, command1.Step.Transitions[0].Key).Val())
	ref2, err := j.PrepareProjection(ctx, second)
	require.NoError(t, err)
	step2, err := j.ResolveProjectionStep(ctx, ref2)
	require.NoError(t, err)
	require.EqualValues(t, 2, step2.Ordinal)
	require.Equal(t, progress1.CanonicalReceipt, step2.PreviousReceipt)
	_, err = j.ResumeProjection(ctx, second, ref2)
	require.NoError(t, err)
	require.EqualValues(t, 0, client.Exists(ctx, command1.Step.Transitions[0].Key).Val())
	replay, err := j.ResumeProjection(ctx, first, ref1)
	require.NoError(t, err)
	require.Equal(t, progress1, replay, "historical replay returns its original receipt after a later step")
}
