// This maintenance executable is separate from the serving process. Its caller
// must establish fresh installation provenance before supplying owner credentials.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/space/internal/store"
)

const applicationRole = "voice_space_ci_app"

func install(ctx context.Context) error {
	trusted := flag.Bool("trusted-fresh-installation", false, "caller has verified a newly created, exclusively owned CI database volume")
	flag.Parse()
	password := os.Getenv("SPACE_NATIVE_APP_PASSWORD")
	nonce := os.Getenv("SPACE_NATIVE_INSTALLATION_NONCE")
	decoded, err := hex.DecodeString(password)
	if !*trusted || flag.NArg() != 0 || err != nil || len(decoded) != 32 || len(nonce) != 32 {
		return errors.New("verified fresh CI installation and private application credentials required")
	}
	owner, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("installer database connection unavailable")
	}
	defer owner.Close()
	// Authenticate the trusted owner and exact catalog BEFORE provisioning the
	// serving login. This never copies a live catalog digest into an expected pin.
	receipt, err := (&store.SpaceStore{Pool: owner}).TransitionNativeMembershipCatalog45(ctx)
	if err != nil {
		return fmt.Errorf("trusted native45 installation: %w", err)
	}
	// This fixed login is deliberately not reused or upgraded from an unknown
	// prior installation. The orchestration rejects existing database volumes.
	if _, err = owner.Exec(ctx, "CREATE ROLE "+applicationRole+" LOGIN PASSWORD '"+password+"'"); err != nil {
		return errors.New("fresh nonowner application login could not be created")
	}
	if _, err = owner.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+applicationRole+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+applicationRole); err != nil {
		return errors.New("application table privileges unavailable")
	}
	config := owner.Config().Copy()
	config.ConnConfig.User, config.ConnConfig.Password = applicationRole, password
	app, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return errors.New("nonowner application connection unavailable")
	}
	defer app.Close()
	var valid bool
	if err = app.QueryRow(ctx, `SELECT current_user=session_user AND session_user=$1
 AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolbypassrls
 AND NOT has_schema_privilege(session_user,'public','CREATE')
 AND NOT pg_has_role(session_user,c.relowner,'MEMBER')
 FROM pg_roles r,pg_class c WHERE r.rolname=session_user AND c.oid='public.spaces'::regclass`, applicationRole).Scan(&valid); err != nil || !valid {
		return errors.New("application session is not a DDL-incapable nonowner login")
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Native            *store.NativeMembershipCatalog45Receipt `json:"native45"`
		ApplicationRole   string                                  `json:"application_role"`
		InstallationNonce string                                  `json:"installation_nonce"`
	}{receipt, applicationRole, nonce})
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := install(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
