DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM catalog_actor_gates) THEN
        RAISE EXCEPTION 'catalog actor anti-resurrection state requires an explicit migration; nonempty rollback is forbidden';
    END IF;
END $$;
DROP TABLE catalog_asset_bindings;
DROP TABLE catalog_actor_event_fences;
DROP TABLE catalog_actor_aliases;
DROP TABLE catalog_actor_gates;
DROP TABLE catalog_actor_keys;
