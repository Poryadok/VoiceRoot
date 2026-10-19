//go:build role_space_native

package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

type fixtureBotSeed struct {
	SchemaVersion      int    `json:"schema_version"`
	BotID              string `json:"bot_id"`
	ActorProfileID     string `json:"actor_profile_id"`
	SpaceID            string `json:"space_id"`
	InstallationID     string `json:"installation_id"`
	InstallOperationID string `json:"install_operation_id"`
	Generation         uint64 `json:"generation"`
}

type fixtureBotPostcondition struct {
	fixtureBotSeed
	OperationID string `json:"operation_id"`
	ReceiptID   string `json:"receipt_id"`
	DecisionID  string `json:"decision_id"`
}

type fixtureBotRows struct {
	SchemaVersion          int            `json:"schema_version"`
	Seed                   fixtureBotSeed `json:"seed"`
	CreatedRoleID          string         `json:"created_role_id"`
	ControlRoleID          string         `json:"control_role_id"`
	ControlProfileID       string         `json:"control_profile_id"`
	SystemRoleID           string         `json:"system_role_id"`
	DeletedRoleCount       uint64         `json:"deleted_role_count"`
	RemovedAssignmentCount uint64         `json:"removed_assignment_count"`
}

type fixtureBotCleanup struct {
	pool             *pgxpool.Pool
	space, owner     string
	seedRaw, postRaw []byte
	rows             fixtureBotRows
	complete         bool
}

func fixtureCanonicalID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}

func (s fixtureBotSeed) validate(space, owner string) error {
	if s.SchemaVersion != 1 || s.SpaceID != space || s.Generation == 0 || s.Generation > math.MaxInt64 || s.ActorProfileID == owner {
		return fmt.Errorf("invalid fixture seed binding")
	}
	for _, id := range []string{s.BotID, s.ActorProfileID, s.SpaceID, s.InstallationID, s.InstallOperationID} {
		if !fixtureCanonicalID(id) {
			return fmt.Errorf("noncanonical fixture seed identity")
		}
	}
	return nil
}

func fixtureReadJSON(path string, target any) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16384 {
		return nil, fmt.Errorf("invalid fixture synchronization file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return nil, err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("trailing fixture JSON")
	}
	return raw, nil
}

func (f *fixtureBotCleanup) poll(ctx context.Context, dir string) error {
	var seed fixtureBotSeed
	raw, err := fixtureReadJSON(filepath.Join(dir, "seed.request.json"), &seed)
	if os.IsNotExist(err) && f.seedRaw == nil {
		return nil
	}
	if err != nil {
		return err
	}
	if f.seedRaw != nil {
		if !bytes.Equal(raw, f.seedRaw) {
			return fmt.Errorf("fixture seed changed after admission")
		}
	} else {
		if err = seed.validate(f.space, f.owner); err != nil {
			return err
		}
		rows, err := f.seed(ctx, seed)
		if err != nil {
			return err
		}
		f.rows, f.seedRaw = rows, bytes.Clone(raw)
		if err = fixtureWriteJSON(dir, "seed.ready.json", rows); err != nil {
			return err
		}
	}
	var post fixtureBotPostcondition
	raw, err = fixtureReadJSON(filepath.Join(dir, "postcondition.request.json"), &post)
	if os.IsNotExist(err) && f.postRaw == nil {
		return nil
	}
	if err != nil {
		return err
	}
	if f.postRaw != nil {
		if !bytes.Equal(raw, f.postRaw) {
			return fmt.Errorf("fixture postcondition changed")
		}
		return nil
	}
	if post.fixtureBotSeed != f.rows.Seed || !fixtureCanonicalID(post.OperationID) || !fixtureCanonicalID(post.ReceiptID) || !fixtureCanonicalID(post.DecisionID) {
		return fmt.Errorf("postcondition differs from seed")
	}
	if err = f.check(ctx, post); err != nil {
		return err
	}
	f.postRaw, f.complete = bytes.Clone(raw), true
	return fixtureWriteJSON(dir, "postcondition.ready.json", map[string]any{"schema_version": 1, "operation_id": post.OperationID, "receipt_id": post.ReceiptID, "decision_id": post.DecisionID, "deleted_role_count": f.rows.DeletedRoleCount, "removed_assignment_count": f.rows.RemovedAssignmentCount, "database_checked": true})
}

