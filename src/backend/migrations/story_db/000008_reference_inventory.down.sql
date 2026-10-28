BEGIN;
LOCK TABLE story_reference_inventory_gate,story_reference_inventory_snapshots,
    story_reference_inventory_pages,story_reference_inventory_journal,
    story_reference_inventory_state,story_reference_inventory_retirements,
    story_media_reference_intents IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM story_reference_inventory_gate WHERE revision<>0)
       OR EXISTS(SELECT 1 FROM story_reference_inventory_snapshots)
       OR EXISTS(SELECT 1 FROM story_reference_inventory_journal)
       OR EXISTS(SELECT 1 FROM story_reference_inventory_state)
       OR EXISTS(SELECT 1 FROM story_reference_inventory_retirements)
       OR EXISTS(SELECT 1 FROM story_media_reference_intents) THEN
        RAISE EXCEPTION 'reference inventory has retained ownership; rollback refused';
    END IF;
END $$;
DROP TABLE story_reference_inventory_pages,story_reference_inventory_snapshots,
    story_reference_inventory_journal,story_reference_inventory_state,
    story_reference_inventory_retirements,story_reference_inventory_gate;
ALTER TABLE story_media_reference_intents DROP COLUMN author_profile_id;
ALTER TABLE story_media_reference_intents DROP COLUMN media_type;
COMMIT;
