//go:build linux

package mmadmissionctl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	w "voice/backend/pkg/mmadmissionwire"
)

func TestOwnedDockerActualPrivateResources(t *testing.T) {
	if os.Getenv("MM_ADMISSION_DOCKER_TEST") != "1" {
		t.Skip("requires an explicitly owned Docker controller runner")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	scope, e := w.ParseID(os.Getenv("MM_ADMISSION_LOCAL_SCOPE"))
	if e != nil || scope == (w.ID{}) {
		t.Fatal("missing own scope")
	}
	j, e := CreateLocalJournal(filepath.Join(t.TempDir(), "control.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	d, e := newOwnedDocker(j, scope)
	if e != nil {
		t.Fatal(e)
	}
	selfbytes, e := os.ReadFile("/etc/hostname")
	if e != nil {
		t.Fatal(e)
	}
	var self struct {
		ID     string `json:"Id"`
		Config struct{ Labels map[string]string }
	}
	if e = d.call(ctx, http.MethodGet, "/containers/"+strings.TrimSpace(string(selfbytes))+"/json", nil, &self); e != nil || self.Config.Labels[ownLabel] != scope.String() {
		t.Fatal("runner is not the exact source-created own controller", e)
	}
	network, e := d.create(ctx, "network", "private", map[string]any{"Internal": true, "CheckDuplicate": true})
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := d.cleanup(cleanupCtx, self.ID); err != nil {
			t.Error("owned resource cleanup incomplete", err)
		}
	}()
	if e = d.call(ctx, http.MethodPost, "/networks/"+network.ID+"/connect", map[string]any{"Container": self.ID}, nil); e != nil {
		t.Fatal(e)
	}
	volume, e := d.create(ctx, "volume", "pg-data", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	pg, e := d.create(ctx, "container", "pg", map[string]any{"Image": "postgres:16-bookworm", "Env": []string{"POSTGRES_PASSWORD=mm-local-fixture-only", "POSTGRES_DB=matchmaking_db"}, "HostConfig": map[string]any{"NetworkMode": network.ID, "Mounts": []map[string]any{{"Type": "volume", "Source": volume.Name, "Target": "/var/lib/postgresql/data"}}}})
	if e != nil {
		t.Fatal(e)
	}
	redis, e := d.create(ctx, "container", "redis", map[string]any{"Image": "redis:7.4-alpine", "Cmd": []string{"redis-server", "--save", "", "--appendonly", "no"}, "HostConfig": map[string]any{"NetworkMode": network.ID}})
	if e != nil {
		t.Fatal(e)
	}
	broker, e := d.create(ctx, "container", "nats", map[string]any{"Image": "nats:2.12-alpine", "Cmd": []string{"-js"}, "HostConfig": map[string]any{"NetworkMode": network.ID}})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []ownedObject{pg, redis, broker} {
		if e = d.start(ctx, o); e != nil {
			t.Fatal(e)
		}
	}
	if e = d.verifyUniverse(ctx, network, []string{self.ID, pg.ID, redis.ID, broker.ID}); e != nil {
		t.Fatal(e)
	}
	ip := func(o ownedObject) string {
		raw, e := d.inspect(ctx, o)
		if e != nil {
			t.Fatal(e)
		}
		var v struct {
			NetworkSettings struct {
				Networks map[string]struct{ IPAddress string }
			}
		}
		if json.Unmarshal(raw, &v) != nil {
			t.Fatal("bad inspect")
		}
		for _, n := range v.NetworkSettings.Networks {
			if n.IPAddress != "" {
				return n.IPAddress
			}
		}
		t.Fatal("no owned private address")
		return ""
	}
	pgIP := ip(pg)
	var conn *pgx.Conn
	for {
		conn, e = pgx.Connect(ctx, "postgres://postgres:mm-local-fixture-only@"+pgIP+":5432/matchmaking_db?sslmode=disable")
		if e == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(e)
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer conn.Close(context.Background())
	var database, user string
	var oid uint32
	if e = conn.QueryRow(ctx, `SELECT current_database(),session_user,(SELECT oid FROM pg_roles WHERE rolname=session_user)`).Scan(&database, &user, &oid); e != nil || database != "matchmaking_db" || user != "postgres" || oid == 0 {
		t.Fatal("actual PG login observation", e)
	}
	for _, target := range []struct {
		o                 ownedObject
		port              string
		request, expected string
	}{{redis, "6379", "*1\r\n$4\r\nPING\r\n", "+PONG"}, {broker, "4222", "PING\r\n", "PONG"}} {
		socket, e := net.DialTimeout("tcp", net.JoinHostPort(ip(target.o), target.port), time.Second)
		if e != nil {
			t.Fatal(e)
		}
		_ = socket.SetDeadline(time.Now().Add(2 * time.Second))
		if _, e = fmt.Fprint(socket, target.request); e != nil {
			t.Fatal(e)
		}
		reader := bufio.NewReader(socket)
		seen := false
		for i := 0; i < 3; i++ {
			line, e := reader.ReadString('\n')
			if e != nil {
				break
			}
			if strings.TrimSpace(line) == target.expected {
				seen = true
				break
			}
		}
		_ = socket.Close()
		if !seen {
			t.Fatal("real resource protocol check failed", target.o.Kind, target.o.Name)
		}
	}
	testPGInstaller(t, ctx, conn, pgIP, scope)
	// A copied object name/label is not a creation intent; mutating identity fails.
	forged := pg
	forged.Nonce[0]++
	if _, e = d.inspect(ctx, forged); e == nil {
		t.Fatal("copied resource accepted")
	}
	if e = d.verifyUniverse(ctx, network, []string{self.ID, pg.ID, redis.ID}); e == nil {
		t.Fatal("unregistered broker allowed into closed universe")
	}
	t.Log("LOCAL_OWNED real PG/Redis/NATS; controller journal outside PG volume; no native/provider qualification")
}