func (f *fixtureBotCleanup) seed(ctx context.Context, s fixtureBotSeed) (fixtureBotRows, error) {
	r := fixtureBotRows{SchemaVersion: 1, Seed: s, CreatedRoleID: uuid.NewString(), ControlRoleID: uuid.NewString(), ControlProfileID: uuid.NewString(), DeletedRoleCount: 1, RemovedAssignmentCount: 2}
	tx, err := f.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return r, err
	}
	defer tx.Rollback(ctx)
	// Fixture setup only: no native contribution, generation, authorization or
	// cleanup receipt is fabricated. These raw assignments are cleanup targets.
	if err = tx.QueryRow(ctx, "SELECT id FROM roles WHERE space_id=$1 AND is_system AND name='Member'", s.SpaceID).Scan(&r.SystemRoleID); err != nil {
		return r, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO roles(id,space_id,name,is_system,created_by_profile_id) VALUES($1,$2,'Fixture Bot created',false,$3),($4,$2,'Fixture human control',false,$5)`, r.CreatedRoleID, s.SpaceID, s.ActorProfileID, r.ControlRoleID, f.owner)
	if err != nil {
		return r, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$2),($1,$4,$5,$4),($1,$2,$5,$2),($1,$4,$6,$4)`, s.SpaceID, r.ControlProfileID, r.CreatedRoleID, s.ActorProfileID, r.ControlRoleID, r.SystemRoleID)
	if err != nil {
		return r, err
	}
	if err = tx.Commit(ctx); err != nil {
		return r, err
	}
	// Read through the independent pool after COMMIT, before publishing ready.
	var count int
	err = f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM roles WHERE id=$1 AND space_id=$2 AND created_by_profile_id=$3 AND NOT is_system AND name<>'Owner')+(SELECT count(*) FROM member_roles WHERE space_id=$2 AND ((profile_id=$4 AND role_id IN ($1,$5)) OR (profile_id=$3 AND role_id IN ($5,$6))))`, r.CreatedRoleID, s.SpaceID, s.ActorProfileID, r.ControlProfileID, r.ControlRoleID, r.SystemRoleID).Scan(&count)
	if err == nil && count != 5 {
		err = fmt.Errorf("seed committed readback differs")
	}
	return r, err
}

func (f *fixtureBotCleanup) check(ctx context.Context, p fixtureBotPostcondition) error {
	var request, response, storedHash []byte
	if err := f.pool.QueryRow(ctx, "SELECT request_bytes,response_bytes,request_sha256 FROM role_bot_removal_operations WHERE operation_id=$1 AND bot_id=$2 AND actor_profile_id=$3 AND space_id=$4 AND installation_id=$5", p.OperationID, p.BotID, p.ActorProfileID, p.SpaceID, p.InstallationID).Scan(&request, &response, &storedHash); err != nil {
		return err
	}
	var req rolev1.ApplyBotInstallationRemovalRequest
	var res rolev1.ApplyBotInstallationRemovalResponse
	if err := proto.Unmarshal(request, &req); err != nil {
		return err
	}
	if err := proto.Unmarshal(response, &res); err != nil {
		return err
	}
	hash := sha256.Sum256(request)
	n := req.GetNative()
	receipt := res.GetReceipt()
	if n == nil || n.InstallationId != p.InstallationID || n.InstallOperationId != p.InstallOperationID || n.Generation != p.Generation || req.BotId != p.BotID || req.ActorProfileId != p.ActorProfileID || req.SpaceId != p.SpaceID || req.OperationId != p.OperationID || req.DecisionId != p.DecisionID || req.Purpose != "space_uninstall" || !bytes.Equal(hash[:], storedHash) {
		return fmt.Errorf("committed cleanup request differs from seed")
	}
	if receipt == nil || receipt.ReceiptId != p.ReceiptID || receipt.OperationId != p.OperationID || receipt.DecisionId != p.DecisionID || receipt.BotId != p.BotID || receipt.ActorProfileId != p.ActorProfileID || receipt.SpaceId != p.SpaceID || receipt.Participant != "role" || receipt.Outcome != "COMPLETED" || receipt.DeletedRoleCount != f.rows.DeletedRoleCount || receipt.RemovedAssignmentCount != f.rows.RemovedAssignmentCount || !bytes.Equal(receipt.RequestSha256, hash[:]) {
		return fmt.Errorf("committed cleanup receipt differs")
	}
	r := f.rows
	var gone, controls, operations int
	if err := f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM roles WHERE id=$1)+(SELECT count(*) FROM member_roles WHERE space_id=$2 AND (role_id=$1 OR (profile_id=$3 AND role_id=$4)))),(SELECT count(*) FROM roles WHERE id=$4 AND space_id=$2 AND created_by_profile_id=$5 AND NOT is_system)+(SELECT count(*) FROM member_roles WHERE space_id=$2 AND ((profile_id=$6 AND role_id=$4) OR (profile_id=$3 AND role_id=$7)))+(SELECT count(*) FROM member_roles m JOIN roles x ON x.id=m.role_id WHERE m.space_id=$2 AND m.profile_id=$5 AND x.is_system AND x.name='Owner'),(SELECT count(*) FROM role_bot_removal_operations WHERE bot_id=$8 AND space_id=$2 AND installation_id=$9 AND response_bytes IS NOT NULL)`, r.CreatedRoleID, p.SpaceID, p.ActorProfileID, r.ControlRoleID, f.owner, r.ControlProfileID, r.SystemRoleID, p.BotID, p.InstallationID).Scan(&gone, &controls, &operations); err != nil {
		return err
	}
	if gone != 0 || controls != 4 || operations != 1 {
		return fmt.Errorf("committed effects differ: targets=%d controls=%d receipts=%d", gone, controls, operations)
	}
	return nil
}

