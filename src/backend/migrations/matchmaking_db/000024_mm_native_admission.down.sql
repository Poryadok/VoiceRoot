BEGIN;
-- Same gate-first order as internal/mmadmission. This key is the signed
-- big-endian first 64 bits of SHA256("voice.mm.native-admission.gate.v1\0").
SELECT pg_catalog.pg_advisory_xact_lock(2060719636617962328);
-- Serialize even a writer that does not cooperate with the advisory gate.
-- The retained-marker check must use a fresh snapshot after this lock waits.
LOCK TABLE public.mm_native_admission_head IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM public.mm_native_admission_head) THEN
        RAISE EXCEPTION 'MM native admission marker must be retained';
    END IF;
END $$;
DROP TABLE public.mm_native_admission_head;
COMMIT;
