DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM file_catalog_writes WHERE owner_kind <> 'catalog_source') THEN
        RAISE EXCEPTION 'cannot erase retained derived-object write custody';
    END IF;
END $$;
ALTER TABLE file_catalog_writes DROP CONSTRAINT file_catalog_writes_owner_kind_check;
ALTER TABLE file_catalog_writes DROP COLUMN owner_kind;