func fixtureWriteJSON(dir, name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, name)
	if err = os.WriteFile(path+".tmp", raw, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func fixtureWaitFile(ctx context.Context, dir, name string) error {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("fixture handshake missing %s", name)
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// The provider self-test uses the real protected signed client; it does not
// qualify the independent Bot journal/target selector owned by consumer247.
func startFixtureBotControl(t *testing.T, ctx context.Context, client rolev1.RoleServiceClient, key *rsa.PrivateKey, space string) <-chan error {
	mode := os.Getenv("ROLE_NATIVE_FIXTURE_BOT_SELFTEST")
	if mode != "1" && mode != "default" {
		return nil
	}
	dir := os.Getenv("ROLE_NATIVE_FIXTURE_EXPORT_DIR")
	done := make(chan error, 1)
	go func() {
		err := func() error {
			if err := fixtureWaitFile(ctx, dir, "ready.json"); err != nil {
				return err
			}
			if mode == "default" {
				if _, err := os.Stat(filepath.Join(dir, "seed.ready.json")); !os.IsNotExist(err) {
					return fmt.Errorf("default fixture unexpectedly seeded")
				}
				return nil
			}
			seed := fixtureBotSeed{1, uuid.NewString(), uuid.NewString(), space, uuid.NewString(), uuid.NewString(), 1}
			if err := fixtureWriteJSON(dir, "seed.request.json", seed); err != nil {
				return err
			}
			if err := fixtureWaitFile(ctx, dir, "seed.ready.json"); err != nil {
				return err
			}
			req := &rolev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: uuid.NewString(), DecisionSha256: bytes.Repeat([]byte{3}, 32), Purpose: "space_uninstall", BotId: seed.BotID, ActorProfileId: seed.ActorProfileID, SpaceId: space, FrozenScopeSha256: bytes.Repeat([]byte{4}, 32), Evidence: &rolev1.ApplyBotInstallationRemovalRequest_Native{Native: &rolev1.BotRemovalNativeEvidence{InstallationId: seed.InstallationID, InstallOperationId: seed.InstallOperationID, Generation: seed.Generation}}}
			call := func() (*rolev1.ApplyBotInstallationRemovalResponse, error) {
				return client.ApplyBotInstallationRemoval(sourceCallContext(t, key, "bot", rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName, req.OperationId, req), req)
			}
			first, err := call()
			if err != nil {
				return err
			}
			if first.GetReceipt().GetDeletedRoleCount() != 1 || first.GetReceipt().GetRemovedAssignmentCount() != 2 {
				return fmt.Errorf("nonzero effects differ: roles=%d assignments=%d", first.GetReceipt().GetDeletedRoleCount(), first.GetReceipt().GetRemovedAssignmentCount())
			}
			again, err := call()
			if err != nil {
				return err
			}
			if !proto.Equal(first, again) {
				return fmt.Errorf("replay changed receipt")
			}
			post := fixtureBotPostcondition{seed, req.OperationId, first.Receipt.ReceiptId, req.DecisionId}
			if err := fixtureWriteJSON(dir, "postcondition.request.json", post); err != nil {
				return err
			}
			return fixtureWaitFile(ctx, dir, "postcondition.ready.json")
		}()
		_ = os.WriteFile(filepath.Join(dir, "stop"), nil, 0600)
		done <- err
	}()
	return done
}

func TestNativeFixtureBotCleanupNonzero(t *testing.T) {
	t.Setenv("ROLE_NATIVE_FIXTURE_EXPORT_DIR", filepath.Join(t.TempDir(), "export"))
	t.Setenv("ROLE_NATIVE_FIXTURE_ROLE_SOURCE", os.Getenv("ROLE_NATIVE_FIXTURE_ROLE_SOURCE"))
	t.Setenv("ROLE_NATIVE_FIXTURE_BOT_CLEANUP", "1")
	t.Setenv("ROLE_NATIVE_FIXTURE_BOT_SELFTEST", "1")
	runNativeMemberProtocol(t, false, "25", "44")
}

func TestNativeFixtureDefaultExportControl(t *testing.T) {
	t.Setenv("ROLE_NATIVE_FIXTURE_EXPORT_DIR", filepath.Join(t.TempDir(), "export"))
	t.Setenv("ROLE_NATIVE_FIXTURE_BOT_CLEANUP", "")
	t.Setenv("ROLE_NATIVE_FIXTURE_BOT_SELFTEST", "default")
	runNativeMemberProtocol(t, false, "25", "44")
}

func TestNativeFixtureBotSeedValidation(t *testing.T) {
	owner := uuid.NewString()
	s := fixtureBotSeed{1, uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), 1}
	require.NoError(t, s.validate(s.SpaceID, owner))
	for name, change := range map[string]func(*fixtureBotSeed){"wrongSpace": func(s *fixtureBotSeed) { s.SpaceID = uuid.NewString() }, "ownerActor": func(s *fixtureBotSeed) { s.ActorProfileID = owner }, "zeroGeneration": func(s *fixtureBotSeed) { s.Generation = 0 }, "overflowGeneration": func(s *fixtureBotSeed) { s.Generation = math.MaxUint64 }, "schema": func(s *fixtureBotSeed) { s.SchemaVersion = 2 }, "zeroBot": func(s *fixtureBotSeed) { s.BotID = uuid.Nil.String() }, "noncanonicalInstallation": func(s *fixtureBotSeed) { s.InstallationID = "{" + s.InstallationID + "}" }} {
		t.Run(name, func(t *testing.T) { bad := s; change(&bad); require.Error(t, bad.validate(s.SpaceID, owner)) })
	}
}

func fixtureCheckControl(t *testing.T, done <-chan error) {
	if done != nil {
		require.NoError(t, <-done)
	}
}
