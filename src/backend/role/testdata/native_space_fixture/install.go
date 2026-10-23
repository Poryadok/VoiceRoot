package main

import (
	"context"
	"fmt"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"voice/backend/space/internal/store"
)

// Follow the owning Space installer, retaining its actual source seal and
// database identity. This is not a test-only enrollment or migration bypass.
func installCurrentCatalog(ctx context.Context, st *store.SpaceStore, dsn string, space uuid.UUID) error {
	const source = "file:///tmp/role-native-space-migrations"
	engine, err := migrate.New(source, dsn)
	if err != nil {
		return err
	}
	defer engine.Close()
	if err = engine.Migrate(44); err != nil {
		return err
	}
	prior, err := st.InspectNativeInstallerSource(ctx, 44)
	if err != nil {
		return err
	}
	if err = engine.Migrate(45); err != nil {
		return err
	}
	r45, err := st.TransitionNativeMembershipCatalog45(ctx)
	if err != nil {
		return err
	}
	if !r45.Changed || r45.OldSealSHA256 != prior.SealSHA256 || r45.DatabaseOID != prior.DatabaseOID || r45.Epoch != prior.Epoch {
		return fmt.Errorf("native45 installer receipt identity mismatch")
	}
	prior, err = st.InspectNativeInstallerSource(ctx, 45)
	if err != nil {
		return err
	}
	if prior.SealSHA256 != r45.NewSealSHA256 {
		return fmt.Errorf("native45 installed seal mismatch")
	}
	if err = engine.Migrate(50); err != nil {
		return err
	}
	r50, err := st.TransitionNativeMembershipCatalog50(ctx)
	if err != nil {
		return err
	}
	if !r50.Changed || r50.OldSealSHA256 != prior.SealSHA256 || r50.DatabaseOID != prior.DatabaseOID || r50.Epoch != prior.Epoch {
		return fmt.Errorf("native50 installer receipt identity mismatch")
	}
	prior, err = st.InspectNativeInstallerSource(ctx, 50)
	if err != nil {
		return err
	}
	if prior.SealSHA256 != r50.NewSealSHA256 {
		return fmt.Errorf("native50 installed seal mismatch")
	}
	if err = st.InitializeNativeMembership(ctx, space); err != nil {
		return err
	}
	if sourceErr, dbErr := engine.Close(); sourceErr != nil || dbErr != nil {
		return fmt.Errorf("close migration source: %v; database: %v", sourceErr, dbErr)
	}
	application := st.Pool.Config().ConnConfig.RuntimeParams["application_name"]
	window := func(ctx context.Context) error {
		var exclusive bool
		err := st.Pool.QueryRow(ctx, `SELECT current_user=session_user AND NOT pg_is_in_recovery()
		 AND NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
		 AND backend_type='client backend' AND application_name<>$1)`, application).Scan(&exclusive)
		if err != nil || !exclusive {
			return store.ErrMemberAuthorityUnavailable
		}
		return nil
	}
	driver, err := st.NewNativeCatalog51Migration(ctx, prior, window)
	if err != nil {
		return err
	}
	defer driver.Close()
	qualified, err := migrate.NewWithDatabaseInstance(source, "role-native51", driver)
	if err != nil {
		return err
	}
	defer qualified.Close()
	migrationErr := qualified.Migrate(51)
	receipt, err := driver.CommitQualified(migrationErr)
	if migrationErr != nil {
		return migrationErr
	}
	if err != nil {
		return err
	}
	if !receipt.Changed || receipt.OldSealSHA256 != prior.SealSHA256 {
		return fmt.Errorf("native51 installer receipt mismatch")
	}
	if err = driver.Close(); err != nil {
		return err
	}
	current, err := st.InspectNativeInstallerSource(ctx, 51)
	if err != nil {
		return err
	}
	if current.SealSHA256 != receipt.NewSealSHA256 || current.DatabaseOID != prior.DatabaseOID || current.Epoch != prior.Epoch {
		return fmt.Errorf("native51 installed source identity mismatch")
	}
	return nil
}
