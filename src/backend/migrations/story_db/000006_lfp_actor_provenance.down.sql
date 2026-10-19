-- Empty-only rollback is not erasure or authority to remove personal evidence.
DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Story LFP rollback requires a fresh READ COMMITTED transaction';
    END IF;
    -- Same transaction covers waiting, fresh emptiness observation and DROP.
    -- Head first serializes qualified writers; relation locks also cover direct
    -- uncommitted first inserts without relying on their admission cooperation.
    LOCK TABLE story_lfp_admission_head IN ACCESS EXCLUSIVE MODE;
    LOCK TABLE story_lfp_publication_actors, story_lfp_consent_actors,
      story_lfp_actor_bound_acks IN ACCESS EXCLUSIVE MODE;
    IF EXISTS (SELECT 1 FROM story_lfp_publication_actors)
      OR EXISTS (SELECT 1 FROM story_lfp_consent_actors)
      OR EXISTS (SELECT 1 FROM story_lfp_actor_bound_acks) THEN
        RAISE EXCEPTION 'retained Story LFP actor evidence prevents rollback';
    END IF;
    DROP TABLE story_lfp_actor_bound_acks;
    DROP TABLE story_lfp_consent_actors;
    DROP TABLE story_lfp_publication_actors;
END $$;
