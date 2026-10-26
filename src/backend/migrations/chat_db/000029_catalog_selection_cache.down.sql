DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM catalog_actor_gates)
       OR EXISTS (SELECT 1 FROM catalog_selector_slots)
       OR EXISTS (SELECT 1 FROM catalog_asset_bindings) THEN
        RAISE EXCEPTION 'cannot remove nonempty ordered catalog cache';
    END IF;
END $$;

DROP TABLE catalog_asset_bindings;
DROP TABLE catalog_selector_aliases;
DROP TABLE catalog_selector_slots;
ALTER TABLE catalog_legacy_asset_bindings RENAME TO catalog_asset_bindings;
ALTER TABLE catalog_asset_bindings RENAME CONSTRAINT catalog_legacy_asset_bindings_pkey TO catalog_asset_bindings_pkey;
ALTER INDEX catalog_legacy_asset_bindings_account RENAME TO catalog_asset_bindings_account;
ALTER INDEX catalog_legacy_asset_bindings_profile RENAME TO catalog_asset_bindings_profile;
