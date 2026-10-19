// This executable is compiled inside the immutable Space producer source tree.
// Its stdin protocol only drives actual stores; it never constructs authority
// responses, signatures, READY receipts, or private verified capabilities.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"net"
	"net/url"
	"os"
	"strings"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/space/internal/grpcsvc"
	"voice/backend/space/internal/memberauthorityprincipal"
	"voice/backend/space/internal/store"
)

type setup struct {
	DSN                                string
	Owner, Profile, Account, Operation uuid.UUID
	Config                             memberauthorityprincipal.Config
}
type reply struct {
	SpaceID          string
	Address          string
	NodeID           string
	Binding          *memberadmission.EnsureBinding
	Published        bool
	ServingLogin     string
	SchemaVersion    int64
	RegistryPresent  bool
	ServingDSN       string
	DatabaseName     string
	DatabaseSystemID string
}
type command struct {
	Action, RoleID string
	ChatID, NodeID string
	Receipt        []byte
}

func run() error {
	ctx := context.Background()
	input, output := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	var init setup
	if err := input.Decode(&init); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, init.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	st := &store.SpaceStore{Pool: pool}
	// Version34 creator runs before the real forward39 migration, so the test
	// also exercises the actual legacy owner backfill rather than raw SQL seeds.
	space, err := st.CreateSpace(ctx, init.Owner, "Native Role receipt proof", "", "public")
	if err != nil {
		return err
	}
	if err = output.Encode(reply{SpaceID: space.ID.String()}); err != nil {
		return err
	}
	var next command
	if err = input.Decode(&next); err != nil {
		return err
	}
	unknownRegistry := next.Action == "serve_unknown"
	prepare44 := next.Action == "prepare44"
	if next.Action != "prepare" && !prepare44 && !unknownRegistry {
		return fmt.Errorf("prepare required")
	}
	var binding *memberadmission.EnsureBinding
	var servingLogin string
	var schemaVersion int64
	var registryPresent bool
	var servingDSN, databaseName, databaseSystemID string
	if unknownRegistry || prepare44 {
		if prepare44 {
			// The source39 build deliberately lacks this administrative API. Only
			// the actual immutable44 producer can supply it; no SQL substitute,
			// scoped Store injection or readiness callback is accepted here.
			initializer, ok := any(st).(interface {
				InitializeNativeMembership(context.Context, uuid.UUID) error
			})
			if !ok {
				return fmt.Errorf("actual Space administrative initialization API unavailable")
			}
			if err = initializer.InitializeNativeMembership(ctx, space.ID); err != nil {
				return err
			}
		}
		// Both44 branches serve through an independent nonowner LOGIN. UNKNOWN
		// deliberately omits administrative initialization and remains a denial.
		login := "role_fixture_" + uuid.New().String()[:8]
		quoted := pgx.Identifier{login}.Sanitize()
		if _, err = pool.Exec(ctx, "CREATE ROLE "+quoted+" LOGIN PASSWORD 'isolated-role-fixture-only'"); err != nil {
			return err
		}
		if _, err = pool.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+quoted+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+quoted); err != nil {
			return err
		}
		cfg := pool.Config().Copy()
		cfg.ConnConfig.User, cfg.ConnConfig.Password = login, "isolated-role-fixture-only"
		application, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return err
		}
		defer application.Close()
		var actual string
		if err = application.QueryRow(ctx, "SELECT session_user").Scan(&actual); err != nil || actual != login {
			return fmt.Errorf("independent application login unavailable: %w", err)
		}
		servingLogin = actual
		// ConnString retains the ORIGINAL parsed text, not our modified Config
		// credentials. Rebind the original syntax without discarding TLS/options.
		servingDSN, err = fixtureDSNWithCredentials(pool.Config().ConnString(), login, cfg.ConnConfig.Password)
		if err != nil {
			return err
		}
		reopened, err := pgxpool.New(ctx, servingDSN)
		if err != nil {
			return fmt.Errorf("exported application connection unavailable")
		}
		defer reopened.Close()
		var reopenedLogin, reopenedDatabase string
		var reopenedSuperuser bool
		if err = reopened.QueryRow(ctx, "SELECT session_user,current_database(),(SELECT rolsuper FROM pg_roles WHERE rolname=session_user)").Scan(&reopenedLogin, &reopenedDatabase, &reopenedSuperuser); err != nil || reopenedLogin != login || reopenedSuperuser {
			return fmt.Errorf("exported application identity roundtrip failed")
		}
		if reopenedDatabase != cfg.ConnConfig.Database {
			return fmt.Errorf("exported application database roundtrip failed")
		}
		if prepare44 {
			// This is a genuinely fresh consumer pool from the exported DSN. The
			// actual getter enforces nonowner+COMPLETE; no readiness callback or
			// administrator fallback can supply this prospective NONE observation.
			external := &store.SpaceStore{Pool: reopened}
			if _, err = external.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: uuid.NewString()}); err != nil {
				return fmt.Errorf("exported application scoped authority unavailable")
			}
		}
		if err = application.QueryRow(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
			return err
		}
		if err = pool.QueryRow(ctx, "SELECT system_identifier::text FROM pg_control_system()").Scan(&databaseSystemID); err != nil {
			return err
		}
		if err = application.QueryRow(ctx, "SELECT version FROM schema_migrations WHERE NOT dirty").Scan(&schemaVersion); err != nil {
			return err
		}
		if err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1)", space.ID).Scan(&registryPresent); err != nil {
			return err
		}
		st = &store.SpaceStore{Pool: application}
	}
	if !unknownRegistry {
		intent, err := st.PrepareMemberAdmission(ctx, store.MemberAdmissionInput{OperationID: init.Operation, SpaceID: space.ID, AccountID: init.Account, ProfileID: init.Profile})
		if err != nil {
			return err
		}
		b := intent.EnsureBinding()
		binding = &b
	}
	runtime, err := memberauthorityprincipal.New(ctx, init.Config)
	if err != nil {
		return err
	}
	defer runtime.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	server := grpc.NewServer(runtime.ServerOptions()...)
	grpcsvc.RegisterMemberAuthorityServer(server, &grpcsvc.MemberAuthorityGRPC{Store: st})
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	defer func() { server.Stop(); <-served }()
	if err = output.Encode(reply{Address: listener.Addr().String(), Binding: binding, ServingLogin: servingLogin, SchemaVersion: schemaVersion, RegistryPresent: registryPresent, ServingDSN: servingDSN, DatabaseName: databaseName, DatabaseSystemID: databaseSystemID}); err != nil {
		return err
	}
	for {
		if err = input.Decode(&next); err != nil {
			return err
		}
		switch next.Action {
		case "stop":
			return nil
		case "attach_chat":
			chat, err := uuid.Parse(next.ChatID)
			if err != nil {
				return err
			}
			node, err := st.UpsertTreeNode(ctx, store.UpsertTreeNodeInput{SpaceID: space.ID, ActorProfileID: init.Owner, Kind: store.TreeKindTextChat, ChatID: &chat})
			if err != nil {
				return err
			}
			if err = output.Encode(reply{NodeID: node.ID.String()}); err != nil {
				return err
			}
		case "remove_chat":
			node, err := uuid.Parse(next.NodeID)
			if err != nil {
				return err
			}
			if err = st.RemoveTreeNode(ctx, space.ID, node); err != nil {
				return err
			}
			if err = output.Encode(reply{NodeID: node.String()}); err != nil {
				return err
			}
		case "publish":
			role, err := uuid.Parse(next.RoleID)
			if err != nil {
				return err
			}
			if _, err = st.PublishMemberAdmission(ctx, init.Operation, role, next.Receipt, nil); err != nil {
				return err
			}
			if err = output.Encode(reply{Published: true}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown fixture action")
		}
	}
}

