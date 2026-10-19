-- Preserve all retained private evidence. Empty-only rollback is not erasure.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM story_lfp_offers) OR EXISTS (SELECT 1 FROM story_lfp_consents)
      OR EXISTS (SELECT 1 FROM story_lfp_publications) OR EXISTS (SELECT 1 FROM story_lfp_registration_acks)
      OR EXISTS (SELECT 1 FROM story_lfp_admission_head) THEN
        RAISE EXCEPTION 'retained Story LFP evidence prevents rollback';
    END IF;
END $$;
DROP TABLE story_lfp_registration_acks;
DROP TABLE story_lfp_consents;
DROP TABLE story_lfp_publications;
DROP TABLE story_lfp_offer_heads;
DROP TABLE story_lfp_offers;
DROP TABLE story_lfp_admission_head;
DROP FUNCTION story_lfp_immutable_update();
