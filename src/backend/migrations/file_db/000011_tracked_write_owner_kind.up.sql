-- One physical write ledger serves every File-owned producer.
ALTER TABLE file_catalog_writes
    ADD COLUMN owner_kind TEXT NOT NULL DEFAULT 'catalog_source',
    ADD CONSTRAINT file_catalog_writes_owner_kind_check
        CHECK (owner_kind IN ('catalog_source','pdf_thumbnail','gif_full','gif_thumbnail'));
