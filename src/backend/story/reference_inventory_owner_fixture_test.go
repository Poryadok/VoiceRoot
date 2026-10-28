package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/story/internal/inventorygrpc"
	"voice/backend/story/internal/store"
)

// This child-process fixture lets the File integration package exercise the
// actual Story Store/private runtime without crossing Go internal boundaries.
// It is reachable only from an explicitly configured owned test subprocess.
func TestReferenceInventoryOwnerFixtureProcess(t *testing.T) {
	inputPath := os.Getenv("VOICE_REFERENCE_INVENTORY_OWNER_FIXTURE")
	if inputPath == "" || testing.Short() {
		t.Skip("owned cross-service integration fixture only")
	}
	type pin struct{ FileID, AuthorID string }
	var in struct {
		DatabaseURL, ReadyPath, StopPath string
		Pins                             []pin
		Config                           inventorygrpc.Config
	}
	f, err := os.Open(inputPath)
	if err != nil {
		t.Fatal("fixture input unavailable")
	}
	decoder := json.NewDecoder(io.LimitReader(f, 65536))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&in); err != nil {
		_ = f.Close()
		t.Fatal("invalid fixture input")
	}
	_ = f.Close()
	if !filepath.IsAbs(inputPath) || !filepath.IsAbs(in.ReadyPath) || !filepath.IsAbs(in.StopPath) || len(in.Pins) == 0 || len(in.Pins) > 10 {
		t.Fatal("invalid fixture binding")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, in.DatabaseURL)
	if err != nil {
		t.Fatal("Story fixture database unavailable")
	}
	defer pool.Close()
	migrations, err := filepath.Glob(filepath.Join("..", "migrations", "story_db", "*.up.sql"))
	if err != nil || len(migrations) == 0 {
		t.Fatal("Story fixture migrations unavailable")
	}
	for _, name := range migrations {
		sql, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(sql)); e != nil {
			t.Fatalf("Story fixture migration %s: %v", filepath.Base(name), e)
		}
	}
	st := &store.StoryStore{Pool: pool}
	ready := struct {
		Address, OwnerIncarnationID string
		Stories                     map[string]string
	}{Stories: map[string]string{}}
	for _, p := range in.Pins {
		fileID, e := uuid.Parse(p.FileID)
		if e != nil || fileID == uuid.Nil {
			t.Fatal("invalid fixture media")
		}
		author, e := uuid.Parse(p.AuthorID)
		if e != nil || author == uuid.Nil {
			t.Fatal("invalid fixture author")
		}
		// Deliberately historical: this owner predates the File ledger, so no
		// ClaimStoryMedia call or test release authority is manufactured.
		s, e := st.CreateStory(ctx, store.CreateStoryInput{AuthorProfileID: author, Type: "photo", MediaFileID: &fileID, Visibility: "everyone"})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, `UPDATE stories SET expired_at=now(),archived_until=now()-interval '1 day' WHERE id=$1`, s.ID); e != nil {
			t.Fatal(e)
		}
		h, e := st.CreateHighlight(ctx, author, "historical retained media", "everyone")
		if e != nil {
			t.Fatal(e)
		}
		if e = st.AddToHighlight(ctx, h.ID, author, s.ID); e != nil {
			t.Fatal(e)
		}
		ready.Stories[fileID.String()] = s.ID.String()
	}
	if err = pool.QueryRow(ctx, `SELECT owner_incarnation_id FROM story_reference_inventory_gate WHERE singleton`).Scan(&ready.OwnerIncarnationID); err != nil {
		t.Fatal(err)
	}
	runtime, err := startReferenceInventory(ctx, in.Config, true, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		closeCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = runtime.Close(closeCtx)
	}()
	ready.Address = runtime.listener.Addr().String()
	raw, err := json.Marshal(ready)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(in.ReadyPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("owned Story fixture deadline")
		case <-runtime.errors:
			t.Fatal("owned Story fixture listener exited")
		case <-ticker.C:
			if _, err = os.Stat(in.StopPath); err == nil {
				return
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
	}
}