// fixtureDSNWithCredentials preserves every noncredential parameter in the
// original pgx URI/keyword syntax. Never rebuild a DSN from a subset of Config.
func fixtureDSNWithCredentials(source, login, password string) (string, error) {
	if source == "" || login == "" || password == "" || strings.ContainsRune(source+login+password, 0) {
		return "", fmt.Errorf("invalid fixture connection settings")
	}
	if strings.HasPrefix(source, "postgres://") || strings.HasPrefix(source, "postgresql://") {
		parsed, err := url.Parse(source)
		if err != nil {
			return "", fmt.Errorf("invalid fixture connection URI")
		}
		query, err := url.ParseQuery(parsed.RawQuery)
		if err != nil {
			return "", fmt.Errorf("invalid fixture connection parameters")
		}
		parsed.User = url.UserPassword(login, password)
		// pgx query credentials override userinfo, so replace them as well when
		// present. Other query values (including TLS and fallback hosts) survive.
		if query.Has("user") {
			query.Set("user", login)
		}
		if query.Has("password") {
			query.Set("password", password)
		}
		parsed.RawQuery = query.Encode()
		return parsed.String(), nil
	}
	// pgx keyword parsing uses the final occurrence. Escape both quote and
	// backslash per its quoted-value grammar rather than shell/URL escaping.
	escape := strings.NewReplacer("\\", "\\\\", "'", "\\'")
	return source + " user='" + escape.Replace(login) + "' password='" + escape.Replace(password) + "'", nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
