BEGIN;

LOCK TABLE voice_lifecycle_redis_divergences IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM voice_lifecycle_redis_divergences) THEN
        RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'voice lifecycle redis divergence evidence exists; refusing DOWN migration';
    END IF;
END
$$;

DROP TRIGGER voice_lifecycle_redis_divergence_update_guard ON voice_lifecycle_redis_divergences;
DROP FUNCTION voice_lifecycle_redis_divergence_update_guard_fn();
DROP TABLE voice_lifecycle_redis_divergences;

COMMIT;
