-- Empty-only rollback is not erasure or authority to remove personal evidence.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM story_lfp_publication_actors)
      OR EXISTS (SELECT 1 FROM story_lfp_consent_actors)
      OR EXISTS (SELECT 1 FROM story_lfp_actor_bound_acks) THEN
        RAISE EXCEPTION 'retained Story LFP actor evidence prevents rollback';
    END IF;
END $$;
DROP TABLE story_lfp_actor_bound_acks;
DROP TABLE story_lfp_consent_actors;
DROP TABLE story_lfp_publication_actors;
